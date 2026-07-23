package authz

import (
	"context"

	"github.com/go-tangra/go-tangra-common/grpcx"

	netbootV1 "github.com/go-tangra/go-tangra-netboot/gen/go/netboot/service/v1"
)

// Checker answers "may this caller do X?" from the roles carried on the
// request's gRPC metadata, which the admin-service transcoder populates from
// the authenticated session.
type Checker struct{}

// NewChecker returns a Checker. It is stateless and safe for concurrent use.
func NewChecker() *Checker { return &Checker{} }

// Allowed reports whether the caller holds perm through any of its roles.
//
// It fails closed: a request with no roles - an unauthenticated caller, or
// one that reached us without passing through the transcoder - is denied
// every permission.
func (c *Checker) Allowed(ctx context.Context, perm Permission) bool {
	for _, role := range grpcx.GetRolesFromContext(ctx) {
		perms, known := PermissionsForRole(role)
		if !known {
			continue
		}
		if _, ok := perms[perm]; ok {
			return true
		}
	}
	return false
}

// Require returns nil when the caller holds perm and a permission-denied
// error otherwise. Service methods call this before touching the upstream.
//
// The error names the missing permission but never the caller's actual
// roles, so a probing client learns what it needed rather than what other
// principals hold.
func (c *Checker) Require(ctx context.Context, perm Permission) error {
	if c.Allowed(ctx, perm) {
		return nil
	}
	return netbootV1.ErrorInsufficientPermissions("permission %q is required", perm)
}
