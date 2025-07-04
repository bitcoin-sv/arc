package create

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/pkg/woc_client"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:   "create",
	Short: "Create a UTXO set",
	RunE: func(_ *cobra.Command, _ []string) error {
		outputs := helper.GetUint64("outputs")
		if outputs == 0 {
			return errors.New("outputs must be a value greater than 0")
		}

		satoshisPerOutput := helper.GetUint64("satoshis")
		if satoshisPerOutput == 0 {
			return errors.New("satoshis must be a value greater than 0")
		}

		isTestnet := helper.GetBool("testnet")
		authorization := helper.GetString("authorization")

		keySetsMap, err := helper.GetSelectedKeySets()
		if err != nil {
			return err
		}

		miningFeeSat := helper.GetUint64("miningFeeSatPerKb")

		arcServer := helper.GetString("apiURL")
		if arcServer == "" {
			return errors.New("no api URL was given")
		}

		wocAPIKey := helper.GetString("wocAPIKey")

		names := helper.GetOrderedKeys(keySetsMap)

		logLevel := helper.GetString("logLevel")
		logFormat := helper.GetString("logFormat")
		logger := helper.NewLogger(logLevel, logFormat)

		client, err := helper.CreateClient(&broadcaster.Auth{Authorization: authorization}, arcServer, logger)
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}

		wocClient := woc_client.New(!isTestnet, woc_client.WithAuth(wocAPIKey), woc_client.WithLogger(logger))
		creators := make([]broadcaster.Creator, 0, len(keySetsMap)) // Use the Creator interface for flexibility
		for _, keyName := range names {
			ks := keySetsMap[keyName]
			creator, err := broadcaster.NewUTXOCreator(
				logger.With(slog.String("address", ks.Address(!isTestnet)), slog.String("name", keyName)),
				client, ks, wocClient, broadcaster.WithIsTestnet(isTestnet), broadcaster.WithFees(miningFeeSat),
			)
			if err != nil {
				return err
			}

			creators = append(creators, creator)
		}

		multiCreator := broadcaster.NewMultiKeyUTXOCreator(logger, creators)

		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt) // Listen for Ctrl+C

		// Graceful shutdown on interrupt signal
		go func() {
			<-signalChan
			multiCreator.Shutdown()
		}()

		logger.Info("Starting UTXO creation")
		multiCreator.Start(outputs, satoshisPerOutput)
		return nil
	},
}

func init() {
	logger := helper.NewLogger("INFO", "tint")

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

	Cmd.Flags().Int("outputs", 0, "Nr of requested outputs")
	err = viper.BindPFlag("outputs", Cmd.Flags().Lookup("outputs"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "outputs"), slog.String("err", err.Error()))
		return
	}
}
