package split

import (
	"errors"
	"fmt"
	"log"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/pkg/keyset"
)

var Cmd = &cobra.Command{
	Use:   "split",
	Short: "Split a UTXO",
	RunE: func(_ *cobra.Command, _ []string) error {
		txid, err := helper.GetString("txid")
		if err != nil {
			return err
		}
		if txid == "" {
			return errors.New("txid is required")
		}

		from, err := helper.GetString("from")
		if err != nil {
			return err
		}
		if from == "" {
			return errors.New("from is required")
		}

		satoshis, err := helper.GetUint64("satoshis")
		if err != nil {
			return err
		}
		if satoshis == 0 {
			return errors.New("satoshis is required")
		}

		vout, err := helper.GetUint32("vout")
		if err != nil {
			return err
		}

		dryrun, err := helper.GetBool("dryrun")
		if err != nil {
			return err
		}

		isTestnet, err := helper.GetBool("testnet")
		if err != nil {
			return err
		}

		authorization, err := helper.GetString("authorization")
		if err != nil {
			return err
		}

		miningFeeSat, err := helper.GetUint64("miningFeeSatPerKb")
		if err != nil {
			return err
		}

		arcServer, err := helper.GetString("apiURL")
		if err != nil {
			return err
		}
		if arcServer == "" {
			return errors.New("no api URL was given")
		}

		allKeysMap, err := helper.GetAllKeySets()
		if err != nil {
			return err
		}

		keySetsMap, err := helper.GetSelectedKeySets()
		if err != nil {
			return err
		}

		logger := helper.GetLogger()

		client, err := helper.CreateClient(&broadcaster.Auth{
			Authorization: authorization,
		}, arcServer)
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}

		ks := make([]*keyset.KeySet, len(keySetsMap))
		counter := 0
		for _, keySet := range keySetsMap {
			ks[counter] = keySet
			counter++
		}
		fromKs, ok := allKeysMap[from]
		if !ok {
			return fmt.Errorf("from not found in keySetsMap: %v", from)
		}

		splitter, err := broadcaster.NewUTXOSplitter(logger, client, fromKs, ks, isTestnet,
			broadcaster.WithFees(miningFeeSat),
		)
		if err != nil {
			return fmt.Errorf("failed to create broadcaster: %v", err)
		}

		err = splitter.SplitUtxo(txid, satoshis, vout, dryrun)
		if err != nil {
			return fmt.Errorf("failed to create utxos: %v", err)
		}

		return nil
	},
}

func init() {
	logger := helper.GetLogger()
	Cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		// Hide unused persistent flags
		err := command.Flags().MarkHidden("fullStatusUpdates")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("err", err.Error()))
		}
		err = command.Flags().MarkHidden("callback")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("err", err.Error()))
		}
		err = command.Flags().MarkHidden("callbackToken")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("err", err.Error()))
		}
		// Call parent help func
		command.Parent().HelpFunc()(command, strings)
	})

	var err error
	Cmd.Flags().String("from", "", "Key from which to split")
	err = viper.BindPFlag("from", Cmd.Flags().Lookup("from"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().String("txid", "", "TX ID of UTXO to split")
	err = viper.BindPFlag("txid", Cmd.Flags().Lookup("txid"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Uint32("vout", 0, "UTXO position")
	err = viper.BindPFlag("vout", Cmd.Flags().Lookup("vout"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Bool("dryrun", false, "Whether or not to submit the splitting tx")
	err = viper.BindPFlag("dryrun", Cmd.Flags().Lookup("dryrun"))
	if err != nil {
		log.Fatal(err)
	}
}
