package registry

import (
	"fmt"

	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/storageprovider"
	"github.com/nbigot/ministream/storageprovider/inmemoryprovider"
	"github.com/nbigot/ministream/storageprovider/jsonfileprovider"
	"go.uber.org/zap"
)

// Factory is used to register functions creating new stream storage provider instances.
type Factory = func(logger *zap.Logger, conf *config.Config) (storageprovider.IStorageProvider, error)

var registry = make(map[string]Factory)

func Register(name string, factory Factory) error {
	if name == "" {
		return fmt.Errorf("error registering storage provider: name cannot be empty")
	}

	if factory == nil {
		return fmt.Errorf("error registering storage provider '%v': factory cannot be empty", name)
	}

	if _, found := registry[name]; found {
		return fmt.Errorf("error registering storage provider '%v': already registered", name)
	}

	registry[name] = factory
	return nil
}

func GetFactory(name string) (Factory, error) {
	if _, found := registry[name]; !found {
		return nil, fmt.Errorf("error creating storage provider. No such storage provider type exists: '%v'", name)
	}
	return registry[name], nil
}

func SetupStorageProviders() error {
	var err error

	err = Register("JSONFile", jsonfileprovider.NewStorageProvider)
	if err != nil {
		return err
	}

	err = Register("InMemory", inmemoryprovider.NewStorageProvider)
	if err != nil {
		return err
	}

	return nil
}

func NewStorageProvider(logger *zap.Logger, conf *config.Config) (storageprovider.IStorageProvider, error) {
	if factory, err := GetFactory(conf.Storage.Type); err != nil {
		return nil, err
	} else {
		spLogger, err := NewLogger(&conf.Storage.LoggerConfig)
		if err != nil {
			return nil, err
		}
		return factory(spLogger, conf)
	}
}

func NewLogger(loggerConfig *zap.Config) (*zap.Logger, error) {
	logger, err := loggerConfig.Build()
	if err != nil {
		return nil, err
	}
	return logger, nil
}
