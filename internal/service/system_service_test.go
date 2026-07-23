package service

import (
	"net/http"
	"testing"

	"github.com/go-kratos/kratos/v2/errors"

	netbootV1 "github.com/go-tangra/go-tangra-netboot/gen/go/netboot/service/v1"
)

// statsRoutes registers every endpoint GetStats probes, each answering with
// the given total so a single call can assert the whole aggregation.
func statsRoutes(stub *stubUpstream, machineTotals map[string]string) {
	stub.on(http.MethodGet, "/api/v1/machines", func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		total, ok := machineTotals[state]
		if !ok {
			total = "0"
		}
		writeData(w, http.StatusOK, map[string]any{
			"machines": []any{}, "meta": map[string]any{"total": total},
		})
	})
	stub.reply(http.MethodGet, "/api/v1/profiles", http.StatusOK, map[string]any{
		"profiles": []any{}, "meta": map[string]any{"total": "9"},
	})
	stub.reply(http.MethodGet, "/api/v1/sessions", http.StatusOK, map[string]any{
		"sessions": []any{}, "meta": map[string]any{"total": "2"},
	})
	stub.reply(http.MethodGet, "/api/v1/machines/unknown", http.StatusOK, map[string]any{
		"boots": []any{}, "meta": map[string]any{"total": "5"},
	})
	stub.reply(http.MethodGet, "/api/v1/dhcp/leases", http.StatusOK, map[string]any{
		"leases": []any{}, "meta": map[string]any{"total": "31"},
	})
	stub.reply(http.MethodGet, "/api/v1/dhcp/config", http.StatusOK, dhcpConfigFixture(true))
}

func TestGetStatsAggregatesUpstreamTotals(t *testing.T) {
	stub := newStubUpstream(t)
	statsRoutes(stub, map[string]string{
		"":           "100",
		"installing": "3",
		"installed":  "90",
		"failed":     "7",
	})
	svc := newSystemServiceForTest(newTestClient(t, stub))

	stats, err := svc.GetStats(adminCtx(), &netbootV1.GetStatsRequest{})
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	checks := []struct {
		name string
		got  int64
		want int64
	}{
		{"totalMachines", stats.GetTotalMachines(), 100},
		{"installingMachines", stats.GetInstallingMachines(), 3},
		{"installedMachines", stats.GetInstalledMachines(), 90},
		{"failedMachines", stats.GetFailedMachines(), 7},
		{"totalProfiles", stats.GetTotalProfiles(), 9},
		{"activeSessions", stats.GetActiveSessions(), 2},
		{"unknownBoots", stats.GetUnknownBoots(), 5},
		{"activeLeases", stats.GetActiveLeases(), 31},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	if !stats.GetDhcpEnabled() {
		t.Error("dhcpEnabled = false, want true")
	}
}

// A dashboard that renders eight of nine counters is more useful than one
// that renders an error, so a single failing probe degrades to zero.
func TestGetStatsDegradesGracefully(t *testing.T) {
	stub := newStubUpstream(t)
	statsRoutes(stub, map[string]string{"": "100"})
	stub.failWith(http.MethodGet, "/api/v1/profiles", http.StatusInternalServerError, "", "boom")
	svc := newSystemServiceForTest(newTestClient(t, stub))

	stats, err := svc.GetStats(adminCtx(), &netbootV1.GetStatsRequest{})
	if err != nil {
		t.Fatalf("GetStats() error = %v, want a partial result rather than a failure", err)
	}
	if got := stats.GetTotalMachines(); got != 100 {
		t.Errorf("totalMachines = %d, want the healthy probe to still report 100", got)
	}
	if got := stats.GetTotalProfiles(); got != 0 {
		t.Errorf("totalProfiles = %d, want 0 for the failed probe", got)
	}
}

func TestGetStatsRequiresPermission(t *testing.T) {
	stub := newStubUpstream(t)
	svc := newSystemServiceForTest(newTestClient(t, stub))

	if _, err := svc.GetStats(anonCtx(), &netbootV1.GetStatsRequest{}); err == nil {
		t.Fatal("GetStats() as anonymous succeeded, want a denial")
	}
}

func TestHealthReportsHealthyUpstream(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodGet, "/api/v1/auth/me", http.StatusOK, map[string]any{
		"id": "op-1", "username": "operator", "active": true,
	})
	svc := newSystemServiceForTest(newTestClient(t, stub))

	// Health is unauthenticated on purpose: it is the platform's liveness
	// probe and discloses no configuration.
	resp, err := svc.Health(anonCtx(), nil)
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if resp.GetStatus() != netbootV1.HealthStatus_HEALTH_STATUS_HEALTHY {
		t.Errorf("status = %v, want HEALTHY", resp.GetStatus())
	}
	if got := resp.GetComponents()["netbootd"].GetStatus(); got != netbootV1.HealthStatus_HEALTH_STATUS_HEALTHY {
		t.Errorf("netbootd component status = %v, want HEALTHY", got)
	}
}

func TestHealthReportsUnreachableUpstream(t *testing.T) {
	stub := newStubUpstream(t)
	stub.failWith(http.MethodGet, "/api/v1/auth/me", http.StatusInternalServerError, "", "boom")
	svc := newSystemServiceForTest(newTestClient(t, stub))

	resp, err := svc.Health(anonCtx(), nil)
	if err != nil {
		t.Fatalf("Health() error = %v, want a healthy-shaped reply describing the fault", err)
	}
	if resp.GetStatus() != netbootV1.HealthStatus_HEALTH_STATUS_UNHEALTHY {
		t.Errorf("status = %v, want UNHEALTHY", resp.GetStatus())
	}
}

func TestHealthWithUnconfiguredUpstream(t *testing.T) {
	svc := newSystemServiceForTest(unconfiguredClient(t))

	resp, err := svc.Health(anonCtx(), nil)
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if resp.GetStatus() != netbootV1.HealthStatus_HEALTH_STATUS_UNHEALTHY {
		t.Errorf("status = %v, want UNHEALTHY", resp.GetStatus())
	}
	if got := resp.GetComponents()["netbootd"].GetMessage(); got != "upstream not configured" {
		t.Errorf("message = %q, want it to name the missing configuration", got)
	}
}

func TestCheckUpstreamReportsPosture(t *testing.T) {
	stub := newStubUpstream(t)
	stub.reply(http.MethodGet, "/api/v1/auth/me", http.StatusOK, map[string]any{
		"id": "op-1", "username": "operator", "active": true,
	})
	svc := newSystemServiceForTest(newTestClient(t, stub))

	resp, err := svc.CheckUpstream(adminCtx(), nil)
	if err != nil {
		t.Fatalf("CheckUpstream() error = %v", err)
	}
	if !resp.GetConnected() {
		t.Error("connected = false, want true")
	}
	if !resp.GetAuthenticated() {
		t.Error("authenticated = false, want true")
	}
	if resp.GetTls() {
		t.Error("tls = true for an httptest HTTP server, want false")
	}
	if resp.GetEndpoint() != stub.URL {
		t.Errorf("endpoint = %q, want %q", resp.GetEndpoint(), stub.URL)
	}
}

// The endpoint tells an attacker where the netboot control plane lives, so
// even this read is permission-gated.
func TestCheckUpstreamRequiresPermission(t *testing.T) {
	stub := newStubUpstream(t)
	svc := newSystemServiceForTest(newTestClient(t, stub))

	_, err := svc.CheckUpstream(anonCtx(), nil)
	if err == nil {
		t.Fatal("CheckUpstream() as anonymous succeeded, want a denial")
	}
	if code := errors.Code(err); code != http.StatusForbidden {
		t.Errorf("error code = %d, want 403", code)
	}
}

func TestCheckUpstreamWhenUnconfigured(t *testing.T) {
	svc := newSystemServiceForTest(unconfiguredClient(t))

	resp, err := svc.CheckUpstream(adminCtx(), nil)
	if err != nil {
		t.Fatalf("CheckUpstream() error = %v", err)
	}
	if resp.GetConnected() {
		t.Error("connected = true, want false")
	}
	if resp.GetMessage() != "upstream not configured" {
		t.Errorf("message = %q, want it to name the missing configuration", resp.GetMessage())
	}
}

// A rejected operator credential is an operator-actionable fault distinct
// from an unreachable host, so the message must distinguish them.
func TestCheckUpstreamDistinguishesBadCredentials(t *testing.T) {
	stub := newStubUpstream(t)
	stub.on(http.MethodPost, "/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success":false,"error":{"reason":"UNAUTHENTICATED","message":"nope"}}`))
	})
	svc := newSystemServiceForTest(newTestClient(t, stub))

	resp, err := svc.CheckUpstream(adminCtx(), nil)
	if err != nil {
		t.Fatalf("CheckUpstream() error = %v", err)
	}
	if resp.GetConnected() {
		t.Error("connected = true, want false")
	}
	if !containsSubstring(resp.GetMessage(), "credentials") {
		t.Errorf("message = %q, want it to name the credential problem", resp.GetMessage())
	}
}

func TestGetInfoReportsBuildMetadata(t *testing.T) {
	svc := newSystemServiceForTest(unconfiguredClient(t))

	info, err := svc.GetInfo(adminCtx(), nil)
	if err != nil {
		t.Fatalf("GetInfo() error = %v", err)
	}
	if info.GetVersion() == "" {
		t.Error("version is empty")
	}
	if info.GetGoVersion() == "" {
		t.Error("goVersion is empty")
	}
}

// An unconfigured module must say so explicitly rather than reporting the
// upstream as merely unreachable, since the remedy is different.
func TestUnconfiguredUpstreamYieldsAConfigurationError(t *testing.T) {
	svc := newMachineServiceForTest(unconfiguredClient(t))

	_, err := svc.ListMachines(adminCtx(), &netbootV1.ListMachinesRequest{})
	if err == nil {
		t.Fatal("ListMachines() = nil error, want a configuration error")
	}
	if reason := errors.Reason(err); reason != "CONFIGURATION_ERROR" {
		t.Errorf("error reason = %q, want CONFIGURATION_ERROR", reason)
	}
}
