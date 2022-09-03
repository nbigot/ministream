package config

import (
	"fmt"
	"io/ioutil"
	"ministream/log"
	"time"

	"os"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type User struct {
	Login    string `yaml:"login"`
	Password string `yaml:"password"`
}

type Config struct {
	DataDirectory string     `yaml:"dataDirectory"`
	LoggerConfig  zap.Config `yaml:"logger"`
	AccountFile   string     `yaml:"-"`
	StreamsFile   string     `yaml:"-"`
	Streams       struct {
		BulkFlushFrequency        int `yaml:"bulkFlushFrequency"`
		BulkMaxSize               int `yaml:"bulkMaxSize"`
		ChannelBufferSize         int `yaml:"channelBufferSize"`
		MaxIteratorsPerStream     int `yaml:"maxAllowedIteratorsPerStream"`
		MaxMessagePerGetOperation int `yaml:"maxMessagePerGetOperation"`
	}
	Auth struct {
		Enable  bool   `yaml:"enable"`
		Method  string `yaml:"method"`
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

// example:
// https://www.programmerall.com/article/67701658436/

func LoadConfig(filename string) {

	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Error while opening configuration file %s\n", filename)
		panic(err)
	}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Printf("Error while loading configuration file %s\n", filename)
		panic(err)
	}

	err = yaml.Unmarshal(content, &Configuration)
	if err != nil {
		fmt.Printf("Error while parsing configuration file %s\n", filename)
		panic(err)
	}

	Configuration.AccountFile = Configuration.DataDirectory + "account.json"
	Configuration.StreamsFile = Configuration.DataDirectory + "streams.json"

	if Configuration.RBAC.Filename == "" {
		Configuration.RBAC.Filename = Configuration.DataDirectory + "rbac.json"
	}

	log.InitLogger(&Configuration.LoggerConfig)
	log.Logger.Info(
		"Configuration loaded",
		zap.String("topic", "server"),
		zap.String("method", "LoadConfig"),
		zap.String("filename", filename),
	)
}
