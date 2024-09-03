package config

import (
	"time"

	"github.com/ordishs/go-bitcoin"
)

type ArcConfig struct {
	LogLevel           string              `mapstructure:"logLevel"`
	LogFormat          string              `mapstructure:"logFormat"`
	ProfilerAddr       string              `mapstructure:"profilerAddr"`
	PrometheusEndpoint string              `mapstructure:"prometheusEndpoint"`
	PrometheusAddr     string              `mapstructure:"prometheusAddr"`
	GrpcMessageSize    int                 `mapstructure:"grpcMessageSize"`
	Network            string              `mapstructure:"network"`
	MessageQueue       *MessageQueueConfig `mapstructure:"messageQueue"`
	Tracing            *TracingConfig      `mapstructure:"tracing"`
	PeerRpc            *PeerRpcConfig      `mapstructure:"peerRpc"`
	Peers              []*PeerConfig       `mapstructure:"peers"`
	Metamorph          *MetamorphConfig    `mapstructure:"metamorph"`
	Blocktx            *BlocktxConfig      `mapstructure:"blocktx"`
	Api                *ApiConfig          `mapstructure:"api"`
	K8sWatcher         *K8sWatcherConfig   `mapstructure:"k8sWatcher"`
	Callbacker         *CallbackerConfig   `mapstructure:"callbacker"`
}

type MessageQueueConfig struct {
	URL       string                `mapstructure:"url"`
	Streaming MessageQueueStreaming `mapstructure:"streaming"`
}

type MessageQueueStreaming struct {
	Enabled     bool `mapstructure:"enabled"`
	FileStorage bool `mapstructure:"fileStorage"`
}

type TracingConfig struct {
	DialAddr string `mapstructure:"dialAddr"`
}

type PeerRpcConfig struct {
	Password string `mapstructure:"password"`
	User     string `mapstructure:"user"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
}

type PeerConfig struct {
	Host string          `mapstructure:"host"`
	Port *PeerPortConfig `mapstructure:"port"`
}

type PeerPortConfig struct {
	P2P int `mapstructure:"p2p"`
	ZMQ int `mapstructure:"zmq"`
}

type MetamorphConfig struct {
	ListenAddr                  string        `mapstructure:"listenAddr"`
	DialAddr                    string        `mapstructure:"dialAddr"`
	Db                          *DbConfig     `mapstructure:"db"`
	ProcessorCacheExpiryTime    time.Duration `mapstructure:"processorCacheExpiryTime"`
	MaxRetries                  int           `mapstructure:"maxRetries"`
	ProcessStatusUpdateInterval time.Duration `mapstructure:"processStatusUpdateInterval"`
	CheckSeenOnNetworkOlderThan time.Duration `mapstructure:"checkSeenOnNetworkOlderThan"`
	CheckSeenOnNetworkPeriod    time.Duration `mapstructure:"checkSeenOnNetworkPeriod"`
	MonitorPeers                bool          `mapstructure:"monitorPeers"`
	CheckUtxos                  bool          `mapstructure:"checkUtxos"`
	Health                      *HealthConfig `mapstructure:"health"`
	RejectCallbackContaining    []string      `mapstructure:"rejectCallbackContaining"`
	Stats                       *StatsConfig  `mapstructure:"stats"`
}

type BlocktxConfig struct {
	ListenAddr                    string              `mapstructure:"listenAddr"`
	DialAddr                      string              `mapstructure:"dialAddr"`
	HealthServerDialAddr          string              `mapstructure:"healthServerDialAddr"`
	Db                            *DbConfig           `mapstructure:"db"`
	RecordRetentionDays           int                 `mapstructure:"recordRetentionDays"`
	RegisterTxsInterval           time.Duration       `mapstructure:"registerTxsInterval"`
	MonitorPeers                  bool                `mapstructure:"monitorPeers"`
	FillGapsInterval              time.Duration       `mapstructure:"fillGapsInterval"`
	MaxAllowedBlockHeightMismatch int                 `mapstructure:"maxAllowedBlockHeightMismatch"`
	MessageQueue                  *MessageQueueConfig `mapstructure:"mq"`
}

type DbConfig struct {
	Mode     string          `mapstructure:"mode"`
	Postgres *PostgresConfig `mapstructure:"postgres"`
}

type PostgresConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Name         string `mapstructure:"name"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	MaxIdleConns int    `mapstructure:"maxIdleConns"`
	MaxOpenConns int    `mapstructure:"maxOpenConns"`
	SslMode      string `mapstructure:"sslMode"`
}

type HealthConfig struct {
	SeverDialAddr             string `mapstructure:"serverDialAddr"`
	MinimumHealthyConnections int    `mapstructure:"minimumHealthyConnections"`
}

type StatsConfig struct {
	NotSeenTimeLimit  time.Duration `mapstructure:"notSeenTimeLimit"`
	NotMinedTimeLimit time.Duration `mapstructure:"notMinedTimeLimit"`
}

type ApiConfig struct {
	Address       string            `mapstructure:"address"`
	WocApiKey     string            `mapstructure:"wocApiKey"`
	WocMainnet    bool              `mapstructure:"wocMainnet"`
	DefaultPolicy *bitcoin.Settings `mapstructure:"defaultPolicy"`
}

type K8sWatcherConfig struct {
	Namespace string `mapstructure:"namespace"`
}

type CallbackerConfig struct {
	ListenAddr string        `mapstructure:"listenAddr"`
	DialAddr   string        `mapstructure:"dialAddr"`
	Health     *HealthConfig `mapstructure:"health"`
	Pause      time.Duration `mapstructure:"pause"`
	Db         *DbConfig     `mapstructure:"db"`
}
