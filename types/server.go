package types

import (
	"context"

	"github.com/nbigot/ministream/config"
)

type IServer interface {
	Initialize(ctx context.Context) error
	Start() error
	Shutdown() error
	GetWebConfig() *config.WebServerConfig
}
