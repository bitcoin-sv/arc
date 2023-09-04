package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/bitcoin-sv/arc/broadcaster"
	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/ordishs/gocore"
	"github.com/spf13/viper"
)

var (
	isDryRun    = false
	isRegtest   = true
	isAPIClient = false
)

func main() {
	logLevel := gocore.NewLogLevelFromString("debug")
	logger := gocore.Log("brdcst", logLevel)

	dryRun := flag.Bool("dryrun", false, "whether to not send transactions or just output to console")
	apiClient := flag.Bool("api", true, "whether to not send transactions to api or metamorph")
	consolidate := flag.Bool("consolidate", false, "whether to consolidate all output transactions back into the original")
	useKey := flag.Bool("key", false, "private key to use for funding transactions")
	keyFile := flag.String("keyfile", "", "private key from file (arc.key) to use for funding transactions")
	waitForStatus := flag.Int("wait", 0, "wait for transaction to be in a certain status before continuing")
	authorization := flag.String("authorization", "", "Authorization header to use for the http api client")
	printTxIDs := flag.Bool("print", false, "Whether to print out all the tx ids of the transactions")
	concurrency := flag.Int("concurrency", 0, "How many transactions to send concurrently")
	batch := flag.Int("batch", 0, "send transactions in batches of this size")
	isTestnet := flag.Bool("testnet", false, "send transactions to testnet")
	flag.Parse()

	args := flag.Args()

	if len(args) != 1 {
		fmt.Println("usage: broadcaster [options] <number of transactions to send>")
		fmt.Println("where options are:")
		fmt.Println("")
		fmt.Println("    -dryrun")
		fmt.Println("          whether to not send transactions or just output to console")
		fmt.Println("")
		fmt.Println("    -consolidate")
		fmt.Println("          whether to consolidate all output transactions back into the original address")
		fmt.Println("")
		fmt.Println("    -print")
		fmt.Println("          whether to print out all the tx ids of the transactions")
		fmt.Println("")
		fmt.Println("    -wait=<status number>")
		fmt.Println("          wait for transaction to be in a certain status before continuing (2=RECEIVED, 3=STORED, 4=ANNOUNCED, 5=SENT, 6=SEEN)")
		fmt.Println("")
		fmt.Println("    -api=<true|false>")
		fmt.Println("          whether to not send transactions to api or metamorph (default=true)")
		fmt.Println("")
		fmt.Println("    -key=<extended private key")
		fmt.Println("          private key (xprv) to use for funding transactions")
		fmt.Println("")
		fmt.Println("    -keyfile=<key file>")
		fmt.Println("          private key from file (arc.key) to use for funding transactions")
		fmt.Println("")
		fmt.Println("    -concurrency=<number of concurrent requests>")
		fmt.Println("          how many transactions to send concurrently to the server, in go routines")
		fmt.Println("")
		fmt.Println("    -authorization=<raw authorization header>")
		fmt.Println("          authorization header to use for the http api client (header \"Authorization: ....\")")
		fmt.Println("")
		fmt.Println("    -batch=<batch size>")
		fmt.Println("          send transactions in batches of this size (default=0, no batching), only in api mode")
		fmt.Println("")
		fmt.Println("    -testnet=<true|false>")
		fmt.Println("          whether to send testnet or mainnet transactions, default=false")
		fmt.Println("")
		return
	}

	if dryRun != nil && *dryRun {
		isDryRun = true
	}

	if apiClient != nil && *apiClient {
		isAPIClient = true
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("../../")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("failed to read config file config.yaml: %v \n", err)
		return
	}

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
	client, err = createClient(&broadcaster.Auth{
		Authorization: *authorization,
	})
	if err != nil {
		panic(err)
	}

	var fundingKeySet *keyset.KeySet
	var receivingKeySet *keyset.KeySet
	fundingKeySet, receivingKeySet, err = getKeySets(xpriv, keyFile)
	if err != nil {
		panic(err)
	}

	bCaster := broadcaster.New(logger, client, fundingKeySet, receivingKeySet, sendNrOfTransactions)
	bCaster.IsRegtest = isRegtest
	bCaster.IsDryRun = isDryRun
	bCaster.WaitForStatus = *waitForStatus
	bCaster.PrintTxIDs = *printTxIDs
	bCaster.BatchSend = *batch
	bCaster.Consolidate = *consolidate
	bCaster.IsTestnet = *isTestnet

	err = bCaster.Run(ctx, *concurrency)
	if err != nil {
		panic(err)
	}
}

func createClient(auth *broadcaster.Auth) (broadcaster.ClientI, error) {
	var client broadcaster.ClientI
	if isDryRun {
		client = broadcaster.NewDryRunClient()
	} else if isAPIClient {
		arcServer := viper.GetString("broadcaster.apiURL")
		if arcServer == "" {
			return nil, errors.New("arcUrl not found in config")
		}

		arcServerUrl, err := url.Parse(arcServer)
		if err != nil {
			return nil, errors.New("arcUrl is not a valid url")
		}

		// create a http connection to the arc node
		client = broadcaster.NewHTTPBroadcaster(arcServerUrl.String(), auth)
	} else {
		addresses := viper.GetString("metamorph.dialAddr")
		if addresses == "" {
			return nil, errors.New("metamorph.dialAddr not found in config")
		}
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
