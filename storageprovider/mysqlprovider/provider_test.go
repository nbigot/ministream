package mysqlprovider

import (
	"testing"

	"github.com/google/uuid"
)

func TestSafeOverrideDSN(t *testing.T) {
	// Test case 1: add missing parseTime parameter
	t.Run("Override DSN with valid input", func(t *testing.T) {
		dsn := "mysql://user:password@localhost:3306/database"
		overriddenDSN, err := SafeOverrideDSN(dsn)
		if err != nil {
			t.Errorf("Expected no error, but got: %v", err)
		}
		expectedDSN := "mysql://user:password@localhost:3306/database?parseTime=true"
		if overriddenDSN != expectedDSN {
			t.Errorf("Expected overridden DSN to be %q, but got %q", expectedDSN, overriddenDSN)
		}
	})

	// Test case 2: add missing parseTime parameter to a DNS with existing parameter(s)
	t.Run("Override DSN with valid input", func(t *testing.T) {
		dsn := "mysql://user:password@localhost:3306/database?multiStatements=true"
		overriddenDSN, err := SafeOverrideDSN(dsn)
		if err != nil {
			t.Errorf("Expected no error, but got: %v", err)
		}
		expectedDSN := "mysql://user:password@localhost:3306/database?multiStatements=true&parseTime=true"
		if overriddenDSN != expectedDSN {
			t.Errorf("Expected overridden DSN to be %q, but got %q", expectedDSN, overriddenDSN)
		}
	})

	// Test case 3: parseTime parameter is already present
	t.Run("Override DSN with existing parseTime parameter", func(t *testing.T) {
		dsn := "mysql://user:password@localhost:3306/database?parseTime=true"
		overriddenDSN, err := SafeOverrideDSN(dsn)
		if err != nil {
			t.Errorf("Expected no error, but got: %v", err)
		}
		expectedDSN := "mysql://user:password@localhost:3306/database?parseTime=true"
		if overriddenDSN != expectedDSN {
			t.Errorf("Expected overridden DSN to be %q, but got %q", expectedDSN, overriddenDSN)
		}
	})

	// Test case 4: invalid DSN input
	t.Run("Override DSN with invalid input", func(t *testing.T) {
		dsn := " badscheme://"
		_, err := SafeOverrideDSN(dsn)
		if err == nil {
			t.Error("Expected an error, but got nil")
		}
	})
}

func TestMySQLStorage_getStreamTableName(t *testing.T) {
	t.Run("Get stream table name", func(t *testing.T) {
		streamUUID := uuid.MustParse("9cf1eba6-0f6d-425e-ada9-e5b6cf293ffc")
		mysqlStorage := &MySQLStorage{
			mysqlConfig: &MySQLConfig{
				StreamTablePrefix: "prefix_",
			},
		}
		streamTableName := mysqlStorage.getStreamTableName(streamUUID)
		expectedStreamTableName := "prefix_9cf1eba6_0f6d_425e_ada9_e5b6cf293ffc"
		if streamTableName != expectedStreamTableName {
			t.Errorf("Expected stream table name to be %q, but got %q", expectedStreamTableName, streamTableName)
		}
	})
}
