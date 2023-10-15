package web

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/swagger"

	"github.com/nbigot/ministream/auditlog"
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/rbac"
	"github.com/nbigot/ministream/service"
)

type WebAPIServer struct {
	service            *service.Service
	funcShutdownServer func()
	funcRestartServer  func()
	app                *fiber.App
	appConfig          *config.Config
}

func (w *WebAPIServer) AddRoutes(app *fiber.App) {
	enableRBAC := w.appConfig.RBAC.Enable

	auditlogRBACHandlerLogAccessGranted := func(c *fiber.Ctx) error {
		if !w.appConfig.AuditLog.Enable || !w.appConfig.AuditLog.EnableLogAccessGranted {
			return c.Next()
		} else {
			return auditlog.RBACHandlerLogAccessGranted(c)
		}
	}

	auditlogRBACHandlerLogAccessDeny := func(c *fiber.Ctx, err error) error {
		return auditlog.RBACHandlerLogAccessDeny(c, err, w.appConfig.AuditLog.Enable)
	}

	rateLimiterEnable := w.appConfig.WebServer.RateLimiter.Enable
	rateLimiterMaxRequests := w.appConfig.WebServer.RateLimiter.RouteStream.MaxRequests      // max count of requests
	rateDurationInSeconds := w.appConfig.WebServer.RateLimiter.RouteStream.DurationInSeconds // expiration time of the limit

	// Optimization: order of routes registration matters for performance
	// Please register most used routes first
	api := app.Group("/api/v1")

	apiStream := api.Group("/stream", JWTProtected(), RateLimiterStreams(rateLimiterEnable, rateLimiterMaxRequests, rateDurationInSeconds))
	apiStream.Get("/:streamuuid/iterator/:streamiteratoruuid/records", rbac.RBACProtected(enableRBAC, rbac.ActionGetRecords, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.GetRecords)
	apiStream.Put("/:streamuuid/records", rbac.RBACProtected(enableRBAC, rbac.ActionPutRecords, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.PutRecords)
	apiStream.Put("/:streamuuid/record", rbac.RBACProtected(enableRBAC, rbac.ActionPutRecord, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.PutRecord)
	apiStream.Post("/:streamuuid/iterator", rbac.RBACProtected(enableRBAC, rbac.ActionCreateRecordsIterator, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.CreateRecordsIterator)
	apiStream.Get("/:streamuuid/iterator/:streamiteratoruuid/stats", rbac.RBACProtected(enableRBAC, rbac.ActionGetRecordsIteratorStats, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.GetRecordsIteratorStats)
	apiStream.Delete("/:streamuuid/iterator/:streamiteratoruuid", rbac.RBACProtected(enableRBAC, rbac.ActionCloseRecordsIterator, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.CloseRecordsIterator)
	apiStream.Get("/:streamuuid", rbac.RBACProtected(enableRBAC, rbac.ActionGetStreamDescription, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.GetStreamInformation)
	apiStream.Get("/:streamuuid/properties", rbac.RBACProtected(enableRBAC, rbac.ActionGetStreamProperties, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.GetStreamProperties)
	apiStream.Post("/:streamuuid/properties", rbac.RBACProtected(enableRBAC, rbac.ActionSetStreamProperties, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.SetStreamProperties)
	apiStream.Patch("/:streamuuid/properties", rbac.RBACProtected(enableRBAC, rbac.ActionUpdateStreamProperties, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.UpdateStreamProperties)
	apiStream.Post("/", rbac.RBACProtected(enableRBAC, rbac.ActionCreateStream, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.CreateStream)
	apiStream.Delete("/:streamuuid", rbac.RBACProtected(enableRBAC, rbac.ActionDeleteStream, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.DeleteStream)
	apiStream.Post("/:streamuuid/index/rebuild", rbac.RBACProtected(enableRBAC, rbac.ActionRebuildIndex, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.RebuildIndex)

	apiStreams := api.Group("/streams", JWTProtected(), RateLimiterStreams(rateLimiterEnable, rateLimiterMaxRequests, rateDurationInSeconds))
	apiStreams.Get("/", rbac.RBACProtected(enableRBAC, rbac.ActionListStreams, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.ListStreams)
	apiStreams.Get("/properties", rbac.RBACProtected(enableRBAC, rbac.ActionListStreamsProperties, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.ListStreamsProperties)

	apiUser := api.Group("/user", RateLimiterAccounts(rateLimiterEnable, rateLimiterMaxRequests, rateDurationInSeconds))
	apiUser.Get("/login", w.LoginUser)

	apiUsers := api.Group("/users", RateLimiterAccounts(rateLimiterEnable, rateLimiterMaxRequests, rateDurationInSeconds))
	apiUsers.Get("/", JWTProtected(), rbac.RBACProtected(enableRBAC, rbac.ActionListUsers, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.ListUsers)

	apiAccount := api.Group("/account", RateLimiterAccounts(rateLimiterEnable, rateLimiterMaxRequests, rateDurationInSeconds))
	apiAccount.Get("/validate", w.ValidateApiKey)
	apiAccount.Get("/login", w.LoginAccount)
	apiAccount.Get("/", JWTProtected(), rbac.RBACProtected(enableRBAC, rbac.ActionGetAccount, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.GetAccount)

	apiAdmin := api.Group("/admin", JWTProtected())
	apiAdmin.Post("/server/shutdown", rbac.RBACProtected(enableRBAC, rbac.ActionShutdownServer, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.ApiServerShutdown)
	apiAdmin.Post("/server/restart", rbac.RBACProtected(enableRBAC, rbac.ActionRestartServer, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.ApiServerRestart)
	apiAdmin.Post("/jwt/revoke", rbac.RBACProtected(enableRBAC, rbac.ActionJWTRevokeAll, nil, auditlogRBACHandlerLogAccessGranted, auditlogRBACHandlerLogAccessDeny), w.ActionJWTRevokeAll)

	apiUtils := api.Group("/utils")
	apiUtils.Post("/pbkdf2", RateLimiterUtils(rateLimiterEnable), w.ApiServerUtilsPbkdf2)
	apiUtils.Get("/ping", w.Ping)

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Welcome to ministream!")
	})

	if w.appConfig.WebServer.Monitor.Enable {
		app.Get("/monitor", monitor.New())
	}

	if w.appConfig.WebServer.Swagger.Enable {
		// Create swagger routes group.
		apiSwagger := app.Group("/docs")

		// Routes for GET method:
		apiSwagger.Get("*", swagger.HandlerDefault)
		apiSwagger.Get("*", swagger.New(swagger.Config{
			URL:         "/swagger/doc.json",
			DeepLinking: false,
		}))
	}
}

func (w *WebAPIServer) ShutdownServer() {
	w.funcShutdownServer()
}

func (w *WebAPIServer) RestartServer() {
	w.funcRestartServer()
}

func NewWebAPIServer(appConfig *config.Config, fiberConfig fiber.Config, service *service.Service, funcShutdownServer func(), funcRestartServer func()) *WebAPIServer {
	return &WebAPIServer{
		service:            service,
		funcShutdownServer: funcShutdownServer,
		funcRestartServer:  funcRestartServer,
		app:                fiber.New(fiberConfig),
		appConfig:          appConfig,
	}
}
