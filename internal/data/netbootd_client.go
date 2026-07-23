// Package data owns this module's outbound resources. The netboot module has
// no database - every entity it serves lives on the upstream netbootd - so
// the only resource here is the upstream client.
package data

import (
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-netboot/internal/netbootd"
)

// NewNetbootdConfig loads and validates the upstream configuration.
//
// A misconfigured endpoint (bad URL, plaintext without the acknowledgement,
// missing credentials) fails startup: silently degrading to "no netboot" would
// leave operators looking at an empty machine list with no explanation.
func NewNetbootdConfig(ctx *bootstrap.Context) (*netbootd.Config, error) {
	cfg, err := netbootd.LoadConfig()
	if err != nil {
		ctx.NewLoggerHelper("netboot/data").Errorf("invalid netbootd configuration: %v", err)
		return nil, err
	}
	return cfg, nil
}

// NewNetbootdClient builds the upstream client and returns a cleanup that
// drops the operator session on shutdown.
func NewNetbootdClient(
	ctx *bootstrap.Context, cfg *netbootd.Config,
) (*netbootd.Client, func(), error) {
	client, err := netbootd.NewClient(cfg, ctx.GetLogger())
	if err != nil {
		return nil, nil, err
	}
	return client, client.Close, nil
}
