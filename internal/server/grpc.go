package server

import (
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/metadata"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-common/middleware/audit"
	"github.com/go-tangra/go-tangra-common/middleware/mtls"

	netbootV1 "github.com/go-tangra/go-tangra-netboot/gen/go/netboot/service/v1"
	"github.com/go-tangra/go-tangra-netboot/internal/cert"
	"github.com/go-tangra/go-tangra-netboot/internal/metrics"
	"github.com/go-tangra/go-tangra-netboot/internal/service"
)

// healthOperations are reachable without a client certificate; everything
// else must present one.
var healthOperations = []string{
	"/grpc.health.v1.Health/Check",
	"/grpc.health.v1.Health/Watch",
	"/netboot.service.v1.NetbootSystemService/Health",
}

// NewGRPCServer creates the module's gRPC server with mTLS, metrics, audit
// logging and request validation.
func NewGRPCServer(
	ctx *bootstrap.Context,
	certManager *cert.CertManager,
	collector *metrics.Collector,
	machineSvc *service.MachineService,
	profileSvc *service.ProfileService,
	dhcpSvc *service.DhcpService,
	sessionSvc *service.SessionService,
	artifactSvc *service.ArtifactService,
	systemSvc *service.SystemService,
) *grpc.Server {
	cfg := ctx.GetConfig()
	l := ctx.NewLoggerHelper("netboot/grpc")

	var opts []grpc.ServerOption

	if cfg.Server != nil && cfg.Server.Grpc != nil {
		if cfg.Server.Grpc.Network != "" {
			opts = append(opts, grpc.Network(cfg.Server.Grpc.Network))
		}
		if cfg.Server.Grpc.Addr != "" {
			opts = append(opts, grpc.Address(cfg.Server.Grpc.Addr))
		}
		if cfg.Server.Grpc.Timeout != nil {
			opts = append(opts, grpc.Timeout(cfg.Server.Grpc.Timeout.AsDuration()))
		}
	}

	tlsEnabled := certManager != nil && certManager.IsTLSEnabled()
	if tlsEnabled {
		tlsConfig, err := certManager.GetServerTLSConfig()
		if err != nil {
			l.Warnf("Failed to get TLS config, running without TLS: %v", err)
			tlsEnabled = false
		} else {
			opts = append(opts, grpc.TLSConfig(tlsConfig))
			l.Info("gRPC server configured with mTLS")
		}
	}
	if !tlsEnabled {
		l.Warn("TLS not enabled, running without mTLS")
	}

	var ms []middleware.Middleware
	ms = append(ms, recovery.Recovery())
	ms = append(ms, collector.Middleware())
	ms = append(ms, tracing.Server())
	ms = append(ms, metadata.Server())
	ms = append(ms, logging.Server(ctx.GetLogger()))

	if tlsEnabled {
		ms = append(ms, mtls.MTLSMiddleware(
			ctx.GetLogger(),
			mtls.WithPublicEndpoints(healthOperations...),
		))
	}

	// Every mutation of netboot state - arming a machine, rewriting the DHCP
	// scope, deleting a kernel - is audited. Health and info are skipped
	// because they carry no actor and would otherwise dominate the log.
	ms = append(ms, audit.Server(
		ctx.GetLogger(),
		audit.WithServiceName("netboot-service"),
		audit.WithSkipOperations(
			"/grpc.health.v1.Health/Check",
			"/grpc.health.v1.Health/Watch",
			"/netboot.service.v1.NetbootSystemService/Health",
			"/netboot.service.v1.NetbootSystemService/GetInfo",
		),
	))

	ms = append(ms, validate.Validator())

	opts = append(opts, grpc.Middleware(ms...))

	srv := grpc.NewServer(opts...)

	// The redacted registrars strip fields marked with (redact.v3.value) from
	// responses; passing a nil bypass means no caller can opt out.
	netbootV1.RegisterRedactedNetbootMachineServiceServer(srv, machineSvc, nil)
	netbootV1.RegisterRedactedNetbootProfileServiceServer(srv, profileSvc, nil)
	netbootV1.RegisterRedactedNetbootDhcpServiceServer(srv, dhcpSvc, nil)
	netbootV1.RegisterRedactedNetbootSessionServiceServer(srv, sessionSvc, nil)
	netbootV1.RegisterRedactedNetbootArtifactServiceServer(srv, artifactSvc, nil)
	netbootV1.RegisterRedactedNetbootSystemServiceServer(srv, systemSvc, nil)

	return srv
}
