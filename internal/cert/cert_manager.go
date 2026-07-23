// Package cert is the module-side bridge to go-tangra-common's certificate
// bootstrap pipeline. NewCertManager runs cert.Ensure at every boot - when
// the local cert is valid and fresh it is a fast no-op; when it is missing,
// expired, or inside the renewal window it dials LCM, signs a CSR, and writes
// the new cert to disk before returning.
package cert

import (
	"context"

	commonCert "github.com/go-tangra/go-tangra-common/cert"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
)

// CertManager is the shared certificate manager.
type CertManager = commonCert.CertManager

// NewCertManager bootstraps and loads the module's mTLS certificates. All
// knobs come from the environment - see go-tangra-common's cert.EnsureConfig.
// The required ones are:
//
//	LCM_BOOTSTRAP_ENDPOINT   lcm-service:9101
//	MODULE_BOOTSTRAP_SECRET  shared secret matching LCM's config
//	LCM_CA_FINGERPRINT       SHA-256 hex of the LCM root CA
//
// CERTS_DIR (default /app/certs) is where the {ca,server,client} subdirs live.
func NewCertManager(ctx *bootstrap.Context) (*CertManager, error) {
	return commonCert.Ensure(context.Background(), commonCert.EnsureConfig{
		ModuleID: "netboot",
		Logger:   ctx.GetLogger(),
	})
}
