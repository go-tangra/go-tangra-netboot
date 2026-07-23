package netbootd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// fakeNetbootd is an in-process stand-in for the remote netbootd admin API.
// It reproduces the two behaviours that actually shape this client: the
// {success,data,error} envelope, and session-cookie authentication in which
// any request without a valid nb_session cookie is answered with 401.
type fakeNetbootd struct {
	*httptest.Server

	mu sync.Mutex

	// issuedToken is the cookie value handed out by the next login.
	issuedToken string
	// validTokens is the set of cookies the server currently accepts.
	validTokens map[string]bool

	logins   atomic.Int64
	requests atomic.Int64

	// handler serves everything that is not /api/v1/auth/login.
	handler func(w http.ResponseWriter, r *http.Request)
}

func newFakeNetbootd(t *testing.T) *fakeNetbootd {
	t.Helper()

	f := &fakeNetbootd{
		issuedToken: "token-1",
		validTokens: map[string]bool{},
	}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeNetbootd) serve(w http.ResponseWriter, r *http.Request) {
	f.requests.Add(1)

	if r.URL.Path == pathLogin {
		f.serveLogin(w, r)
		return
	}
	if !f.authenticated(r) {
		writeEnvelopeError(w, http.StatusUnauthorized, ReasonUnauthenticated, "authentication required")
		return
	}
	if f.handler != nil {
		f.handler(w, r)
		return
	}
	writeEnvelopeData(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (f *fakeNetbootd) serveLogin(w http.ResponseWriter, r *http.Request) {
	f.logins.Add(1)

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeEnvelopeError(w, http.StatusBadRequest, "BAD_REQUEST", "malformed body")
		return
	}
	if body.Username != "operator" || body.Password != "correct-horse" {
		writeEnvelopeError(w, http.StatusUnauthorized, ReasonUnauthenticated, "authentication required")
		return
	}

	f.mu.Lock()
	token := f.issuedToken
	f.validTokens[token] = true
	f.mu.Unlock()

	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true})
	writeEnvelopeData(w, http.StatusOK, Operator{ID: "op-1", Username: body.Username, Active: true})
}

func (f *fakeNetbootd) authenticated(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.validTokens[c.Value]
}

// expireSessions invalidates every issued cookie and arms the next login to
// hand out a fresh one, modelling an upstream restart or a TTL expiry.
func (f *fakeNetbootd) expireSessions(nextToken string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.validTokens = map[string]bool{}
	f.issuedToken = nextToken
}

func writeEnvelopeData(w http.ResponseWriter, status int, data any) {
	raw, _ := json.Marshal(data)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Success: true, Data: raw})
}

func writeEnvelopeError(w http.ResponseWriter, status int, reason, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{
		Success: false,
		Error:   &envelopeError{Reason: reason, Message: message},
	})
}

// newTestClient builds a client pointed at f, with retries and timeouts tuned
// for fast tests.
func newTestClient(t *testing.T, f *fakeNetbootd, mutate ...func(*Config)) *Client {
	t.Helper()

	cfg := &Config{
		Endpoint:         f.URL,
		Username:         "operator",
		Password:         Secret("correct-horse"),
		AllowPlaintext:   true, // httptest serves plain HTTP
		Timeout:          2 * time.Second,
		MaxRetries:       2,
		MaxResponseBytes: DefaultMaxResponseBytes,
	}
	for _, m := range mutate {
		m(cfg)
	}

	client, err := NewClient(cfg, log.NewStdLogger(newDiscardWriter()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func newDiscardWriter() discardWriter { return discardWriter{} }

func TestNewClientRejectsNilConfig(t *testing.T) {
	if _, err := NewClient(nil, log.NewStdLogger(newDiscardWriter())); err == nil {
		t.Fatal("NewClient(nil) = nil error, want a failure")
	}
}

func TestNewClientRejectsInvalidConfig(t *testing.T) {
	cfg := &Config{
		Endpoint:         "ftp://nope",
		Timeout:          time.Second,
		MaxResponseBytes: 1024,
	}
	if _, err := NewClient(cfg, log.NewStdLogger(newDiscardWriter())); err == nil {
		t.Fatal("NewClient() = nil error, want a failure for an unsupported scheme")
	}
}

func TestUnconfiguredClientFailsEveryCall(t *testing.T) {
	cfg := &Config{
		Timeout:          time.Second,
		MaxRetries:       0,
		MaxResponseBytes: 1024,
	}
	client, err := NewClient(cfg, log.NewStdLogger(newDiscardWriter()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(client.Close)

	if client.Configured() {
		t.Error("Configured() = true, want false")
	}
	if _, err := client.Ping(context.Background()); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("Ping() error = %v, want ErrNotConfigured", err)
	}
	if err := client.Login(context.Background()); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("Login() error = %v, want ErrNotConfigured", err)
	}
	if _, err := client.ListMachines(context.Background(), MachineFilter{}); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("ListMachines() error = %v, want ErrNotConfigured", err)
	}
}

func TestLoginEstablishesSession(t *testing.T) {
	f := newFakeNetbootd(t)
	client := newTestClient(t, f)

	if client.HasSession() {
		t.Fatal("HasSession() = true before login, want false")
	}
	if err := client.Login(context.Background()); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if !client.HasSession() {
		t.Error("HasSession() = false after a successful login, want true")
	}
}

func TestLoginWithBadCredentialsFails(t *testing.T) {
	f := newFakeNetbootd(t)
	client := newTestClient(t, f, func(c *Config) { c.Password = Secret("wrong") })

	err := client.Login(context.Background())
	if err == nil {
		t.Fatal("Login() = nil error, want a failure")
	}
	apiErr, ok := AsAPIError(err)
	if !ok {
		t.Fatalf("Login() error = %v, want an *APIError", err)
	}
	if !apiErr.IsUnauthenticated() {
		t.Errorf("IsUnauthenticated() = false for %v, want true", apiErr)
	}
	if strings.Contains(err.Error(), "wrong") {
		t.Errorf("Login() error leaked the password: %v", err)
	}
}

func TestPingLogsInAutomatically(t *testing.T) {
	f := newFakeNetbootd(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		writeEnvelopeData(w, http.StatusOK, Operator{ID: "op-1", Username: "operator", Active: true})
	}
	client := newTestClient(t, f)

	op, err := client.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if op.Username != "operator" {
		t.Errorf("Username = %q, want %q", op.Username, "operator")
	}
	if got := f.logins.Load(); got != 1 {
		t.Errorf("logins = %d, want exactly 1", got)
	}
}

// An expired session must be renewed transparently: the caller sees a
// successful result, not a 401.
func TestExpiredSessionTriggersReauthentication(t *testing.T) {
	f := newFakeNetbootd(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		writeEnvelopeData(w, http.StatusOK, Operator{ID: "op-1", Username: "operator", Active: true})
	}
	client := newTestClient(t, f)

	if err := client.Login(context.Background()); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	f.expireSessions("token-2")

	if _, err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() after expiry error = %v, want a transparent recovery", err)
	}
	if got := f.logins.Load(); got != 2 {
		t.Errorf("logins = %d, want 2 (initial + one renewal)", got)
	}
}

// Concurrent 401s must collapse into a single re-login rather than one per
// in-flight request.
func TestConcurrentReauthenticationLogsInOnce(t *testing.T) {
	f := newFakeNetbootd(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		writeEnvelopeData(w, http.StatusOK, Operator{ID: "op-1", Username: "operator", Active: true})
	}
	client := newTestClient(t, f)

	if err := client.Login(context.Background()); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	f.expireSessions("token-2")

	const callers = 8
	var wg sync.WaitGroup
	errs := make([]error, callers)
	wg.Add(callers)
	for i := range callers {
		go func() {
			defer wg.Done()
			_, errs[i] = client.Ping(context.Background())
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("caller %d: Ping() error = %v", i, err)
		}
	}
	// One initial login plus one renewal. Without the single-flight guard
	// this would be up to callers+1.
	if got := f.logins.Load(); got > 2 {
		t.Errorf("logins = %d, want at most 2 - concurrent 401s should collapse", got)
	}
}

func TestRetriesIdempotentRequestsOnServerError(t *testing.T) {
	f := newFakeNetbootd(t)
	var attempts atomic.Int64
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			writeEnvelopeError(w, http.StatusInternalServerError, "", "internal error")
			return
		}
		writeEnvelopeData(w, http.StatusOK, ListMachinesReply{})
	}
	client := newTestClient(t, f)

	if _, err := client.ListMachines(context.Background(), MachineFilter{}); err != nil {
		t.Fatalf("ListMachines() error = %v, want the third attempt to succeed", err)
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

// A POST may already have been applied upstream, so a failed one is never
// replayed automatically.
func TestDoesNotRetryNonIdempotentRequests(t *testing.T) {
	f := newFakeNetbootd(t)
	var attempts atomic.Int64
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		writeEnvelopeError(w, http.StatusInternalServerError, "", "internal error")
	}
	client := newTestClient(t, f)

	if _, err := client.CreateMachine(context.Background(), &CreateMachineBody{}); err == nil {
		t.Fatal("CreateMachine() = nil error, want a failure")
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("attempts = %d, want exactly 1 - POSTs must not be replayed", got)
	}
}

func TestDoesNotRetryClientErrors(t *testing.T) {
	f := newFakeNetbootd(t)
	var attempts atomic.Int64
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		writeEnvelopeError(w, http.StatusNotFound, ReasonNotFound, "machine not found")
	}
	client := newTestClient(t, f)

	_, err := client.GetMachine(context.Background(), "m-1")
	apiErr, ok := AsAPIError(err)
	if !ok {
		t.Fatalf("GetMachine() error = %v, want an *APIError", err)
	}
	if !apiErr.IsNotFound() {
		t.Errorf("IsNotFound() = false for %v, want true", apiErr)
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("attempts = %d, want exactly 1 - a 404 is not retryable", got)
	}
}

func TestOversizedResponseIsRefused(t *testing.T) {
	f := newFakeNetbootd(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("a", 4096)))
	}
	client := newTestClient(t, f, func(c *Config) {
		c.MaxResponseBytes = 512
		c.MaxRetries = 0
	})

	_, err := client.GetMachine(context.Background(), "m-1")
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("GetMachine() error = %v, want ErrResponseTooLarge", err)
	}
}

func TestUndecodableResponseIsATransportError(t *testing.T) {
	f := newFakeNetbootd(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this is not json"))
	}
	client := newTestClient(t, f, func(c *Config) { c.MaxRetries = 0 })

	_, err := client.GetMachine(context.Background(), "m-1")
	var transportErr *TransportError
	if !errors.As(err, &transportErr) {
		t.Fatalf("GetMachine() error = %v, want a *TransportError", err)
	}
}

// success:false with a 200 status is still a failure; the client must not
// hand back a zero-valued result.
func TestSuccessFalseIsAnError(t *testing.T) {
	f := newFakeNetbootd(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(envelope{
			Success: false,
			Error:   &envelopeError{Reason: ReasonConflict, Message: "active session exists"},
		})
	}
	client := newTestClient(t, f, func(c *Config) { c.MaxRetries = 0 })

	_, err := client.GetMachine(context.Background(), "m-1")
	apiErr, ok := AsAPIError(err)
	if !ok {
		t.Fatalf("GetMachine() error = %v, want an *APIError", err)
	}
	if apiErr.Reason != ReasonConflict {
		t.Errorf("Reason = %q, want %q", apiErr.Reason, ReasonConflict)
	}
}

func TestCancelledContextStopsRetrying(t *testing.T) {
	f := newFakeNetbootd(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		writeEnvelopeError(w, http.StatusInternalServerError, "", "internal error")
	}
	client := newTestClient(t, f)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.ListMachines(ctx, MachineFilter{}); err == nil {
		t.Fatal("ListMachines() = nil error, want a failure on a cancelled context")
	}
}

func TestCrossHostRedirectIsRefused(t *testing.T) {
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If the client followed the redirect it would present its session
		// cookie here, which is exactly what must not happen.
		writeEnvelopeData(w, http.StatusOK, map[string]string{"stolen": "yes"})
	}))
	t.Cleanup(elsewhere.Close)

	f := newFakeNetbootd(t)
	f.handler = func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/api/v1/machines/m-1", http.StatusFound)
	}
	client := newTestClient(t, f, func(c *Config) { c.MaxRetries = 0 })

	if _, err := client.GetMachine(context.Background(), "m-1"); err == nil {
		t.Fatal("GetMachine() = nil error, want the cross-host redirect to be refused")
	}
}

func TestCloseDropsTheSession(t *testing.T) {
	f := newFakeNetbootd(t)
	client := newTestClient(t, f)

	if err := client.Login(context.Background()); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	client.Close()

	if client.HasSession() {
		t.Error("HasSession() = true after Close(), want false")
	}
}

func TestEndpointAccessors(t *testing.T) {
	f := newFakeNetbootd(t)
	client := newTestClient(t, f)

	if got := client.Endpoint(); got != f.URL {
		t.Errorf("Endpoint() = %q, want %q", got, f.URL)
	}
	if client.IsTLS() {
		t.Error("IsTLS() = true for an httptest HTTP server, want false")
	}
	if !client.Configured() {
		t.Error("Configured() = false, want true")
	}
}

func TestBackoffIsCapped(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{1, retryBaseDelay},
		{2, 2 * retryBaseDelay},
		{3, 4 * retryBaseDelay},
		{10, maxRetryDelay},
		{40, maxRetryDelay},
	}
	for _, tt := range tests {
		if got := backoff(tt.attempt); got != tt.want {
			t.Errorf("backoff(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestEscapeSegment(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"m-1", "m-1"},
		{"../../etc/passwd", "....etcpasswd"},
		{"a b", "a%20b"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := escapeSegment(tt.in); got != tt.want {
			t.Errorf("escapeSegment(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSleepCtxHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepCtx(ctx, time.Hour); err == nil {
		t.Fatal("sleepCtx() = nil error on a cancelled context, want ctx.Err()")
	}
	if err := sleepCtx(context.Background(), time.Millisecond); err != nil {
		t.Errorf("sleepCtx() error = %v, want nil", err)
	}
}
