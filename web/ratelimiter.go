package web

import (
	"fmt"
	"ministream/config"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// inspiration:
// https://docs.aws.amazon.com/streams/latest/dev/service-sizes-and-limits.html
// https://docs.datadoghq.com/fr/api/latest/rate-limits/

func RateLimiterStreams() func(*fiber.Ctx) error {
	return limiter.New(limiter.Config{
		Next: func(c *fiber.Ctx) bool {
			return !config.Configuration.WebServer.RateLimiter.Enable
		},
		SkipFailedRequests: true,
		Max:                int(config.Configuration.WebServer.RateLimiter.RouteStream.MaxRequests),                               // max count of requests
		Expiration:         time.Duration(config.Configuration.WebServer.RateLimiter.RouteStream.DurationInSeconds) * time.Second, // expiration time of the limit
		//LimiterMiddleware:  limiter.SlidingWindow{},
		KeyGenerator: func(c *fiber.Ctx) string {
			// Handle multiple accounts and connections: each account has it's own rate limit keys
			// It does not matter which IP the queries come from
			// key example: "4ce589e2-b483-467b-8b59-758b339801db#/api/v1/streams/869a0b57-e15c-4272-8235-f7de9ec8e056#POST"
			// key example: "4ce589e2-b483-467b-8b59-758b339801db#/api/v1/streams/869a0b57-e15c-4272-8235-f7de9ec8e056#GET"
			// key example: "4ce589e2-b483-467b-8b59-758b339801db#/api/v1/streams/869a0b57-e15c-4272-8235-f7de9ec8e056/records#GET"

			// get account uuid from JWT
			accountId, _ := GetJWTClaim(c, "account")
			var strAccountId string
			if accountId == nil {
				// API is not protected by JWT, therefore/or no account is used
				strAccountId = "noaccount"
			} else {
				strAccountId = fmt.Sprintf("%s", accountId)
			}

			return fmt.Sprintf("%s#%s#%s", strAccountId, c.Path(), c.Method())
		},
	})
}

func RateLimiterJobs() func(*fiber.Ctx) error {
	return limiter.New(limiter.Config{
		Next: func(c *fiber.Ctx) bool {
			return !config.Configuration.WebServer.RateLimiter.Enable
		},
		SkipFailedRequests: true,
		Max:                int(config.Configuration.WebServer.RateLimiter.RouteAccount.MaxRequests),                               // max count of requests
		Expiration:         time.Duration(config.Configuration.WebServer.RateLimiter.RouteAccount.DurationInSeconds) * time.Second, // expiration time of the limit
		KeyGenerator: func(c *fiber.Ctx) string {
			// get account uuid from JWT (if exists)
			accountId, err := GetJWTClaim(c, "account")
			if err != nil || accountId == nil {
				// API is not protected by JWT, therefore/or no account is used
				return fmt.Sprintf("noaccount#%s#%s", c.Path(), c.Method())
			}

			return fmt.Sprintf("%s#%s#%s", accountId, c.Path(), c.Method())
		},
	})
}

func RateLimiterAccounts() func(*fiber.Ctx) error {
	return limiter.New(limiter.Config{
		Next: func(c *fiber.Ctx) bool {
			return !config.Configuration.WebServer.RateLimiter.Enable
		},
		SkipFailedRequests: true,
		Max:                int(config.Configuration.WebServer.RateLimiter.RouteAccount.MaxRequests),                               // max count of requests
		Expiration:         time.Duration(config.Configuration.WebServer.RateLimiter.RouteAccount.DurationInSeconds) * time.Second, // expiration time of the limit
		KeyGenerator: func(c *fiber.Ctx) string {
			// get account uuid from JWT (if exists)
			accountId, err := GetJWTClaim(c, "account")
			if err != nil || accountId == nil {
				// API is not protected by JWT, therefore/or no account is used
				return fmt.Sprintf("noaccount#%s#%s", c.Path(), c.Method())
			}

			return fmt.Sprintf("%s#%s#%s", accountId, c.Path(), c.Method())
		},
	})
}

func RateLimiterUtils() func(*fiber.Ctx) error {
	return limiter.New(limiter.Config{
		Next: func(c *fiber.Ctx) bool {
			return !config.Configuration.WebServer.RateLimiter.Enable
		},
		SkipFailedRequests: true,
		Max:                5,           // max count of connections
		Expiration:         time.Second, // expiration time of the limit
		KeyGenerator: func(c *fiber.Ctx) string {
			return "utils"
		},
	})
}
