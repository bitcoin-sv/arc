//go:build e2e

package test

import (
	"github.com/ordishs/go-bitcoin"
)

var (
	nodeHost = "node1"

	arcEndpoint     = "http://api:9090/"
	arcEndpointV1Tx = arcEndpoint + v1Tx
)

const (
	nodePort     = 18332
	nodeUser     = "bitcoin"
	nodePassword = "bitcoin"
)

const (
	v1Tx  = "v1/tx"
	v1Txs = "v1/txs"
)

var bitcoind *bitcoin.Bitcoind
