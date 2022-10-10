// Package config provides a configuration for the API
package config

import (
	"time"

	"github.com/mrz1836/go-cachestore"
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

// The global configuration settings
type (

	// AppConfig is the configuration values and associated env vars
	AppConfig struct {
		Cachestore       *CachestoreConfig `json:"cache" mapstructure:"cache"`
		Datastore        *DatastoreConfig  `json:"datastore" mapstructure:"datastore"`
		Debug            bool              `json:"debug" mapstructure:"debug"`
		VerboseDebug     bool              `json:"verbose_debug" mapstructure:"verbose_debug"`
		Environment      Environment       `json:"environment" mapstructure:"environment"`
		MinerID          string            `json:"miner_id" mapstructure:"miner_id"`
		Profile          bool              `json:"profile" mapstructure:"profile"` // whether to start the profiling http server
		RestClient       *RestClientConfig `json:"rest_client" mapstructure:"rest_client"`
		RPCClient        *RPCClientConfig  `json:"rpc_client" mapstructure:"rpc_client"`
		Redis            *RedisConfig      `json:"redis" mapstructure:"redis"`
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
		Debug bool   `json:"debug" mapstructure:"debug"`
		Type  string `json:"type" mapstructure:"type"`
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

	// RestClientConfig is a configuration for the bitcoin node rest interface
	RestClientConfig struct {
		Server string `json:"server" mapstructure:"server"`
	}

	// RPCClientConfig is a configuration for the bitcoin node rpc interface
	RPCClientConfig struct {
		Host     string `json:"host" mapstructure:"host"`
		User     string `json:"user" mapstructure:"user"`
		Password string `json:"password" mapstructure:"password"`
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
