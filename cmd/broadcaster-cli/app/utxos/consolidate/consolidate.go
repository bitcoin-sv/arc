package consolidate

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/pkg/woc_client"
)

var Cmd = &cobra.Command{
	Use:   "consolidate",
	Short: "Consolidate UTXO set to 1 output",
	RunE: func(_ *cobra.Command, _ []string) error {
		isTestnet, err := helper.GetBool("testnet")
		if err != nil {
			return err
		}

		authorization, err := helper.GetString("authorization")
		if err != nil {
			return err
		}

		keySetsMap, err := helper.GetSelectedKeySets()
		if err != nil {
			return err
		}
		miningFeeSat, err := helper.GetInt("miningFeeSatPerKb")
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

		wocAPIKey, err := helper.GetString("wocAPIKey")
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

		names := helper.GetOrderedKeys(keySetsMap)

		wocClient := woc_client.New(!isTestnet, woc_client.WithAuth(wocAPIKey), woc_client.WithLogger(logger))
		cs := make([]broadcaster.Consolidator, 0, len(keySetsMap))
		for _, keyName := range names {
			ks := keySetsMap[keyName]
			c, err := broadcaster.NewUTXOConsolidator(logger.With(slog.String("address", ks.Address(!isTestnet)), slog.String("name", keyName)), client, ks, wocClient, isTestnet, broadcaster.WithFees(miningFeeSat))
			if err != nil {
				return err
			}

			cs = append(cs, c)
		}
		consolidator := broadcaster.NewMultiKeyUtxoConsolidator(logger, cs)

		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt) // Listen for Ctrl+C

		go func() {
			<-signalChan
			consolidator.Shutdown()
		}()

		logger.Info("Starting consolidator")
		consolidator.Start()

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
		err = command.Flags().MarkHidden("satoshis")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("err", err.Error()))
		}
		// Call parent help func
		command.Parent().HelpFunc()(command, strings)
	})
}
