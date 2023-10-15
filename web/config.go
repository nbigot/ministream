package web

import (
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

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
