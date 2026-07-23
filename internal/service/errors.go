package service

import (
	"context"
	"errors"
	"net/http"

	netbootV1 "github.com/go-tangra/go-tangra-netboot/gen/go/netboot/service/v1"
	"github.com/go-tangra/go-tangra-netboot/internal/netbootd"
)

// resourceKind lets the error translation name the thing that was missing or
// in conflict without every call site repeating the mapping.
type resourceKind int

const (
	resourceGeneric resourceKind = iota
	resourceMachine
	resourceProfile
	resourceSession
	resourceArtifact
)

// translateUpstreamError converts a netbootd client failure into a Kratos
// error carrying this module's own reason and status code.
//
// It deliberately does not forward upstream 5xx text verbatim into a 5xx of
// our own with the same reason: an operator seeing UPSTREAM_ERROR knows the
// fault is on the netboot host rather than in the Tangra control plane, which
// is the distinction that matters when triaging.
func translateUpstreamError(kind resourceKind, err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, netbootd.ErrNotConfigured) {
		return netbootV1.ErrorConfigurationError(
			"the netboot upstream is not configured; set %s", netbootd.EnvEndpoint)
	}
	if errors.Is(err, netbootd.ErrResponseTooLarge) {
		return netbootV1.ErrorUpstreamError("the netboot server returned an oversized response")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return netbootV1.ErrorUpstreamTimeout("the netboot server did not respond in time")
	}
	if errors.Is(err, context.Canceled) {
		return netbootV1.ErrorUpstreamError("the request was cancelled")
	}

	apiErr, ok := netbootd.AsAPIError(err)
	if !ok {
		// Anything left is a transport failure: DNS, TCP, TLS, malformed
		// payload. The underlying text can name internal hosts, so it is
		// logged by the caller and not returned.
		return netbootV1.ErrorUpstreamUnavailable("the netboot server is unreachable")
	}

	switch apiErr.StatusCode {
	case http.StatusBadRequest:
		return netbootV1.ErrorBadRequest("%s", apiErr.Message)
	case http.StatusUnauthorized:
		return netbootV1.ErrorUpstreamUnauthenticated(
			"the netboot server rejected this module's operator credentials")
	case http.StatusForbidden:
		return netbootV1.ErrorAccessDenied("%s", apiErr.Message)
	case http.StatusNotFound:
		return notFoundFor(kind, apiErr.Message)
	case http.StatusConflict:
		return conflictFor(kind, apiErr.Message)
	case http.StatusPreconditionFailed:
		return netbootV1.ErrorDhcpDisabled("%s", apiErr.Message)
	case http.StatusUnprocessableEntity:
		return netbootV1.ErrorValidationFailed("%s", apiErr.Message).WithMetadata(apiErr.Details)
	case http.StatusTooManyRequests:
		return netbootV1.ErrorRateLimited("the netboot server is rate limiting this client")
	case http.StatusBadGateway:
		return netbootV1.ErrorUpstreamBadGateway("the netboot server reported a bad gateway")
	case http.StatusServiceUnavailable:
		return netbootV1.ErrorUpstreamUnavailable("the netboot server is unavailable")
	case http.StatusGatewayTimeout:
		return netbootV1.ErrorUpstreamTimeout("the netboot server timed out")
	}

	if apiErr.StatusCode >= 500 {
		return netbootV1.ErrorUpstreamError("the netboot server reported an internal error")
	}
	return netbootV1.ErrorUpstreamError("%s", apiErr.Message)
}

func notFoundFor(kind resourceKind, msg string) error {
	switch kind {
	case resourceMachine:
		return netbootV1.ErrorMachineNotFound("%s", msg)
	case resourceProfile:
		return netbootV1.ErrorProfileNotFound("%s", msg)
	case resourceSession:
		return netbootV1.ErrorSessionNotFound("%s", msg)
	case resourceArtifact:
		return netbootV1.ErrorArtifactNotFound("%s", msg)
	default:
		return netbootV1.ErrorNotFound("%s", msg)
	}
}

func conflictFor(kind resourceKind, msg string) error {
	switch kind {
	case resourceMachine:
		return netbootV1.ErrorMachineAlreadyExists("%s", msg)
	case resourceProfile:
		return netbootV1.ErrorProfileAlreadyExists("%s", msg)
	default:
		return netbootV1.ErrorConflict("%s", msg)
	}
}
