package app

import (
	"fmt"
	"github.com/spf13/viper"

	"github.com/bitcoin-sv/arc/broadcaster"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/gocore"
	"github.com/spf13/cobra"
)

var prepCmd = &cobra.Command{
	Use:   "prepare-utxos",
	Short: "Create UTXO set to be used with broadcaster",
	RunE: func(cmd *cobra.Command, args []string) error {

		isTestnet := viper.GetBool("isTestnet")
		isPayback := viper.GetBool("isPayback")
		callbackURL := viper.GetString("callback")
		authorization := viper.GetString("authorization")
		keyFile := viper.GetString("keyFile")

		outputs := viper.GetInt64("broadcaster.utxoSet.outputs")
		satoshisPerOutput := viper.GetInt64("broadcaster.utxoSet.outputSat")
		miningFeeSat := viper.GetInt("broadcaster.miningFeeSatPerKb")

		logLevel := gocore.NewLogLevelFromString("debug")
		logger := gocore.Log("brdcst", logLevel)

		var client broadcaster.ClientI
		client, err := helper.CreateClient(&broadcaster.Auth{
			Authorization: authorization,
		}, false, true)
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}

		var fundingKeySet *keyset.KeySet
		var receivingKeySet *keyset.KeySet

		fundingKeySet, receivingKeySet, err = helper.GetKeySetsKeyFile(keyFile)
		if err != nil {
			return fmt.Errorf("failed to get key sets: %v", err)
		}

		var feeOpts []func(fee *bt.Fee)
		if err == nil {
			feeOpts = append(feeOpts, broadcaster.WithMiningFee(miningFeeSat))
		}

		preparer := broadcaster.NewUTXOPreparer(logger, client, fundingKeySet, receivingKeySet, feeOpts...)
		preparer.IsTestnet = isTestnet
		preparer.CallbackURL = callbackURL

		if isPayback {
			return preparer.Payback()
		}

		err = preparer.PrepareUTXOSet(uint64(outputs), uint64(satoshisPerOutput))
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {

	rootCmd.AddCommand(prepCmd)

	prepCmd.Flags().Bool("payback", false, "send all funds from receiving key set to funding key set")
	viper.BindPFlag("isPayback", prepCmd.Flags().Lookup("payback"))
}
