package authz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/errors"
	grpcMD "google.golang.org/grpc/metadata"

	"github.com/go-tangra/go-tangra-common/grpcx"
)

// ctxWithRoles builds a context carrying roles the way the admin-service
// transcoder delivers them.
func ctxWithRoles(roles ...string) context.Context {
	md := grpcMD.New(nil)
	if len(roles) > 0 {
		joined := roles[0]
		for _, r := range roles[1:] {
			joined += "," + r
		}
		md.Set(grpcx.MDRoles, joined)
	}
	return grpcMD.NewIncomingContext(context.Background(), md)
}

func TestCheckerAllowed(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		perm  Permission
		want  bool
	}{
		{"platform admin may provision", []string{RolePlatformAdmin}, PermMachineProvision, true},
		{"platform admin may manage dhcp", []string{RolePlatformAdmin}, PermDhcpManage, true},
		{"super admin may delete artifacts", []string{RoleSuperAdmin}, PermArtifactDelete, true},
		{"netboot admin may manage dhcp", []string{RoleNetbootAdmin}, PermDhcpManage, true},
		{"operator may provision", []string{RoleNetbootOperator}, PermMachineProvision, true},
		{"operator may not manage dhcp", []string{RoleNetbootOperator}, PermDhcpManage, false},
		{"operator may not delete machines", []string{RoleNetbootOperator}, PermMachineDelete, false},
		{"operator may not delete profiles", []string{RoleNetbootOperator}, PermProfileDelete, false},
		{"operator may not delete artifacts", []string{RoleNetbootOperator}, PermArtifactDelete, false},
		{"viewer may view", []string{RoleNetbootViewer}, PermMachineView, true},
		{"viewer may not create", []string{RoleNetbootViewer}, PermMachineCreate, false},
		{"viewer may not provision", []string{RoleNetbootViewer}, PermMachineProvision, false},
		{"viewer may not preview profiles", []string{RoleNetbootViewer}, PermProfileUpdate, false},
		{"unknown role grants nothing", []string{"marketing:intern"}, PermMachineView, false},
		{"no roles grants nothing", nil, PermMachineView, false},
		{"union of roles applies", []string{"marketing:intern", RoleNetbootAdmin}, PermDhcpManage, true},
	}

	checker := NewChecker()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ctxWithRoles(tt.roles...)
			if got := checker.Allowed(ctx, tt.perm); got != tt.want {
				t.Errorf("Allowed(%v, %q) = %v, want %v", tt.roles, tt.perm, got, tt.want)
			}
		})
	}
}

// A context that never passed through the transcoder carries no metadata at
// all; the checker must still deny rather than panic or default-allow.
func TestCheckerFailsClosedWithoutMetadata(t *testing.T) {
	checker := NewChecker()
	for _, perm := range AllPermissions() {
		if checker.Allowed(context.Background(), perm) {
			t.Errorf("Allowed(bare context, %q) = true, want false", perm)
		}
	}
}

func TestCheckerRequire(t *testing.T) {
	checker := NewChecker()

	if err := checker.Require(ctxWithRoles(RolePlatformAdmin), PermDhcpManage); err != nil {
		t.Errorf("Require() for a permitted caller = %v, want nil", err)
	}

	err := checker.Require(ctxWithRoles(RoleNetbootViewer), PermDhcpManage)
	if err == nil {
		t.Fatal("Require() for a denied caller = nil, want an error")
	}
	if code := errors.Code(err); code != 403 {
		t.Errorf("error code = %d, want 403", code)
	}
}

// The denial must name the permission the caller lacked, so an operator can
// self-diagnose, without disclosing the roles other principals hold.
func TestRequireErrorNamesThePermission(t *testing.T) {
	err := NewChecker().Require(ctxWithRoles(RoleNetbootViewer), PermMachineDelete)
	if err == nil {
		t.Fatal("Require() = nil, want an error")
	}
	if msg := errors.FromError(err).Message; msg == "" {
		t.Fatal("error message is empty")
	} else if want := string(PermMachineDelete); !contains(msg, want) {
		t.Errorf("error message = %q, want it to name %q", msg, want)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle || len(needle) == 0 || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// Role tables are duplicated between Go and menus.yaml; this guards the Go
// side against a permission being defined and then never granted to anyone.
func TestEveryPermissionIsGrantedByTheAdminRole(t *testing.T) {
	adminPerms, ok := PermissionsForRole(RoleNetbootAdmin)
	if !ok {
		t.Fatal("PermissionsForRole(netboot.admin) reported an unknown role")
	}
	for _, perm := range AllPermissions() {
		if _, granted := adminPerms[perm]; !granted {
			t.Errorf("permission %q is granted by no role", perm)
		}
	}
}

func TestRolePermissionsAreOrdered(t *testing.T) {
	viewer, _ := PermissionsForRole(RoleNetbootViewer)
	operator, _ := PermissionsForRole(RoleNetbootOperator)
	admin, _ := PermissionsForRole(RoleNetbootAdmin)

	// Each tier must be a superset of the one below it.
	for perm := range viewer {
		if _, ok := operator[perm]; !ok {
			t.Errorf("operator is missing viewer permission %q", perm)
		}
	}
	for perm := range operator {
		if _, ok := admin[perm]; !ok {
			t.Errorf("admin is missing operator permission %q", perm)
		}
	}
	if len(admin) <= len(operator) || len(operator) <= len(viewer) {
		t.Errorf("role tiers are not strictly increasing: viewer=%d operator=%d admin=%d",
			len(viewer), len(operator), len(admin))
	}
}

func TestPermissionsForUnknownRole(t *testing.T) {
	if _, ok := PermissionsForRole("nobody"); ok {
		t.Error("PermissionsForRole(nobody) reported a known role")
	}
}

// AllPermissions hands out a copy; mutating it must not corrupt the tables.
func TestAllPermissionsReturnsACopy(t *testing.T) {
	first := AllPermissions()
	if len(first) == 0 {
		t.Fatal("AllPermissions() is empty")
	}
	first[0] = "tampered"

	if AllPermissions()[0] == "tampered" {
		t.Error("AllPermissions() exposed the backing array")
	}
}
