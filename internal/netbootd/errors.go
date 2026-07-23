package netbootd

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrNotConfigured is returned by every call when no upstream endpoint has
// been configured. It is deliberately distinguishable from a transport
// failure so the system service can report "misconfigured" rather than
// "unreachable".
var ErrNotConfigured = errors.New("netbootd upstream is not configured")

// Upstream reason strings emitted by netbootd's ErrorEncoder. They are part
// of the remote API contract (backend/internal/server/errors.go).
const (
	ReasonValidationFailed = "VALIDATION_FAILED"
	ReasonNotFound         = "NOT_FOUND"
	ReasonConflict         = "CONFLICT"
	ReasonUnauthenticated  = "UNAUTHENTICATED"
	ReasonPermissionDenied = "PERMISSION_DENIED"
	ReasonDhcpDisabled     = "DHCP_DISABLED"
	ReasonRateLimited      = "RATE_LIMITED"
)

// APIError is a structured failure reported by the upstream netbootd.
//
// Message and Details come from netbootd, which already suppresses internal
// detail for 5xx responses; we pass them through unchanged rather than
// inventing our own text, so operators see exactly what the netboot server
// said.
type APIError struct {
	StatusCode int
	Reason     string
	Message    string
	Details    map[string]string
}

func (e *APIError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("netbootd: http %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("netbootd: %s (http %d): %s", e.Reason, e.StatusCode, e.Message)
}

// IsUnauthenticated reports whether the upstream rejected our session.
func (e *APIError) IsUnauthenticated() bool {
	return e.StatusCode == http.StatusUnauthorized || e.Reason == ReasonUnauthenticated
}

// IsNotFound reports whether the upstream resource does not exist.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound || e.Reason == ReasonNotFound
}

// AsAPIError extracts an *APIError from an error chain.
func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

// TransportError wraps a failure that prevented us from obtaining a
// well-formed upstream response: DNS, TCP, TLS, timeout, body limit or an
// undecodable payload.
type TransportError struct {
	Op  string
	Err error
}

func (e *TransportError) Error() string { return fmt.Sprintf("netbootd: %s: %v", e.Op, e.Err) }
func (e *TransportError) Unwrap() error { return e.Err }

// ErrResponseTooLarge is reported when the upstream body exceeds the
// configured limit. Truncated JSON is never parsed, so a hostile or
// malfunctioning upstream cannot smuggle a partial object past us.
var ErrResponseTooLarge = errors.New("upstream response exceeds the configured size limit")
