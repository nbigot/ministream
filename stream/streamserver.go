package stream

import (
	"github.com/nbigot/ministream/account"
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/log"
	"github.com/nbigot/ministream/rbac"

	"go.uber.org/zap"
)

func init() {
}

func LoadServerAuthConfig() {
	log.Logger.Info(
		"Loading server auth configuration",
		zap.String("topic", "server"),
		zap.String("method", "LoadServerAuthConfig"),
	)

	account, err := account.LoadAccount(config.Configuration.Account.Filename)
	if err != nil {
		log.Logger.Fatal("Error while loading account",
			zap.String("topic", "server"),
			zap.String("method", "GoServer"),
			zap.String("filename", config.Configuration.Account.Filename),
			zap.Error(err),
		)
	}

	if account.Status != "active" {
		log.Logger.Fatal("Account is not active, exit program now!",
			zap.String("topic", "server"),
			zap.String("method", "GoServer"),
			zap.String("accountId", account.Id.String()),
		)
		panic("Account is not active, please check configuration file!")
	}

	if config.Configuration.RBAC.Enable {
		err2 := rbac.RbacMgr.Initialize(log.Logger, rbac.ActionList, config.Configuration.RBAC.Filename)
		if err2 != nil {
			log.Logger.Fatal("Error while loading RBAC",
				zap.String("topic", "server"),
				zap.String("method", "GoServer"),
				zap.String("filename", config.Configuration.RBAC.Filename),
				zap.Error(err2),
			)
		}
	} else {
		log.Logger.Info(
			"RBAC is disabled in configuration",
			zap.String("topic", "server"),
			zap.String("method", "GoServer"),
		)
	}
}
