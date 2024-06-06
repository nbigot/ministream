package account

import (
	"github.com/google/uuid"
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/generators"

	"go.uber.org/zap"
)

type AccountManager struct {
	Account *config.Account
}

var AccountMgr AccountManager

func (m *AccountManager) Initialize(logger *zap.Logger, account *config.Account) error {
	m.Account = account
	if account.Id.ID() == 0 {
		account.Id = uuid.New()
		logger.Info(
			"Set random account id",
			zap.String("topic", "account"),
			zap.String("method", "Initialize"),
			zap.String("accountid", account.Id.String()),
		)
	}
	if account.SecretAPIKey == "" {
		// Generate a random SecretAPIKey because it is not set in the configuration file
		account.SecretAPIKey = generators.GenerateRandomSecretAPIKey(32)
		// Output the SecretAPIKey in the logs for the administrator to know it
		logger.Info(
			"Set random SecretAPIKey",
			zap.String("topic", "account"),
			zap.String("method", "Initialize"),
			zap.String("secretapikey", account.SecretAPIKey),
		)
	}

	return nil
}

func (m *AccountManager) GetAccount() *config.Account {
	return m.Account
}

func (m *AccountManager) Finalize() {
	m.Account = nil
}
