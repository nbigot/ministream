package startup

import (
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/storageprovider/registry"
	"github.com/nbigot/ministream/web"
)

func Start(appConfig *config.Config) error {
	web.JWTInit(appConfig.WebServer.JWT)
	return registry.SetupStorageProviders()
}
