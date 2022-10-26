// Package config provides a configuration for the API
package config

import (
	"encoding/hex"
	"time"

	"github.com/TAAL-GmbH/mapi"
	"github.com/labstack/echo/v4"
	"github.com/libsv/go-bk/wif"
	"github.com/mrz1836/go-cachestore"
	"github.com/mrz1836/go-datastore"
)

type Environment string

// Config constants used for optimization and value testing
const (
	ApplicationName                    = "MAPI"
	CurrentMajorVersion                = "v2"
	Version                            = "v2.0.0"
	EnvironmentKey                     = "MAPI_ENVIRONMENT"
	EnvironmentPrefix                  = "mapi"
	EnvironmentDevelopment Environment = "development"
	EnvironmentProduction  Environment = "production"
	EnvironmentStaging     Environment = "staging"
	EnvironmentTest        Environment = "test"
)

// Local variables for configuration
var (
	environments = []Environment{
		EnvironmentDevelopment,
		EnvironmentProduction,
		EnvironmentStaging,
		EnvironmentTest,
	}
)

type SecurityType string

var (
	SecurityTypeJWT    SecurityType = "jwt"
	SecurityTypeCustom SecurityType = "custom"
)

// The global configuration settings
type (

	// AppConfig is the configuration values and associated env vars
	AppConfig struct {
		LogRequestBody   bool              `json:"log_request_body" mapstructure:"log_request_body"`   // Whether the whole request body should be logged
		LogResponseBody  bool              `json:"log_response_body" mapstructure:"log_response_body"` // Whether the whole response body should be logged
		Cachestore       *CachestoreConfig `json:"cache" mapstructure:"cache"`
		Datastore        *DatastoreConfig  `json:"datastore" mapstructure:"datastore"`
		Debug            bool              `json:"debug" mapstructure:"debug"`
		VerboseDebug     bool              `json:"verbose_debug" mapstructure:"verbose_debug"`
		Environment      Environment       `json:"environment" mapstructure:"environment"`
		MinerID          *MinerIDConfig    `json:"miner_id" mapstructure:"miner_id"`
		Profile          bool              `json:"profile" mapstructure:"profile"` // whether to start the profiling http server
		Nodes            []*NodeConfig     `json:"nodes" mapstructure:"nodes"`
		Redis            *RedisConfig      `json:"redis" mapstructure:"redis"`
		Security         *SecurityConfig   `json:"security" mapstructure:"security"`
		Server           *ServerConfig     `json:"server" mapstructure:"server"`
		WorkingDirectory string            `json:"working_directory" mapstructure:"working_directory"`
		ZeroMQ           *ZeroMQConfig     `json:"zero_mq" mapstructure:"zero_mq"`
	}

	// CachestoreConfig is a configuration for cachestore
	CachestoreConfig struct {
		Engine       cachestore.Engine `json:"engine" mapstructure:"engine"` // Cache engine to use (redis, freecache)
		Transactions string            `json:"transactions" mapstructure:"transactions"`
	}

	// DatastoreConfig is a configuration for the datastore
	DatastoreConfig struct {
		AutoMigrate bool                     `json:"auto_migrate" mapstructure:"auto_migrate"` // loads a blank database
		Debug       bool                     `json:"debug" mapstructure:"debug"`               // true for sql statements
		Engine      datastore.Engine         `json:"engine" mapstructure:"engine"`             // mysql, sqlite
		TablePrefix string                   `json:"table_prefix" mapstructure:"table_prefix"` // pre_users (pre)
		Mongo       *datastore.MongoDBConfig `json:"mongodb" mapstructure:"mongodb"`
		SQL         *datastore.SQLConfig     `json:"sql" mapstructure:"sql"`
		SQLite      *datastore.SQLiteConfig  `json:"sqlite" mapstructure:"sqlite"`
	}

	// RedisConfig is a configuration for Redis cachestore or taskmanager
	RedisConfig struct {
		DependencyMode        bool          `json:"dependency_mode" mapstructure:"dependency_mode"`                 // Only in Redis with script enabled
		MaxActiveConnections  int           `json:"max_active_connections" mapstructure:"max_active_connections"`   // Max active connections
		MaxConnectionLifetime time.Duration `json:"max_connection_lifetime" mapstructure:"max_connection_lifetime"` // Max connection lifetime
		MaxIdleConnections    int           `json:"max_idle_connections" mapstructure:"max_idle_connections"`       // Max idle connections
		MaxIdleTimeout        time.Duration `json:"max_idle_timeout" mapstructure:"max_idle_timeout"`               // Max idle timeout
		URL                   string        `json:"url" mapstructure:"url"`                                         // Redis URL connection string
		UseTLS                bool          `json:"use_tls" mapstructure:"use_tls"`                                 // Flag for using TLS
	}

	// MinerIDConfig is a configuration for the miner id
	MinerIDConfig struct {
		PrivateKey string `json:"private_key" mapstructure:"private_key"`
	}

	// NodeConfig is a configuration for the bitcoin node rpc interface
	NodeConfig struct {
		Host     string `json:"host" mapstructure:"host"`
		Port     int    `json:"port" mapstructure:"port"`
		User     string `json:"user" mapstructure:"user"`
		Password string `json:"password" mapstructure:"password"`
		UseSSL   bool   `json:"use_ssl" mapstructure:"use_ssl"`
	}

	// SecurityConfig is a configuration for the security of the MAPI server
	SecurityConfig struct {
		Type          SecurityType                               `json:"type" mapstructure:"type"`             // jwt or custom
		Issuer        string                                     `json:"issuer" mapstructure:"issuer"`         // Token issuer
		BearerKey     string                                     `json:"bearer_key" mapstructure:"bearer_key"` // JWT bearer secret key
		CustomGetUser func(ctx echo.Context) (*mapi.User, error) `json:"-"`
	}

	// ServerConfig is a configuration for the MAPI server
	ServerConfig struct {
		IPAddress string `json:"ip_address" mapstructure:"ip_address"`
		Port      uint   `json:"port" mapstructure:"port"`
	}

	// ZeroMQConfig is a configuration for zero MQ
	ZeroMQConfig struct {
		Enabled  bool     `json:"enabled" mapstructure:"enabled"`
		Endpoint string   `json:"endpoint" mapstructure:"endpoint"`
		Topics   []string `json:"topics" mapstructure:"topics"`
	}
)

// GetUserAgent will return the outgoing user agent
func (a *AppConfig) GetUserAgent() string {
	return "MAPI " + string(a.Environment) + " " + Version
}

func (m *MinerIDConfig) GetMinerID() (string, error) {

	minerIDPrivKey, err := wif.DecodeWIF(m.PrivateKey)
	if err != nil {
		return "", err
	}

	minerID := hex.EncodeToString(minerIDPrivKey.SerialisePubKey())

	return minerID, nil
}
