package broadcast

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/bitcoin-sv/arc/broadcaster"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/gocore"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:   "broadcast",
	Short: "Submit transactions to ARC",
	RunE: func(cmd *cobra.Command, args []string) error {

		useKey := viper.GetBool("useKey")
		isDryRun := viper.GetBool("isDryRun")
		isAPIClient := viper.GetBool("isAPIClient")
		isTestnet := viper.GetBool("testnet")
		printTxIDs := viper.GetBool("printTxIDs")
		callbackURL := viper.GetString("callback")
		consolidate := viper.GetBool("consolidate")
		authorization := viper.GetString("authorization")
		keyFile := viper.GetString("keyFile")
		waitForStatus := viper.GetInt("waitForStatus")
		concurrency := viper.GetInt("concurrency")
		batch := viper.GetInt("batch")
		miningFeeSat := viper.GetInt("broadcaster.miningFeeSatPerKb")

		logLevel := gocore.NewLogLevelFromString("debug")
		logger := gocore.Log("brdcst", logLevel)

		var fundingKeySet *keyset.KeySet
		var receivingKeySet *keyset.KeySet
		var err error

		if useKey {
			fmt.Print("Enter xpriv: ")
			reader := bufio.NewReader(os.Stdin)
			var inputKey string
			inputKey, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %v", err)
			}
			xpriv := strings.TrimSpace(inputKey)
			fundingKeySet, receivingKeySet, err = helper.GetKeySetsXpriv(xpriv)
			if err != nil {
				return fmt.Errorf("failed to get key sets: %v", err)
			}
			logger.Infof("xpriv: %s", xpriv)
		} else if keyFile != "" {
			fundingKeySet, receivingKeySet, err = helper.GetKeySetsKeyFile(keyFile)
			if err != nil {
				return fmt.Errorf("failed to get key sets: %v", err)
			}
		} else {
			fundingKeySet, receivingKeySet, err = helper.GetNewKeySets()
			if err != nil {
				return fmt.Errorf("failed to get key sets: %v", err)
			}
		}

		sendNrOfTransactions, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse number of transactions: %v", err)
		}
		if sendNrOfTransactions == 0 {
			sendNrOfTransactions = 1
		}

		ctx := context.Background()

		var client broadcaster.ArcClient
		client, err = helper.CreateClient(&broadcaster.Auth{
			Authorization: authorization,
		}, isDryRun, isAPIClient)
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}

		var feeOpts []func(fee *bt.Fee)

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

func init() {
	var err error

	Cmd.Flags().Bool("useKey", false, "private key to use for funding transactions")
	err = viper.BindPFlag("useKey", Cmd.Flags().Lookup("useKey"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Bool("dryrun", false, "whether to not send transactions or just output to console")
	err = viper.BindPFlag("isDryRun", Cmd.Flags().Lookup("dryrun"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Bool("api", true, "whether to not send transactions to api or metamorph")
	err = viper.BindPFlag("isAPIClient", Cmd.Flags().Lookup("api"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Bool("print", false, "Whether to print out all the tx ids of the transactions")
	err = viper.BindPFlag("printTxIDs", Cmd.Flags().Lookup("print"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Bool("consolidate", false, "whether to consolidate all output transactions back into the original")
	err = viper.BindPFlag("consolidate", Cmd.Flags().Lookup("consolidate"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Int("waitForStatus", 0, "wait for transaction to be in a certain status before continuing")
	err = viper.BindPFlag("waitForStatus", Cmd.Flags().Lookup("waitForStatus"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Int("concurrency", 0, "How many transactions to send concurrently")
	err = viper.BindPFlag("concurrency", Cmd.Flags().Lookup("concurrency"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Int("batch", 0, "send transactions in batches of this size")
	err = viper.BindPFlag("batch", Cmd.Flags().Lookup("batch"))
	if err != nil {
		log.Fatal(err)
	}
}
