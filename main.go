package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/nbigot/ministream/auth"
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/log"
	"github.com/nbigot/ministream/service"
	"github.com/nbigot/ministream/startup"
	"github.com/nbigot/ministream/stream"
	"github.com/nbigot/ministream/web"
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
	if err := config.LoadConfig(config.ConfigFile); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := startup.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := auth.AuthMgr.Initialize(&config.Configuration); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	stream.LoadServerAuthConfig()
	service.NewGlobalService()
	web.GoServer()
	service.Stop()
	log.Logger.Info("End program")
}
