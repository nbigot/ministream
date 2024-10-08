package webserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/service"
	"github.com/nbigot/ministream/web"
	"go.uber.org/zap"
)

type ServerStatus int

const (
	ServerStatusNone = iota
	ServerStatusInitialized
	ServerStatusRunning
	ServerStatusStopping
)

type Server struct {
	// implements IServer interface
	signals      chan os.Signal
	stopChan     chan bool
	errsChan     chan error
	ctx          context.Context
	status       ServerStatus
	fiberConfig  fiber.Config
	appConfig    *config.Config
	logger       *zap.Logger
	service      *service.Service
	webAPIServer *web.WebAPIServer
}

// Option is a functional option type that allows us to configure the Server.
type ServerOption func(*Server)

var ErrRequestRestart = errors.New("ErrRequestRestart")

func NewServer(logger *zap.Logger, fiberConfig fiber.Config, appConfig *config.Config, service *service.Service) *Server {
	return &Server{
		signals:      make(chan os.Signal, 1),
		stopChan:     make(chan bool, 1),
		errsChan:     make(chan error),
		status:       ServerStatusNone,
		fiberConfig:  fiberConfig,
		appConfig:    appConfig,
		logger:       logger,
		service:      service,
		webAPIServer: nil,
	}
}

func (s *Server) Initialize(ctx context.Context, options ...ServerOption) error {
	if s.status != ServerStatusNone {
		return fmt.Errorf("cannot initilize server: invalid status code %d", s.status)
	}
	s.ctx = ctx
	signal.Notify(s.signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	s.webAPIServer = web.NewWebAPIServer(s.appConfig, s.fiberConfig, s.service, func() { s.RequestShutdownServer() }, func() { s.RequestRestartServer() })

	// Apply all the functional options to configure the client.
	// options examples: fiber logger, cors config, add routes, ...
	for _, opt := range options {
		opt(s)
	}

	s.SetStatus(ServerStatusInitialized)
	return nil
}

func (s *Server) Start() error {
	if s.status != ServerStatusInitialized {
		return fmt.Errorf("cannot start server: invalid status code %d", s.status)
	}
	s.webAPIServer.StartServer()
	s.SetStatus(ServerStatusRunning)
	s.Listen()
	err := s.HandleSignals()
	// server is stopped
	s.SetStatus(ServerStatusInitialized)
	return err
}

func (s *Server) Finalize() {
	if s.GetStatus() == ServerStatusRunning {
		if shutdownErr := s.Shutdown(); shutdownErr != nil {
			s.logger.Error("Server shutdown with error", zap.String("topic", "server"), zap.Error(shutdownErr))
		}
	}
}

func (s *Server) GetWebAPIServer() *web.WebAPIServer {
	return s.webAPIServer
}

func (s *Server) GetApp() *fiber.App {
	return s.webAPIServer.GetFiberApp()
}

func (s *Server) GetStatus() ServerStatus {
	return s.status
}

func (s *Server) GetWebConfig() *config.WebServerConfig {
	return &s.appConfig.WebServer
}

func (s *Server) Listen() {
	webConfig := s.GetWebConfig()

	if webConfig.HTTP.Enable {
		go func() {
			s.logger.Info(
				"Start HTTP web server",
				zap.String("topic", "server"),
				zap.String("method", "Listen"),
				zap.String("address", webConfig.HTTP.Address),
			)
			if err := s.GetApp().Listen(webConfig.HTTP.Address); err != nil {
				s.errsChan <- err
			}
		}()
	}

	if webConfig.HTTPS.Enable {
		go func() {
			s.logger.Info(
				"Start HTTPS web server",
				zap.String("topic", "server"),
				zap.String("method", "Listen"),
				zap.String("address", webConfig.HTTPS.Address),
			)
			if err := s.GetApp().ListenTLS(
				webConfig.HTTPS.Address,
				webConfig.HTTPS.CertFile,
				webConfig.HTTPS.KeyFile,
			); err != nil {
				s.errsChan <- err
			}
		}()
	}
}

func (s *Server) HandleSignals() error {
	// This will run forever until channel receives error or an os signal
	defer s.shutdownListener()

	for {
		select {
		case <-s.ctx.Done():
			return nil
		case sig := <-s.signals:
			switch sig {
			case syscall.SIGHUP:
				// reload configuration and restart server
				s.logger.Info("Reloading configuration and restart server...", zap.String("topic", "server"), zap.String("method", "HandleSignals"))
				return fmt.Errorf("received signal hang up: %w", ErrRequestRestart)
			case syscall.SIGTERM:
				// stop server due to a signal SIGTERM
				return nil
			}
		case err := <-s.errsChan:
			// stop server due to an error
			s.logger.Error("Web server stopped with error", zap.String("topic", "server"), zap.String("method", "HandleSignals"), zap.Error(err))
			return err
		}
	}
}

func (s *Server) shutdownListener() {
	s.logger.Info("Shutdown Server ...", zap.String("topic", "server"), zap.String("method", "shutdownListener"))
	if err := s.GetApp().Shutdown(); err != nil {
		s.logger.Error("Server Shutdown", zap.String("topic", "server"), zap.String("method", "shutdownListener"), zap.Error(err))
	}
	s.logger.Info("Web server stopped", zap.String("topic", "server"), zap.String("method", "shutdownListener"))
}

func (s *Server) Shutdown() error {
	// request to stop the server: the server will not yet be stopped at the end of this function
	if s.status != ServerStatusRunning {
		return fmt.Errorf("cannot stop server: invalid status code %d", s.status)
	}
	s.SetStatus(ServerStatusStopping)
	s.RequestShutdownServer()
	return nil
}

func (s *Server) RequestShutdownServer() {
	// send signal SIGTERM
	s.signals <- syscall.SIGTERM
}

func (s *Server) RequestRestartServer() {
	// send signal SIGHUP
	s.signals <- syscall.SIGHUP
}

func (s *Server) SetStatus(status ServerStatus) {
	s.status = status
}
