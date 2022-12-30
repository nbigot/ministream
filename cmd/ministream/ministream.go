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

// This variable is set at compile time with ldflags arg
//
// Example:
//
//	$ go build -ldflags="-X 'main.Version=v1.0.0'" cmd/ministream/ministream.go
var Version = "v0.0.0"

func argparse() {
	showVersion := flag.Bool("version", false, "Show version")
	configFilePath := flag.String("config", "config.yaml", "Filepath to config.yaml")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Version: %s\n", Version)
		os.Exit(0)
	}

	config.ConfigFile = *configFilePath
}

// @title MiniStream API
// @version 1.0
// @description This documentation describes MiniStream API
// @license.name MIT
// @license.url https://github.com/nbigot/ministream/blob/main/LICENSE
// @host 127.0.0.1:8080
// @BasePath /
func main() {
	// 127.0.0.1:443
	argparse()
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
