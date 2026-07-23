package service

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/errors"

	netbootV1 "github.com/go-tangra/go-tangra-netboot/gen/go/netboot/service/v1"
)

func profileFixture() map[string]any {
	return map[string]any{
		"id":                   "p-1",
		"name":                 "ubuntu-noble-base",
		"version":              3,
		"ubuntu_release":       "noble",
		"storage_layout":       `{"mode":"lvm"}`,
		"network_config":       `{"version":2}`,
		"packages":             []string{"curl", "vim"},
		"ssh_authorized_keys":  []string{"ssh-ed25519 AAAA... operator@example"},
		"user_data_template":   "#cloud-config\n",
		"late_commands":        []string{"echo done"},
		"kernel_cmdline_extra": "console=ttyS0",
		"created_at":           "2026-01-02T03:04:05Z",
		"updated_at":           "2026-01-02T03:04:06Z",
		"assigned_machines":    "12",
		"keyboard_layout":      "us",
		"locale":               "en_US.UTF-8",
		"timezone":             "UTC",
		"install_username":     "ubuntu",
		"has_password":         true,
		"default_dns":          []string{"10.0.0.53"},
	}
}

func TestGetProfileMapsUpstreamPayload(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodGet, "/api/v1/profiles/p-1", http.StatusOK, profileFixture())
	svc := newProfileServiceForTest(newTestClient(t, stub))

	resp, err := svc.GetProfile(adminCtx(), &netbootV1.GetProfileRequest{Id: "p-1"})
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	p := resp.GetProfile()
	if p.GetName() != "ubuntu-noble-base" {
		t.Errorf("name = %q, want ubuntu-noble-base", p.GetName())
	}
	if p.GetAssignedMachines() != 12 {
		t.Errorf("assignedMachines = %d, want 12", p.GetAssignedMachines())
	}
	if !p.GetHasPassword() {
		t.Error("hasPassword = false, want true")
	}
	if got := p.GetPackages(); len(got) != 2 {
		t.Errorf("packages = %v, want 2 entries", got)
	}
}

// The profile message has no password field at all - only has_password - so
// there is no path by which a stored credential can reach a client.
func TestProfileResponseCarriesNoPassword(t *testing.T) {
	stub := newStubUpstream(t)
	fixture := profileFixture()
	// A misbehaving or compromised upstream that volunteers a password must
	// not have it propagate: the field simply does not exist on our message.
	fixture["password"] = "should-never-surface"
	stub.reply(http.MethodGet, "/api/v1/profiles/p-1", http.StatusOK, fixture)
	svc := newProfileServiceForTest(newTestClient(t, stub))

	resp, err := svc.GetProfile(adminCtx(), &netbootV1.GetProfileRequest{Id: "p-1"})
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if rendered := resp.String(); strings.Contains(rendered, "should-never-surface") {
		t.Fatalf("profile response leaked a password: %s", rendered)
	}
}

// The plaintext password must reach netbootd - it is the only party that can
// hash it - but only in the request body.
func TestCreateProfileForwardsPasswordExactlyOnce(t *testing.T) {
	stub := newStubUpstream(t)
	var raw []byte
	stub.on(http.MethodPost, "/api/v1/profiles", func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		writeData(w, http.StatusOK, profileFixture())
	})
	svc := newProfileServiceForTest(newTestClient(t, stub))

	_, err := svc.CreateProfile(adminCtx(), &netbootV1.CreateProfileRequest{
		Profile: &netbootV1.ProfileInput{
			Name:          "ubuntu-noble-base",
			UbuntuRelease: "noble",
			Password:      "install-time-password",
		},
	})
	if err != nil {
		t.Fatalf("CreateProfile() error = %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := body["password"]; got != "install-time-password" {
		t.Errorf("upstream password = %v, want the plaintext to be forwarded for hashing", got)
	}
	if got := strings.Count(string(raw), "install-time-password"); got != 1 {
		t.Errorf("password appears %d times in the request body, want exactly 1", got)
	}
}

// An omitted password must not be sent, since the upstream treats an empty
// value on update as "keep the existing password".
func TestUpdateProfileOmitsAnEmptyPassword(t *testing.T) {
	stub := newStubUpstream(t)
	var body map[string]any
	stub.on(http.MethodPut, "/api/v1/profiles/p-1", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		writeData(w, http.StatusOK, profileFixture())
	})
	svc := newProfileServiceForTest(newTestClient(t, stub))

	_, err := svc.UpdateProfile(adminCtx(), &netbootV1.UpdateProfileRequest{
		Id:      "p-1",
		Profile: &netbootV1.ProfileInput{Name: "renamed", UbuntuRelease: "noble"},
	})
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}

	nested, ok := body["profile"].(map[string]any)
	if !ok {
		t.Fatalf("request body has no nested profile object: %v", body)
	}
	if _, present := nested["password"]; present {
		t.Error("request body carries an empty password, which would be ambiguous upstream")
	}
}

func TestCloneAndDeleteProfile(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodPost, "/api/v1/profiles/p-1/clone", http.StatusOK, profileFixture())
	stub.reply(http.MethodDelete, "/api/v1/profiles/p-1", http.StatusOK, nil)
	svc := newProfileServiceForTest(newTestClient(t, stub))

	if _, err := svc.CloneProfile(adminCtx(), &netbootV1.CloneProfileRequest{Id: "p-1", NewName: "copy"}); err != nil {
		t.Fatalf("CloneProfile() error = %v", err)
	}
	if _, err := svc.DeleteProfile(adminCtx(), &netbootV1.DeleteProfileRequest{Id: "p-1"}); err != nil {
		t.Fatalf("DeleteProfile() error = %v", err)
	}
}

func TestPreviewProfile(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodPost, "/api/v1/profiles/p-1/preview", http.StatusOK, map[string]any{
		"user_data": "#cloud-config\npassword: REDACTED\n",
		"cmdline":   "console=ttyS0",
	})
	svc := newProfileServiceForTest(newTestClient(t, stub))

	resp, err := svc.PreviewProfile(adminCtx(), &netbootV1.PreviewProfileRequest{Id: "p-1"})
	if err != nil {
		t.Fatalf("PreviewProfile() error = %v", err)
	}
	if resp.GetCmdline() != "console=ttyS0" {
		t.Errorf("cmdline = %q, want console=ttyS0", resp.GetCmdline())
	}
	if resp.GetUserData() == "" {
		t.Error("userData is empty, want the rendered seed")
	}
}

// Preview renders the whole installation recipe, so a read-only viewer must
// not reach it.
func TestPreviewProfileRequiresUpdatePermission(t *testing.T) {
	stub := newStubUpstream(t)
	svc := newProfileServiceForTest(newTestClient(t, stub))

	_, err := svc.PreviewProfile(viewerCtx(), &netbootV1.PreviewProfileRequest{Id: "p-1"})
	if err == nil {
		t.Fatal("PreviewProfile() as viewer = nil error, want a denial")
	}
	if code := errors.Code(err); code != http.StatusForbidden {
		t.Errorf("error code = %d, want 403", code)
	}
}

func TestProfileRPCsEnforcePermissions(t *testing.T) {
	stub := newStubUpstream(t)
	svc := newProfileServiceForTest(newTestClient(t, stub))

	tests := []struct {
		name string
		call func() error
	}{
		{"list", func() error {
			_, err := svc.ListProfiles(anonCtx(), &netbootV1.ListProfilesRequest{})
			return err
		}},
		{"get", func() error {
			_, err := svc.GetProfile(anonCtx(), &netbootV1.GetProfileRequest{Id: "p-1"})
			return err
		}},
		{"create as viewer", func() error {
			_, err := svc.CreateProfile(viewerCtx(), &netbootV1.CreateProfileRequest{
				Profile: &netbootV1.ProfileInput{Name: "x", UbuntuRelease: "noble"},
			})
			return err
		}},
		{"update as viewer", func() error {
			_, err := svc.UpdateProfile(viewerCtx(), &netbootV1.UpdateProfileRequest{
				Id: "p-1", Profile: &netbootV1.ProfileInput{Name: "x", UbuntuRelease: "noble"},
			})
			return err
		}},
		{"clone as viewer", func() error {
			_, err := svc.CloneProfile(viewerCtx(), &netbootV1.CloneProfileRequest{Id: "p-1", NewName: "c"})
			return err
		}},
		{"delete as operator", func() error {
			_, err := svc.DeleteProfile(ctxWithRoles("netboot.operator"), &netbootV1.DeleteProfileRequest{Id: "p-1"})
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Fatal("call succeeded, want a permission denial")
			} else if code := errors.Code(err); code != http.StatusForbidden {
				t.Errorf("error code = %d, want 403", code)
			}
		})
	}
}

func TestListProfiles(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodGet, "/api/v1/profiles", http.StatusOK, map[string]any{
		"profiles": []any{profileFixture()},
		"meta":     map[string]any{"total": "1", "page": 1, "page_size": 20},
	})
	svc := newProfileServiceForTest(newTestClient(t, stub))

	resp, err := svc.ListProfiles(viewerCtx(), &netbootV1.ListProfilesRequest{})
	if err != nil {
		t.Fatalf("ListProfiles() error = %v", err)
	}
	if got := len(resp.GetProfiles()); got != 1 {
		t.Errorf("len(profiles) = %d, want 1", got)
	}
}

func TestProfileConflictIsTranslated(t *testing.T) {
	stub := newStubUpstream(t)
	stub.failWith(http.MethodPost, "/api/v1/profiles", http.StatusConflict, "CONFLICT", "name already in use")
	svc := newProfileServiceForTest(newTestClient(t, stub))

	_, err := svc.CreateProfile(adminCtx(), &netbootV1.CreateProfileRequest{
		Profile: &netbootV1.ProfileInput{Name: "dupe", UbuntuRelease: "noble"},
	})
	if err == nil {
		t.Fatal("CreateProfile() = nil error, want a conflict")
	}
	if reason := errors.Reason(err); reason != "PROFILE_ALREADY_EXISTS" {
		t.Errorf("error reason = %q, want PROFILE_ALREADY_EXISTS", reason)
	}
}
