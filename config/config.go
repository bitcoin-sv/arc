package config

import (
	"time"

	"github.com/ordishs/go-bitcoin"
	"go.opentelemetry.io/otel/attribute"
)

const (
	InMemory = "in-memory"
	Redis    = "redis"
)

type ArcConfig struct {
	LogLevel        string              `mapstructure:"logLevel"`
	LogFormat       string              `mapstructure:"logFormat"`
	ProfilerAddr    string              `mapstructure:"profilerAddr"`
	Prometheus      *PrometheusConfig   `mapstructure:"prometheus"`
	GrpcMessageSize int                 `mapstructure:"grpcMessageSize"`
	Network         string              `mapstructure:"network"`
	MessageQueue    *MessageQueueConfig `mapstructure:"messageQueue"`
	Tracing         *TracingConfig      `mapstructure:"tracing"`
	PeerRPC         *PeerRPCConfig      `mapstructure:"peerRpc"`
	Metamorph       *MetamorphConfig    `mapstructure:"metamorph"`
	Blocktx         *BlocktxConfig      `mapstructure:"blocktx"`
	API             *APIConfig          `mapstructure:"api"`
	K8sWatcher      *K8sWatcherConfig   `mapstructure:"k8sWatcher"`
	Callbacker      *CallbackerConfig   `mapstructure:"callbacker"`
	Cache           *CacheConfig        `mapstructure:"cache"`
}

type PrometheusConfig struct {
	Endpoint string `mapstructure:"endpoint"`
	Addr     string `mapstructure:"addr"`
	Enabled  bool   `mapstructure:"enabled"`
}

func (p *PrometheusConfig) IsEnabled() bool {
	return p.Enabled && p.Addr != "" && p.Endpoint != ""
}

type PeerConfig struct {
	Host string          `mapstructure:"host"`
	Port *PeerPortConfig `mapstructure:"port"`
}

type MessageQueueConfig struct {
	URL        string                `mapstructure:"url"`
	Streaming  MessageQueueStreaming `mapstructure:"streaming"`
	Initialize bool                  `mapstructure:"initialize"`
}

type MessageQueueStreaming struct {
	Enabled     bool `mapstructure:"enabled"`
	FileStorage bool `mapstructure:"fileStorage"`
}

type TracingConfig struct {
	Enabled            bool              `mapstructure:"enabled"`
	DialAddr           string            `mapstructure:"dialAddr"`
	Sample             int               `mapstructure:"sample"`
	Attributes         map[string]string `mapstructure:"attributes"`
	KeyValueAttributes []attribute.KeyValue
}

func (a *ArcConfig) IsTracingEnabled() bool {
	return a.Tracing != nil && a.Tracing.IsEnabled()
}

func (t *TracingConfig) IsEnabled() bool {
	return t.Enabled && t.DialAddr != ""
}

type PeerRPCConfig struct {
	Password string `mapstructure:"password"`
	User     string `mapstructure:"user"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
}

type PeerPortConfig struct {
	P2P int `mapstructure:"p2p"`
	ZMQ int `mapstructure:"zmq"`
}

type MetamorphConfig struct {
	ListenAddr                              string                               `mapstructure:"listenAddr"`
	DialAddr                                string                               `mapstructure:"dialAddr"`
	Db                                      *DbConfig                            `mapstructure:"db"`
	ProcessorCacheExpiryTime                time.Duration                        `mapstructure:"processorCacheExpiryTime"`
	UnseenTransactionRebroadcastingInterval time.Duration                        `mapstructure:"unseenTransactionRebroadcastingInterval"`
	MaxRetries                              int                                  `mapstructure:"maxRetries"`
	ProcessStatusUpdateInterval             time.Duration                        `mapstructure:"processStatusUpdateInterval"`
	RecheckSeen                             RecheckSeen                          `mapstructure:"recheckSeen"`
	MonitorPeers                            bool                                 `mapstructure:"monitorPeers"`
	Health                                  *HealthConfig                        `mapstructure:"health"`
	RejectCallbackContaining                []string                             `mapstructure:"rejectCallbackContaining"`
	Stats                                   *StatsConfig                         `mapstructure:"stats"`
	BlockchainNetwork                       *BlockchainNetwork[*MetamorphGroups] `mapstructure:"bcnet"`
}

type HealthConfig struct {
	MinimumHealthyConnections int `mapstructure:"minimumHealthyConnections"`
}

type BlocktxConfig struct {
	ListenAddr                    string                             `mapstructure:"listenAddr"`
	DialAddr                      string                             `mapstructure:"dialAddr"`
	Db                            *DbConfig                          `mapstructure:"db"`
	RecordRetentionDays           int                                `mapstructure:"recordRetentionDays"`
	RegisterTxsInterval           time.Duration                      `mapstructure:"registerTxsInterval"`
	MaxBlockProcessingDuration    time.Duration                      `mapstructure:"maxBlockProcessingDuration"`
	MonitorPeers                  bool                               `mapstructure:"monitorPeers"`
	FillGaps                      *FillGapsConfig                    `mapstructure:"fillGaps"`
	MaxAllowedBlockHeightMismatch int                                `mapstructure:"maxAllowedBlockHeightMismatch"`
	MessageQueue                  *MessageQueueConfig                `mapstructure:"mq"`
	P2pReadBufferSize             int                                `mapstructure:"p2pReadBufferSize"`
	IncomingIsLongest             bool                               `mapstructure:"incomingIsLongest"`
	BlockchainNetwork             *BlockchainNetwork[*BlocktxGroups] `mapstructure:"bcnet"`
}

type BlockchainNetwork[McastT any] struct {
	Mode    string        `mapstructure:"mode"`
	Network string        `mapstructure:"network"`
	Peers   []*PeerConfig `mapstructure:"peers"`
	Mcast   McastT        `mapstructure:"mcast"`
}

type BlocktxGroups struct {
	McastBlock McastGroup `mapstructure:"block"`
}

type MetamorphGroups struct {
	McastTx     McastGroup `mapstructure:"tx"`
	McastReject McastGroup `mapstructure:"reject"`
}

type McastGroup struct {
	Address    string   `mapstructure:"address"`
	Interfaces []string `mapstructure:"interfaces"`
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

type CacheConfig struct {
	Engine string       `mapstructure:"engine"`
	Redis  *RedisConfig `mapstructure:"redis"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type StatsConfig struct {
	NotSeenTimeLimit  time.Duration `mapstructure:"notSeenTimeLimit"`
	NotFinalTimeLimit time.Duration `mapstructure:"notFinalTimeLimit"`
}

type RecheckSeen struct {
	FromAgo  time.Duration `mapstructure:"fromAgo"`
	UntilAgo time.Duration `mapstructure:"untilAgo"`
}

type FillGapsConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Interval time.Duration `mapstructure:"interval"`
}

type APIConfig struct {
	Address                  string            `mapstructure:"address"`
	WocAPIKey                string            `mapstructure:"wocApiKey"`
	WocMainnet               bool              `mapstructure:"wocMainnet"`
	DefaultPolicy            *bitcoin.Settings `mapstructure:"defaultPolicy"`
	ProcessorCacheExpiryTime time.Duration     `mapstructure:"processorCacheExpiryTime"`

	RequestExtendedLogs bool `mapstructure:"requestExtendedLogs"`
}

type K8sWatcherConfig struct {
	Namespace string `mapstructure:"namespace"`
}

type CallbackerConfig struct {
	ListenAddr        string        `mapstructure:"listenAddr"`
	DialAddr          string        `mapstructure:"dialAddr"`
	Health            *HealthConfig `mapstructure:"health"`
	Pause             time.Duration `mapstructure:"pause"`
	BatchSendInterval time.Duration `mapstructure:"batchSendInterval"`
	PruneOlderThan    time.Duration `mapstructure:"pruneOlderThan"`
	PruneInterval     time.Duration `mapstructure:"pruneInterval"`
	Expiration        time.Duration `mapstructure:"expiration"`
	Db                *DbConfig     `mapstructure:"db"`
}
