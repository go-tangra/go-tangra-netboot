// Package assets embeds the static descriptors the admin gateway consumes at
// module registration time, together with the federated frontend bundle.
package assets

import (
	"embed"
	_ "embed"
)

// OpenApiData is the module's OpenAPI v3 document, generated from the protos.
//
//go:embed openapi.yaml
var OpenApiData []byte

// MenusData declares the module's Vben admin menus, dashboard widgets,
// permissions and roles.
//
//go:embed menus.yaml
var MenusData []byte

// DescriptorData is the compiled proto descriptor set, used by the gateway to
// transcode REST calls into gRPC.
//
//go:embed descriptor.bin
var DescriptorData []byte

// FrontendDist is the built module-federation bundle (remoteEntry.js and its
// chunks) produced by the frontend build stage.
//
//go:embed all:frontend-dist
var FrontendDist embed.FS
