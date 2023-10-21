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
	"github.com/nbigot/ministream/rbac"
	"github.com/nbigot/ministream/service"
	"github.com/nbigot/ministream/storageprovider/registry"
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

func argparse() string {
	showVersion := flag.Bool("version", false, "Show version")
	configFilePath := flag.String("config", "config.yaml", "Filepath to config.yaml")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Version: %s\n", Version)
		os.Exit(0)
	}

	return *configFilePath
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

func RunServer(appConfig *config.Config) (bool, error) {
	var err error
	var apiServer *webserver.Server

	log.Logger.Info(
		"Server version",
		zap.String("topic", "server"),
		zap.String("version", Version),
	)

	if !appConfig.Auth.Enable && (appConfig.RBAC.Enable || appConfig.WebServer.JWT.Enable) {
		log.Logger.Warn(
			"Auth is disabled in configuration",
			zap.String("topic", "server"),
		)
	}

	web.JWTMgr.Initialize(appConfig.WebServer.JWT)
	defer web.JWTMgr.Finalize()

	if err = registry.Initialize(); err != nil {
		log.Logger.Error("Startup error", zap.String("topic", "storage providers"), zap.Error(err))
		return false, err
	}
	defer registry.Finalize()

	if err = account.AccountMgr.Initialize(log.Logger, &appConfig.Account); err != nil {
		log.Logger.Error("Cannot initialize account manager", zap.String("topic", "account manager"), zap.Error(err))
		return false, err
	}
	defer account.AccountMgr.Finalize()

	if err = auth.AuthMgr.Initialize(log.Logger, &appConfig.Auth); err != nil {
		log.Logger.Error("Cannot initialize authentication manager", zap.String("topic", "auth manager"), zap.Error(err))
		return false, err
	}
	defer auth.AuthMgr.Finalize()

	rbac.RbacMgr = rbac.NewRBACManager(log.Logger, appConfig.RBAC.Enable, appConfig.RBAC.Filename)
	defer rbac.RbacMgr.Finalize()

	service := service.NewService(appConfig)
	defer service.Finalize()

	// create api server
	apiServer = webserver.NewServer(log.Logger, web.GetFiberConfig(), appConfig, service)
	if err = apiServer.Initialize(context.Background(), WithFiberLogger(), WithCors(), WithAPIRoutes()); err != nil {
		log.Logger.Error("Cannot initialize server", zap.String("topic", "server"), zap.Error(err))
		return false, err
	}
	defer apiServer.Finalize()
	err = apiServer.Start()

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
	configFilePath := argparse()

	// start/restart the server forever (reason is reload config) unless an error occurs
	for {
		var appConfig *config.Config
		var err error
		if appConfig, err = config.LoadConfig(configFilePath); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		restartServer, serverErr := RunServer(appConfig)
		if !restartServer || serverErr != nil {
			log.Logger.Info("End program", zap.String("topic", "server"))
			os.Exit(0)
		}
		if serverErr != nil {
			os.Exit(1)
		}
	}
}
