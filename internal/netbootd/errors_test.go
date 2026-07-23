package netbootd

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestAPIErrorMessage(t *testing.T) {
	withReason := &APIError{StatusCode: 404, Reason: ReasonNotFound, Message: "machine not found"}
	if got, want := withReason.Error(), "netbootd: NOT_FOUND (http 404): machine not found"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	withoutReason := &APIError{StatusCode: 500, Message: "internal error"}
	if got, want := withoutReason.Error(), "netbootd: http 500: internal error"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAPIErrorClassification(t *testing.T) {
	tests := []struct {
		name         string
		err          *APIError
		wantUnauthed bool
		wantNotFound bool
	}{
		{"401 by status", &APIError{StatusCode: http.StatusUnauthorized}, true, false},
		{"401 by reason", &APIError{StatusCode: 200, Reason: ReasonUnauthenticated}, true, false},
		{"404 by status", &APIError{StatusCode: http.StatusNotFound}, false, true},
		{"404 by reason", &APIError{StatusCode: 200, Reason: ReasonNotFound}, false, true},
		{"conflict is neither", &APIError{StatusCode: http.StatusConflict}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.IsUnauthenticated(); got != tt.wantUnauthed {
				t.Errorf("IsUnauthenticated() = %v, want %v", got, tt.wantUnauthed)
			}
			if got := tt.err.IsNotFound(); got != tt.wantNotFound {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.wantNotFound)
			}
		})
	}
}

func TestAsAPIError(t *testing.T) {
	target := &APIError{StatusCode: 404}

	if got, ok := AsAPIError(target); !ok || got != target {
		t.Errorf("AsAPIError(direct) = %v, %v; want the same error and true", got, ok)
	}
	if got, ok := AsAPIError(fmt.Errorf("wrapped: %w", target)); !ok || got != target {
		t.Errorf("AsAPIError(wrapped) = %v, %v; want the same error and true", got, ok)
	}
	if _, ok := AsAPIError(errors.New("unrelated")); ok {
		t.Error("AsAPIError(unrelated) = true, want false")
	}
	if _, ok := AsAPIError(nil); ok {
		t.Error("AsAPIError(nil) = true, want false")
	}
}

func TestTransportErrorWraps(t *testing.T) {
	inner := errors.New("connection refused")
	err := &TransportError{Op: "GET /api/v1/machines", Err: inner}

	if !errors.Is(err, inner) {
		t.Error("errors.Is() = false, want the inner error to be reachable")
	}
	if got, want := err.Error(), "netbootd: GET /api/v1/machines: connection refused"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
