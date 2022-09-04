package stream

import (
	"ministream/account"
	"ministream/config"
	"ministream/cron"
	"ministream/log"
	"ministream/rbac"
	"time"

	"go.uber.org/zap"
)

type CronJobStreamsSaverHandler struct {
	//Coucou int
}

func (h CronJobStreamsSaverHandler) Exec(eventTime *time.Time, c *cron.CronJob) error {
	//log.Logger.Info("On save streams 1", zap.Int("coucou", h.Coucou))
	log.Logger.Info("On save streams", zap.Time("eventTime", *eventTime))
	//panic(errors.New("fake fail"))
	//return errors.New("fake fail")
	return nil
}

var CronJobStreamsSaver *cron.CronJob
var streamsSaverHander cron.CronJobHandler = CronJobStreamsSaverHandler{}

func init() {
	//saverHander.Coucou = 1
	CronJobStreamsSaver = cron.MakeCronJob(5, 10, streamsSaverHander)
	//saverHander.Coucou = 2
}


func LoadServerAuthConfig() {
	log.Logger.Info(
		"Loading server auth configuration",
		zap.String("topic", "server"),
		zap.String("method", "LoadServerAuthConfig"),
	)

	account, err := account.LoadAccount(config.Configuration.AccountFile)
	if err != nil {
		log.Logger.Fatal("Error while loading account",
			zap.String("topic", "server"),
			zap.String("method", "GoServer"),
			zap.String("filename", config.Configuration.AccountFile),
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
