package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	kratosErrors "github.com/go-kratos/kratos/v2/errors"

	"github.com/go-tangra/go-tangra-netboot/internal/netbootd"
)

func TestTranslateUpstreamError(t *testing.T) {
	tests := []struct {
		name       string
		kind       resourceKind
		err        error
		wantCode   int
		wantReason string
	}{
		{"nil stays nil", resourceGeneric, nil, 0, ""},
		{
			name: "not configured", kind: resourceGeneric, err: netbootd.ErrNotConfigured,
			wantCode: 500, wantReason: "CONFIGURATION_ERROR",
		},
		{
			name: "oversized response", kind: resourceGeneric, err: netbootd.ErrResponseTooLarge,
			wantCode: 500, wantReason: "UPSTREAM_ERROR",
		},
		{
			name: "deadline exceeded", kind: resourceGeneric, err: context.DeadlineExceeded,
			wantCode: 504, wantReason: "UPSTREAM_TIMEOUT",
		},
		{
			name: "cancelled", kind: resourceGeneric, err: context.Canceled,
			wantCode: 500, wantReason: "UPSTREAM_ERROR",
		},
		{
			name: "transport failure", kind: resourceGeneric,
			err:      &netbootd.TransportError{Op: "GET /x", Err: errors.New("dial tcp: refused")},
			wantCode: 503, wantReason: "UPSTREAM_UNAVAILABLE",
		},
		{
			name: "400", kind: resourceGeneric,
			err:      &netbootd.APIError{StatusCode: http.StatusBadRequest, Message: "bad"},
			wantCode: 400, wantReason: "BAD_REQUEST",
		},
		{
			name: "401 becomes an upstream-credential fault", kind: resourceGeneric,
			err:      &netbootd.APIError{StatusCode: http.StatusUnauthorized},
			wantCode: 401, wantReason: "UPSTREAM_UNAUTHENTICATED",
		},
		{
			name: "403", kind: resourceGeneric,
			err:      &netbootd.APIError{StatusCode: http.StatusForbidden, Message: "denied"},
			wantCode: 403, wantReason: "ACCESS_DENIED",
		},
		{
			name: "404 machine", kind: resourceMachine,
			err:      &netbootd.APIError{StatusCode: http.StatusNotFound, Message: "gone"},
			wantCode: 404, wantReason: "MACHINE_NOT_FOUND",
		},
		{
			name: "404 profile", kind: resourceProfile,
			err:      &netbootd.APIError{StatusCode: http.StatusNotFound},
			wantCode: 404, wantReason: "PROFILE_NOT_FOUND",
		},
		{
			name: "404 session", kind: resourceSession,
			err:      &netbootd.APIError{StatusCode: http.StatusNotFound},
			wantCode: 404, wantReason: "SESSION_NOT_FOUND",
		},
		{
			name: "404 artifact", kind: resourceArtifact,
			err:      &netbootd.APIError{StatusCode: http.StatusNotFound},
			wantCode: 404, wantReason: "ARTIFACT_NOT_FOUND",
		},
		{
			name: "404 generic", kind: resourceGeneric,
			err:      &netbootd.APIError{StatusCode: http.StatusNotFound},
			wantCode: 404, wantReason: "NOT_FOUND",
		},
		{
			name: "409 machine", kind: resourceMachine,
			err:      &netbootd.APIError{StatusCode: http.StatusConflict},
			wantCode: 409, wantReason: "MACHINE_ALREADY_EXISTS",
		},
		{
			name: "409 generic", kind: resourceSession,
			err:      &netbootd.APIError{StatusCode: http.StatusConflict},
			wantCode: 409, wantReason: "CONFLICT",
		},
		{
			name: "412", kind: resourceGeneric,
			err:      &netbootd.APIError{StatusCode: http.StatusPreconditionFailed},
			wantCode: 412, wantReason: "DHCP_DISABLED",
		},
		{
			name: "422", kind: resourceGeneric,
			err:      &netbootd.APIError{StatusCode: http.StatusUnprocessableEntity, Message: "invalid"},
			wantCode: 422, wantReason: "VALIDATION_FAILED",
		},
		{
			name: "429", kind: resourceGeneric,
			err:      &netbootd.APIError{StatusCode: http.StatusTooManyRequests},
			wantCode: 429, wantReason: "RATE_LIMITED",
		},
		{
			name: "500", kind: resourceGeneric,
			err:      &netbootd.APIError{StatusCode: http.StatusInternalServerError},
			wantCode: 500, wantReason: "UPSTREAM_ERROR",
		},
		{
			name: "502", kind: resourceGeneric,
			err:      &netbootd.APIError{StatusCode: http.StatusBadGateway},
			wantCode: 502, wantReason: "UPSTREAM_BAD_GATEWAY",
		},
		{
			name: "503", kind: resourceGeneric,
			err:      &netbootd.APIError{StatusCode: http.StatusServiceUnavailable},
			wantCode: 503, wantReason: "UPSTREAM_UNAVAILABLE",
		},
		{
			name: "504", kind: resourceGeneric,
			err:      &netbootd.APIError{StatusCode: http.StatusGatewayTimeout},
			wantCode: 504, wantReason: "UPSTREAM_TIMEOUT",
		},
		{
			name: "unmapped 4xx", kind: resourceGeneric,
			err:      &netbootd.APIError{StatusCode: http.StatusTeapot, Message: "teapot"},
			wantCode: 500, wantReason: "UPSTREAM_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := translateUpstreamError(tt.kind, tt.err)
			if tt.err == nil {
				if got != nil {
					t.Fatalf("translateUpstreamError(nil) = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("translateUpstreamError() = nil, want an error")
			}
			if code := kratosErrors.Code(got); code != tt.wantCode {
				t.Errorf("code = %d, want %d", code, tt.wantCode)
			}
			if reason := kratosErrors.Reason(got); reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}

// A 5xx from the upstream may carry internal diagnostics; those must not be
// relayed to a client.
func TestUpstream5xxDetailIsNotRelayed(t *testing.T) {
	err := translateUpstreamError(resourceMachine, &netbootd.APIError{
		StatusCode: http.StatusInternalServerError,
		Message:    "pq: relation \"machines\" does not exist on db-primary.internal",
	})
	if msg := kratosErrors.FromError(err).Message; containsSubstring(msg, "db-primary.internal") {
		t.Errorf("translated message leaked upstream internals: %q", msg)
	}
}

// A transport error can name internal hostnames from DNS or TLS failures.
func TestTransportErrorDetailIsNotRelayed(t *testing.T) {
	err := translateUpstreamError(resourceMachine, &netbootd.TransportError{
		Op:  "GET /api/v1/machines",
		Err: errors.New("dial tcp 10.42.0.7:8080: connect: connection refused"),
	})
	if msg := kratosErrors.FromError(err).Message; containsSubstring(msg, "10.42.0.7") {
		t.Errorf("translated message leaked an internal address: %q", msg)
	}
}

// Validation details are safe and genuinely useful, so they survive.
func TestValidationDetailsArePreserved(t *testing.T) {
	err := translateUpstreamError(resourceMachine, &netbootd.APIError{
		StatusCode: http.StatusUnprocessableEntity,
		Message:    "validation failed",
		Details:    map[string]string{"mac": "must be a 48-bit MAC address"},
	})
	metadata := kratosErrors.FromError(err).Metadata
	if got := metadata["mac"]; got != "must be a 48-bit MAC address" {
		t.Errorf("metadata[mac] = %q, want the upstream field error", got)
	}
}

// Errors are matched through wrapping, which is how they arrive from the
// client's retry loop.
func TestTranslateUnwrapsWrappedErrors(t *testing.T) {
	wrapped := fmt.Errorf("listing machines: %w", &netbootd.APIError{StatusCode: http.StatusNotFound})
	if reason := kratosErrors.Reason(translateUpstreamError(resourceMachine, wrapped)); reason != "MACHINE_NOT_FOUND" {
		t.Errorf("reason = %q, want MACHINE_NOT_FOUND for a wrapped error", reason)
	}
}
