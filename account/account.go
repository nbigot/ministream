package account

import (
	"ministream/log"
	"os"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AccountUUID = uuid.UUID

type AccountSettings struct {
	MaxAllowedStreams uint   `json:"maxAllowedStreams" example:"25"`
	MaxConnections    uint   `json:"maxConnections" example:"50"`
	Backup            string `json:"backup" example:"1 day"`
	MaxStorage        uint64 `json:"storage" example:"5368709120"`
}

type Account struct {
	Id              AccountUUID     `json:"id" example:"4ce589e2-b483-467b-8b59-758b339801d0"`
	Name            string          `json:"name" example:"Nicolas"`
	AccountSettings AccountSettings `json:"accountSettings"`
	SecretAPIKey    string          `json:"secretAPIKey"`
	Status          string          `json:"status"`
}

var (
	account *Account
)

func LoadAccount(filename string) (*Account, error) {
	log.Logger.Info(
		"Loading account",
		zap.String("topic", "server"),
		zap.String("method", "LoadAccount"),
		zap.String("filename", filename),
	)

	file, err := os.Open(filename)
	if err != nil {
		log.Logger.Fatal(
			"Can't open account file",
			zap.String("topic", "server"),
			zap.String("method", "LoadAccount"),
			zap.String("filename", filename),
			zap.Error(err),
		)
		return nil, err
	}
	defer file.Close()

	type AccountSerializeStruct struct {
		Account *Account `json:"account"`
	}
	s := AccountSerializeStruct{}
	jsonDecoder := json.NewDecoder(file)
	err = jsonDecoder.Decode(&s)
	if err != nil {
		log.Logger.Fatal(
			"Can't decode json account",
			zap.String("topic", "server"),
			zap.String("method", "LoadAccount"),
			zap.String("filename", filename),
			zap.Error(err),
		)
		return nil, err
	}

	account = s.Account
	return account, err
}

func GetAccount() *Account {
	return account
}
