package test

import (
	"log"
	"os"
	"testing"
	"time"

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

	arcEndpoint      = "http://arc:9090/"
	v1Tx             = "v1/tx"
	v1Txs            = "v1/txs"
	arcEndpointV1Tx  = arcEndpoint + v1Tx
	arcEndpointV1Txs = arcEndpoint + v1Txs
)

var bitcoind *bitcoin.Bitcoind

func TestMain(m *testing.M) {
	setupSut()

	info, err := bitcoind.GetInfo()
	if err != nil {
		log.Fatalf("failed to get info: %v", err)
	}

	log.Printf("current block height: %d", info.Blocks)

	os.Exit(m.Run())
}

func setupSut() {
	log.Printf("init tests")

	var err error
	bitcoind, err = bitcoin.New(host, port, user, password, false)
	if err != nil {
		log.Fatalln("Failed to create bitcoind instance:", err)
	}

	info, err := bitcoind.GetInfo()
	if err != nil {
		log.Fatalln(err)
	}

	// fund node
	if info.Blocks < 101 {
		_, err = bitcoind.Generate(101)
		if err != nil {
			log.Fatalln(err)
		}
		time.Sleep(5 * time.Second)
	}

	info, err = bitcoind.GetInfo()
	if err != nil {
		log.Fatalln(err)
	}
}
