// Package authz decides whether the calling Tangra principal may perform a
// netboot operation.
//
// The upstream netbootd has a single, coarse operator identity: anything
// this module can do, it can do with full privilege. That makes authorization
// entirely our responsibility - a caller who reaches the netbootd client has
// already been granted the operation. Every service method therefore starts
// with a Checker call, and the checker fails closed.
package authz

// Permission is a fine-grained netboot capability.
type Permission string

const (
	PermMachineView      Permission = "netboot.machine.view"
	PermMachineCreate    Permission = "netboot.machine.create"
	PermMachineUpdate    Permission = "netboot.machine.update"
	PermMachineDelete    Permission = "netboot.machine.delete"
	PermMachineProvision Permission = "netboot.machine.provision"

	PermProfileView   Permission = "netboot.profile.view"
	PermProfileCreate Permission = "netboot.profile.create"
	PermProfileUpdate Permission = "netboot.profile.update"
	PermProfileDelete Permission = "netboot.profile.delete"

	PermDhcpView   Permission = "netboot.dhcp.view"
	PermDhcpManage Permission = "netboot.dhcp.manage"

	PermSessionView Permission = "netboot.session.view"

	PermArtifactView   Permission = "netboot.artifact.view"
	PermArtifactDelete Permission = "netboot.artifact.delete"

	PermSystemView Permission = "netboot.system.view"
)

// Platform-wide roles that carry every netboot permission implicitly.
const (
	RolePlatformAdmin = "platform:admin"
	RoleSuperAdmin    = "super:admin"
)

// Module roles, mirroring the roles declared in cmd/server/assets/menus.yaml.
const (
	RoleNetbootAdmin    = "netboot.admin"
	RoleNetbootOperator = "netboot.operator"
	RoleNetbootViewer   = "netboot.viewer"
	RoleTenantManager   = "tenant:manager"
)

// readOnlyPermissions is the viewer capability set.
var readOnlyPermissions = []Permission{
	PermMachineView,
	PermProfileView,
	PermDhcpView,
	PermSessionView,
	PermArtifactView,
	PermSystemView,
}

// operatorPermissions is the viewer set plus day-to-day machine operations.
// Note that it deliberately excludes DHCP management and profile/artifact
// deletion: mis-scoping the provisioning network or dropping a kernel is a
// change of a different magnitude to arming one machine.
var operatorPermissions = append(append([]Permission{}, readOnlyPermissions...),
	PermMachineCreate,
	PermMachineUpdate,
	PermMachineProvision,
	PermProfileCreate,
	PermProfileUpdate,
)

// adminPermissions is every permission in the module.
var adminPermissions = append(append([]Permission{}, operatorPermissions...),
	PermMachineDelete,
	PermProfileDelete,
	PermDhcpManage,
	PermArtifactDelete,
)

// rolePermissions maps a role code to the permissions it grants.
var rolePermissions = map[string]map[Permission]struct{}{
	RolePlatformAdmin:   permissionSet(adminPermissions),
	RoleSuperAdmin:      permissionSet(adminPermissions),
	RoleNetbootAdmin:    permissionSet(adminPermissions),
	RoleTenantManager:   permissionSet(adminPermissions),
	RoleNetbootOperator: permissionSet(operatorPermissions),
	RoleNetbootViewer:   permissionSet(readOnlyPermissions),
}

func permissionSet(perms []Permission) map[Permission]struct{} {
	set := make(map[Permission]struct{}, len(perms))
	for _, p := range perms {
		set[p] = struct{}{}
	}
	return set
}

// PermissionsForRole returns the permissions granted by a role code. The
// second result reports whether the role is known at all.
func PermissionsForRole(role string) (map[Permission]struct{}, bool) {
	perms, ok := rolePermissions[role]
	return perms, ok
}

// AllPermissions returns every permission the module defines, sorted by
// declaration order. It backs the module's permission catalogue.
func AllPermissions() []Permission {
	out := make([]Permission, len(adminPermissions))
	copy(out, adminPermissions)
	return out
}
