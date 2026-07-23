package metrics

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	netbootV1 "github.com/go-tangra/go-tangra-netboot/gen/go/netboot/service/v1"
)

func testCollector() *Collector {
	return newCollector(log.NewHelper(log.NewStdLogger(discardWriter{})))
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// gaugeValue reads a gauge back out of the metric itself, so the assertion
// exercises the same path Prometheus scrapes.
func gaugeValue(t *testing.T, g prometheus.Gauge) float64 {
	t.Helper()
	var m dto.Metric
	if err := g.Write(&m); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	return m.GetGauge().GetValue()
}

func gaugeVecValue(t *testing.T, v *prometheus.GaugeVec, labels ...string) float64 {
	t.Helper()
	g, err := v.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues() error = %v", err)
	}
	return gaugeValue(t, g)
}

func counterVecValue(t *testing.T, v *prometheus.CounterVec, labels ...string) float64 {
	t.Helper()
	c, err := v.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues() error = %v", err)
	}
	var m dto.Metric
	if err := c.Write(&m); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	return m.GetCounter().GetValue()
}

func TestObserveStatsPopulatesGauges(t *testing.T) {
	c := testCollector()

	c.ObserveStats(&netbootV1.GetStatsResponse{
		TotalMachines:      100,
		InstallingMachines: 3,
		InstalledMachines:  90,
		FailedMachines:     7,
		TotalProfiles:      9,
		ActiveSessions:     2,
		UnknownBoots:       5,
		ActiveLeases:       31,
		DhcpEnabled:        true,
	})

	checks := []struct {
		name string
		got  float64
		want float64
	}{
		{"machines all", gaugeVecValue(t, c.MachinesByState, "all"), 100},
		{"machines installing", gaugeVecValue(t, c.MachinesByState, "installing"), 3},
		{"machines installed", gaugeVecValue(t, c.MachinesByState, "installed"), 90},
		{"machines failed", gaugeVecValue(t, c.MachinesByState, "failed"), 7},
		{"profiles", gaugeValue(t, c.ProfilesTotal), 9},
		{"sessions", gaugeValue(t, c.ActiveSessions), 2},
		{"unknown boots", gaugeValue(t, c.UnknownBoots), 5},
		{"leases", gaugeValue(t, c.ActiveLeases), 31},
		{"dhcp enabled", gaugeValue(t, c.DhcpEnabled), 1},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %v, want %v", check.name, check.got, check.want)
		}
	}
}

func TestUpstreamHealthAndFailures(t *testing.T) {
	c := testCollector()

	c.UpstreamHealthy(true)
	if got := gaugeValue(t, c.UpstreamUp); got != 1 {
		t.Errorf("upstreamUp = %v, want 1", got)
	}
	c.UpstreamHealthy(false)
	if got := gaugeValue(t, c.UpstreamUp); got != 0 {
		t.Errorf("upstreamUp = %v, want 0", got)
	}

	c.UpstreamFailure("list")
	c.UpstreamFailure("list")
	c.UpstreamFailure("get")
	if got := counterVecValue(t, c.UpstreamFailures, "list"); got != 2 {
		t.Errorf("upstreamFailures{list} = %v, want 2", got)
	}
	if got := counterVecValue(t, c.UpstreamFailures, "get"); got != 1 {
		t.Errorf("upstreamFailures{get} = %v, want 1", got)
	}
}

func TestOperationCounters(t *testing.T) {
	c := testCollector()

	tests := []struct {
		name      string
		call      func()
		operation string
	}{
		{"machine created", c.MachineCreated, "machine_created"},
		{"machine deleted", c.MachineDeleted, "machine_deleted"},
		{"provision requested", c.ProvisionRequested, "provision_requested"},
		{"provision cancelled", c.ProvisionCancelled, "provision_cancelled"},
		{"profile created", c.ProfileCreated, "profile_created"},
		{"profile deleted", c.ProfileDeleted, "profile_deleted"},
		{"artifact deleted", c.ArtifactDeleted, "artifact_deleted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.call()
			if got := counterVecValue(t, c.OperationsTotal, tt.operation); got != 1 {
				t.Errorf("operations{%s} = %v, want 1", tt.operation, got)
			}
		})
	}
}

// Toggling DHCP must move both the state gauge and the operation counter, so
// a dashboard and an alert rule see the same event.
func TestDhcpStateChanged(t *testing.T) {
	c := testCollector()

	c.DhcpStateChanged(true)
	if got := gaugeValue(t, c.DhcpEnabled); got != 1 {
		t.Errorf("dhcpEnabled = %v, want 1", got)
	}
	if got := counterVecValue(t, c.OperationsTotal, "dhcp_enabled"); got != 1 {
		t.Errorf("operations{dhcp_enabled} = %v, want 1", got)
	}

	c.DhcpStateChanged(false)
	if got := gaugeValue(t, c.DhcpEnabled); got != 0 {
		t.Errorf("dhcpEnabled = %v, want 0", got)
	}
	if got := counterVecValue(t, c.OperationsTotal, "dhcp_disabled"); got != 1 {
		t.Errorf("operations{dhcp_disabled} = %v, want 1", got)
	}
}

// Services accept a nil collector so a unit test need not stand up a
// Prometheus registry; every method must therefore tolerate a nil receiver.
func TestNilCollectorIsSafe(t *testing.T) {
	var c *Collector

	c.ObserveStats(&netbootV1.GetStatsResponse{TotalMachines: 1})
	c.UpstreamHealthy(true)
	c.UpstreamFailure("list")
	c.MachineCreated()
	c.MachineDeleted()
	c.ProvisionRequested()
	c.ProvisionCancelled()
	c.ProfileCreated()
	c.ProfileDeleted()
	c.ArtifactDeleted()
	c.DhcpStateChanged(true)
	c.Stop(context.Background())
}

func TestObserveStatsIgnoresNilPayload(t *testing.T) {
	c := testCollector()
	c.ObserveStats(nil)

	if got := gaugeValue(t, c.ProfilesTotal); got != 0 {
		t.Errorf("profilesTotal = %v, want it untouched", got)
	}
}

func TestStopWithoutAServerIsSafe(t *testing.T) {
	testCollector().Stop(context.Background())
}

func TestMiddlewareIsConstructed(t *testing.T) {
	if testCollector().Middleware() == nil {
		t.Error("Middleware() = nil, want a middleware")
	}
}

// Registering with a private registry proves the metric definitions are
// mutually consistent, without touching the global default registry.
func TestCollectorsRegisterCleanly(t *testing.T) {
	registry := prometheus.NewRegistry()
	for _, collector := range testCollector().collectors() {
		if err := registry.Register(collector); err != nil {
			t.Errorf("Register() error = %v", err)
		}
	}
}

func TestBoolToFloat(t *testing.T) {
	if got := boolToFloat(true); got != 1 {
		t.Errorf("boolToFloat(true) = %v, want 1", got)
	}
	if got := boolToFloat(false); got != 0 {
		t.Errorf("boolToFloat(false) = %v, want 0", got)
	}
}
