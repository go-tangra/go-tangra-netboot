// Command server runs the go-tangra-netboot module: a Tangra control-plane
// module that federates a remote netbootd (universe) instance into the
// platform's gRPC mesh and Vben admin UI.
package main

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"

	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-common/registration"
	"github.com/go-tangra/go-tangra-common/service"

	"github.com/go-tangra/go-tangra-netboot/cmd/server/assets"
)

var (
	// Module identity advertised to the admin gateway.
	moduleID    = "netboot"
	moduleName  = "Netboot"
	version     = "1.0.0"
	description = "Network provisioning: PXE/iPXE machines, autoinstall profiles, DHCP and boot artifacts"
)

// globalRegHelper is retained so the deferred Stop in runApp can unregister
// the module cleanly on shutdown.
var globalRegHelper *registration.RegistrationHelper

// go build -ldflags "-X main.version=x.y.z"

func newApp(
	ctx *bootstrap.Context,
	gs *grpc.Server,
	hs *kratosHttp.Server,
) *kratos.App {
	globalRegHelper = registration.StartRegistration(ctx, ctx.GetLogger(), &registration.Config{
		ModuleID:          moduleID,
		ModuleName:        moduleName,
		Version:           version,
		Description:       description,
		GRPCEndpoint:      registration.GetGRPCAdvertiseAddr(ctx, "0.0.0.0:10000"),
		AdminEndpoint:     registration.GetEnvOrDefault("ADMIN_GRPC_ENDPOINT", ""),
		FrontendEntryUrl:  registration.GetEnvOrDefault("FRONTEND_ENTRY_URL", ""),
		HttpEndpoint:      registration.GetEnvOrDefault("HTTP_ADVERTISE_ADDR", ""),
		OpenapiSpec:       assets.OpenApiData,
		ProtoDescriptor:   assets.DescriptorData,
		MenusYaml:         assets.MenusData,
		HeartbeatInterval: 30 * time.Second,
		RetryInterval:     5 * time.Second,
		MaxRetries:        60,
	})

	return bootstrap.NewApp(ctx, gs, hs)
}

func runApp() error {
	ctx := bootstrap.NewContext(
		context.Background(),
		&conf.AppInfo{
			Project: service.Project,
			AppId:   "netboot.service",
			Version: version,
		},
	)

	// Ensure registration cleanup on exit.
	defer globalRegHelper.Stop()

	return bootstrap.RunApp(ctx, initApp)
}

func main() {
	if err := runApp(); err != nil {
		panic(err)
	}
}
