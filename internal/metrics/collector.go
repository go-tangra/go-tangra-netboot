// Package metrics owns the module's Prometheus surface.
package metrics

import (
	"context"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	commonMetrics "github.com/go-tangra/go-tangra-common/metrics"

	netbootV1 "github.com/go-tangra/go-tangra-netboot/gen/go/netboot/service/v1"
)

const (
	namespace = "tangra"
	subsystem = "netboot"

	defaultMetricsAddr = ":10010"
)

// Collector holds every Prometheus metric the netboot module exports.
//
// Because the module is stateless, the gauges are snapshots refreshed on each
// GetStats call rather than counters maintained by this process; the counters
// below track only what this process itself did.
type Collector struct {
	log    *log.Helper
	server *commonMetrics.MetricsServer

	// Snapshot gauges, refreshed from the upstream on ObserveStats.
	MachinesByState *prometheus.GaugeVec
	ProfilesTotal   prometheus.Gauge
	ActiveSessions  prometheus.Gauge
	UnknownBoots    prometheus.Gauge
	ActiveLeases    prometheus.Gauge
	DhcpEnabled     prometheus.Gauge

	// Counters for operations this module performed.
	OperationsTotal *prometheus.CounterVec

	// UpstreamFailures counts failed upstream calls by logical operation.
	UpstreamFailures *prometheus.CounterVec

	// UpstreamUp is 1 when the last health probe succeeded.
	UpstreamUp prometheus.Gauge

	// gRPC request metrics.
	RequestDuration *prometheus.HistogramVec
	RequestsTotal   *prometheus.CounterVec
}

// NewCollector builds the collector, registers its metrics with the default
// Prometheus registry, and starts the metrics HTTP server.
func NewCollector(ctx *bootstrap.Context) *Collector {
	c := newCollector(ctx.NewLoggerHelper("netboot/metrics"))

	prometheus.MustRegister(c.collectors()...)

	addr := os.Getenv("METRICS_ADDR")
	if addr == "" {
		addr = defaultMetricsAddr
	}
	c.server = commonMetrics.NewMetricsServer(addr, nil, ctx.GetLogger())

	go func() {
		if err := c.server.Start(); err != nil {
			c.log.Errorf("Metrics server failed: %v", err)
		}
	}()

	return c
}

// newCollector constructs the metrics without registering them or starting a
// server. Splitting construction from registration keeps the global registry
// and a listening socket out of unit tests.
func newCollector(logger *log.Helper) *Collector {
	c := &Collector{
		log: logger,

		MachinesByState: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: subsystem,
			Name: "machines_by_state",
			Help: "Number of netboot machines by provisioning state.",
		}, []string{"state"}),

		ProfilesTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: subsystem,
			Name: "profiles_total",
			Help: "Total number of installation profiles.",
		}),

		ActiveSessions: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: subsystem,
			Name: "active_sessions",
			Help: "Number of active provisioning sessions.",
		}),

		UnknownBoots: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: subsystem,
			Name: "unknown_boots",
			Help: "Number of unregistered MACs that attempted to boot.",
		}),

		ActiveLeases: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: subsystem,
			Name: "active_leases",
			Help: "Number of active DHCP leases.",
		}),

		DhcpEnabled: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: subsystem,
			Name: "dhcp_enabled",
			Help: "1 when the upstream authoritative DHCP server is enabled.",
		}),

		OperationsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: subsystem,
			Name: "operations_total",
			Help: "Netboot operations performed through this module.",
		}, []string{"operation"}),

		UpstreamFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: subsystem,
			Name: "upstream_failures_total",
			Help: "Failed calls to the upstream netbootd, by operation.",
		}, []string{"operation"}),

		UpstreamUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: subsystem,
			Name: "upstream_up",
			Help: "1 when the last upstream health probe succeeded.",
		}),

		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Subsystem: subsystem,
			Name:    "grpc_request_duration_seconds",
			Help:    "Histogram of gRPC request durations in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method"}),

		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: subsystem,
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests by method and status.",
		}, []string{"method", "status"}),
	}

	return c
}

// collectors lists every metric this module exports.
func (c *Collector) collectors() []prometheus.Collector {
	return []prometheus.Collector{
		c.MachinesByState,
		c.ProfilesTotal,
		c.ActiveSessions,
		c.UnknownBoots,
		c.ActiveLeases,
		c.DhcpEnabled,
		c.OperationsTotal,
		c.UpstreamFailures,
		c.UpstreamUp,
		c.RequestDuration,
		c.RequestsTotal,
	}
}

// Stop shuts down the metrics HTTP server.
func (c *Collector) Stop(ctx context.Context) {
	if c == nil || c.server == nil {
		return
	}
	c.server.Stop(ctx)
}

// Middleware returns the Kratos middleware that records request metrics.
func (c *Collector) Middleware() middleware.Middleware {
	return commonMetrics.NewServerMiddleware(c.RequestDuration, c.RequestsTotal)
}

// ObserveStats refreshes the snapshot gauges from a stats reply. A nil
// Collector or reply is a no-op so tests can construct services without a
// metrics server.
func (c *Collector) ObserveStats(stats *netbootV1.GetStatsResponse) {
	if c == nil || stats == nil {
		return
	}
	c.MachinesByState.WithLabelValues("installing").Set(float64(stats.GetInstallingMachines()))
	c.MachinesByState.WithLabelValues("installed").Set(float64(stats.GetInstalledMachines()))
	c.MachinesByState.WithLabelValues("failed").Set(float64(stats.GetFailedMachines()))
	c.MachinesByState.WithLabelValues("all").Set(float64(stats.GetTotalMachines()))
	c.ProfilesTotal.Set(float64(stats.GetTotalProfiles()))
	c.ActiveSessions.Set(float64(stats.GetActiveSessions()))
	c.UnknownBoots.Set(float64(stats.GetUnknownBoots()))
	c.ActiveLeases.Set(float64(stats.GetActiveLeases()))
	c.DhcpEnabled.Set(boolToFloat(stats.GetDhcpEnabled()))
}

// UpstreamHealthy records the outcome of a health probe.
func (c *Collector) UpstreamHealthy(healthy bool) {
	if c == nil {
		return
	}
	c.UpstreamUp.Set(boolToFloat(healthy))
}

// UpstreamFailure counts a failed upstream call.
func (c *Collector) UpstreamFailure(operation string) {
	if c == nil {
		return
	}
	c.UpstreamFailures.WithLabelValues(operation).Inc()
}

func (c *Collector) operation(name string) {
	if c == nil {
		return
	}
	c.OperationsTotal.WithLabelValues(name).Inc()
}

// MachineCreated counts a machine registration.
func (c *Collector) MachineCreated() { c.operation("machine_created") }

// MachineDeleted counts a machine removal.
func (c *Collector) MachineDeleted() { c.operation("machine_deleted") }

// ProvisionRequested counts a machine being armed for provisioning.
func (c *Collector) ProvisionRequested() { c.operation("provision_requested") }

// ProvisionCancelled counts a provisioning cancellation.
func (c *Collector) ProvisionCancelled() { c.operation("provision_cancelled") }

// ProfileCreated counts a profile creation or clone.
func (c *Collector) ProfileCreated() { c.operation("profile_created") }

// ProfileDeleted counts a profile removal.
func (c *Collector) ProfileDeleted() { c.operation("profile_deleted") }

// ArtifactDeleted counts a boot-artifact removal.
func (c *Collector) ArtifactDeleted() { c.operation("artifact_deleted") }

// DhcpStateChanged records an enable/disable of the upstream DHCP server.
func (c *Collector) DhcpStateChanged(enabled bool) {
	if c == nil {
		return
	}
	c.DhcpEnabled.Set(boolToFloat(enabled))
	if enabled {
		c.operation("dhcp_enabled")
		return
	}
	c.operation("dhcp_disabled")
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
