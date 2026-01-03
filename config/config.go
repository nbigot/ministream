package config

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nbigot/ministream/log"

	"os"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type AccountUUID = uuid.UUID

type Account struct {
	// A unique Id is necessary so that we cannot use a jwt on another server instance
	Id           AccountUUID `yaml:"id" example:"4ce589e2-b483-467b-8b59-758b339801d0"`
	Name         string      `yaml:"name"`
	SecretAPIKey string      `yaml:"secretAPIKey"`
}

type AuthConfig struct {
	Enable  bool   `yaml:"enable"`
	Method  string `yaml:"method" example:"FILE"`
	Methods struct {
		File struct {
			Filename string `yaml:"filename"`
		}
		HTTP struct {
			Url                string `yaml:"url"`
			ProxyUrl           string `yaml:"proxy"`
			AuthToken          string `yaml:"authToken"`
			Timeout            int    `yaml:"timeout"`
			CacheDurationInSec int    `yaml:"cacheDurationInSec"`
		}
	}
}

type JWTConfig struct {
	Enable                  bool      `yaml:"enable"`
	SecretKey               string    `yaml:"secretKey"`
	TokenExpireInMinutes    int       `yaml:"tokenExpireInMinutes"`
	ISS                     string    `yaml:"iss"`
	Sub                     string    `yaml:"sub"`
	Aud                     string    `yaml:"aud"`
	AccountId               string    `yaml:"accountId"`
	RevokeEmittedBeforeDate time.Time `yaml:"revokeEmittedBeforeDate"`
}

type WebServerConfig struct {
	HTTP struct {
		Enable  bool   `yaml:"enable"`
		Address string `yaml:"address"`
	}
	HTTPS struct {
		Enable   bool   `yaml:"enable"`
		Address  string `yaml:"address"`
		CertFile string `yaml:"certFile"`
		KeyFile  string `yaml:"keyFile"`
	}
	Logs struct {
		Enable bool `yaml:"enable"`
	}
	Cors struct {
		Enable       bool   `yaml:"enable"`
		AllowOrigins string `yaml:"allowOrigins"`
		AllowHeaders string `yaml:"allowHeaders"`
	}
	RateLimiter struct {
		Enable      bool `yaml:"enable"`
		RouteStream struct {
			MaxRequests       int `yaml:"maxRequests"`
			DurationInSeconds int `yaml:"durationInSeconds"`
		} `yaml:"routeStream"`
		RouteJob struct {
			MaxRequests       int `yaml:"maxRequests"`
			DurationInSeconds int `yaml:"durationInSeconds"`
		} `yaml:"routeJob"`
		RouteAccount struct {
			MaxRequests       int `yaml:"maxRequests"`
			DurationInSeconds int `yaml:"durationInSeconds"`
		} `yaml:"routeAccount"`
	} `yaml:"rateLimiter"`
	JWT     JWTConfig `yaml:"jwt"`
	Monitor struct {
		Enable bool `yaml:"enable"`
	}
	Metrics struct {
		Enable bool `yaml:"enable"`
	}
	Swagger struct {
		Enable bool `yaml:"enable"`
	}
}

type Config struct {
	Storage struct {
		Type         string     `yaml:"type" example:"JSONFile"`
		LoggerConfig zap.Config `yaml:"logger"`
		LogVerbosity int        `yaml:"logVerbosity"`
		JSONFile     struct {
			DataDirectory string `yaml:"dataDirectory"`
		} `yaml:"jsonfile"`
		InMemory struct {
			MaxRecordsByStream uint64 `yaml:"maxRecordsByStream"`
			MaxSize            string `yaml:"maxSize"`
		} `yaml:"inmemory"`
		MySQL struct {
			// https://github.com/Go-SQL-Driver/MySQL/?tab=readme-ov-file#dsn-data-source-name
			DataSourceName    string `yaml:"dsn" example:"user:password@tcp(localhost:3306)/ministream?tls=skip-verify"`
			ConnMaxLifetime   uint   `yaml:"connMaxLifetime" example:"0"`
			MaxIdleConns      uint   `yaml:"maxIdleConns" example:"3"`
			MaxOpenConns      uint   `yaml:"maxOpenConns" example:"3"`
			SchemaName        string `yaml:"schemaName" example:"ministream"`
			CatalogTableName  string `yaml:"catalogTableName" example:"streams"`
			StreamTablePrefix string `yaml:"streamTablePrefix" example:"stream_"`
		} `yaml:"mysql"`
	}
	DataDirectory string     `yaml:"dataDirectory"`
	LoggerConfig  zap.Config `yaml:"logger"`
	Account       Account    `yaml:"account"`
	Streams       struct {
		BulkFlushFrequency        int  `yaml:"bulkFlushFrequency"`
		BulkMaxSize               int  `yaml:"bulkMaxSize"`
		ChannelBufferSize         int  `yaml:"channelBufferSize"`
		MaxIteratorsPerStream     int  `yaml:"maxAllowedIteratorsPerStream"`
		MaxMessagePerGetOperation uint `yaml:"maxMessagePerGetOperation"`
		LogVerbosity              int  `yaml:"logVerbosity"`
		MaxAllowedStreams         uint `yaml:"maxAllowedStreams" example:"25"`
	}
	Auth AuthConfig `yaml:"auth"`
	RBAC struct {
		Enable   bool   `yaml:"enable"`
		Filename string `yaml:"filename"`
	}
	AuditLog struct {
		Enable                 bool `yaml:"enable"`
		EnableLogAccessGranted bool `yaml:"enableLogAccessGranted"`
	}
	WebServer WebServerConfig `yaml:"webserver"`
}

func LoadConfig(filename string) (*Config, error) {
	var configuration Config

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error while opening configuration file %s : %s", filename, err.Error())
	}
	defer func() {
		_ = file.Close()
	}()

	err = yaml.NewDecoder(file).Decode(&configuration)
	if err != nil {
		return nil, fmt.Errorf("error while parsing configuration file %s : %s", filename, err.Error())
	}

	// example: Configuration.RBAC.Filename = "/app/data/secrets/rbac.json"
	if configuration.RBAC.Enable && configuration.RBAC.Filename == "" {
		return nil, fmt.Errorf("error in configuration file: you must specify a filename for RBAC")
	}

	log.InitLogger(&configuration.LoggerConfig)
	log.Logger.Info(
		"Configuration loaded",
		zap.String("topic", "server"),
		zap.String("method", "LoadConfig"),
		zap.String("filename", filename),
	)

	return &configuration, nil
}
