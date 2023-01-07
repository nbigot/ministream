package account

import (
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/generators"

	"go.uber.org/zap"
)

type AccountManager struct {
	Account *config.Account
}

var AccountMgr AccountManager

func (m *AccountManager) init() {
}

func (m *AccountManager) Initialize(logger *zap.Logger, account *config.Account) error {
	m.Account = account
	if account.SecretAPIKey == "" {
		account.SecretAPIKey = generators.GenerateRandomSecretAPIKey(32)
		logger.Info(
			"Set random SecretAPIKey",
			zap.String("topic", "server"),
			zap.String("method", "Initialize"),
			zap.String("secretapikey", account.SecretAPIKey),
		)
	}

	return nil
}

func (m *AccountManager) GetAccount() *config.Account {
	return m.Account
}

func init() {
	AccountMgr.init()
}
