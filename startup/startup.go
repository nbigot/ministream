package startup

import (
	"ministream/storageprovider"
	"ministream/storageprovider/inmemoryprovider"
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

	err = storageprovider.Register("InMemory", inmemoryprovider.NewStorageProvider)
	if err != nil {
		return err
	}

	return nil
}
