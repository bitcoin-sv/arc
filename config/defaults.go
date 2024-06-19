package config

import (
	"time"

	"github.com/ordishs/go-bitcoin"
)

func getDefaultArcConfig() *ArcConfig {
	return &ArcConfig{
		LogLevel:           "DEBUG",
		LogFormat:          "text",
		ProfilerAddr:       "", // optional
		PrometheusEndpoint: "", // optional
		GrpcMessageSize:    100000000,
		Network:            "regtest",
		QueueURL:           "nats://nats:4222",
		Tracing:            nil, // optional
		PeerRpc:            getDefaultPeerRpcConfig(),
		Peers:              getPeersConfig(),
		Metamorph:          getMetamorphConfig(),
		Blocktx:            getBlocktxConfig(),
		Api:                getApiConfig(),
		K8sWatcher:         nil, // optional
	}
}

func getDefaultPeerRpcConfig() *PeerRpcConfig {
	return &PeerRpcConfig{
		Password: "bitcoin",
		User:     "bitcoin",
		Host:     "localhost",
		Port:     18332,
	}
}

func getPeersConfig() []*PeerConfig {
	return []*PeerConfig{
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
	}
}

func getMetamorphConfig() *MetamorphConfig {
	return &MetamorphConfig{
		ListenAddr:                  "localhost:8001",
		DialAddr:                    "localhost:8001",
		Db:                          getDbConfig("metamorph"),
		ProcessorCacheExpiryTime:    24 * time.Hour,
		CheckSeenOnNetworkOlderThan: 3 * time.Hour,
		CheckSeenOnNetworkPeriod:    4 * time.Hour,
		MonitorPeersInterval:        0, // optional
		CheckUtxos:                  false,
		Health: &HealthConfig{
			SeverDialAddr:             "localhost:8005",
			MinimumHealthyConnections: 2,
		},
		RejectCallbackContaining: []string{"http://localhost", "https://localhost"},
		Stats: &StatsConfig{
			NotSeenTimeLimit:  10 * time.Minute,
			NotMinedTimeLimit: 20 * time.Minute,
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
		FillGapsInterval:              15 * time.Minute,
		MaxAllowedBlockHeightMismatch: 3,
		MessageQueue: &MessageQueueConfig{
			TxsMinedMaxBatchSize: 20,
		},
	}
}

func getApiConfig() *ApiConfig {
	return &ApiConfig{
		Address:   "localhost:9090",
		WocApiKey: "mainnet_XXXXXXXXXXXXXXXXXXXX",
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
