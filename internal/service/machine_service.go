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

// MachineService proxies machine management to the upstream netbootd.
type MachineService struct {
	netbootV1.UnimplementedNetbootMachineServiceServer

	log       *log.Helper
	client    *netbootd.Client
	checker   *authz.Checker
	collector *metrics.Collector
}

// NewMachineService wires the machine service.
func NewMachineService(
	ctx *bootstrap.Context,
	client *netbootd.Client,
	checker *authz.Checker,
	collector *metrics.Collector,
) *MachineService {
	return &MachineService{
		log:       ctx.NewLoggerHelper("netboot/service/machine"),
		client:    client,
		checker:   checker,
		collector: collector,
	}
}

// fail logs the underlying upstream failure - which may name internal hosts
// or carry netbootd's own diagnostics - and returns the sanitised error that
// the caller is allowed to see.
func (s *MachineService) fail(ctx context.Context, op string, err error) error {
	s.log.WithContext(ctx).Errorf("machine %s failed: %v", op, err)
	s.collector.UpstreamFailure(op)
	return translateUpstreamError(resourceMachine, err)
}

func (s *MachineService) ListMachines(
	ctx context.Context, req *netbootV1.ListMachinesRequest,
) (*netbootV1.ListMachinesResponse, error) {
	if err := s.checker.Require(ctx, authz.PermMachineView); err != nil {
		return nil, err
	}

	reply, err := s.client.ListMachines(ctx, netbootd.MachineFilter{
		Page:      toPage(req.GetPage()),
		State:     provisionStateToUpstream[req.GetState()],
		ProfileID: req.GetProfileId(),
		Query:     req.GetQ(),
	})
	if err != nil {
		return nil, s.fail(ctx, "list", err)
	}

	return &netbootV1.ListMachinesResponse{
		Machines: toMachines(reply.Machines),
		Meta:     toPageMeta(reply.Meta),
	}, nil
}

func (s *MachineService) GetMachine(
	ctx context.Context, req *netbootV1.GetMachineRequest,
) (*netbootV1.GetMachineResponse, error) {
	if err := s.checker.Require(ctx, authz.PermMachineView); err != nil {
		return nil, err
	}

	machine, err := s.client.GetMachine(ctx, req.GetId())
	if err != nil {
		return nil, s.fail(ctx, "get", err)
	}
	return &netbootV1.GetMachineResponse{Machine: toMachine(machine)}, nil
}

func (s *MachineService) CreateMachine(
	ctx context.Context, req *netbootV1.CreateMachineRequest,
) (*netbootV1.CreateMachineResponse, error) {
	if err := s.checker.Require(ctx, authz.PermMachineCreate); err != nil {
		return nil, err
	}

	machine, err := s.client.CreateMachine(ctx, &netbootd.CreateMachineBody{
		MAC:            normaliseMAC(req.GetMac()),
		Name:           req.GetName(),
		Firmware:       firmwareToUpstream[req.GetFirmware()],
		ProfileID:      req.GetProfileId(),
		ReservationIP:  req.GetReservationIp(),
		Notes:          req.GetNotes(),
		NetworkConfig:  req.GetNetworkConfig(),
		InstallNetwork: fromInstallNetwork(req.GetInstallNetwork()),
	})
	if err != nil {
		return nil, s.fail(ctx, "create", err)
	}

	s.log.WithContext(ctx).Infof("machine %s (%s) created by %s",
		machine.ID, machine.MAC, getUsernameFromContext(ctx))
	s.collector.MachineCreated()

	return &netbootV1.CreateMachineResponse{Machine: toMachine(machine)}, nil
}

func (s *MachineService) UpdateMachine(
	ctx context.Context, req *netbootV1.UpdateMachineRequest,
) (*netbootV1.UpdateMachineResponse, error) {
	if err := s.checker.Require(ctx, authz.PermMachineUpdate); err != nil {
		return nil, err
	}

	// Only the fields the caller actually set are forwarded: the optional
	// proto fields arrive as nil pointers when omitted, and passing them
	// through unchanged is what stops a partial update from silently
	// clearing a value the operator never mentioned.
	body := &netbootd.UpdateMachineBody{
		Name:           req.Name,
		ProfileID:      req.ProfileId,
		ReservationIP:  req.ReservationIp,
		Notes:          req.Notes,
		NetworkConfig:  req.NetworkConfig,
		InstallNetwork: fromInstallNetwork(req.GetInstallNetwork()),
	}

	machine, err := s.client.UpdateMachine(ctx, req.GetId(), body)
	if err != nil {
		return nil, s.fail(ctx, "update", err)
	}
	return &netbootV1.UpdateMachineResponse{Machine: toMachine(machine)}, nil
}

func (s *MachineService) DeleteMachine(
	ctx context.Context, req *netbootV1.DeleteMachineRequest,
) (*emptypb.Empty, error) {
	if err := s.checker.Require(ctx, authz.PermMachineDelete); err != nil {
		return nil, err
	}

	if err := s.client.DeleteMachine(ctx, req.GetId()); err != nil {
		return nil, s.fail(ctx, "delete", err)
	}

	s.log.WithContext(ctx).Infof("machine %s deleted by %s", req.GetId(), getUsernameFromContext(ctx))
	s.collector.MachineDeleted()

	return &emptypb.Empty{}, nil
}

func (s *MachineService) ProvisionMachine(
	ctx context.Context, req *netbootV1.ProvisionMachineRequest,
) (*netbootV1.ProvisionMachineResponse, error) {
	if err := s.checker.Require(ctx, authz.PermMachineProvision); err != nil {
		return nil, err
	}

	machine, err := s.client.ProvisionMachine(ctx, req.GetId())
	if err != nil {
		return nil, s.fail(ctx, "provision", err)
	}

	// Arming a machine will wipe it on next boot, so the actor is recorded
	// at info level in addition to the audit middleware's own record.
	s.log.WithContext(ctx).Infof("machine %s armed for provisioning by %s (tenant %d)",
		machine.ID, getUsernameFromContext(ctx), getTenantIDFromContext(ctx))
	s.collector.ProvisionRequested()

	return &netbootV1.ProvisionMachineResponse{Machine: toMachine(machine)}, nil
}

func (s *MachineService) CancelProvision(
	ctx context.Context, req *netbootV1.CancelProvisionRequest,
) (*netbootV1.CancelProvisionResponse, error) {
	if err := s.checker.Require(ctx, authz.PermMachineProvision); err != nil {
		return nil, err
	}

	machine, err := s.client.CancelProvision(ctx, req.GetId())
	if err != nil {
		return nil, s.fail(ctx, "cancel", err)
	}

	s.log.WithContext(ctx).Infof("provisioning of machine %s cancelled by %s",
		machine.ID, getUsernameFromContext(ctx))
	s.collector.ProvisionCancelled()

	return &netbootV1.CancelProvisionResponse{Machine: toMachine(machine)}, nil
}

func (s *MachineService) ListUnknownBoots(
	ctx context.Context, req *netbootV1.ListUnknownBootsRequest,
) (*netbootV1.ListUnknownBootsResponse, error) {
	if err := s.checker.Require(ctx, authz.PermMachineView); err != nil {
		return nil, err
	}

	reply, err := s.client.ListUnknownBoots(ctx, toPage(req.GetPage()))
	if err != nil {
		return nil, s.fail(ctx, "list unknown boots", err)
	}

	return &netbootV1.ListUnknownBootsResponse{
		Boots: toUnknownBoots(reply.Boots),
		Meta:  toPageMeta(reply.Meta),
	}, nil
}

func (s *MachineService) RegisterUnknownMachine(
	ctx context.Context, req *netbootV1.RegisterUnknownMachineRequest,
) (*netbootV1.RegisterUnknownMachineResponse, error) {
	if err := s.checker.Require(ctx, authz.PermMachineCreate); err != nil {
		return nil, err
	}

	machine, err := s.client.RegisterFromUnknown(ctx, &netbootd.RegisterFromUnknownBody{
		MAC:       normaliseMAC(req.GetMac()),
		Name:      req.GetName(),
		ProfileID: req.GetProfileId(),
	})
	if err != nil {
		return nil, s.fail(ctx, "register unknown", err)
	}

	s.log.WithContext(ctx).Infof("unknown boot %s registered as machine %s by %s",
		machine.MAC, machine.ID, getUsernameFromContext(ctx))
	s.collector.MachineCreated()

	return &netbootV1.RegisterUnknownMachineResponse{Machine: toMachine(machine)}, nil
}
