package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	netbootV1 "github.com/go-tangra/go-tangra-netboot/gen/go/netboot/service/v1"
	"github.com/go-tangra/go-tangra-netboot/internal/authz"
	"github.com/go-tangra/go-tangra-netboot/internal/metrics"
	"github.com/go-tangra/go-tangra-netboot/internal/netbootd"
)

// ArtifactService exposes boot artifacts and the boot-path transfer log.
//
// Upload and replace are intentionally absent: those upstream endpoints are
// multipart streams of kernel-sized payloads, and relaying them through the
// admin gateway would give any authorized operator a way to pin hundreds of
// megabytes of this module's memory per request.
type ArtifactService struct {
	netbootV1.UnimplementedNetbootArtifactServiceServer

	log       *log.Helper
	client    *netbootd.Client
	checker   *authz.Checker
	collector *metrics.Collector
}

// NewArtifactService wires the artifact service.
func NewArtifactService(
	ctx *bootstrap.Context,
	client *netbootd.Client,
	checker *authz.Checker,
	collector *metrics.Collector,
) *ArtifactService {
	return &ArtifactService{
		log:       ctx.NewLoggerHelper("netboot/service/artifact"),
		client:    client,
		checker:   checker,
		collector: collector,
	}
}

func (s *ArtifactService) fail(ctx context.Context, op string, err error) error {
	s.log.WithContext(ctx).Errorf("artifact %s failed: %v", op, err)
	s.collector.UpstreamFailure(op)
	return translateUpstreamError(resourceArtifact, err)
}

func (s *ArtifactService) ListArtifacts(
	ctx context.Context, req *netbootV1.ListArtifactsRequest,
) (*netbootV1.ListArtifactsResponse, error) {
	if err := s.checker.Require(ctx, authz.PermArtifactView); err != nil {
		return nil, err
	}

	reply, err := s.client.ListArtifacts(ctx, toPage(req.GetPage()))
	if err != nil {
		return nil, s.fail(ctx, "list", err)
	}

	return &netbootV1.ListArtifactsResponse{
		Artifacts: toArtifacts(reply.Artifacts),
		Meta:      toPageMeta(reply.Meta),
	}, nil
}

func (s *ArtifactService) GetArtifact(
	ctx context.Context, req *netbootV1.GetArtifactRequest,
) (*netbootV1.GetArtifactResponse, error) {
	if err := s.checker.Require(ctx, authz.PermArtifactView); err != nil {
		return nil, err
	}

	artifact, err := s.client.GetArtifact(ctx, req.GetId())
	if err != nil {
		return nil, s.fail(ctx, "get", err)
	}
	return &netbootV1.GetArtifactResponse{Artifact: toArtifact(artifact)}, nil
}

func (s *ArtifactService) DeleteArtifact(
	ctx context.Context, req *netbootV1.DeleteArtifactRequest,
) (*emptypb.Empty, error) {
	if err := s.checker.Require(ctx, authz.PermArtifactDelete); err != nil {
		return nil, err
	}

	if err := s.client.DeleteArtifact(ctx, req.GetId()); err != nil {
		return nil, s.fail(ctx, "delete", err)
	}

	// Deleting a kernel or initrd breaks every profile that references it.
	s.log.WithContext(ctx).Infof("boot artifact %s deleted by %s",
		req.GetId(), getUsernameFromContext(ctx))
	s.collector.ArtifactDeleted()

	return &emptypb.Empty{}, nil
}

func (s *ArtifactService) ListTransfers(
	ctx context.Context, req *netbootV1.ListTransfersRequest,
) (*netbootV1.ListTransfersResponse, error) {
	if err := s.checker.Require(ctx, authz.PermArtifactView); err != nil {
		return nil, err
	}

	reply, err := s.client.ListTransfers(ctx, netbootd.TransferFilter{
		Page:     toPage(req.GetPage()),
		Filename: req.GetFilename(),
	})
	if err != nil {
		return nil, s.fail(ctx, "list transfers", err)
	}

	return &netbootV1.ListTransfersResponse{
		Transfers: toTransfers(reply.Transfers),
		Meta:      toPageMeta(reply.Meta),
	}, nil
}
