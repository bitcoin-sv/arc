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
	const minNumbeOfBlocks = 101

	if info.Blocks < minNumbeOfBlocks {
		// generate blocks in part to ensure blocktx is able to process all blocks
		const blockBatch = 20 // should be leq n*10 where n is number of blocktx instances

		for {
			_, err = bitcoind.Generate(blockBatch)
			if err != nil {
				log.Fatalln(err)
			}

			// give time to send all INV messages
			time.Sleep(5 * time.Second)

			info, err = bitcoind.GetInfo()
			if err != nil {
				log.Fatalln(err)
			}

			missingBlocks := minNumbeOfBlocks - info.Blocks
			log.Println(missingBlocks)

			if missingBlocks < 0 {
				break
			}
		}
	}
}
