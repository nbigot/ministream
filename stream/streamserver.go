package stream

import (
	"github.com/nbigot/ministream/log"
	"github.com/nbigot/ministream/rbac"

	"go.uber.org/zap"
)

func LoadServerAuthConfig(enableRBAC bool, configurationFilenameRBAC string) {
	log.Logger.Info(
		"Loading server auth configuration",
		zap.String("topic", "server"),
		zap.String("method", "LoadServerAuthConfig"),
	)

	if enableRBAC {
		err2 := rbac.RbacMgr.Initialize(log.Logger, enableRBAC, rbac.ActionList, configurationFilenameRBAC) // TODO: remove use of global
		if err2 != nil {
			log.Logger.Fatal("Error while loading RBAC",
				zap.String("topic", "server"),
				zap.String("method", "LoadServerAuthConfig"),
				zap.String("filename", configurationFilenameRBAC),
				zap.Error(err2),
			)
		}
	} else {
		log.Logger.Info(
			"RBAC is disabled in configuration",
			zap.String("topic", "server"),
			zap.String("method", "LoadServerAuthConfig"),
		)
	}
}
