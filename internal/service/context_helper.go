package service

import "github.com/go-tangra/go-tangra-common/grpcx"

// Thin aliases over the shared metadata accessors, matching the convention
// used by the other Tangra modules.
var (
	getTenantIDFromContext = grpcx.GetTenantIDFromContext
	getUserIDFromContext   = grpcx.GetUserIDFromContext
	getUsernameFromContext = grpcx.GetUsernameFromContext
	getRolesFromContext    = grpcx.GetRolesFromContext
	isPlatformAdmin        = grpcx.IsPlatformAdmin
)
