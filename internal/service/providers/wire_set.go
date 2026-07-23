//go:build wireinject
// +build wireinject

//go:generate go run github.com/google/wire/cmd/wire

// This file defines the dependency injection ProviderSet for the service
// layer and contains no business logic. The `wireinject` build tag excludes
// it from normal builds and from the final binary.

package providers

import (
	"github.com/google/wire"

	"github.com/go-tangra/go-tangra-netboot/internal/authz"
	"github.com/go-tangra/go-tangra-netboot/internal/metrics"
	"github.com/go-tangra/go-tangra-netboot/internal/service"
)

// ProviderSet is the Wire provider set for the service layer.
var ProviderSet = wire.NewSet(
	service.NewMachineService,
	service.NewProfileService,
	service.NewDhcpService,
	service.NewSessionService,
	service.NewArtifactService,
	service.NewSystemService,
	metrics.NewCollector,
	authz.NewChecker,
)
