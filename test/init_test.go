//go:build e2e

package test

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/ordishs/go-bitcoin"
)

func TestMain(m *testing.M) {
	setupSut()

	info, err := bitcoind.GetInfo()
	if err != nil {
		log.Printf("failed to get info: %v", err)
		return
	}

	log.Printf("block height: %d", info.Blocks)
	m.Run()
}

func setupSut() {
	log.Println("init tests")

	if os.Getenv("TEST_LOCAL") != "" {
		nodeHost = "localhost"
		arcEndpoint = "http://localhost:9090/"
		arcEndpointV1Tx = arcEndpoint + v1Tx
		arcEndpointV1Txs = arcEndpoint + v1Txs
	}

	var err error
	bitcoind, err = bitcoin.New(nodeHost, nodePort, nodeUser, nodePassword, false)
	if err != nil {
		log.Fatalln("Failed to create bitcoind instance:", err)
	}

	info, err := bitcoind.GetInfo()
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("block height: %d", info.Blocks)
	// fund node
	const minNumbeOfBlocks = 101

	blocksToGenerate := minNumbeOfBlocks - info.Blocks

	log.Printf("generate %d blocks", blocksToGenerate)

	// generate blocks in part to ensure blocktx is able to process all blocks
	blockBatch := int32(20)
	if os.Getenv("TEST_LOCAL_MCAST") != "" {
		blockBatch = 4
	}

	for blocksToGenerate > 0 {
		_, err = bitcoind.Generate(float64(blockBatch))
		if err != nil {
			log.Fatalln(err)
		}

		// give time to send all INV messages
		time.Sleep(5 * time.Second)

		info, err = bitcoind.GetInfo()
		if err != nil {
			log.Fatalln(err)
		}

		log.Printf("block height: %d", info.Blocks)

		blocksToGenerate = blocksToGenerate - blockBatch
	}

	time.Sleep(15 * time.Second) // wait for fillGaps to fill eventual gaps
}
