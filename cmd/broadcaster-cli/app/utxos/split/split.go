package split

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/pkg/keyset"
)

var (
	logger *slog.Logger
	Cmd    = &cobra.Command{
		Use:   "split",
		Short: "Split a UTXO",
		RunE: func(_ *cobra.Command, _ []string) error {
			txid := helper.GetString("txid")
			if txid == "" {
				return errors.New("txid is required")
			}

			from := helper.GetString("from")
			if from == "" {
				return errors.New("from is required")
			}

			satoshis := helper.GetUint64("satoshis")
			if satoshis == 0 {
				return errors.New("satoshis is required")
			}

			vout := helper.GetUint32("vout")

			dryrun, err := helper.GetBool("dryrun")
			if err != nil {
				return err
			}

			isTestnet, err := helper.GetBool("testnet")
			if err != nil {
				return err
			}

			authorization := helper.GetString("authorization")

			miningFeeSat := helper.GetUint64("miningFeeSatPerKb")

			arcServer := helper.GetString("apiURL")
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
			logLevel := helper.GetString("logLevel")
			logFormat := helper.GetString("logFormat")

			logger := helper.NewLogger(logLevel, logFormat)

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

			splitter, err := broadcaster.NewUTXOSplitter(logger, client, fromKs, ks, broadcaster.WithIsTestnet(isTestnet),
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
)

func init() {
	//logger := log.Default()

	logLevel := helper.GetString("logLevel")
	logFormat := helper.GetString("logFormat")
	logger = helper.NewLogger(logLevel, logFormat)

	Cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		// Hide unused persistent flags
		err := command.Flags().MarkHidden("fullStatusUpdates")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("flag", "fullStatusUpdates"), slog.String("err", err.Error()))
		}
		err = command.Flags().MarkHidden("callback")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("flag", "callback"), slog.String("err", err.Error()))
		}
		err = command.Flags().MarkHidden("callbackToken")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("flag", "callbackToken"), slog.String("err", err.Error()))
		}
		// Call parent help func
		command.Parent().HelpFunc()(command, strings)
	})

	var err error
	Cmd.Flags().String("from", "", "Key from which to split")
	err = viper.BindPFlag("from", Cmd.Flags().Lookup("from"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "from"), slog.String("err", err.Error()))
		return
	}

	Cmd.Flags().String("txid", "", "TX ID of UTXO to split")
	err = viper.BindPFlag("txid", Cmd.Flags().Lookup("txid"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "txid"), slog.String("err", err.Error()))
		return
	}

	Cmd.Flags().Uint32("vout", 0, "UTXO position")
	err = viper.BindPFlag("vout", Cmd.Flags().Lookup("vout"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "vout"), slog.String("err", err.Error()))
		return
	}

	Cmd.Flags().Bool("dryrun", false, "Whether or not to submit the splitting tx")
	err = viper.BindPFlag("dryrun", Cmd.Flags().Lookup("dryrun"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "dryrun"), slog.String("err", err.Error()))
		return
	}
}
