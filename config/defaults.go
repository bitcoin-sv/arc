package config

import (
	"time"

	"github.com/ordishs/go-bitcoin"
)

func getDefaultArcConfig() *ArcConfig {
	return &ArcConfig{
		LogLevel:              "DEBUG",
		LogFormat:             "text",
		ProfilerAddr:          "", // optional
		Prometheus:            getDefaultPrometheusConfig(),
		GrpcMessageSize:       100000000,
		Network:               "regtest",
		ReBroadcastExpiration: 24 * time.Hour,
		MessageQueue:          getDefaultMessageQueueConfig(),
		Tracing:               getDefaultTracingConfig(),
		PeerRPC:               getDefaultPeerRPCConfig(),
		Metamorph:             getMetamorphConfig(),
		Blocktx:               getBlocktxConfig(),
		API:                   getAPIConfig(),
		K8sWatcher:            nil, // optional
		Callbacker:            getCallbackerConfig(),
		Cache:                 getCacheConfig(),
	}
}

func getDefaultPrometheusConfig() *PrometheusConfig {
	return &PrometheusConfig{
		Enabled:  false,
		Endpoint: "/metrics", // optional
		Addr:     ":2112",    // optional
	}
}

func getDefaultMessageQueueConfig() *MessageQueueConfig {
	return &MessageQueueConfig{
		URL: "nats://nats:4222",
		Streaming: MessageQueueStreaming{
			Enabled:     true,
			FileStorage: false,
		},
		Initialize: true,
	}
}

func getDefaultPeerRPCConfig() *PeerRPCConfig {
	return &PeerRPCConfig{
		Password: "bitcoin",
		User:     "bitcoin",
		Host:     "localhost",
		Port:     18332,
	}
}

func getMetamorphConfig() *MetamorphConfig {
	return &MetamorphConfig{
		ListenAddr:               "localhost:8001",
		DialAddr:                 "localhost:8001",
		Db:                       getDbConfig("metamorph"),
		ReAnnounceUnseenInterval: 60 * time.Second,
		ReAnnounceSeen:           10 * time.Minute,
		ReRegisterSeen:           10 * time.Minute,
		MaxRetries:               1000,
		StatusUpdateInterval:     5 * time.Second,
		MonitorPeers:             false,
		Health: &HealthConfig{
			MinimumHealthyConnections: 2,
		},
		RejectCallbackContaining: []string{"http://localhost", "https://localhost"},
		Stats: &StatsConfig{
			NotSeenTimeLimit:  10 * time.Minute,
			NotFinalTimeLimit: 20 * time.Minute,
		},
		BlockchainNetwork: &BlockchainNetwork[*MetamorphGroups]{
			Mode:    "classic",
			Network: "regtest",
			Peers: []*PeerConfig{
				{
					Host: "localhost",
					Port: &PeerPortConfig{
						P2P: 18333,
						ZMQ: 28332,
					},
				},
			},
		},
	}
}

func getBlocktxConfig() *BlocktxConfig {
	return &BlocktxConfig{
		ListenAddr:                    "localhost:8011",
		DialAddr:                      "localhost:8011",
		Db:                            getDbConfig("blocktx"),
		RecordRetentionDays:           28,
		RegisterTxsInterval:           10 * time.Second,
		MonitorPeers:                  false,
		AutoHeal:                      getAutoHealConfig(),
		FillGaps:                      getFillGapsConfig(),
		MaxAllowedBlockHeightMismatch: 3,
		MaxBlockProcessingDuration:    5 * time.Minute,
		MessageQueue:                  &MessageQueueConfig{},
		P2pReadBufferSize:             8 * 1024 * 1024,
		IncomingIsLongest:             false,
		BlockchainNetwork: &BlockchainNetwork[*BlocktxGroups]{
			Mode:    "classic",
			Network: "regtest",
			Peers: []*PeerConfig{
				{
					Host: "localhost",
					Port: &PeerPortConfig{
						P2P: 18333,
						ZMQ: 28332,
					},
				},
			},
		},
	}
}

func getAPIConfig() *APIConfig {
	return &APIConfig{
		Address:             "localhost:9090",
		WocAPIKey:           "mainnet_XXXXXXXXXXXXXXXXXXXX",
		WocMainnet:          false,
		RequestExtendedLogs: false,
		DefaultPolicy: &bitcoin.Settings{
			ExcessiveBlockSize:              2000000000,
			BlockMaxSize:                    512000000,
			MaxTxSizePolicy:                 100000000,
			MaxOrphanTxSize:                 1000000000,
			DataCarrierSize:                 4294967295,
			MaxScriptSizePolicy:             100000000,
			MaxOpsPerScriptPolicy:           4294967295,
			MaxScriptNumLengthPolicy:        10000,
			MaxPubKeysPerMultisigPolicy:     4294967295,
			MaxTxSigopsCountsPolicy:         4294967295,
			MaxStackMemoryUsagePolicy:       100000000,
			MaxStackMemoryUsageConsensus:    200000000,
			LimitAncestorCount:              10000,
			LimitCPFPGroupMembersCount:      25,
			MaxMempool:                      2000000000,
			MaxMempoolSizedisk:              0,
			MempoolMaxPercentCPFP:           10,
			AcceptNonStdOutputs:             true,
			DataCarrier:                     true,
			MinMiningTxFee:                  1e-8,
			MaxStdTxValidationDuration:      3,
			MaxNonStdTxValidationDuration:   1000,
			MaxTxChainValidationBudget:      50,
			ValidationClockCpu:              true,
			MinConsolidationFactor:          20,
			MaxConsolidationInputScriptSize: 150,
			MinConfConsolidationInput:       6,
			MinConsolidationInputMaturity:   6,
			AcceptNonStdConsolidationInput:  false,
		},
	}
}

func getDbConfig(dbName string) *DbConfig {
	return &DbConfig{
		Mode: "postgres",
		Postgres: &PostgresConfig{
			Host:         "localhost",
			Port:         5432,
			Name:         dbName,
			User:         "arc",
			Password:     "arc",
			MaxIdleConns: 10,
			MaxOpenConns: 80,
			SslMode:      "disable",
		},
	}
}

func getCallbackerConfig() *CallbackerConfig {
	return &CallbackerConfig{
		ListenAddr:        "localhost:8021",
		DialAddr:          "localhost:8021",
		Pause:             0,
		BatchSendInterval: 5 * time.Second,
		PruneOlderThan:    14 * 24 * time.Hour,
		PruneInterval:     24 * time.Hour,
		Expiration:        24 * time.Hour,
		Db:                getDbConfig("callbacker"),
	}
}

func getFillGapsConfig() *FillGapsConfig {
	return &FillGapsConfig{
		Enabled:  true,
		Interval: 15 * time.Minute,
	}
}

func getAutoHealConfig() *AutoHealConfig {
	return &AutoHealConfig{
		Enabled:  true,
		Interval: 5 * time.Minute,
	}
}

func getCacheConfig() *CacheConfig {
	return &CacheConfig{
		Engine: InMemory, // use in memory cache
		Redis: &RedisConfig{ // example of Redis config
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
		},
	}
}

func getDefaultTracingConfig() *TracingConfig {
	return &TracingConfig{
		DialAddr: "", // optional
		Sample:   100,
		Enabled:  false,
	}
}
