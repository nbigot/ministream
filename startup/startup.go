package startup

import (
	"ministream/storageprovider"
	"ministream/storageprovider/jsonfileprovider"
)

func Start() error {
	return SetupStorageProviders()
}

func SetupStorageProviders() error {
	var err error

	err = storageprovider.Register("JSONFile", jsonfileprovider.NewStorageProvider)
	if err != nil {
		return err
	}

	return nil
}
