// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"goships/internal/appserver/config"
	"goships/internal/appserver/service"
	"goships/internal/appserver/server"
	"goships/internal/appserver/data"
	"github.com/google/wire"
)

// initApp init kratos application.
func initApp(*config.Config) (func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, service.ProviderSet, StartServer))
}
