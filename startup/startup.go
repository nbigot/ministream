package startup

import (
	"github.com/nbigot/ministream/storageprovider"
	"github.com/nbigot/ministream/storageprovider/inmemoryprovider"
	"github.com/nbigot/ministream/storageprovider/jsonfileprovider"
	"github.com/nbigot/ministream/web"
)

func Start() error {
	web.JWTInit()
	return SetupStorageProviders()
}

func SetupStorageProviders() error {
	var err error

	err = storageprovider.Register("JSONFile", jsonfileprovider.NewStorageProvider)
	if err != nil {
		return err
	}

	err = storageprovider.Register("InMemory", inmemoryprovider.NewStorageProvider)
	if err != nil {
		return err
	}

	return nil
}
