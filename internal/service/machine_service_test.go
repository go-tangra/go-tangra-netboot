package service

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/go-kratos/kratos/v2/errors"

	netbootV1 "github.com/go-tangra/go-tangra-netboot/gen/go/netboot/service/v1"
)

// machineFixture is one machine as netbootd's protojson encoder emits it:
// snake_case names, 64-bit integers as strings, RFC 3339 timestamps.
func machineFixture() map[string]any {
	return map[string]any{
		"id":                "m-1",
		"mac":               "aa:bb:cc:dd:ee:ff",
		"name":              "worker-01",
		"firmware":          "uefi_x64",
		"profile_id":        "p-1",
		"reservation_ip":    "10.0.0.10",
		"provision_state":   "installing",
		"notes":             "rack 4",
		"created_at":        "2026-01-02T03:04:05Z",
		"updated_at":        "2026-01-02T03:04:06Z",
		"active_session_id": "s-1",
		"network_config":    `{"version":2}`,
		"install_network": map[string]any{
			"address": "10.1.0.10/24",
			"gateway": "10.1.0.1",
			"dns":     []string{"10.1.0.53"},
		},
	}
}

func TestListMachinesMapsUpstreamPayload(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodGet, "/api/v1/machines", http.StatusOK, map[string]any{
		"machines": []any{machineFixture()},
		// protojson renders int64 as a string; the client must cope.
		"meta": map[string]any{"total": "42", "page": 1, "page_size": 20},
	})
	svc := newMachineServiceForTest(newTestClient(t, stub))

	resp, err := svc.ListMachines(adminCtx(), &netbootV1.ListMachinesRequest{})
	if err != nil {
		t.Fatalf("ListMachines() error = %v", err)
	}
	if got := len(resp.GetMachines()); got != 1 {
		t.Fatalf("len(machines) = %d, want 1", got)
	}
	if got := resp.GetMeta().GetTotal(); got != 42 {
		t.Errorf("meta.total = %d, want 42 (decoded from a JSON string)", got)
	}

	m := resp.GetMachines()[0]
	if m.GetFirmware() != netbootV1.Firmware_FIRMWARE_UEFI_X64 {
		t.Errorf("firmware = %v, want UEFI_X64", m.GetFirmware())
	}
	if m.GetProvisionState() != netbootV1.ProvisionState_PROVISION_STATE_INSTALLING {
		t.Errorf("provisionState = %v, want INSTALLING", m.GetProvisionState())
	}
	if m.GetCreateTime() == nil {
		t.Error("createTime = nil, want the parsed timestamp")
	}
	if got := m.GetInstallNetwork().GetAddress(); got != "10.1.0.10/24" {
		t.Errorf("installNetwork.address = %q, want %q", got, "10.1.0.10/24")
	}
}

// The filter enums must be translated back into the upstream's string
// vocabulary, not passed through as protobuf enum names.
func TestListMachinesTranslatesFilters(t *testing.T) {
	stub := newStubUpstream(t)
	var gotQuery string
	stub.on(http.MethodGet, "/api/v1/machines", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeData(w, http.StatusOK, map[string]any{"machines": []any{}, "meta": map[string]any{"total": 0}})
	})
	svc := newMachineServiceForTest(newTestClient(t, stub))

	state := netbootV1.ProvisionState_PROVISION_STATE_FAILED
	profile := "p-9"
	query := "worker"
	_, err := svc.ListMachines(adminCtx(), &netbootV1.ListMachinesRequest{
		Page:      &netbootV1.PageRequest{Page: ptr(uint32(2)), PageSize: ptr(uint32(50))},
		State:     &state,
		ProfileId: &profile,
		Q:         &query,
	})
	if err != nil {
		t.Fatalf("ListMachines() error = %v", err)
	}

	for _, want := range []string{"state=failed", "profile_id=p-9", "q=worker", "page=2", "page_size=50"} {
		if !containsSubstring(gotQuery, want) {
			t.Errorf("upstream query = %q, want it to contain %q", gotQuery, want)
		}
	}
}

func TestGetMachineNotFoundIsTranslated(t *testing.T) {
	stub := newStubUpstream(t)
	stub.failWith(http.MethodGet, "/api/v1/machines/m-404", http.StatusNotFound, "NOT_FOUND", "machine not found")
	svc := newMachineServiceForTest(newTestClient(t, stub))

	_, err := svc.GetMachine(adminCtx(), &netbootV1.GetMachineRequest{Id: "m-404"})
	if err == nil {
		t.Fatal("GetMachine() = nil error, want a not-found failure")
	}
	if code := errors.Code(err); code != http.StatusNotFound {
		t.Errorf("error code = %d, want 404", code)
	}
	if reason := errors.Reason(err); reason != "MACHINE_NOT_FOUND" {
		t.Errorf("error reason = %q, want MACHINE_NOT_FOUND", reason)
	}
}

// A MAC typed with hyphens or in upper case must reach netbootd in the
// canonical form it stores, so the same NIC cannot be registered twice.
func TestCreateMachineNormalisesMAC(t *testing.T) {
	stub := newStubUpstream(t)
	var body map[string]any
	stub.on(http.MethodPost, "/api/v1/machines", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		writeData(w, http.StatusOK, machineFixture())
	})
	svc := newMachineServiceForTest(newTestClient(t, stub))

	_, err := svc.CreateMachine(adminCtx(), &netbootV1.CreateMachineRequest{
		Mac:      "AA-BB-CC-DD-EE-FF",
		Name:     "worker-01",
		Firmware: netbootV1.Firmware_FIRMWARE_BIOS,
	})
	if err != nil {
		t.Fatalf("CreateMachine() error = %v", err)
	}
	if got := body["mac"]; got != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("upstream mac = %v, want the canonical lower-case colon form", got)
	}
	if got := body["firmware"]; got != "bios" {
		t.Errorf("upstream firmware = %v, want %q", got, "bios")
	}
}

// A field the caller omitted must not be sent at all, or a partial update
// would clear values the operator never mentioned.
func TestUpdateMachineSendsOnlyProvidedFields(t *testing.T) {
	stub := newStubUpstream(t)
	var body map[string]any
	stub.on(http.MethodPatch, "/api/v1/machines/m-1", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		writeData(w, http.StatusOK, machineFixture())
	})
	svc := newMachineServiceForTest(newTestClient(t, stub))

	newName := "worker-renamed"
	_, err := svc.UpdateMachine(adminCtx(), &netbootV1.UpdateMachineRequest{Id: "m-1", Name: &newName})
	if err != nil {
		t.Fatalf("UpdateMachine() error = %v", err)
	}
	if got := body["name"]; got != newName {
		t.Errorf("upstream name = %v, want %q", got, newName)
	}
	for _, absent := range []string{"profile_id", "reservation_ip", "notes", "network_config", "install_network"} {
		if _, present := body[absent]; present {
			t.Errorf("upstream body carries %q, which the caller never set", absent)
		}
	}
}

// An explicitly-set empty string is a request to clear, and must be sent.
func TestUpdateMachineForwardsExplicitClear(t *testing.T) {
	stub := newStubUpstream(t)
	var body map[string]any
	stub.on(http.MethodPatch, "/api/v1/machines/m-1", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		writeData(w, http.StatusOK, machineFixture())
	})
	svc := newMachineServiceForTest(newTestClient(t, stub))

	empty := ""
	_, err := svc.UpdateMachine(adminCtx(), &netbootV1.UpdateMachineRequest{Id: "m-1", NetworkConfig: &empty})
	if err != nil {
		t.Fatalf("UpdateMachine() error = %v", err)
	}
	got, present := body["network_config"]
	if !present {
		t.Fatal("upstream body omits network_config, but the caller asked to clear it")
	}
	if got != "" {
		t.Errorf("network_config = %v, want an empty string", got)
	}
}

func TestProvisionAndCancel(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodPost, "/api/v1/machines/m-1/provision", http.StatusOK, machineFixture())
	stub.reply(http.MethodPost, "/api/v1/machines/m-1/cancel", http.StatusOK, machineFixture())
	svc := newMachineServiceForTest(newTestClient(t, stub))

	if _, err := svc.ProvisionMachine(adminCtx(), &netbootV1.ProvisionMachineRequest{Id: "m-1"}); err != nil {
		t.Fatalf("ProvisionMachine() error = %v", err)
	}
	if _, err := svc.CancelProvision(adminCtx(), &netbootV1.CancelProvisionRequest{Id: "m-1"}); err != nil {
		t.Fatalf("CancelProvision() error = %v", err)
	}
}

func TestDeleteMachine(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodDelete, "/api/v1/machines/m-1", http.StatusOK, nil)
	svc := newMachineServiceForTest(newTestClient(t, stub))

	if _, err := svc.DeleteMachine(adminCtx(), &netbootV1.DeleteMachineRequest{Id: "m-1"}); err != nil {
		t.Fatalf("DeleteMachine() error = %v", err)
	}
}

func TestListUnknownBootsAndRegister(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodGet, "/api/v1/machines/unknown", http.StatusOK, map[string]any{
		"boots": []any{map[string]any{
			"mac": "11:22:33:44:55:66", "last_seen": "2026-01-02T03:04:05Z", "attempts": "3",
		}},
		"meta": map[string]any{"total": "1"},
	})
	stub.reply(http.MethodPost, "/api/v1/machines/register-unknown", http.StatusOK, machineFixture())
	svc := newMachineServiceForTest(newTestClient(t, stub))

	listed, err := svc.ListUnknownBoots(adminCtx(), &netbootV1.ListUnknownBootsRequest{})
	if err != nil {
		t.Fatalf("ListUnknownBoots() error = %v", err)
	}
	if got := len(listed.GetBoots()); got != 1 {
		t.Fatalf("len(boots) = %d, want 1", got)
	}
	if got := listed.GetBoots()[0].GetAttempts(); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}

	registered, err := svc.RegisterUnknownMachine(adminCtx(), &netbootV1.RegisterUnknownMachineRequest{
		Mac: "AA:BB:CC:DD:EE:FF", Name: "worker-01",
	})
	if err != nil {
		t.Fatalf("RegisterUnknownMachine() error = %v", err)
	}
	if registered.GetMachine().GetId() != "m-1" {
		t.Errorf("machine id = %q, want m-1", registered.GetMachine().GetId())
	}
}

// Every machine RPC must refuse a caller lacking the matching permission,
// and must do so before any upstream request is issued - the stub registers
// no routes, so a leaked call fails the test.
func TestMachineRPCsEnforcePermissions(t *testing.T) {
	stub := newStubUpstream(t)
	svc := newMachineServiceForTest(newTestClient(t, stub))

	tests := []struct {
		name string
		call func() error
	}{
		{"list", func() error {
			_, err := svc.ListMachines(anonCtx(), &netbootV1.ListMachinesRequest{})
			return err
		}},
		{"get", func() error {
			_, err := svc.GetMachine(anonCtx(), &netbootV1.GetMachineRequest{Id: "m-1"})
			return err
		}},
		{"create as viewer", func() error {
			_, err := svc.CreateMachine(viewerCtx(), &netbootV1.CreateMachineRequest{Mac: "aa:bb:cc:dd:ee:ff", Name: "x"})
			return err
		}},
		{"update as viewer", func() error {
			_, err := svc.UpdateMachine(viewerCtx(), &netbootV1.UpdateMachineRequest{Id: "m-1"})
			return err
		}},
		{"delete as viewer", func() error {
			_, err := svc.DeleteMachine(viewerCtx(), &netbootV1.DeleteMachineRequest{Id: "m-1"})
			return err
		}},
		{"provision as viewer", func() error {
			_, err := svc.ProvisionMachine(viewerCtx(), &netbootV1.ProvisionMachineRequest{Id: "m-1"})
			return err
		}},
		{"cancel as viewer", func() error {
			_, err := svc.CancelProvision(viewerCtx(), &netbootV1.CancelProvisionRequest{Id: "m-1"})
			return err
		}},
		{"list unknown", func() error {
			_, err := svc.ListUnknownBoots(anonCtx(), &netbootV1.ListUnknownBootsRequest{})
			return err
		}},
		{"register unknown as viewer", func() error {
			_, err := svc.RegisterUnknownMachine(viewerCtx(), &netbootV1.RegisterUnknownMachineRequest{
				Mac: "aa:bb:cc:dd:ee:ff", Name: "x",
			})
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil {
				t.Fatal("call succeeded, want a permission denial")
			}
			if code := errors.Code(err); code != http.StatusForbidden {
				t.Errorf("error code = %d, want 403", code)
			}
		})
	}
}

// A viewer may still read, which proves the denials above are about the
// permission and not about a blanket rejection of non-admins.
func TestViewerMayListMachines(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodGet, "/api/v1/machines", http.StatusOK, map[string]any{
		"machines": []any{}, "meta": map[string]any{"total": "0"},
	})
	svc := newMachineServiceForTest(newTestClient(t, stub))

	if _, err := svc.ListMachines(viewerCtx(), &netbootV1.ListMachinesRequest{}); err != nil {
		t.Fatalf("ListMachines() as viewer error = %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

func containsSubstring(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
