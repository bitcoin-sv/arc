package broadcast

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/bitcoin-sv/arc/broadcaster"
	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/gocore"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func InitCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "broadcast",
		Short: "submit transactions to ARC",
		RunE: func(cmd *cobra.Command, args []string) error {

			useKey := v.GetBool("useKey")
			isDryRun := v.GetBool("isDryRun")
			isAPIClient := v.GetBool("isAPIClient")
			isTestnet := v.GetBool("isTestnet")
			printTxIDs := v.GetBool("printTxIDs")
			callbackURL := v.GetString("callback")
			consolidate := v.GetBool("consolidate")
			authorization := v.GetString("authorization")
			keyFile := v.GetString("keyFile")
			waitForStatus := v.GetInt("waitForStatus")
			concurrency := v.GetInt("concurrency")
			batch := v.GetInt("batch")

			logLevel := gocore.NewLogLevelFromString("debug")
			logger := gocore.Log("brdcst", logLevel)

			var xpriv string
			if useKey {
				fmt.Print("Enter xpriv: ")
				reader := bufio.NewReader(os.Stdin)

				var inputKey string
				inputKey, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read input: %v", err)
				}
				xpriv = strings.TrimSpace(inputKey)
			}

			fmt.Println(xpriv)

			sendNrOfTransactions, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse number of transactions: %v", err)
			}
			if sendNrOfTransactions == 0 {
				sendNrOfTransactions = 1
			}

			ctx := context.Background()

			var client broadcaster.ClientI
			client, err = createClient(&broadcaster.Auth{
				Authorization: authorization,
			}, isDryRun, isAPIClient)
			if err != nil {
				return fmt.Errorf("failed to create client: %v", err)
			}

			var fundingKeySet *keyset.KeySet
			var receivingKeySet *keyset.KeySet
			fundingKeySet, receivingKeySet, err = getKeySets(xpriv, keyFile)
			if err != nil {
				return fmt.Errorf("failed to get key sets: %v", err)
			}

			var feeOpts []func(fee *bt.Fee)
			miningFeeSat, err := config.GetInt("broadcaster.miningFeeSatPerKb")
			if err == nil {
				feeOpts = append(feeOpts, broadcaster.WithMiningFee(miningFeeSat))
			}

			bCaster := broadcaster.New(logger, client, fundingKeySet, receivingKeySet, sendNrOfTransactions, feeOpts...)
			bCaster.IsRegtest = false
			bCaster.IsDryRun = isDryRun
			bCaster.WaitForStatus = waitForStatus
			bCaster.PrintTxIDs = printTxIDs
			bCaster.BatchSend = batch
			bCaster.Consolidate = consolidate
			bCaster.IsTestnet = isTestnet
			bCaster.CallbackURL = callbackURL

			err = bCaster.Run(ctx, concurrency)
			if err != nil {
				return fmt.Errorf("failed to run broadcaster: %v", err)
			}

			return nil
		},
	}

	cmd.Flags().Bool("useKey", false, "private key to use for funding transactions")
	_ = v.BindPFlag("useKey", cmd.Flags().Lookup("useKey"))

	cmd.Flags().Bool("dryrun", false, "whether to not send transactions or just output to console")
	_ = v.BindPFlag("isDryRun", cmd.Flags().Lookup("dryrun"))

	cmd.Flags().Bool("api", true, "whether to not send transactions to api or metamorph")
	_ = v.BindPFlag("isAPIClient", cmd.Flags().Lookup("api"))

	cmd.Flags().Bool("testnet", false, "send transactions to testnet")
	_ = v.BindPFlag("isTestnet", cmd.Flags().Lookup("testnet"))

	cmd.Flags().Bool("print", false, "Whether to print out all the tx ids of the transactions")
	_ = v.BindPFlag("printTxIDs", cmd.Flags().Lookup("print"))

	cmd.Flags().Bool("consolidate", false, "whether to consolidate all output transactions back into the original")
	_ = v.BindPFlag("consolidate", cmd.Flags().Lookup("consolidate"))

	cmd.Flags().String("authorization", "abcd", "Authorization header to use for the http api client")
	_ = v.BindPFlag("authorization", cmd.Flags().Lookup("authorization"))

	cmd.Flags().String("callback", "", "URL which will be called with ARC callbacks")
	_ = v.BindPFlag("callback", cmd.Flags().Lookup("callback"))

	cmd.Flags().String("keyfile", "", "private key from file (arc.key) to use for funding transactions")
	_ = v.BindPFlag("keyFile", cmd.Flags().Lookup("keyfile"))

	cmd.Flags().Int("waitForStatus", 0, "wait for transaction to be in a certain status before continuing")
	_ = v.BindPFlag("waitForStatus", cmd.Flags().Lookup("waitForStatus"))

	cmd.Flags().Int("concurrency", 0, "How many transactions to send concurrently")
	_ = v.BindPFlag("concurrency", cmd.Flags().Lookup("concurrency"))

	cmd.Flags().Int("batch", 0, "send transactions in batches of this size")
	_ = v.BindPFlag("batch", cmd.Flags().Lookup("batch"))

	return cmd
}

func createClient(auth *broadcaster.Auth, isDryRun bool, isAPIClient bool) (broadcaster.ClientI, error) {
	var client broadcaster.ClientI
	var err error
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
		client, err = broadcaster.NewMetamorphBroadcaster(addresses)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

func getKeySets(xpriv string, keyFile string) (fundingKeySet *keyset.KeySet, receivingKeySet *keyset.KeySet, err error) {
	if xpriv != "" || keyFile != "" {
		if xpriv == "" {
			var extendedBytes []byte
			extendedBytes, err = os.ReadFile(keyFile)
			if err != nil {
				if os.IsNotExist(err) {
					return nil, nil, errors.New("arc.key not found. Please create this file with the xpriv you want to use")
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
