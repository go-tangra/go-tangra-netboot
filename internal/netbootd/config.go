package netbootd

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment variables that configure the upstream netbootd connection.
// The endpoint and the credentials are operator-supplied configuration and
// are never derived from an end-user request, which is what keeps this
// client free of server-side request forgery exposure.
const (
	EnvEndpoint         = "NETBOOTD_ENDPOINT"
	EnvUsername         = "NETBOOTD_USERNAME"
	EnvPassword         = "NETBOOTD_PASSWORD"
	EnvPasswordFile     = "NETBOOTD_PASSWORD_FILE"
	EnvCAFile           = "NETBOOTD_CA_FILE"
	EnvInsecureSkipTLS  = "NETBOOTD_INSECURE_SKIP_VERIFY"
	EnvAllowPlaintext   = "NETBOOTD_ALLOW_PLAINTEXT"
	EnvTimeout          = "NETBOOTD_TIMEOUT"
	EnvMaxRetries       = "NETBOOTD_MAX_RETRIES"
	EnvMaxResponseBytes = "NETBOOTD_MAX_RESPONSE_BYTES"
)

// Defaults chosen to bound blast radius rather than to maximise throughput:
// a slow or hostile upstream must not be able to exhaust this module.
const (
	DefaultTimeout          = 15 * time.Second
	DefaultMaxRetries       = 2
	DefaultMaxResponseBytes = 8 << 20 // 8 MiB
	maxAllowedResponseBytes = 64 << 20
	maxAllowedRetries       = 10
	maxAllowedTimeout       = 5 * time.Minute
)

// Config describes how to reach the remote netbootd instance.
type Config struct {
	// Endpoint is the scheme://host[:port] root of the netbootd admin API.
	Endpoint string

	// Username and Password authenticate an operator session.
	Username string
	Password Secret

	// CAFile is an optional PEM bundle used to verify the upstream
	// certificate instead of the system roots.
	CAFile string

	// InsecureSkipVerify disables upstream certificate verification. It
	// exists for local development against a self-signed netbootd and is
	// refused unless AllowPlaintext is also set, so it cannot be enabled
	// by a single stray environment variable in production.
	InsecureSkipVerify bool

	// AllowPlaintext permits an http:// endpoint. netbootd's admin API is
	// expected to be fronted by TLS; plaintext is opt-in only.
	AllowPlaintext bool

	// Timeout bounds a single upstream request, retries excluded.
	Timeout time.Duration

	// MaxRetries bounds automatic retries of idempotent requests.
	MaxRetries int

	// MaxResponseBytes bounds how much of an upstream response is read.
	MaxResponseBytes int64
}

// LoadConfig builds a Config from the process environment and validates it.
// A module with no NETBOOTD_ENDPOINT configured is not an error: the client
// starts in an unconfigured state, reports itself unhealthy, and fails every
// call with a configuration error rather than preventing startup.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Endpoint:           strings.TrimSpace(os.Getenv(EnvEndpoint)),
		Username:           strings.TrimSpace(os.Getenv(EnvUsername)),
		CAFile:             strings.TrimSpace(os.Getenv(EnvCAFile)),
		InsecureSkipVerify: envBool(EnvInsecureSkipTLS),
		AllowPlaintext:     envBool(EnvAllowPlaintext),
		Timeout:            DefaultTimeout,
		MaxRetries:         DefaultMaxRetries,
		MaxResponseBytes:   DefaultMaxResponseBytes,
	}

	password, err := loadPassword()
	if err != nil {
		return nil, err
	}
	cfg.Password = password

	if raw := os.Getenv(EnvTimeout); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", EnvTimeout, err)
		}
		cfg.Timeout = d
	}

	if raw := os.Getenv(EnvMaxRetries); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", EnvMaxRetries, err)
		}
		cfg.MaxRetries = n
	}

	if raw := os.Getenv(EnvMaxResponseBytes); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", EnvMaxResponseBytes, err)
		}
		cfg.MaxResponseBytes = n
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// loadPassword prefers NETBOOTD_PASSWORD_FILE over NETBOOTD_PASSWORD so that
// deployments can mount a secret rather than exposing it in the process
// environment, where it is readable from /proc and leaks into crash dumps.
func loadPassword() (Secret, error) {
	if path := strings.TrimSpace(os.Getenv(EnvPasswordFile)); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", EnvPasswordFile, err)
		}
		return Secret(strings.TrimRight(string(raw), "\r\n")), nil
	}
	return Secret(os.Getenv(EnvPassword)), nil
}

func envBool(key string) bool {
	v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key)))
	return err == nil && v
}

// Configured reports whether an upstream endpoint has been supplied.
func (c *Config) Configured() bool { return c != nil && c.Endpoint != "" }

// Validate normalises and checks the configuration. An unconfigured Config
// (no endpoint) validates successfully; every other field is only meaningful
// once an endpoint exists.
func (c *Config) Validate() error {
	if c.Timeout <= 0 || c.Timeout > maxAllowedTimeout {
		return fmt.Errorf("timeout must be in (0, %s], got %s", maxAllowedTimeout, c.Timeout)
	}
	if c.MaxRetries < 0 || c.MaxRetries > maxAllowedRetries {
		return fmt.Errorf("max retries must be in [0, %d], got %d", maxAllowedRetries, c.MaxRetries)
	}
	if c.MaxResponseBytes <= 0 || c.MaxResponseBytes > maxAllowedResponseBytes {
		return fmt.Errorf("max response bytes must be in (0, %d], got %d",
			maxAllowedResponseBytes, c.MaxResponseBytes)
	}

	if !c.Configured() {
		return nil
	}

	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", EnvEndpoint, err)
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !c.AllowPlaintext {
			return fmt.Errorf("%s uses http://; set %s=true to accept an unencrypted upstream",
				EnvEndpoint, EnvAllowPlaintext)
		}
	default:
		return fmt.Errorf("%s must use http or https, got %q", EnvEndpoint, u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("%s has no host", EnvEndpoint)
	}
	if u.User != nil {
		return fmt.Errorf("%s must not embed credentials in the URL", EnvEndpoint)
	}
	if c.InsecureSkipVerify && !c.AllowPlaintext {
		return fmt.Errorf("%s requires %s=true as an explicit acknowledgement",
			EnvInsecureSkipTLS, EnvAllowPlaintext)
	}
	if c.Username == "" || c.Password.IsZero() {
		return fmt.Errorf("%s and %s (or %s) are required when %s is set",
			EnvUsername, EnvPassword, EnvPasswordFile, EnvEndpoint)
	}

	// Strip any path/query so joins below are unambiguous.
	c.Endpoint = strings.TrimRight(u.Scheme+"://"+u.Host+u.Path, "/")
	return nil
}

// IsTLS reports whether the configured endpoint is TLS-protected.
func (c *Config) IsTLS() bool {
	return c.Configured() && strings.HasPrefix(c.Endpoint, "https://")
}

// SafeEndpoint returns the endpoint for display. Credentials can never be
// present (Validate rejects them) but the accessor documents the intent.
func (c *Config) SafeEndpoint() string {
	if !c.Configured() {
		return ""
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	return u.String()
}
