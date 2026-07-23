package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	netbootV1 "github.com/go-tangra/go-tangra-netboot/gen/go/netboot/service/v1"
	"github.com/go-tangra/go-tangra-netboot/internal/authz"
	"github.com/go-tangra/go-tangra-netboot/internal/metrics"
	"github.com/go-tangra/go-tangra-netboot/internal/netbootd"
)

// DhcpService proxies DHCP configuration and lease inspection to netbootd.
type DhcpService struct {
	netbootV1.UnimplementedNetbootDhcpServiceServer

	log       *log.Helper
	client    *netbootd.Client
	checker   *authz.Checker
	collector *metrics.Collector
}

// NewDhcpService wires the DHCP service.
func NewDhcpService(
	ctx *bootstrap.Context,
	client *netbootd.Client,
	checker *authz.Checker,
	collector *metrics.Collector,
) *DhcpService {
	return &DhcpService{
		log:       ctx.NewLoggerHelper("netboot/service/dhcp"),
		client:    client,
		checker:   checker,
		collector: collector,
	}
}

func (s *DhcpService) fail(ctx context.Context, op string, err error) error {
	s.log.WithContext(ctx).Errorf("dhcp %s failed: %v", op, err)
	s.collector.UpstreamFailure(op)
	return translateUpstreamError(resourceGeneric, err)
}

func (s *DhcpService) GetDhcpConfig(
	ctx context.Context, _ *netbootV1.GetDhcpConfigRequest,
) (*netbootV1.GetDhcpConfigResponse, error) {
	if err := s.checker.Require(ctx, authz.PermDhcpView); err != nil {
		return nil, err
	}

	cfg, err := s.client.GetDhcpConfig(ctx)
	if err != nil {
		return nil, s.fail(ctx, "get config", err)
	}
	return &netbootV1.GetDhcpConfigResponse{Config: toDhcpConfig(cfg)}, nil
}

func (s *DhcpService) UpdateDhcpConfig(
	ctx context.Context, req *netbootV1.UpdateDhcpConfigRequest,
) (*netbootV1.UpdateDhcpConfigResponse, error) {
	if err := s.checker.Require(ctx, authz.PermDhcpManage); err != nil {
		return nil, err
	}

	cfg, err := s.client.UpdateDhcpConfig(ctx, &netbootd.UpdateDhcpConfigBody{
		LeaseTTLSeconds: req.GetLeaseTtlSeconds(),
		Subnets:         fromDhcpSubnets(req.GetSubnets()),
	})
	if err != nil {
		return nil, s.fail(ctx, "update config", err)
	}

	s.log.WithContext(ctx).Infof("dhcp configuration updated to version %d by %s",
		cfg.Version, getUsernameFromContext(ctx))

	return &netbootV1.UpdateDhcpConfigResponse{Config: toDhcpConfig(cfg)}, nil
}

func (s *DhcpService) EnableDhcp(
	ctx context.Context, _ *netbootV1.EnableDhcpRequest,
) (*netbootV1.EnableDhcpResponse, error) {
	if err := s.checker.Require(ctx, authz.PermDhcpManage); err != nil {
		return nil, err
	}

	cfg, err := s.client.EnableDhcp(ctx)
	if err != nil {
		return nil, s.fail(ctx, "enable", err)
	}

	// Turning on an authoritative DHCP server changes the behaviour of an
	// entire network segment; log the actor prominently.
	s.log.WithContext(ctx).Infof("authoritative DHCP ENABLED by %s (tenant %d)",
		getUsernameFromContext(ctx), getTenantIDFromContext(ctx))
	s.collector.DhcpStateChanged(true)

	return &netbootV1.EnableDhcpResponse{Config: toDhcpConfig(cfg)}, nil
}

func (s *DhcpService) DisableDhcp(
	ctx context.Context, _ *netbootV1.DisableDhcpRequest,
) (*netbootV1.DisableDhcpResponse, error) {
	if err := s.checker.Require(ctx, authz.PermDhcpManage); err != nil {
		return nil, err
	}

	cfg, err := s.client.DisableDhcp(ctx)
	if err != nil {
		return nil, s.fail(ctx, "disable", err)
	}

	s.log.WithContext(ctx).Infof("authoritative DHCP DISABLED by %s (tenant %d)",
		getUsernameFromContext(ctx), getTenantIDFromContext(ctx))
	s.collector.DhcpStateChanged(false)

	return &netbootV1.DisableDhcpResponse{Config: toDhcpConfig(cfg)}, nil
}

func (s *DhcpService) ListLeases(
	ctx context.Context, req *netbootV1.ListLeasesRequest,
) (*netbootV1.ListLeasesResponse, error) {
	if err := s.checker.Require(ctx, authz.PermDhcpView); err != nil {
		return nil, err
	}

	reply, err := s.client.ListLeases(ctx, toPage(req.GetPage()))
	if err != nil {
		return nil, s.fail(ctx, "list leases", err)
	}

	return &netbootV1.ListLeasesResponse{
		Leases: toLeases(reply.Leases),
		Meta:   toPageMeta(reply.Meta),
	}, nil
}

func (s *DhcpService) ListForeignServers(
	ctx context.Context, req *netbootV1.ListForeignServersRequest,
) (*netbootV1.ListForeignServersResponse, error) {
	if err := s.checker.Require(ctx, authz.PermDhcpView); err != nil {
		return nil, err
	}

	reply, err := s.client.ListForeignServers(ctx, toPage(req.GetPage()))
	if err != nil {
		return nil, s.fail(ctx, "list conflicts", err)
	}

	return &netbootV1.ListForeignServersResponse{
		Servers: toForeignServers(reply.Servers),
		Meta:    toPageMeta(reply.Meta),
	}, nil
}
