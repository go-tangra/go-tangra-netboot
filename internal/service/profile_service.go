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

// ProfileService proxies installation-profile management to netbootd.
type ProfileService struct {
	netbootV1.UnimplementedNetbootProfileServiceServer

	log       *log.Helper
	client    *netbootd.Client
	checker   *authz.Checker
	collector *metrics.Collector
}

// NewProfileService wires the profile service.
func NewProfileService(
	ctx *bootstrap.Context,
	client *netbootd.Client,
	checker *authz.Checker,
	collector *metrics.Collector,
) *ProfileService {
	return &ProfileService{
		log:       ctx.NewLoggerHelper("netboot/service/profile"),
		client:    client,
		checker:   checker,
		collector: collector,
	}
}

func (s *ProfileService) fail(ctx context.Context, op string, err error) error {
	s.log.WithContext(ctx).Errorf("profile %s failed: %v", op, err)
	s.collector.UpstreamFailure(op)
	return translateUpstreamError(resourceProfile, err)
}

func (s *ProfileService) ListProfiles(
	ctx context.Context, req *netbootV1.ListProfilesRequest,
) (*netbootV1.ListProfilesResponse, error) {
	if err := s.checker.Require(ctx, authz.PermProfileView); err != nil {
		return nil, err
	}

	reply, err := s.client.ListProfiles(ctx, toPage(req.GetPage()))
	if err != nil {
		return nil, s.fail(ctx, "list", err)
	}

	return &netbootV1.ListProfilesResponse{
		Profiles: toProfiles(reply.Profiles),
		Meta:     toPageMeta(reply.Meta),
	}, nil
}

func (s *ProfileService) GetProfile(
	ctx context.Context, req *netbootV1.GetProfileRequest,
) (*netbootV1.GetProfileResponse, error) {
	if err := s.checker.Require(ctx, authz.PermProfileView); err != nil {
		return nil, err
	}

	profile, err := s.client.GetProfile(ctx, req.GetId())
	if err != nil {
		return nil, s.fail(ctx, "get", err)
	}
	return &netbootV1.GetProfileResponse{Profile: toProfile(profile)}, nil
}

func (s *ProfileService) CreateProfile(
	ctx context.Context, req *netbootV1.CreateProfileRequest,
) (*netbootV1.CreateProfileResponse, error) {
	if err := s.checker.Require(ctx, authz.PermProfileCreate); err != nil {
		return nil, err
	}

	profile, err := s.client.CreateProfile(ctx, fromProfileInput(req.GetProfile()))
	if err != nil {
		return nil, s.fail(ctx, "create", err)
	}

	s.log.WithContext(ctx).Infof("profile %s (%s) created by %s",
		profile.ID, profile.Name, getUsernameFromContext(ctx))
	s.collector.ProfileCreated()

	return &netbootV1.CreateProfileResponse{Profile: toProfile(profile)}, nil
}

func (s *ProfileService) UpdateProfile(
	ctx context.Context, req *netbootV1.UpdateProfileRequest,
) (*netbootV1.UpdateProfileResponse, error) {
	if err := s.checker.Require(ctx, authz.PermProfileUpdate); err != nil {
		return nil, err
	}

	profile, err := s.client.UpdateProfile(ctx, req.GetId(), fromProfileInput(req.GetProfile()))
	if err != nil {
		return nil, s.fail(ctx, "update", err)
	}
	return &netbootV1.UpdateProfileResponse{Profile: toProfile(profile)}, nil
}

func (s *ProfileService) CloneProfile(
	ctx context.Context, req *netbootV1.CloneProfileRequest,
) (*netbootV1.CloneProfileResponse, error) {
	// Cloning materialises a new profile, so it is gated on create rather
	// than on read of the source.
	if err := s.checker.Require(ctx, authz.PermProfileCreate); err != nil {
		return nil, err
	}

	profile, err := s.client.CloneProfile(ctx, req.GetId(), req.GetNewName())
	if err != nil {
		return nil, s.fail(ctx, "clone", err)
	}

	s.log.WithContext(ctx).Infof("profile %s cloned from %s by %s",
		profile.ID, req.GetId(), getUsernameFromContext(ctx))
	s.collector.ProfileCreated()

	return &netbootV1.CloneProfileResponse{Profile: toProfile(profile)}, nil
}

func (s *ProfileService) DeleteProfile(
	ctx context.Context, req *netbootV1.DeleteProfileRequest,
) (*emptypb.Empty, error) {
	if err := s.checker.Require(ctx, authz.PermProfileDelete); err != nil {
		return nil, err
	}

	if err := s.client.DeleteProfile(ctx, req.GetId()); err != nil {
		return nil, s.fail(ctx, "delete", err)
	}

	s.log.WithContext(ctx).Infof("profile %s deleted by %s", req.GetId(), getUsernameFromContext(ctx))
	s.collector.ProfileDeleted()

	return &emptypb.Empty{}, nil
}

func (s *ProfileService) PreviewProfile(
	ctx context.Context, req *netbootV1.PreviewProfileRequest,
) (*netbootV1.PreviewProfileResponse, error) {
	// The rendered seed exposes the profile's whole installation recipe, so
	// preview requires update rather than plain view: it is the same
	// audience that may change the thing being rendered.
	if err := s.checker.Require(ctx, authz.PermProfileUpdate); err != nil {
		return nil, err
	}

	reply, err := s.client.PreviewProfile(ctx, req.GetId(), req.GetMachineId())
	if err != nil {
		return nil, s.fail(ctx, "preview", err)
	}

	return &netbootV1.PreviewProfileResponse{
		UserData: reply.UserData,
		Cmdline:  reply.Cmdline,
	}, nil
}
