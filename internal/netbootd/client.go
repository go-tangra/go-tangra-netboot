// Package netbootd is the client for the remote netbootd (universe) admin
// API. netbootd runs on a separate host and authenticates operators with an
// HTTP session cookie, so this package owns the login handshake, the cookie
// lifecycle, transport hardening and the translation of netbootd's response
// envelope into Go values.
//
// Nothing in this package makes an authorization decision: callers in
// internal/service authorize the Tangra principal before a request is ever
// issued. This package's contract is narrower and entirely mechanical - talk
// to exactly one configured host, safely.
package netbootd

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// Upstream API paths. These are compile-time constants; user input only ever
// reaches the upstream as an escaped path segment or a query value.
const (
	pathLogin        = "/api/v1/auth/login"
	pathMe           = "/api/v1/auth/me"
	pathMachines     = "/api/v1/machines"
	pathUnknownBoots = "/api/v1/machines/unknown"
	pathRegisterFrom = "/api/v1/machines/register-unknown"
	pathProfiles     = "/api/v1/profiles"
	pathDhcpConfig   = "/api/v1/dhcp/config"
	pathDhcpEnable   = "/api/v1/dhcp/enable"
	pathDhcpDisable  = "/api/v1/dhcp/disable"
	pathDhcpLeases   = "/api/v1/dhcp/leases"
	pathDhcpConflict = "/api/v1/dhcp/conflicts"
	pathSessions     = "/api/v1/sessions"
	pathArtifacts    = "/api/v1/artifacts"
	pathTransfers    = "/api/v1/artifacts/transfers"
)

// sessionCookie is the cookie name netbootd issues on login.
const sessionCookie = "nb_session"

// Transport hardening constants.
const (
	maxRedirects        = 3
	retryBaseDelay      = 200 * time.Millisecond
	maxRetryDelay       = 2 * time.Second
	dialTimeout         = 5 * time.Second
	tlsHandshakeTimeout = 5 * time.Second
	idleConnTimeout     = 90 * time.Second
	maxIdleConns        = 16

	// maxBackoffShift bounds the exponential shift so that doubling the
	// retry delay can never overflow time.Duration.
	maxBackoffShift = 16
)

// envelope is netbootd's uniform response wrapper.
type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *envelopeError  `json:"error"`
}

type envelopeError struct {
	Reason  string            `json:"reason"`
	Message string            `json:"message"`
	Details map[string]string `json:"details"`
}

// Client is a concurrency-safe client for one netbootd instance.
type Client struct {
	cfg  *Config
	log  *log.Helper
	http *http.Client

	// mu guards the session token; it is held only for the duration of a
	// field read or write, never across a network call.
	mu    sync.Mutex
	token string

	// loginMu serialises re-authentication so that a burst of concurrent
	// 401s produces a single login rather than one per in-flight request.
	loginMu sync.Mutex
}

// NewClient builds a Client from cfg. It performs no network I/O: the first
// request establishes the session, and an unconfigured client is legal and
// simply fails every call with ErrNotConfigured.
func NewClient(cfg *Config, logger log.Logger) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("netbootd: nil config")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	l := log.NewHelper(log.With(logger, "module", "netboot/netbootd"))

	c := &Client{cfg: cfg, log: l}

	if !cfg.Configured() {
		l.Warnf("%s is not set; netboot upstream calls will fail until it is configured", EnvEndpoint)
		c.http = &http.Client{Timeout: cfg.Timeout}
		return c, nil
	}

	tlsCfg, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.InsecureSkipVerify {
		l.Warn("upstream TLS verification is DISABLED; never do this outside development")
	}
	if !cfg.IsTLS() {
		l.Warn("upstream endpoint is plaintext http; operator credentials traverse the network unprotected")
	}

	c.http = &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSClientConfig:       tlsCfg,
			TLSHandshakeTimeout:   tlsHandshakeTimeout,
			ResponseHeaderTimeout: cfg.Timeout,
			ExpectContinueTimeout: time.Second,
			IdleConnTimeout:       idleConnTimeout,
			MaxIdleConns:          maxIdleConns,
			MaxIdleConnsPerHost:   maxIdleConns,
			ForceAttemptHTTP2:     true,
		},
		// A redirect that leaves the configured host would hand our session
		// cookie to a third party, so cross-host redirects are refused
		// outright rather than silently followed.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			if req.URL.Host != via[0].URL.Host {
				return fmt.Errorf("refusing cross-host redirect to %q", req.URL.Host)
			}
			return nil
		},
	}
	return c, nil
}

// buildTLSConfig pins the upstream to TLS 1.2+ and, when a CA bundle is
// supplied, to that bundle alone rather than the system roots.
func buildTLSConfig(cfg *Config) (*tls.Config, error) {
	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec // gated by Config.Validate
	}
	if cfg.CAFile == "" {
		return tlsCfg, nil
	}
	pem, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", EnvCAFile, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("%s contains no usable PEM certificates", EnvCAFile)
	}
	tlsCfg.RootCAs = pool
	return tlsCfg, nil
}

// Configured reports whether an upstream endpoint is set.
func (c *Client) Configured() bool { return c.cfg.Configured() }

// Endpoint returns the credential-free upstream endpoint, for display.
func (c *Client) Endpoint() string { return c.cfg.SafeEndpoint() }

// IsTLS reports whether the upstream transport is TLS-protected.
func (c *Client) IsTLS() bool { return c.cfg.IsTLS() }

// HasSession reports whether an operator session is currently held.
func (c *Client) HasSession() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token != ""
}

// request describes one upstream call.
type request struct {
	method string
	path   string
	query  url.Values
	body   any

	// skipAuth suppresses cookie attachment and the 401 retry; only login
	// uses it.
	skipAuth bool
}

// idempotent reports whether the request may be retried after a transport
// failure. A failed POST may have been applied upstream, so only reads and
// deletes are replayed.
func (r *request) idempotent() bool {
	switch r.method {
	case http.MethodGet, http.MethodHead, http.MethodDelete, http.MethodPut:
		return true
	default:
		return false
	}
}

// do executes req, decoding the envelope's data into out (which may be nil).
//
// The call is attempted up to MaxRetries+1 times for idempotent requests on
// transport errors and 5xx responses, with capped exponential backoff. A 401
// triggers exactly one re-authentication and replay, for any method: an
// expired session means the upstream did not act on the request.
func (c *Client) do(ctx context.Context, req *request, out any) error {
	if !c.cfg.Configured() {
		return ErrNotConfigured
	}

	// Establish the session up front so the cold-start path costs one round
	// trip instead of a guaranteed 401 followed by a replay.
	if !req.skipAuth {
		if err := c.ensureSession(ctx); err != nil {
			return err
		}
	}

	var lastErr error
	attempts := 1
	if req.idempotent() {
		attempts += c.cfg.MaxRetries
	}

	// A session can still expire between that check and the request landing,
	// so one re-authentication is allowed - but only one, so an upstream that
	// answers 401 unconditionally cannot make us loop.
	reauthenticated := false

	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := sleepCtx(ctx, backoff(attempt)); err != nil {
				return err
			}
		}

		// Remember which session this attempt actually presented, so that a
		// 401 for an already-replaced cookie does not trigger a second,
		// redundant login.
		presented := c.currentToken()

		err := c.attempt(ctx, req, out)
		if err == nil {
			return nil
		}
		lastErr = err

		if apiErr, ok := AsAPIError(err); ok {
			if apiErr.IsUnauthenticated() && !req.skipAuth && !reauthenticated {
				reauthenticated = true
				if reauthErr := c.reauthenticate(ctx, presented); reauthErr != nil {
					return reauthErr
				}
				// Replay without consuming a transport retry: a rejected
				// session means the upstream never acted on the request.
				attempt--
				continue
			}
			// A well-formed 4xx is the upstream's verdict and is final; only
			// a 5xx on an idempotent request is worth repeating.
			if apiErr.StatusCode < 500 || !req.idempotent() {
				return err
			}
			continue
		}

		if !req.idempotent() || ctx.Err() != nil {
			return err
		}
	}
	return lastErr
}

// attempt performs exactly one HTTP round trip.
func (c *Client) attempt(ctx context.Context, req *request, out any) error {
	var bodyBytes []byte
	if req.body != nil {
		encoded, err := json.Marshal(req.body)
		if err != nil {
			return &TransportError{Op: "encode request", Err: err}
		}
		bodyBytes = encoded
	}

	endpoint := c.cfg.Endpoint + req.path
	if len(req.query) > 0 {
		endpoint += "?" + req.query.Encode()
	}

	var reader io.Reader
	if bodyBytes != nil {
		reader = bytes.NewReader(bodyBytes)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.method, endpoint, reader)
	if err != nil {
		return &TransportError{Op: "build request", Err: err}
	}
	httpReq.Header.Set("Accept", "application/json")
	if bodyBytes != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if !req.skipAuth {
		if token := c.currentToken(); token != "" {
			httpReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		}
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return &TransportError{Op: req.method + " " + req.path, Err: err}
	}
	defer func() {
		// Drain a bounded amount so the connection can be reused, then close.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
	}()

	// Capture any refreshed session cookie (login, and rotation upstream).
	c.captureSessionCookie(resp)

	payload, err := readLimited(resp.Body, c.cfg.MaxResponseBytes)
	if err != nil {
		return &TransportError{Op: "read response", Err: err}
	}

	return decodeEnvelope(resp.StatusCode, payload, out)
}

// readLimited reads at most limit bytes and reports ErrResponseTooLarge if
// the body would exceed it, rather than silently truncating.
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ErrResponseTooLarge
	}
	return data, nil
}

// decodeEnvelope turns an upstream response into either a decoded value or a
// typed error. A non-2xx status always yields an *APIError, even when the
// body is missing or unparseable, so callers never see a nil error with an
// unpopulated result.
func decodeEnvelope(status int, payload []byte, out any) error {
	var env envelope
	decodeErr := json.Unmarshal(payload, &env)

	if status < 200 || status > 299 {
		apiErr := &APIError{StatusCode: status, Message: http.StatusText(status)}
		if decodeErr == nil && env.Error != nil {
			apiErr.Reason = env.Error.Reason
			if env.Error.Message != "" {
				apiErr.Message = env.Error.Message
			}
			apiErr.Details = env.Error.Details
		}
		return apiErr
	}

	if decodeErr != nil {
		return &TransportError{Op: "decode response", Err: decodeErr}
	}
	if !env.Success {
		apiErr := &APIError{StatusCode: status, Message: "upstream reported failure"}
		if env.Error != nil {
			apiErr.Reason = env.Error.Reason
			if env.Error.Message != "" {
				apiErr.Message = env.Error.Message
			}
			apiErr.Details = env.Error.Details
		}
		return apiErr
	}
	if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return &TransportError{Op: "decode response data", Err: err}
	}
	return nil
}

func (c *Client) currentToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

// captureSessionCookie records a session cookie issued by the upstream. The
// value itself is never logged.
func (c *Client) captureSessionCookie(resp *http.Response) {
	for _, cookie := range resp.Cookies() {
		if cookie.Name != sessionCookie {
			continue
		}
		c.mu.Lock()
		if cookie.Value == "" || cookie.MaxAge < 0 {
			c.token = ""
		} else {
			c.token = cookie.Value
		}
		c.mu.Unlock()
	}
}

// ensureSession logs in when no session is held. The re-check under loginMu
// is what makes a burst of cold-start requests produce one login rather than
// one per caller.
func (c *Client) ensureSession(ctx context.Context) error {
	if c.HasSession() {
		return nil
	}

	c.loginMu.Lock()
	defer c.loginMu.Unlock()

	if c.HasSession() {
		return nil
	}
	return c.login(ctx)
}

// reauthenticate replaces a session the upstream has rejected. rejected is
// the token the failing request actually presented.
//
// It is serialised so that concurrent 401s collapse into one login: a caller
// whose rejected token has already been superseded - either while it waited
// for the lock, or before its request was even sent - reuses the new session
// rather than invalidating it with a redundant login.
func (c *Client) reauthenticate(ctx context.Context, rejected string) error {
	c.loginMu.Lock()
	defer c.loginMu.Unlock()

	if current := c.currentToken(); current != "" && current != rejected {
		return nil
	}
	return c.login(ctx)
}

// Login establishes an operator session, replacing any existing one.
func (c *Client) Login(ctx context.Context) error {
	if !c.cfg.Configured() {
		return ErrNotConfigured
	}

	c.loginMu.Lock()
	defer c.loginMu.Unlock()

	return c.login(ctx)
}

// login performs the credential exchange. The caller must hold loginMu.
//
// The request is issued with skipAuth: presenting a cookie we have already
// given up on would be pointless, and it keeps login out of the 401 retry
// path so a bad password can never become a login loop.
func (c *Client) login(ctx context.Context) error {
	if !c.cfg.Configured() {
		return ErrNotConfigured
	}

	c.mu.Lock()
	c.token = ""
	c.mu.Unlock()

	body := map[string]string{
		"username": c.cfg.Username,
		"password": c.cfg.Password.Reveal(),
	}

	var op Operator
	if err := c.do(ctx, &request{
		method:   http.MethodPost,
		path:     pathLogin,
		body:     body,
		skipAuth: true,
	}, &op); err != nil {
		// The error text originates upstream and never echoes the password.
		c.log.Warnf("netbootd login failed for operator %q: %v", c.cfg.Username, err)
		return err
	}

	if !c.HasSession() {
		return &TransportError{
			Op:  "login",
			Err: errors.New("upstream accepted the credentials but issued no session cookie"),
		}
	}

	c.log.Infof("authenticated with netbootd as %q", op.Username)
	return nil
}

// Ping verifies connectivity and the session, logging in when required.
func (c *Client) Ping(ctx context.Context) (*Operator, error) {
	if !c.cfg.Configured() {
		return nil, ErrNotConfigured
	}
	// do() establishes the session itself, under the single-flight lock.
	var op Operator
	if err := c.do(ctx, &request{method: http.MethodGet, path: pathMe}, &op); err != nil {
		return nil, err
	}
	return &op, nil
}

// Close releases the upstream session and idle connections. It is safe to
// call on a client that never authenticated.
func (c *Client) Close() {
	c.mu.Lock()
	c.token = ""
	c.mu.Unlock()
	if transport, ok := c.http.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

// backoff returns the delay before the given (1-based) retry attempt.
func backoff(attempt int) time.Duration {
	if attempt < 1 {
		return 0
	}
	// The shift is capped before it is applied: a large attempt count would
	// otherwise overflow time.Duration and yield a negative - effectively
	// zero - delay, turning backoff into a tight retry loop.
	shift := attempt - 1
	if shift > maxBackoffShift {
		return maxRetryDelay
	}
	delay := retryBaseDelay << shift
	if delay > maxRetryDelay {
		return maxRetryDelay
	}
	return delay
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// escapeSegment renders an identifier as a single safe path segment. Callers
// have already validated the value against a strict pattern; this is the
// belt-and-braces layer that keeps a future looser pattern from turning into
// path traversal against the upstream.
func escapeSegment(s string) string {
	return url.PathEscape(strings.ReplaceAll(s, "/", ""))
}
