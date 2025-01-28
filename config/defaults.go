package config

import (
	"time"

	"github.com/ordishs/go-bitcoin"
)

func getDefaultArcConfig() *ArcConfig {
	return &ArcConfig{
		LogLevel:        "DEBUG",
		LogFormat:       "text",
		ProfilerAddr:    "", // optional
		Prometheus:      getDefaultPrometheusConfig(),
		GrpcMessageSize: 100000000,
		Network:         "regtest",
		MessageQueue:    getDefaultMessageQueueConfig(),
		Tracing:         getDefaultTracingConfig(),
		PeerRPC:         getDefaultPeerRPCConfig(),
		Broadcasting:    getBroadcastingConfig(),
		Metamorph:       getMetamorphConfig(),
		Blocktx:         getBlocktxConfig(),
		API:             getAPIConfig(),
		K8sWatcher:      nil, // optional
		Callbacker:      getCallbackerConfig(),
		Cache:           getCacheConfig(),
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
			Enabled:     false,
			FileStorage: false,
		},
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

func getBroadcastingConfig() *BroadcastingConfig {
	return &BroadcastingConfig{
		Mode: "unicast",
		Unicast: &Unicast{
			Peers: []*PeerConfig{
				{
					Host: "localhost",
					Port: &PeerPortConfig{
						P2P: 18333,
						ZMQ: 28332,
					},
				},
				{
					Host: "localhost",
					Port: &PeerPortConfig{
						P2P: 18334,
					},
				},
				{
					Host: "localhost",
					Port: &PeerPortConfig{
						P2P: 18335,
					},
				},
			},
		},
		Multicast: &Mulsticast{
			Ipv6Enabled:     false,
			MulticastGroups: nil,
			Interfaces:      nil,
		},
	}
}

func getMetamorphConfig() *MetamorphConfig {
	return &MetamorphConfig{
		ListenAddr:                              "localhost:8001",
		DialAddr:                                "localhost:8001",
		Db:                                      getDbConfig("metamorph"),
		ProcessorCacheExpiryTime:                24 * time.Hour,
		UnseenTransactionRebroadcastingInterval: 60 * time.Second,
		MaxRetries:                              1000,
		ProcessStatusUpdateInterval:             5 * time.Second,
		RecheckSeen: RecheckSeen{
			FromAgo:  24 * time.Hour,
			UntilAgo: 1 * time.Hour,
		},
		MonitorPeers: false,
		Health: &HealthConfig{
			SeverDialAddr:             "localhost:8005",
			MinimumHealthyConnections: 2,
		},
		RejectCallbackContaining: []string{"http://localhost", "https://localhost"},
		Stats: &StatsConfig{
			NotSeenTimeLimit:  10 * time.Minute,
			NotFinalTimeLimit: 20 * time.Minute,
		},
	}
}

func getBlocktxConfig() *BlocktxConfig {
	return &BlocktxConfig{
		ListenAddr:                    "localhost:8011",
		DialAddr:                      "localhost:8011",
		HealthServerDialAddr:          "localhost:8006",
		Db:                            getDbConfig("blocktx"),
		RecordRetentionDays:           28,
		RegisterTxsInterval:           10 * time.Second,
		MonitorPeers:                  false,
		FillGaps:                      getFillGapsConfig(),
		MaxAllowedBlockHeightMismatch: 3,
		MaxBlockProcessingDuration:    5 * time.Minute,
		MessageQueue:                  &MessageQueueConfig{},
		P2pReadBufferSize:             8 * 1024 * 1024,
		IncomingIsLongest:             false,
	}
}

func getAPIConfig() *APIConfig {
	return &APIConfig{
		Address:   "localhost:9090",
		WocAPIKey: "mainnet_XXXXXXXXXXXXXXXXXXXX",
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
		ListenAddr: "localhost:8021",
		DialAddr:   "localhost:8021",
		Health: &HealthConfig{
			SeverDialAddr: "localhost:8025",
		},
		Delay:                       0,
		Pause:                       0,
		BatchSendInterval:           time.Duration(5 * time.Second),
		Db:                          getDbConfig("callbacker"),
		PruneInterval:               24 * time.Hour,
		PruneOlderThan:              14 * 24 * time.Hour,
		FailedCallbackCheckInterval: time.Minute,
		Expiration:                  24 * time.Hour,
	}
}

func getFillGapsConfig() *FillGapsConfig {
	return &FillGapsConfig{
		Enabled:  true,
		Interval: 15 * time.Minute,
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
