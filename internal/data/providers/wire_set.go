//go:build wireinject
// +build wireinject

//go:generate go run github.com/google/wire/cmd/wire

// This file defines the dependency injection ProviderSet for the data layer.
//
// The netboot module keeps no database of its own: every entity it exposes
// lives on the upstream netbootd. The "data layer" is therefore just the
// upstream client and its configuration.

package providers

import (
	"github.com/google/wire"

	"github.com/go-tangra/go-tangra-netboot/internal/data"
)

// ProviderSet is the Wire provider set for the data layer.
var ProviderSet = wire.NewSet(
	data.NewNetbootdConfig,
	data.NewNetbootdClient,
)
