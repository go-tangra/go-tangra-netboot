package netbootd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// baseConfig is a minimal valid configuration; tests mutate one field at a
// time so a failure names exactly the rule that was violated.
func baseConfig() *Config {
	return &Config{
		Endpoint:         "https://netbootd.example.test:8080",
		Username:         "operator",
		Password:         Secret("pw"),
		Timeout:          DefaultTimeout,
		MaxRetries:       DefaultMaxRetries,
		MaxResponseBytes: DefaultMaxResponseBytes,
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{"valid https", func(*Config) {}, ""},
		{
			name:    "http requires explicit acknowledgement",
			mutate:  func(c *Config) { c.Endpoint = "http://netbootd.example.test:8080" },
			wantErr: EnvAllowPlaintext,
		},
		{
			name: "http accepted once acknowledged",
			mutate: func(c *Config) {
				c.Endpoint = "http://netbootd.example.test:8080"
				c.AllowPlaintext = true
			},
		},
		{
			name:    "unsupported scheme",
			mutate:  func(c *Config) { c.Endpoint = "ftp://netbootd.example.test" },
			wantErr: "must use http or https",
		},
		{
			name:    "missing host",
			mutate:  func(c *Config) { c.Endpoint = "https://" },
			wantErr: "no host",
		},
		{
			name:    "credentials in the URL are refused",
			mutate:  func(c *Config) { c.Endpoint = "https://user:pw@netbootd.example.test" },
			wantErr: "must not embed credentials",
		},
		{
			name:    "insecure skip verify needs acknowledgement",
			mutate:  func(c *Config) { c.InsecureSkipVerify = true },
			wantErr: EnvInsecureSkipTLS,
		},
		{
			name: "insecure skip verify accepted once acknowledged",
			mutate: func(c *Config) {
				c.InsecureSkipVerify = true
				c.AllowPlaintext = true
			},
		},
		{
			name:    "username required",
			mutate:  func(c *Config) { c.Username = "" },
			wantErr: "are required",
		},
		{
			name:    "password required",
			mutate:  func(c *Config) { c.Password = "" },
			wantErr: "are required",
		},
		{
			name:    "zero timeout",
			mutate:  func(c *Config) { c.Timeout = 0 },
			wantErr: "timeout must be",
		},
		{
			name:    "excessive timeout",
			mutate:  func(c *Config) { c.Timeout = time.Hour },
			wantErr: "timeout must be",
		},
		{
			name:    "negative retries",
			mutate:  func(c *Config) { c.MaxRetries = -1 },
			wantErr: "max retries must be",
		},
		{
			name:    "excessive retries",
			mutate:  func(c *Config) { c.MaxRetries = 99 },
			wantErr: "max retries must be",
		},
		{
			name:    "zero response limit",
			mutate:  func(c *Config) { c.MaxResponseBytes = 0 },
			wantErr: "max response bytes must be",
		},
		{
			name:    "excessive response limit",
			mutate:  func(c *Config) { c.MaxResponseBytes = 1 << 40 },
			wantErr: "max response bytes must be",
		},
		{
			// An unconfigured module is legal: it starts, reports itself
			// unhealthy, and refuses calls with a configuration error.
			name:   "unconfigured is valid",
			mutate: func(c *Config) { c.Endpoint = ""; c.Username = ""; c.Password = "" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseConfig()
			tt.mutate(cfg)

			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want an error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestConfigValidateNormalisesEndpoint(t *testing.T) {
	cfg := baseConfig()
	cfg.Endpoint = "https://netbootd.example.test:8080/base/?x=1"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if want := "https://netbootd.example.test:8080/base"; cfg.Endpoint != want {
		t.Errorf("Endpoint = %q, want %q", cfg.Endpoint, want)
	}
}

func TestConfigValidateErrorNeverContainsPassword(t *testing.T) {
	cfg := baseConfig()
	cfg.Password = Secret("top-secret-value")
	cfg.Endpoint = "ftp://nope"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error")
	}
	if strings.Contains(err.Error(), "top-secret-value") {
		t.Errorf("Validate() error leaked the password: %v", err)
	}
}

func TestConfigAccessors(t *testing.T) {
	cfg := baseConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if !cfg.Configured() {
		t.Error("Configured() = false, want true")
	}
	if !cfg.IsTLS() {
		t.Error("IsTLS() = false, want true")
	}
	if got, want := cfg.SafeEndpoint(), "https://netbootd.example.test:8080"; got != want {
		t.Errorf("SafeEndpoint() = %q, want %q", got, want)
	}

	empty := &Config{}
	if empty.Configured() {
		t.Error("Configured() on empty = true, want false")
	}
	if empty.IsTLS() {
		t.Error("IsTLS() on empty = true, want false")
	}
	if got := empty.SafeEndpoint(); got != "" {
		t.Errorf("SafeEndpoint() on empty = %q, want empty", got)
	}

	var nilCfg *Config
	if nilCfg.Configured() {
		t.Error("Configured() on nil = true, want false")
	}
}

func TestLoadConfigFromEnvironment(t *testing.T) {
	t.Setenv(EnvEndpoint, "https://netbootd.example.test")
	t.Setenv(EnvUsername, "operator")
	t.Setenv(EnvPassword, "pw")
	t.Setenv(EnvTimeout, "3s")
	t.Setenv(EnvMaxRetries, "5")
	t.Setenv(EnvMaxResponseBytes, "1024")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Timeout != 3*time.Second {
		t.Errorf("Timeout = %v, want 3s", cfg.Timeout)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}
	if cfg.MaxResponseBytes != 1024 {
		t.Errorf("MaxResponseBytes = %d, want 1024", cfg.MaxResponseBytes)
	}
	if cfg.Password.Reveal() != "pw" {
		t.Error("Password was not loaded")
	}
}

// The password file is the preferred source precisely because it keeps the
// credential out of the process environment.
func TestLoadConfigPrefersPasswordFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "password")
	if err := os.WriteFile(path, []byte("from-file\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv(EnvEndpoint, "https://netbootd.example.test")
	t.Setenv(EnvUsername, "operator")
	t.Setenv(EnvPassword, "from-env")
	t.Setenv(EnvPasswordFile, path)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got := cfg.Password.Reveal(); got != "from-file" {
		t.Errorf("Password = %q, want %q (trailing newline stripped)", got, "from-file")
	}
}

func TestLoadConfigRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"unreadable password file", map[string]string{EnvPasswordFile: "/nonexistent/netbootd-password"}},
		{"bad duration", map[string]string{EnvTimeout: "soon"}},
		{"bad retries", map[string]string{EnvMaxRetries: "many"}},
		{"bad response limit", map[string]string{EnvMaxResponseBytes: "big"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvEndpoint, "https://netbootd.example.test")
			t.Setenv(EnvUsername, "operator")
			t.Setenv(EnvPassword, "pw")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			if _, err := LoadConfig(); err == nil {
				t.Fatal("LoadConfig() = nil error, want a failure")
			}
		})
	}
}

func TestLoadConfigUnconfigured(t *testing.T) {
	t.Setenv(EnvEndpoint, "")
	t.Setenv(EnvUsername, "")
	t.Setenv(EnvPassword, "")
	t.Setenv(EnvPasswordFile, "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil for an unconfigured module", err)
	}
	if cfg.Configured() {
		t.Error("Configured() = true, want false")
	}
}

func TestEnvBool(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"1", true},
		{"TRUE", true},
		{"false", false},
		{"0", false},
		{"", false},
		{"yes", false},
	}
	for _, tt := range tests {
		t.Run("value="+tt.value, func(t *testing.T) {
			t.Setenv("NETBOOTD_TEST_BOOL", tt.value)
			if got := envBool("NETBOOTD_TEST_BOOL"); got != tt.want {
				t.Errorf("envBool(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
