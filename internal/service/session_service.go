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

// SessionService exposes provisioning history from netbootd.
type SessionService struct {
	netbootV1.UnimplementedNetbootSessionServiceServer

	log       *log.Helper
	client    *netbootd.Client
	checker   *authz.Checker
	collector *metrics.Collector
}

// NewSessionService wires the session service.
func NewSessionService(
	ctx *bootstrap.Context,
	client *netbootd.Client,
	checker *authz.Checker,
	collector *metrics.Collector,
) *SessionService {
	return &SessionService{
		log:       ctx.NewLoggerHelper("netboot/service/session"),
		client:    client,
		checker:   checker,
		collector: collector,
	}
}

func (s *SessionService) fail(ctx context.Context, op string, err error) error {
	s.log.WithContext(ctx).Errorf("session %s failed: %v", op, err)
	s.collector.UpstreamFailure(op)
	return translateUpstreamError(resourceSession, err)
}

func (s *SessionService) ListSessions(
	ctx context.Context, req *netbootV1.ListSessionsRequest,
) (*netbootV1.ListSessionsResponse, error) {
	if err := s.checker.Require(ctx, authz.PermSessionView); err != nil {
		return nil, err
	}

	reply, err := s.client.ListSessions(ctx, netbootd.SessionFilter{
		Page:      toPage(req.GetPage()),
		MachineID: req.GetMachineId(),
		State:     sessionStateToUpstream[req.GetState()],
	})
	if err != nil {
		return nil, s.fail(ctx, "list", err)
	}

	return &netbootV1.ListSessionsResponse{
		Sessions: toSessions(reply.Sessions),
		Meta:     toPageMeta(reply.Meta),
	}, nil
}

func (s *SessionService) GetSession(
	ctx context.Context, req *netbootV1.GetSessionRequest,
) (*netbootV1.GetSessionResponse, error) {
	if err := s.checker.Require(ctx, authz.PermSessionView); err != nil {
		return nil, err
	}

	detail, err := s.client.GetSession(ctx, req.GetId())
	if err != nil {
		return nil, s.fail(ctx, "get", err)
	}

	return &netbootV1.GetSessionResponse{
		Session:  toSession(detail.Session),
		Timeline: toEvents(detail.Timeline),
		Evidence: rawJSONString(detail.Evidence),
	}, nil
}
