package storageprovider

import (
	"fmt"
	"ministream/config"

	"go.uber.org/zap"
)

// Factory is used to register functions creating new stream storage provider instances.
type Factory = func(logger *zap.Logger, conf *config.Config) (IStorageProvider, error)

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
