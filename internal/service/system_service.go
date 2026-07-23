package service

import (
	"context"
	"runtime"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	netbootV1 "github.com/go-tangra/go-tangra-netboot/gen/go/netboot/service/v1"
	"github.com/go-tangra/go-tangra-netboot/internal/authz"
	"github.com/go-tangra/go-tangra-netboot/internal/metrics"
	"github.com/go-tangra/go-tangra-netboot/internal/netbootd"
)

// Build metadata, injected via -ldflags at build time.
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// statsPageSize is the window used when tallying dashboard counters. The
// upstream returns a total alongside every page, so one small page is enough
// to read a count without pulling the whole table across the network.
const statsPageSize = 1

// SystemService reports module health, build info and dashboard counters.
type SystemService struct {
	netbootV1.UnimplementedNetbootSystemServiceServer

	log       *log.Helper
	client    *netbootd.Client
	checker   *authz.Checker
	collector *metrics.Collector
}

// NewSystemService wires the system service.
func NewSystemService(
	ctx *bootstrap.Context,
	client *netbootd.Client,
	checker *authz.Checker,
	collector *metrics.Collector,
) *SystemService {
	return &SystemService{
		log:       ctx.NewLoggerHelper("netboot/service/system"),
		client:    client,
		checker:   checker,
		collector: collector,
	}
}

// Health reports the module's own health plus the reachability of the
// upstream. It is intentionally unauthenticated: it is the liveness signal
// the platform's orchestrator polls, and it discloses no configuration.
func (s *SystemService) Health(ctx context.Context, _ *emptypb.Empty) (*netbootV1.HealthResponse, error) {
	upstream := &netbootV1.ComponentHealth{
		Status:  netbootV1.HealthStatus_HEALTH_STATUS_HEALTHY,
		Message: "connected",
	}

	switch {
	case !s.client.Configured():
		upstream.Status = netbootV1.HealthStatus_HEALTH_STATUS_UNHEALTHY
		upstream.Message = "upstream not configured"
	default:
		if _, err := s.client.Ping(ctx); err != nil {
			s.log.WithContext(ctx).Warnf("upstream health probe failed: %v", err)
			upstream.Status = netbootV1.HealthStatus_HEALTH_STATUS_UNHEALTHY
			upstream.Message = "upstream unreachable"
		}
	}

	status := netbootV1.HealthStatus_HEALTH_STATUS_HEALTHY
	message := "all systems operational"
	if upstream.Status != netbootV1.HealthStatus_HEALTH_STATUS_HEALTHY {
		status = upstream.Status
		message = "the netboot upstream is not healthy"
	}
	s.collector.UpstreamHealthy(upstream.Status == netbootV1.HealthStatus_HEALTH_STATUS_HEALTHY)

	return &netbootV1.HealthResponse{
		Status:     status,
		Message:    message,
		Components: map[string]*netbootV1.ComponentHealth{"netbootd": upstream},
	}, nil
}

// GetInfo returns build metadata for this module.
func (s *SystemService) GetInfo(_ context.Context, _ *emptypb.Empty) (*netbootV1.GetInfoResponse, error) {
	return &netbootV1.GetInfoResponse{
		Version:   Version,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
		GitCommit: GitCommit,
	}, nil
}

// CheckUpstream probes the configured netbootd and reports the connection's
// posture. The endpoint it echoes is credential-free, and the call requires
// a system-view permission because knowing where the netboot control plane
// lives is itself useful to an attacker.
func (s *SystemService) CheckUpstream(
	ctx context.Context, _ *emptypb.Empty,
) (*netbootV1.CheckUpstreamResponse, error) {
	if err := s.checker.Require(ctx, authz.PermSystemView); err != nil {
		return nil, err
	}

	resp := &netbootV1.CheckUpstreamResponse{
		Endpoint: s.client.Endpoint(),
		Tls:      s.client.IsTLS(),
	}

	if !s.client.Configured() {
		resp.Message = "upstream not configured"
		return resp, nil
	}

	start := time.Now()
	operator, err := s.client.Ping(ctx)
	resp.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		s.log.WithContext(ctx).Warnf("upstream check failed: %v", err)
		resp.Message = upstreamCheckMessage(err)
		return resp, nil
	}

	resp.Connected = true
	resp.Authenticated = s.client.HasSession()
	resp.Message = "connected as " + operator.Username
	return resp, nil
}

// upstreamCheckMessage renders a diagnostic that distinguishes the failure
// classes an operator can act on, without echoing upstream internals.
func upstreamCheckMessage(err error) string {
	if apiErr, ok := netbootd.AsAPIError(err); ok {
		if apiErr.IsUnauthenticated() {
			return "the netboot server rejected this module's operator credentials"
		}
		return "the netboot server returned an error"
	}
	return "the netboot server is unreachable"
}

// GetStats aggregates the counters behind the dashboard widgets.
//
// Each counter is a one-row page whose meta.total carries the number we
// actually want. A failure of any single probe degrades that counter to zero
// rather than failing the whole dashboard, since a half-populated dashboard
// is more useful to an operator than an error card.
func (s *SystemService) GetStats(
	ctx context.Context, _ *netbootV1.GetStatsRequest,
) (*netbootV1.GetStatsResponse, error) {
	if err := s.checker.Require(ctx, authz.PermSystemView); err != nil {
		return nil, err
	}

	page := netbootd.Page{Page: 1, PageSize: statsPageSize}
	resp := &netbootV1.GetStatsResponse{}

	resp.TotalMachines = s.countMachines(ctx, page, "")
	resp.InstallingMachines = s.countMachines(ctx, page, "installing")
	resp.InstalledMachines = s.countMachines(ctx, page, "installed")
	resp.FailedMachines = s.countMachines(ctx, page, "failed")

	if reply, err := s.client.ListProfiles(ctx, page); err != nil {
		s.logStatFailure(ctx, "profiles", err)
	} else {
		resp.TotalProfiles = totalOf(reply.Meta)
	}

	if reply, err := s.client.ListSessions(ctx, netbootd.SessionFilter{
		Page: page, State: "active",
	}); err != nil {
		s.logStatFailure(ctx, "sessions", err)
	} else {
		resp.ActiveSessions = totalOf(reply.Meta)
	}

	if reply, err := s.client.ListUnknownBoots(ctx, page); err != nil {
		s.logStatFailure(ctx, "unknown boots", err)
	} else {
		resp.UnknownBoots = totalOf(reply.Meta)
	}

	if reply, err := s.client.ListLeases(ctx, page); err != nil {
		s.logStatFailure(ctx, "leases", err)
	} else {
		resp.ActiveLeases = totalOf(reply.Meta)
	}

	if cfg, err := s.client.GetDhcpConfig(ctx); err != nil {
		s.logStatFailure(ctx, "dhcp config", err)
	} else {
		resp.DhcpEnabled = cfg.Enabled
	}

	s.collector.ObserveStats(resp)
	return resp, nil
}

func (s *SystemService) countMachines(ctx context.Context, page netbootd.Page, state string) int64 {
	reply, err := s.client.ListMachines(ctx, netbootd.MachineFilter{Page: page, State: state})
	if err != nil {
		s.logStatFailure(ctx, "machines/"+state, err)
		return 0
	}
	return totalOf(reply.Meta)
}

func (s *SystemService) logStatFailure(ctx context.Context, what string, err error) {
	s.log.WithContext(ctx).Warnf("stats probe for %s failed: %v", what, err)
	s.collector.UpstreamFailure("stats")
}

func totalOf(meta *netbootd.PageMeta) int64 {
	if meta == nil {
		return 0
	}
	return meta.Total.Int64()
}
