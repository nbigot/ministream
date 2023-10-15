package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/nbigot/ministream/account"
	"github.com/nbigot/ministream/auth"
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/log"
	"github.com/nbigot/ministream/service"
	"github.com/nbigot/ministream/startup"
	"github.com/nbigot/ministream/stream"
	"github.com/nbigot/ministream/web"
	"github.com/nbigot/ministream/web/webserver"
	"go.uber.org/zap"
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

func WithFiberLogger() webserver.ServerOption {
	return func(s *webserver.Server) {
		if s.GetWebConfig().Logs.Enable {
			s.GetApp().Use(logger.New(web.GetFiberLogger()))
		}
	}
}

func WithCors() webserver.ServerOption {
	return func(s *webserver.Server) {
		if s.GetWebConfig().Cors.Enable {
			s.GetApp().Use(cors.New(cors.Config{
				AllowOrigins: s.GetWebConfig().Cors.AllowOrigins,
				AllowHeaders: s.GetWebConfig().Cors.AllowHeaders,
			}))
		}
	}
}

func WithAPIRoutes() webserver.ServerOption {
	return func(s *webserver.Server) {
		s.GetWebAPIServer().AddRoutes(s.GetApp())
	}
}

func RunServer() (bool, error) {
	var err error
	var appConfig *config.Config
	var apiServer *webserver.Server

	if appConfig, err = config.LoadConfig(config.ConfigFile); err != nil {
		fmt.Println(err)
		return false, err
	}
	log.Logger.Info(
		"Server version",
		zap.String("topic", "server"),
		zap.String("version", Version),
	)
	if err = startup.Start(appConfig); err != nil {
		log.Logger.Error("Startup error", zap.String("topic", "startup"), zap.Error(err))
		fmt.Println(err)
		return false, err
	}
	if err = account.AccountMgr.Initialize(log.Logger, &appConfig.Account); err != nil {
		log.Logger.Error("Cannot initialize account manager", zap.String("topic", "account manager"), zap.Error(err))
		return false, err
	}
	if err = auth.AuthMgr.Initialize(log.Logger, &appConfig.Auth); err != nil {
		log.Logger.Error("Cannot initialize authentication manager", zap.String("topic", "auth manager"), zap.Error(err))
		return false, err
	}
	stream.LoadServerAuthConfig(appConfig.RBAC.Enable, appConfig.RBAC.Filename)

	service := service.NewService(appConfig)
	defer service.Stop()

	// create api server
	apiServer = webserver.NewServer(log.Logger, web.GetFiberConfig(), appConfig, service)
	if err = apiServer.Initialize(context.Background(), WithFiberLogger(), WithCors(), WithAPIRoutes()); err != nil {
		log.Logger.Error("Cannot initialize server", zap.String("topic", "server"), zap.Error(err))
		return false, err
	}

	err = apiServer.Start()

	defer func() {
		if apiServer.GetStatus() == webserver.ServerStatusRunning {
			apiServer.Shutdown()
		}
	}()

	switch {
	case err == nil:
		log.Logger.Info("Server stopped", zap.String("topic", "server"))
		return false, nil
	case errors.Is(err, webserver.ErrRequestRestart):
		log.Logger.Info("Restarting server ...", zap.String("topic", "server"))
		return true, nil
	default:
		log.Logger.Error("Server stopped with error", zap.String("topic", "server"))
		return false, err
	}

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

	// start/restart the server forever (reason is reload config) unless an error occurs
	for {
		restartServer, serverErr := RunServer()
		if !restartServer || serverErr != nil {
			log.Logger.Info("End program", zap.String("topic", "server"))
			os.Exit(0)
		}
		if serverErr != nil {
			os.Exit(1)
		}
	}
}
