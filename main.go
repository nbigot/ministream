package main

import (
	"flag"
	"fmt"
	"ministream/auth"
	"ministream/config"
	"ministream/log"
	"ministream/stream"
	"ministream/web"
)

func argparse() {
	configFilePath := flag.String("config", "config.yaml", "Filepath to config.yaml")
	flag.Parse()
	config.ConfigFile = *configFilePath
}

// @title MiniStream API
// @version 1.0
// @description This documentation describes MiniStream API
// @termsOfService http://swagger.io/terms/
// @license.name MIT
// @host 127.0.0.1:8080
// @BasePath /
func main() {
	// 127.0.0.1:443
	argparse()
	fmt.Println("Start")
	config.LoadConfig(config.ConfigFile)
	if err := auth.AuthMgr.Initialize(&config.Configuration); err != nil {
		panic(err)
	}
	stream.GoServer()
	stream.CronJobStreamsSaver.Start()
	web.GoServer()
	stream.CronJobStreamsSaver.Stop()
	log.Logger.Info("End program")
}
