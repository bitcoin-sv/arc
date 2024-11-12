//go:build e2e

package test

import (
	"github.com/ordishs/go-bitcoin"
)

const (
	host     = "node1"
	port     = 18332
	user     = "bitcoin"
	password = "bitcoin"
)

const (
	feeSat = 10

	arcEndpoint      = "http://api:9090/"
	v1Tx             = "v1/tx"
	v1Txs            = "v1/txs"
	arcEndpointV1Tx  = arcEndpoint + v1Tx
	arcEndpointV1Txs = arcEndpoint + v1Txs
)

var bitcoind *bitcoin.Bitcoind
