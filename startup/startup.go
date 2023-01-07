package startup

import (
	"github.com/nbigot/ministream/storageprovider/registry"
	"github.com/nbigot/ministream/web"
)

func Start() error {
	web.JWTInit()
	return registry.SetupStorageProviders()
}
