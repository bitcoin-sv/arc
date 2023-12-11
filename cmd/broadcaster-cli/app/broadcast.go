package app

import (
	"bufio"
	"context"
	"fmt"
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

var broadcastCmd = &cobra.Command{
	Use:   "broadcast",
	Short: "submit transactions to ARC",
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

		var client broadcaster.ClientI
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
	rootCmd.AddCommand(broadcastCmd)

	broadcastCmd.Flags().Bool("useKey", false, "private key to use for funding transactions")
	viper.BindPFlag("useKey", broadcastCmd.Flags().Lookup("useKey"))

	broadcastCmd.Flags().Bool("dryrun", false, "whether to not send transactions or just output to console")
	viper.BindPFlag("isDryRun", broadcastCmd.Flags().Lookup("dryrun"))

	broadcastCmd.Flags().Bool("api", true, "whether to not send transactions to api or metamorph")
	viper.BindPFlag("isAPIClient", broadcastCmd.Flags().Lookup("api"))

	broadcastCmd.Flags().Bool("print", false, "Whether to print out all the tx ids of the transactions")
	viper.BindPFlag("printTxIDs", broadcastCmd.Flags().Lookup("print"))

	broadcastCmd.Flags().Bool("consolidate", false, "whether to consolidate all output transactions back into the original")
	viper.BindPFlag("consolidate", broadcastCmd.Flags().Lookup("consolidate"))

	broadcastCmd.Flags().Int("waitForStatus", 0, "wait for transaction to be in a certain status before continuing")
	viper.BindPFlag("waitForStatus", broadcastCmd.Flags().Lookup("waitForStatus"))

	broadcastCmd.Flags().Int("concurrency", 0, "How many transactions to send concurrently")
	viper.BindPFlag("concurrency", broadcastCmd.Flags().Lookup("concurrency"))

	broadcastCmd.Flags().Int("batch", 0, "send transactions in batches of this size")
	viper.BindPFlag("batch", broadcastCmd.Flags().Lookup("batch"))
}
