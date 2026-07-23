package service

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/go-kratos/kratos/v2/errors"

	netbootV1 "github.com/go-tangra/go-tangra-netboot/gen/go/netboot/service/v1"
)

func dhcpConfigFixture(enabled bool) map[string]any {
	return map[string]any{
		"enabled":           enabled,
		"version":           4,
		"lease_ttl_seconds": 3600,
		"subnets": []any{map[string]any{
			"id": "sn-1", "network": "10.0.0.0/24",
			"range_start": "10.0.0.100", "range_end": "10.0.0.200",
			"gateway": "10.0.0.1", "dns": []string{"10.0.0.53"},
		}},
		"updated_at": "2026-01-02T03:04:05Z",
	}
}

func TestGetDhcpConfig(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodGet, "/api/v1/dhcp/config", http.StatusOK, dhcpConfigFixture(true))
	svc := newDhcpServiceForTest(newTestClient(t, stub))

	resp, err := svc.GetDhcpConfig(viewerCtx(), &netbootV1.GetDhcpConfigRequest{})
	if err != nil {
		t.Fatalf("GetDhcpConfig() error = %v", err)
	}
	cfg := resp.GetConfig()
	if !cfg.GetEnabled() {
		t.Error("enabled = false, want true")
	}
	if got := cfg.GetLeaseTtlSeconds(); got != 3600 {
		t.Errorf("leaseTtlSeconds = %d, want 3600", got)
	}
	if got := len(cfg.GetSubnets()); got != 1 {
		t.Fatalf("len(subnets) = %d, want 1", got)
	}
	if got := cfg.GetSubnets()[0].GetNetwork(); got != "10.0.0.0/24" {
		t.Errorf("subnet network = %q, want 10.0.0.0/24", got)
	}
}

func TestUpdateDhcpConfigRoundTripsSubnets(t *testing.T) {
	stub := newStubUpstream(t)
	var body map[string]any
	stub.on(http.MethodPut, "/api/v1/dhcp/config", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		writeData(w, http.StatusOK, dhcpConfigFixture(true))
	})
	svc := newDhcpServiceForTest(newTestClient(t, stub))

	_, err := svc.UpdateDhcpConfig(adminCtx(), &netbootV1.UpdateDhcpConfigRequest{
		LeaseTtlSeconds: 7200,
		Subnets: []*netbootV1.DhcpSubnet{{
			Network: "192.168.0.0/24", RangeStart: "192.168.0.10",
			RangeEnd: "192.168.0.99", Gateway: "192.168.0.1",
			Dns: []string{"192.168.0.53"},
		}},
	})
	if err != nil {
		t.Fatalf("UpdateDhcpConfig() error = %v", err)
	}
	if got := body["lease_ttl_seconds"]; got != float64(7200) {
		t.Errorf("lease_ttl_seconds = %v, want 7200", got)
	}
	subnets, _ := body["subnets"].([]any)
	if len(subnets) != 1 {
		t.Fatalf("subnets = %v, want one entry", body["subnets"])
	}
	if got := subnets[0].(map[string]any)["network"]; got != "192.168.0.0/24" {
		t.Errorf("subnet network = %v, want 192.168.0.0/24", got)
	}
}

func TestEnableAndDisableDhcp(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodPost, "/api/v1/dhcp/enable", http.StatusOK, dhcpConfigFixture(true))
	stub.reply(http.MethodPost, "/api/v1/dhcp/disable", http.StatusOK, dhcpConfigFixture(false))
	svc := newDhcpServiceForTest(newTestClient(t, stub))

	enabled, err := svc.EnableDhcp(adminCtx(), &netbootV1.EnableDhcpRequest{})
	if err != nil {
		t.Fatalf("EnableDhcp() error = %v", err)
	}
	if !enabled.GetConfig().GetEnabled() {
		t.Error("enabled = false after EnableDhcp(), want true")
	}

	disabled, err := svc.DisableDhcp(adminCtx(), &netbootV1.DisableDhcpRequest{})
	if err != nil {
		t.Fatalf("DisableDhcp() error = %v", err)
	}
	if disabled.GetConfig().GetEnabled() {
		t.Error("enabled = true after DisableDhcp(), want false")
	}
}

// Enabling an authoritative DHCP server reshapes a whole network segment, so
// it is reserved for the manage permission rather than any authenticated user.
func TestDhcpMutationsRequireManagePermission(t *testing.T) {
	stub := newStubUpstream(t)
	svc := newDhcpServiceForTest(newTestClient(t, stub))

	tests := []struct {
		name string
		call func() error
	}{
		{"update as operator", func() error {
			_, err := svc.UpdateDhcpConfig(ctxWithRoles("netboot.operator"),
				&netbootV1.UpdateDhcpConfigRequest{LeaseTtlSeconds: 3600})
			return err
		}},
		{"enable as operator", func() error {
			_, err := svc.EnableDhcp(ctxWithRoles("netboot.operator"), &netbootV1.EnableDhcpRequest{})
			return err
		}},
		{"disable as viewer", func() error {
			_, err := svc.DisableDhcp(viewerCtx(), &netbootV1.DisableDhcpRequest{})
			return err
		}},
		{"read as anonymous", func() error {
			_, err := svc.GetDhcpConfig(anonCtx(), &netbootV1.GetDhcpConfigRequest{})
			return err
		}},
		{"leases as anonymous", func() error {
			_, err := svc.ListLeases(anonCtx(), &netbootV1.ListLeasesRequest{})
			return err
		}},
		{"conflicts as anonymous", func() error {
			_, err := svc.ListForeignServers(anonCtx(), &netbootV1.ListForeignServersRequest{})
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

func TestListLeasesAndConflicts(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodGet, "/api/v1/dhcp/leases", http.StatusOK, map[string]any{
		"leases": []any{map[string]any{
			"ip": "10.0.0.101", "mac": "aa:bb:cc:dd:ee:ff",
			"machine_id": "m-1", "machine_name": "worker-01",
			"expires_at": "2026-01-02T04:04:05Z",
		}},
		"meta": map[string]any{"total": "1"},
	})
	stub.reply(http.MethodGet, "/api/v1/dhcp/conflicts", http.StatusOK, map[string]any{
		"servers": []any{map[string]any{
			"server_id": "192.168.1.1", "last_seen": "2026-01-02T03:04:05Z", "offers_seen": "17",
		}},
		"meta": map[string]any{"total": "1"},
	})
	svc := newDhcpServiceForTest(newTestClient(t, stub))

	leases, err := svc.ListLeases(viewerCtx(), &netbootV1.ListLeasesRequest{})
	if err != nil {
		t.Fatalf("ListLeases() error = %v", err)
	}
	if got := leases.GetLeases()[0].GetIp(); got != "10.0.0.101" {
		t.Errorf("lease ip = %q, want 10.0.0.101", got)
	}

	conflicts, err := svc.ListForeignServers(viewerCtx(), &netbootV1.ListForeignServersRequest{})
	if err != nil {
		t.Fatalf("ListForeignServers() error = %v", err)
	}
	if got := conflicts.GetServers()[0].GetOffersSeen(); got != 17 {
		t.Errorf("offersSeen = %d, want 17", got)
	}
}

// netbootd answers 412 when provisioning is attempted with DHCP off; that
// distinct precondition must survive translation rather than collapsing into
// a generic error.
func TestDhcpDisabledPreconditionIsPreserved(t *testing.T) {
	stub := newStubUpstream(t)
	stub.failWith(http.MethodPost, "/api/v1/machines/m-1/provision",
		http.StatusPreconditionFailed, "DHCP_DISABLED", "DHCP service is disabled")
	svc := newMachineServiceForTest(newTestClient(t, stub))

	_, err := svc.ProvisionMachine(adminCtx(), &netbootV1.ProvisionMachineRequest{Id: "m-1"})
	if err == nil {
		t.Fatal("ProvisionMachine() = nil error, want a precondition failure")
	}
	if code := errors.Code(err); code != http.StatusPreconditionFailed {
		t.Errorf("error code = %d, want 412", code)
	}
	if reason := errors.Reason(err); reason != "DHCP_DISABLED" {
		t.Errorf("error reason = %q, want DHCP_DISABLED", reason)
	}
}

func TestListAndGetSession(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodGet, "/api/v1/sessions", http.StatusOK, map[string]any{
		"sessions": []any{map[string]any{
			"id": "s-1", "machine_id": "m-1", "machine_name": "worker-01",
			"machine_mac": "aa:bb:cc:dd:ee:ff", "profile_id": "p-1",
			"profile_version": 3, "state": "failed",
			"started_at": "2026-01-02T03:04:05Z", "ended_at": "2026-01-02T03:40:05Z",
			"failure_phase": "seed_fetch",
		}},
		"meta": map[string]any{"total": "1"},
	})
	stub.reply(http.MethodGet, "/api/v1/sessions/s-1", http.StatusOK, map[string]any{
		"session": map[string]any{"id": "s-1", "state": "active"},
		"timeline": []any{map[string]any{
			"time": "2026-01-02T03:04:05Z", "session_id": "s-1",
			"machine_mac": "aa:bb:cc:dd:ee:ff", "phase": "dhcp_offer",
			"outcome": "ok", "detail": map[string]any{"ip": "10.0.0.101"},
		}},
		"evidence": map[string]any{"checksum": "abc"},
	})
	svc := newSessionServiceForTest(newTestClient(t, stub))

	listed, err := svc.ListSessions(viewerCtx(), &netbootV1.ListSessionsRequest{})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if got := listed.GetSessions()[0].GetState(); got != netbootV1.SessionState_SESSION_STATE_FAILED {
		t.Errorf("state = %v, want FAILED", got)
	}
	if got := listed.GetSessions()[0].GetFailurePhase(); got != "seed_fetch" {
		t.Errorf("failurePhase = %q, want seed_fetch", got)
	}

	detail, err := svc.GetSession(viewerCtx(), &netbootV1.GetSessionRequest{Id: "s-1"})
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if got := detail.GetSession().GetState(); got != netbootV1.SessionState_SESSION_STATE_ACTIVE {
		t.Errorf("state = %v, want ACTIVE", got)
	}
	if got := len(detail.GetTimeline()); got != 1 {
		t.Fatalf("len(timeline) = %d, want 1", got)
	}
	if got := detail.GetTimeline()[0].GetOutcome(); got != netbootV1.EventOutcome_EVENT_OUTCOME_OK {
		t.Errorf("outcome = %v, want OK", got)
	}
	// The opaque detail and evidence blobs pass through as raw JSON text.
	if got := detail.GetTimeline()[0].GetDetail(); !containsSubstring(got, "10.0.0.101") {
		t.Errorf("detail = %q, want the upstream JSON preserved", got)
	}
	if got := detail.GetEvidence(); !containsSubstring(got, "abc") {
		t.Errorf("evidence = %q, want the upstream JSON preserved", got)
	}
}

func TestSessionStateFilterIsTranslated(t *testing.T) {
	stub := newStubUpstream(t)
	var gotQuery string
	stub.on(http.MethodGet, "/api/v1/sessions", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeData(w, http.StatusOK, map[string]any{"sessions": []any{}, "meta": map[string]any{"total": "0"}})
	})
	svc := newSessionServiceForTest(newTestClient(t, stub))

	state := netbootV1.SessionState_SESSION_STATE_ACTIVE
	machineID := "m-1"
	if _, err := svc.ListSessions(viewerCtx(), &netbootV1.ListSessionsRequest{
		State: &state, MachineId: &machineID,
	}); err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	for _, want := range []string{"state=active", "machine_id=m-1"} {
		if !containsSubstring(gotQuery, want) {
			t.Errorf("upstream query = %q, want it to contain %q", gotQuery, want)
		}
	}
}

func TestSessionRPCsEnforcePermissions(t *testing.T) {
	stub := newStubUpstream(t)
	svc := newSessionServiceForTest(newTestClient(t, stub))

	if _, err := svc.ListSessions(anonCtx(), &netbootV1.ListSessionsRequest{}); err == nil {
		t.Error("ListSessions() as anonymous succeeded, want a denial")
	}
	if _, err := svc.GetSession(anonCtx(), &netbootV1.GetSessionRequest{Id: "s-1"}); err == nil {
		t.Error("GetSession() as anonymous succeeded, want a denial")
	}
}

func TestArtifactsAndTransfers(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodGet, "/api/v1/artifacts", http.StatusOK, map[string]any{
		"artifacts": []any{map[string]any{
			"id": "a-1", "kind": "kernel", "ubuntu_release": "noble",
			"filename": "vmlinuz", "size_bytes": "13631488",
			"sha256": "deadbeef", "uploaded_by": "operator",
			"created_at": "2026-01-02T03:04:05Z", "updated_at": "2026-01-02T03:04:05Z",
		}},
		"meta": map[string]any{"total": "1"},
	})
	stub.reply(http.MethodGet, "/api/v1/artifacts/a-1", http.StatusOK, map[string]any{
		"id": "a-1", "kind": "initrd", "filename": "initrd.img", "size_bytes": "9999",
	})
	stub.reply(http.MethodDelete, "/api/v1/artifacts/a-1", http.StatusOK, nil)
	stub.reply(http.MethodGet, "/api/v1/artifacts/transfers", http.StatusOK, map[string]any{
		"transfers": []any{map[string]any{
			"time": "2026-01-02T03:04:05Z", "client_ip": "10.0.0.101",
			"filename": "vmlinuz", "bytes_sent": "13631488",
			"success": true, "protocol": "tftp",
		}},
		"meta": map[string]any{"total": "1"},
	})
	svc := newArtifactServiceForTest(newTestClient(t, stub))

	listed, err := svc.ListArtifacts(viewerCtx(), &netbootV1.ListArtifactsRequest{})
	if err != nil {
		t.Fatalf("ListArtifacts() error = %v", err)
	}
	a := listed.GetArtifacts()[0]
	if a.GetKind() != netbootV1.ArtifactKind_ARTIFACT_KIND_KERNEL {
		t.Errorf("kind = %v, want KERNEL", a.GetKind())
	}
	if a.GetSizeBytes() != 13631488 {
		t.Errorf("sizeBytes = %d, want 13631488", a.GetSizeBytes())
	}

	got, err := svc.GetArtifact(viewerCtx(), &netbootV1.GetArtifactRequest{Id: "a-1"})
	if err != nil {
		t.Fatalf("GetArtifact() error = %v", err)
	}
	if got.GetArtifact().GetKind() != netbootV1.ArtifactKind_ARTIFACT_KIND_INITRD {
		t.Errorf("kind = %v, want INITRD", got.GetArtifact().GetKind())
	}

	transfers, err := svc.ListTransfers(viewerCtx(), &netbootV1.ListTransfersRequest{})
	if err != nil {
		t.Fatalf("ListTransfers() error = %v", err)
	}
	tr := transfers.GetTransfers()[0]
	if tr.GetProtocol() != netbootV1.TransferProtocol_TRANSFER_PROTOCOL_TFTP {
		t.Errorf("protocol = %v, want TFTP", tr.GetProtocol())
	}
	if !tr.GetSuccess() {
		t.Error("success = false, want true")
	}

	if _, err := svc.DeleteArtifact(adminCtx(), &netbootV1.DeleteArtifactRequest{Id: "a-1"}); err != nil {
		t.Fatalf("DeleteArtifact() error = %v", err)
	}
}

// Deleting a kernel breaks every profile that references it, so it is an
// admin-only operation.
func TestDeleteArtifactRequiresAdmin(t *testing.T) {
	stub := newStubUpstream(t)
	svc := newArtifactServiceForTest(newTestClient(t, stub))

	for _, ctxName := range []string{"netboot.operator", "netboot.viewer"} {
		t.Run(ctxName, func(t *testing.T) {
			_, err := svc.DeleteArtifact(ctxWithRoles(ctxName), &netbootV1.DeleteArtifactRequest{Id: "a-1"})
			if err == nil {
				t.Fatal("DeleteArtifact() succeeded, want a denial")
			}
			if code := errors.Code(err); code != http.StatusForbidden {
				t.Errorf("error code = %d, want 403", code)
			}
		})
	}
}

func TestArtifactReadsRequireAPermission(t *testing.T) {
	stub := newStubUpstream(t)
	svc := newArtifactServiceForTest(newTestClient(t, stub))

	if _, err := svc.ListArtifacts(anonCtx(), &netbootV1.ListArtifactsRequest{}); err == nil {
		t.Error("ListArtifacts() as anonymous succeeded, want a denial")
	}
	if _, err := svc.GetArtifact(anonCtx(), &netbootV1.GetArtifactRequest{Id: "a-1"}); err == nil {
		t.Error("GetArtifact() as anonymous succeeded, want a denial")
	}
	if _, err := svc.ListTransfers(anonCtx(), &netbootV1.ListTransfersRequest{}); err == nil {
		t.Error("ListTransfers() as anonymous succeeded, want a denial")
	}
}

// The upstream-failure path of each remaining service must translate rather
// than leak, and must not be reachable only through the machine service.
func TestUpstreamFailuresAreTranslatedPerService(t *testing.T) {
	tests := []struct {
		name       string
		register   func(*stubUpstream)
		call       func(*stubUpstream) error
		wantReason string
	}{
		{
			name: "dhcp config",
			register: func(s *stubUpstream) {
				s.failWith(http.MethodGet, "/api/v1/dhcp/config",
					http.StatusInternalServerError, "", "db down")
			},
			call: func(s *stubUpstream) error {
				_, err := newDhcpServiceForTest(newTestClient(t, s)).
					GetDhcpConfig(viewerCtx(), &netbootV1.GetDhcpConfigRequest{})
				return err
			},
			wantReason: "UPSTREAM_ERROR",
		},
		{
			name: "session lookup",
			register: func(s *stubUpstream) {
				s.failWith(http.MethodGet, "/api/v1/sessions/s-404",
					http.StatusNotFound, "NOT_FOUND", "session not found")
			},
			call: func(s *stubUpstream) error {
				_, err := newSessionServiceForTest(newTestClient(t, s)).
					GetSession(viewerCtx(), &netbootV1.GetSessionRequest{Id: "s-404"})
				return err
			},
			wantReason: "SESSION_NOT_FOUND",
		},
		{
			name: "artifact lookup",
			register: func(s *stubUpstream) {
				s.failWith(http.MethodGet, "/api/v1/artifacts/a-404",
					http.StatusNotFound, "NOT_FOUND", "artifact not found")
			},
			call: func(s *stubUpstream) error {
				_, err := newArtifactServiceForTest(newTestClient(t, s)).
					GetArtifact(viewerCtx(), &netbootV1.GetArtifactRequest{Id: "a-404"})
				return err
			},
			wantReason: "ARTIFACT_NOT_FOUND",
		},
		{
			name: "artifact deletion",
			register: func(s *stubUpstream) {
				s.failWith(http.MethodDelete, "/api/v1/artifacts/a-1",
					http.StatusConflict, "CONFLICT", "still referenced by a profile")
			},
			call: func(s *stubUpstream) error {
				_, err := newArtifactServiceForTest(newTestClient(t, s)).
					DeleteArtifact(adminCtx(), &netbootV1.DeleteArtifactRequest{Id: "a-1"})
				return err
			},
			wantReason: "CONFLICT",
		},
		{
			name: "session listing",
			register: func(s *stubUpstream) {
				s.failWith(http.MethodGet, "/api/v1/sessions",
					http.StatusServiceUnavailable, "", "restarting")
			},
			call: func(s *stubUpstream) error {
				_, err := newSessionServiceForTest(newTestClient(t, s)).
					ListSessions(viewerCtx(), &netbootV1.ListSessionsRequest{})
				return err
			},
			wantReason: "UPSTREAM_UNAVAILABLE",
		},
		{
			name: "transfer listing",
			register: func(s *stubUpstream) {
				s.failWith(http.MethodGet, "/api/v1/artifacts/transfers",
					http.StatusInternalServerError, "", "boom")
			},
			call: func(s *stubUpstream) error {
				_, err := newArtifactServiceForTest(newTestClient(t, s)).
					ListTransfers(viewerCtx(), &netbootV1.ListTransfersRequest{})
				return err
			},
			wantReason: "UPSTREAM_ERROR",
		},
		{
			name: "lease listing",
			register: func(s *stubUpstream) {
				s.failWith(http.MethodGet, "/api/v1/dhcp/leases",
					http.StatusInternalServerError, "", "boom")
			},
			call: func(s *stubUpstream) error {
				_, err := newDhcpServiceForTest(newTestClient(t, s)).
					ListLeases(viewerCtx(), &netbootV1.ListLeasesRequest{})
				return err
			},
			wantReason: "UPSTREAM_ERROR",
		},
		{
			name: "profile preview",
			register: func(s *stubUpstream) {
				s.failWith(http.MethodPost, "/api/v1/profiles/p-1/preview",
					http.StatusUnprocessableEntity, "VALIDATION_FAILED", "template does not render")
			},
			call: func(s *stubUpstream) error {
				_, err := newProfileServiceForTest(newTestClient(t, s)).
					PreviewProfile(adminCtx(), &netbootV1.PreviewProfileRequest{Id: "p-1"})
				return err
			},
			wantReason: "VALIDATION_FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := newStubUpstream(t)
			tt.register(stub)

			err := tt.call(stub)
			if err == nil {
				t.Fatal("call succeeded, want the upstream failure to surface")
			}
			if reason := errors.Reason(err); reason != tt.wantReason {
				t.Errorf("error reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}
