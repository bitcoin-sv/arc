package config

import (
	"time"

	"github.com/ordishs/go-bitcoin"
)

type ArcConfig struct {
	LogLevel           string            `json:"logLevel" mapstructure:"logLevel"`
	LogFormat          string            `json:"logFormat" mapstructure:"logFormat"`
	ProfilerAddr       string            `json:"profilerAddr" mapstructure:"profilerAddr"`
	PrometheusEndpoint string            `json:"prometheusEndpoint" mapstructure:"prometheusEndpoint"`
	PrometheusAddr     string            `json:"prometheusAddr" mapstructure:"prometheusAddr"`
	GrpcMessageSize    int               `json:"grpcMessageSize" mapstructure:"grpcMessageSize"`
	Network            string            `json:"network" mapstructure:"network"`
	QueueURL           string            `json:"queueURL" mapstructure:"queueURL"`
	Tracing            *TracingConfig    `json:"tracing" mapstructure:"tracing"`
	PeerRpc            *PeerRpcConfig    `json:"peerRpc" mapstructure:"peerRpc"`
	Peers              []*PeerConfig     `json:"peers" mapstructure:"peers"`
	Metamorph          *MetamorphConfig  `json:"metamorph" mapstructure:"metamorph"`
	Blocktx            *BlocktxConfig    `json:"blocktx" mapstructure:"blocktx"`
	Api                *ApiConfig        `json:"api" mapstructure:"api"`
	K8sWatcher         *K8sWatcherConfig `json:"k8sWatcher" mapstructure:"k8sWatcher"`
}

type TracingConfig struct {
	DialAddr string `json:"dialAddr" mapstructure:"dialAddr"`
}

type PeerRpcConfig struct {
	Password string `json:"password" mapstructure:"password"`
	User     string `json:"user" mapstructure:"user"`
	Host     string `json:"host" mapstructure:"host"`
	Port     int    `json:"port" mapstructure:"port"`
}

type PeerConfig struct {
	Host string          `json:"host" mapstructure:"host"`
	Port *PeerPortConfig `json:"port" mapstructure:"port"`
}

type PeerPortConfig struct {
	P2P int `json:"p2p" mapstructure:"p2p"`
	ZMQ int `json:"zmq" mapstructure:"zmq"`
}

type MetamorphConfig struct {
	ListenAddr                  string        `json:"listenAddr" mapstructure:"listenAddr"`
	DialAddr                    string        `json:"dialAddr" mapstructure:"dialAddr"`
	Db                          *DbConfig     `json:"db" mapstructure:"db"`
	ProcessorCacheExpiryTime    time.Duration `json:"processorCacheExpiryTime" mapstructure:"processorCacheExpiryTime"`
	MaxRetries                  int           `json:"maxRetries" mapstructure:"maxRetries"`
	ProcessStatusUpdateInterval time.Duration `json:"processStatusUpdateInterval" mapstructure:"processStatusUpdateInterval"`
	CheckSeenOnNetworkOlderThan time.Duration `json:"checkSeenOnNetworkOlderThan" mapstructure:"checkSeenOnNetworkOlderThan"`
	CheckSeenOnNetworkPeriod    time.Duration `json:"checkSeenOnNetworkPeriod" mapstructure:"checkSeenOnNetworkPeriod"`
	MonitorPeersInterval        time.Duration `json:"monitorPeersInterval" mapstructure:"monitorPeersInterval"`
	CheckUtxos                  bool          `json:"checkUtxos" mapstructure:"checkUtxos"`
	ProfilerAddr                string        `json:"profilerAddr" mapstructure:"profilerAddr"`
	Health                      *HealthConfig `json:"health" mapstructure:"health"`
	RejectCallbackContaining    []string      `json:"rejectCallbackContaining" mapstructure:"rejectCallbackContaining"`
	Stats                       *StatsConfig  `json:"stats" mapstructure:"stats"`
}

type BlocktxConfig struct {
	ListenAddr                    string              `json:"listenAddr" mapstructure:"listenAddr"`
	DialAddr                      string              `json:"dialAddr" mapstructure:"dialAddr"`
	HealthServerDialAddr          string              `json:"healthServerDialAddr" mapstructure:"healthServerDialAddr"`
	Db                            *DbConfig           `json:"db" mapstructure:"db"`
	RecordRetentionDays           int                 `json:"recordRetentionDays" mapstructure:"recordRetentionDays"`
	ProfilerAddr                  string              `json:"profilerAddr" mapstructure:"profilerAddr"`
	RegisterTxsInterval           time.Duration       `json:"registerTxsInterval" mapstructure:"registerTxsInterval"`
	FillGapsInterval              time.Duration       `json:"fillGapsInterval" mapstructure:"fillGapsInterval"`
	MaxAllowedBlockHeightMismatch int                 `json:"maxAllowedBlockHeightMismatch" mapstructure:"maxAllowedBlockHeightMismatch"`
	MessageQueue                  *MessageQueueConfig `json:"mq" mapstructure:"mq"`
}

type DbConfig struct {
	Mode     string          `json:"mode" mapstructure:"mode"`
	Postgres *PostgresConfig `json:"postgres" mapstructure:"postgres"`
}

type PostgresConfig struct {
	Host         string `json:"host" mapstructure:"host"`
	Port         int    `json:"port" mapstructure:"port"`
	Name         string `json:"name" mapstructure:"name"`
	User         string `json:"user" mapstructure:"user"`
	Password     string `json:"password" mapstructure:"password"`
	MaxIdleConns int    `json:"maxIdleConns" mapstructure:"maxIdleConns"`
	MaxOpenConns int    `json:"maxOpenConns" mapstructure:"maxOpenConns"`
	SslMode      string `json:"sslMode" mapstructure:"sslMode"`
}

type HealthConfig struct {
	SeverDialAddr             string `json:"serverDialAddr" mapstructure:"serverDialAddr"`
	MinimumHealthyConnections int    `json:"minimumHealthyConnections" mapstructure:"minimumHealthyConnections"`
}

type StatsConfig struct {
	NotSeenTimeLimit  time.Duration `json:"notSeenTimeLimit" mapstructure:"notSeenTimeLimit"`
	NotMinedTimeLimit time.Duration `json:"notMinedTimeLimit" mapstructure:"notMinedTimeLimit"`
}

type MessageQueueConfig struct {
	TxsMinedMaxBatchSize int `json:"txsMinedMaxBatchSize" mapstructure:"txsMinedMaxBatchSize"`
}

type ApiConfig struct {
	Address       string            `json:"address" mapstructure:"address"`
	WocApiKey     string            `json:"wocApiKey" mapstructure:"wocApiKey"`
	DefaultPolicy *bitcoin.Settings `json:"defaultPolicy" mapstructure:"defaultPolicy"`
}

type K8sWatcherConfig struct {
	Namespace string `json:"namespace" mapstructure:"namespace"`
}
