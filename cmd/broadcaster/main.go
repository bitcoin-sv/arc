package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/TAAL-GmbH/arc/broadcaster"
	"github.com/TAAL-GmbH/arc/lib/keyset"
	"github.com/ordishs/gocore"
)

var (
	isDryRun    = false
	isRegtest   = true
	isAPIClient = false
)

func main() {
	sendOnChannel := flag.Int("buffer", 0, "whether to send transactions over a buffered channel and how big the buffer should be")
	dryRun := flag.Bool("dryrun", false, "whether to not send transactions or just output to console")
	apiClient := flag.Bool("api", true, "whether to not send transactions to api or metamorph")
	consolidate := flag.Bool("consolidate", false, "whether to consolidate all output transactions back into the original")
	useKey := flag.Bool("key", false, "private key to use for funding transactions")
	keyFile := flag.String("keyfile", "", "private key from file (arc.key) to use for funding transactions")
	flag.Parse()

	args := flag.Args()

	if len(args) != 1 {
		fmt.Println("usage: broadcaster [options] <number of transactions to send>")
		fmt.Println("where options are:")
		fmt.Println("    -buffer=<number of buffered channels>")
		fmt.Println("    -dryrun")
		fmt.Println("    -key=<extended private key")
		fmt.Println("    -keyfile=<key file>")
		return
	}

	if dryRun != nil && *dryRun {
		isDryRun = true
	}

	if apiClient != nil && *apiClient {
		isAPIClient = true
	}

	var err error
	var xpriv string
	if useKey != nil && *useKey {
		fmt.Print("Enter xpriv: ")
		reader := bufio.NewReader(os.Stdin)

		var inputKey string
		inputKey, err = reader.ReadString('\n')
		if err != nil {
			panic("An error occurred while reading input. Please try again:" + err.Error())
		}
		xpriv = strings.TrimSpace(inputKey)
	}

	sendNrOfTransactions, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		panic(err)
	}
	if sendNrOfTransactions == 0 {
		sendNrOfTransactions = 1
	}

	ctx := context.Background()

	var client broadcaster.ClientI
	client, err = createClient()
	if err != nil {
		panic(err)
	}

	var fundingKeySet *keyset.KeySet
	var receivingKeySet *keyset.KeySet
	fundingKeySet, receivingKeySet, err = getKeySets(xpriv, keyFile)
	if err != nil {
		panic(err)
	}

	bCaster := broadcaster.New(client, fundingKeySet, receivingKeySet, sendNrOfTransactions)
	bCaster.IsRegtest = isRegtest
	bCaster.IsDryRun = isDryRun

	err = bCaster.Run(ctx, sendOnChannel)
	if err != nil {
		panic(err)
	}

	if consolidate != nil && *consolidate {
		if err = bCaster.ConsolidateOutputsToOriginal(ctx); err != nil {
			panic(err)
		}
	}
}

func createClient() (client broadcaster.ClientI, err error) {
	if isDryRun {
		client = broadcaster.NewDryRunClient()
	} else if isAPIClient {
		arcAddresses, _ := gocore.Config().Get("arcAddresses", "http://localhost:9090")
		useAddress := strings.Split(arcAddresses, ",")[0]

		// create a http connection to the arc node
		client = broadcaster.NewHTTPBroadcaster(useAddress)
	} else {
		addresses, _ := gocore.Config().Get("metamorphAddresses") //, "localhost:8000")
		fmt.Printf("Metamorph addresses: %s\n", addresses)
		client = broadcaster.NewMetamorphBroadcaster(addresses)
	}

	return client, nil
}

func getKeySets(xpriv string, keyFile *string) (fundingKeySet *keyset.KeySet, receivingKeySet *keyset.KeySet, err error) {
	if xpriv != "" || *keyFile != "" {
		if xpriv == "" {
			var extendedBytes []byte
			extendedBytes, err = os.ReadFile(*keyFile)
			if err != nil {
				if os.IsNotExist(err) {
					panic("arc.key not found. Please create this file with the xpriv you want to use")
				}
				return nil, nil, err
			}
			xpriv = strings.TrimRight(strings.TrimSpace((string)(extendedBytes)), "\n")
		}

		fundingKeySet, err = keyset.NewFromExtendedKeyStr(xpriv, "0/0")
		if err != nil {
			return nil, nil, err
		}

		receivingKeySet, err = keyset.NewFromExtendedKeyStr(xpriv, "0/1")
		if err != nil {
			return nil, nil, err
		}

		isRegtest = false
	} else {
		// create random key set
		fundingKeySet, err = keyset.New()
		if err != nil {
			return nil, nil, err
		}
		receivingKeySet = fundingKeySet
	}

	return fundingKeySet, receivingKeySet, err
}
