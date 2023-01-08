package config

import (
	"fmt"
	"io/ioutil"
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
		MaxAllowedStreams         uint `json:"maxAllowedStreams" example:"25"`
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
	WebServer struct {
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
		JWT struct {
			Enable                  bool      `yaml:"enable"`
			SecretKey               string    `yaml:"secretKey"`
			TokenExpireInMinutes    int       `yaml:"tokenExpireInMinutes"`
			ISS                     string    `yaml:"iss"`
			Sub                     string    `yaml:"sub"`
			Aud                     string    `yaml:"aud"`
			RevokeEmittedBeforeDate time.Time `yaml:"revokeEmittedBeforeDate"`
		} `yaml:"jwt"`
		Monitor struct {
			Enable bool `yaml:"enable"`
		}
		Swagger struct {
			Enable   bool   `yaml:"enable"`
			Https    bool   `yaml:"https"`
			Address  string `yaml:"address"`
			CertFile string `yaml:"certFile"`
			KeyFile  string `yaml:"keyFile"`
		}
	} `yaml:"webserver"`
}

var ConfigFile = ""
var Configuration Config

func LoadConfig(filename string) error {

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("error while opening configuration file %s", filename)
	}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		return fmt.Errorf("error while loading configuration file %s", filename)
	}

	err = yaml.Unmarshal(content, &Configuration)
	if err != nil {
		return fmt.Errorf("error while parsing configuration file %s", filename)
	}

	// example: Configuration.RBAC.Filename = "/app/data/secrets/rbac.json"
	if Configuration.RBAC.Enable && Configuration.RBAC.Filename == "" {
		return fmt.Errorf("error in configuration file: you must specify a filename for RBAC")
	}

	log.InitLogger(&Configuration.LoggerConfig)
	log.Logger.Info(
		"Configuration loaded",
		zap.String("topic", "server"),
		zap.String("method", "LoadConfig"),
		zap.String("filename", filename),
	)

	return nil
}
