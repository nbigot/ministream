package mysqlprovider

import (
	"errors"
	"os"
	"regexp"

	"github.com/nbigot/ministream/config"
)

type MySQLConfig struct {
	Dsn               string // data source name
	SchemaName        string // mysql schema name
	CatalogTableName  string // mysql table name to store the catalog of streams
	StreamTablePrefix string // prefix for the stream tables
	ConnMaxLifetime   uint
	MaxIdleConns      uint
	MaxOpenConns      uint
}

func CheckMySQLConfiguration(conf *config.Config) (string, string, string, string, error) {
	// check if MySQL configuration is valid
	if conf.Storage.MySQL.DataSourceName == "" {
		return "", "", "", "", errors.New("empty data source name")
	}

	if conf.Storage.MySQL.MaxIdleConns == 0 {
		return "", "", "", "", errors.New("invalid MaxIdleConns value")
	}

	// if then DSN string value starts with "$" then it is an environment variable name
	var dataSourceName string
	if conf.Storage.MySQL.DataSourceName[0] == '$' {
		// load secret from environment variable
		dataSourceName = os.Getenv(conf.Storage.MySQL.DataSourceName[1:])
	} else {
		dataSourceName = conf.Storage.MySQL.DataSourceName
	}

	// check if schema name and catalog table name are valid (only alphanumeric characters and underscore)
	re := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	schemaName := conf.Storage.MySQL.SchemaName
	if schemaName == "" {
		schemaName = "ministream"
	}

	if !re.Match([]byte(schemaName)) {
		return "", "", "", "", errors.New("invalid schema name")
	}

	// check if catalog table name is valid
	catalogTableName := conf.Storage.MySQL.CatalogTableName
	if catalogTableName == "" {
		catalogTableName = "streams"
	}

	if !re.Match([]byte(catalogTableName)) {
		return "", "", "", "", errors.New("invalid catalog table name")
	}

	// check if stream table prefix is valid
	streamTablePrefix := conf.Storage.MySQL.StreamTablePrefix
	if streamTablePrefix == "" {
		streamTablePrefix = "stream_"
	}

	if !re.Match([]byte(streamTablePrefix)) {
		return "", "", "", "", errors.New("invalid stream table prefix")
	}

	return dataSourceName, schemaName, catalogTableName, streamTablePrefix, nil
}

func NewMySQLConfig(conf *config.Config) (*MySQLConfig, error) {
	dataSourceName, schemaName, catalogTableName, streamTablePrefix, err := CheckMySQLConfiguration(conf)
	if err != nil {
		return nil, err
	}

	mySQLConfig := MySQLConfig{
		Dsn:               dataSourceName,
		SchemaName:        schemaName,
		CatalogTableName:  catalogTableName,
		StreamTablePrefix: streamTablePrefix,
		ConnMaxLifetime:   conf.Storage.MySQL.ConnMaxLifetime,
		MaxIdleConns:      conf.Storage.MySQL.MaxIdleConns,
		MaxOpenConns:      conf.Storage.MySQL.MaxOpenConns,
	}

	return &mySQLConfig, nil
}
