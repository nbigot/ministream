package web

import (
	"ministream/auditlog"
	"ministream/config"
	"ministream/log"
	"ministream/rbac"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"go.uber.org/zap"
)

var quit chan os.Signal

func GetFiberConfig() fiber.Config {
	return fiber.Config{
		StrictRouting:           true,
		CaseSensitive:           true,
		UnescapePath:            false,
		BodyLimit:               10485760,
		Concurrency:             262144,
		IdleTimeout:             60000,
		ReadBufferSize:          4096,
		WriteBufferSize:         4096,
		CompressedFileSuffix:    ".gz",
		GETOnly:                 false,
		DisableKeepalive:        false,
		DisableStartupMessage:   true,
		ReduceMemoryUsage:       false,
		EnableTrustedProxyCheck: false,
		EnablePrintRoutes:       false,
	}
}

func GetFiberLogger() logger.Config {
	return logger.Config{
		Next:         nil,
		Format:       "[${time}] ${status} - ${ip}:${port} - ${latency} ${method} ${path}\n",
		TimeFormat:   "2006-01-02T15:04:05-0700",
		TimeZone:     "Local",
		TimeInterval: 500 * time.Millisecond,
		Output:       os.Stdout,
	}
}

func GoServer() {
	errs := make(chan error)
	quit = make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	appMinistream := StartMinistreamServer(errs)
	appSwagger := StartSwaggerServer(errs)

	// This will run forever until channel receives error or an os signal
	select {
	case <-quit:
		log.Logger.Info("Shutdown Server ...", zap.String("topic", "server"), zap.String("method", "GoServer"))
		if err := appMinistream.Shutdown(); err != nil {
			log.Logger.Error("Server Shutdown", zap.String("topic", "server"), zap.String("method", "GoServer"), zap.Error(err))
		}
		if appSwagger != nil {
			if err := appSwagger.Shutdown(); err != nil {
				log.Logger.Error("Server Shutdown", zap.String("topic", "server"), zap.String("method", "GoServer"), zap.Error(err))
			}
		}
		log.Logger.Info("Web server stopped", zap.String("topic", "server"), zap.String("method", "GoServer"))
		return

	case err := <-errs:
		log.Logger.Error("Web server error", zap.String("topic", "server"), zap.String("method", "GoServer"), zap.Error(err))
		log.Logger.Info("Web server stopped", zap.String("topic", "server"), zap.String("method", "GoServer"))
		return
	}
}

func AddRoutes(app *fiber.App) {
	// Optimization: order of routes registration matters for performance
	// Please register most used routes first
	api := app.Group("/api/v1")

	apiStream := api.Group("/stream", JWTProtected(), RateLimiterStreams())
	apiStream.Get("/:streamuuid/iterator/:iteratoruuid/records", rbac.RBACProtected(rbac.ActionGetRecords, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), GetRecords)
	apiStream.Put("/:streamuuid/records", rbac.RBACProtected(rbac.ActionPutRecords, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), PutRecords)
	apiStream.Put("/:streamuuid/record", rbac.RBACProtected(rbac.ActionPutRecord, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), PutRecord)
	apiStream.Post("/:streamuuid/iterator", rbac.RBACProtected(rbac.ActionCreateRecordsIterator, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), CreateRecordsIterator)
	apiStream.Get("/:streamuuid/iterator/:iteratoruuid/stats", rbac.RBACProtected(rbac.ActionGetRecordsIteratorStats, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), GetRecordsIteratorStats)
	apiStream.Delete("/:streamuuid/iterator/:iteratoruuid", rbac.RBACProtected(rbac.ActionCloseRecordsIterator, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), CloseRecordsIterator)
	apiStream.Get("/:streamuuid", rbac.RBACProtected(rbac.ActionGetStreamDescription, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), GetStreamDescription)
	apiStream.Get("/:streamuuid/properties", rbac.RBACProtected(rbac.ActionGetStreamProperties, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), GetStreamProperties)
	apiStream.Post("/:streamuuid/properties", rbac.RBACProtected(rbac.ActionSetStreamProperties, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), SetStreamProperties)
	apiStream.Patch("/:streamuuid/properties", rbac.RBACProtected(rbac.ActionUpdateStreamProperties, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), UpdateStreamProperties)
	//apiStream.Get("/:streamuuid/raw", rbac.RBACProtected(rbac.ActionGetStreamRawFile, GetStreamPropertiesForABAC, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), GetStreamRawFile)
	apiStream.Post("/", rbac.RBACProtected(rbac.ActionCreateStream, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), CreateStream)
	apiStream.Delete("/:streamuuid", rbac.RBACProtected(rbac.ActionDeleteStream, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), DeleteStream)
	apiStream.Post("/:streamuuid/index/rebuild", rbac.RBACProtected(rbac.ActionRebuildIndex, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), RebuildIndex)

	apiStreams := api.Group("/streams", JWTProtected(), RateLimiterStreams())
	apiStreams.Get("/", rbac.RBACProtected(rbac.ActionListStreams, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), ListStreams)
	apiStreams.Get("/properties", rbac.RBACProtected(rbac.ActionListStreamsProperties, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), ListStreamsProperties)

	apiJob := api.Group("/job", JWTProtected(), RateLimiterJobs())
	apiJob.Get("/:jobuuid", GetJob)
	apiJob.Post("/", CreateJob)
	apiJob.Delete("/:jobuuid", DeleteJob)

	apiJobs := api.Group("/jobs", JWTProtected(), RateLimiterJobs())
	apiJobs.Get("/", ListJobs)

	apiUser := api.Group("/user", RateLimiterAccounts())
	apiUser.Get("/login", LoginUser)

	apiUsers := api.Group("/users", RateLimiterAccounts())
	apiUsers.Get("/", JWTProtected(), rbac.RBACProtected(rbac.ActionListUsers, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), ListUsers)

	apiAccount := api.Group("/account", RateLimiterAccounts())
	apiAccount.Get("/validate", rbac.RBACProtected(rbac.ActionValidateApiKey, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), ValidateApiKey)
	apiAccount.Get("/login", LoginAccount)
	apiAccount.Get("/", JWTProtected(), rbac.RBACProtected(rbac.ActionGetAccount, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), GetAccount)

	apiAdmin := api.Group("/admin", JWTProtected())
	apiAdmin.Post("/server/stop", rbac.RBACProtected(rbac.ActionStopServer, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), ApiServerStop)
	apiAdmin.Post("/server/reload/auth", rbac.RBACProtected(rbac.ActionReloadServerAuth, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), ApiServerReloadAuth)
	apiAdmin.Post("/jwt/revoke", rbac.RBACProtected(rbac.ActionJWTRevokeAll, nil, auditlog.RBACHandlerLogAccessGranted, auditlog.RBACHandlerLogAccessDeny), ActionJWTRevokeAll)

	apiUtils := api.Group("/utils")
	apiUtils.Post("/pbkdf2", RateLimiterUtils(), ApiServerUtilsPbkdf2)
	apiUtils.Get("/ping", Ping)

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Welcome to ministream!")
	})

	if config.Configuration.WebServer.Monitor.Enable {
		app.Get("/monitor", monitor.New())
	}
}

func StartMinistreamServer(errs chan error) *fiber.App {
	// https://docs.gofiber.io/api/app
	// https://dev.to/koddr/go-fiber-by-examples-delving-into-built-in-functions-1p3k
	// https://dev.to/koddr/build-a-restful-api-on-go-fiber-postgresql-jwt-and-swagger-docs-in-isolated-docker-containers-475j
	fiberconfig := GetFiberConfig()
	app := fiber.New(fiberconfig)

	if config.Configuration.WebServer.Logs.Enable {
		app.Use(logger.New(GetFiberLogger()))
	}

	if config.Configuration.WebServer.Cors.Enable {
		app.Use(cors.New(cors.Config{
			AllowOrigins: config.Configuration.WebServer.Cors.AllowOrigins,
			AllowHeaders: config.Configuration.WebServer.Cors.AllowHeaders,
		}))
	}

	AddRoutes(app)

	if config.Configuration.WebServer.HTTP.Enable {
		go func() {
			log.Logger.Info(
				"Start HTTP web server",
				zap.String("topic", "server"),
				zap.String("method", "GoServer"),
			)
			if err := app.Listen(config.Configuration.WebServer.HTTP.Address); err != nil {
				errs <- err
			}
		}()
	}

	if config.Configuration.WebServer.HTTPS.Enable {
		go func() {
			log.Logger.Info(
				"Start HTTPS web server",
				zap.String("topic", "server"),
				zap.String("method", "GoServer"),
			)
			if err := app.ListenTLS(
				config.Configuration.WebServer.HTTPS.Address,
				config.Configuration.WebServer.HTTPS.CertFile,
				config.Configuration.WebServer.HTTPS.KeyFile,
			); err != nil {
				errs <- err
			}
		}()
	}

	return app
}

func StartSwaggerServer(errs chan error) *fiber.App {
	if !config.Configuration.WebServer.Swagger.Enable {
		return nil
	}

	fiberconfig := GetFiberConfig()
	appSwagger := fiber.New(fiberconfig)
	if config.Configuration.WebServer.Cors.Enable {
		appSwagger.Use(cors.New(cors.Config{
			AllowOrigins: config.Configuration.WebServer.Cors.AllowOrigins,
			AllowHeaders: config.Configuration.WebServer.Cors.AllowHeaders,
		}))
	}

	AddSwaggerRoute(appSwagger)

	if config.Configuration.WebServer.Swagger.Https {
		go func() {
			log.Logger.Info(
				"Start Swagger HTTPS web server",
				zap.String("topic", "server"),
				zap.String("method", "StartSwaggerServer"),
			)
			if err := appSwagger.ListenTLS(
				config.Configuration.WebServer.Swagger.Address,
				config.Configuration.WebServer.Swagger.CertFile,
				config.Configuration.WebServer.Swagger.KeyFile,
			); err != nil {
				errs <- err
			}
		}()
	} else {
		go func() {
			log.Logger.Info(
				"Start Swagger HTTP web server",
				zap.String("topic", "server"),
				zap.String("method", "StartSwaggerServer"),
			)
			if err := appSwagger.Listen(config.Configuration.WebServer.Swagger.Address); err != nil {
				errs <- err
			}
		}()
	}

	return appSwagger
}

func StopServer() {
	// send signal SIGTERM
	quit <- syscall.SIGTERM
}
