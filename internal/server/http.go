package server

import (
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-netboot/cmd/server/assets"
)

const defaultHTTPAddr = "0.0.0.0:10001"

// NewHTTPServer serves the module's federated frontend bundle and the static
// descriptors the admin gateway fetches at registration time.
//
// It carries no business API: every netboot operation goes through gRPC,
// where the mTLS, audit and validation middleware live. Keeping this server
// to static assets means it needs no authentication of its own and cannot
// become a second, weaker path to the upstream.
func NewHTTPServer(ctx *bootstrap.Context) *kratosHttp.Server {
	l := ctx.NewLoggerHelper("netboot/http")

	addr := os.Getenv("NETBOOT_HTTP_ADDR")
	if addr == "" {
		addr = defaultHTTPAddr
	}

	srv := kratosHttp.NewServer(
		kratosHttp.Address(addr),
		kratosHttp.Timeout(30*time.Second),
		kratosHttp.Middleware(
			recovery.Recovery(),
		),
	)

	route := srv.Route("/")

	route.GET("/health", func(c kratosHttp.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	route.GET("/openapi.yaml", func(c kratosHttp.Context) error {
		c.Response().Header().Set("Content-Type", "application/yaml")
		_, err := c.Response().Write(assets.OpenApiData)
		return err
	})

	route.GET("/proto-descriptor", func(c kratosHttp.Context) error {
		c.Response().Header().Set("Content-Type", "application/octet-stream")
		c.Response().Header().Set("Content-Disposition", "attachment; filename=descriptor.bin")
		_, err := c.Response().Write(assets.DescriptorData)
		return err
	})

	route.GET("/menus.yaml", func(c kratosHttp.Context) error {
		c.Response().Header().Set("Content-Type", "application/yaml")
		_, err := c.Response().Write(assets.MenusData)
		return err
	})

	// The federated remote entry and its chunks are served from the embedded
	// build output produced by the frontend stage of the Dockerfile.
	fsys, err := fs.Sub(assets.FrontendDist, "frontend-dist")
	if err == nil {
		srv.HandlePrefix("/", http.FileServer(http.FS(fsys)))
		l.Info("Serving embedded frontend assets")
	} else {
		l.Warnf("Failed to load embedded frontend assets: %v", err)
	}

	l.Infof("HTTP server listening on %s", addr)
	return srv
}
