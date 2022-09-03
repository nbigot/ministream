package log

import (
	"fmt"

	"go.uber.org/zap"
)

var Logger *zap.Logger
var SugarLogger *zap.SugaredLogger
var LoggerConfig *zap.Config

func InitLogger(loggerConfig *zap.Config) {

	var err error
	LoggerConfig = loggerConfig
	Logger, err = LoggerConfig.Build()
	if err != nil {
		fmt.Println("Error while initilize logger")
		panic(err)
	}
	SugarLogger = Logger.Sugar()
	Logger.Info(
		"Initialize logger",
		zap.String("topic", "server"),
		zap.String("method", "InitLogger"),
	)
}
