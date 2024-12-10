package create

import (
	"errors"
	"fmt"
	"log"
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
		outputs, err := helper.GetInt("outputs")
		if err != nil {
			return err
		}
		if outputs == 0 {
			return errors.New("outputs must be a value greater than 0")
		}

		satoshisPerOutput, err := helper.GetUint64("satoshis")
		if err != nil {
			return err
		}
		if satoshisPerOutput == 0 {
			return errors.New("satoshis must be a value greater than 0")
		}

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
		creators := make([]broadcaster.Creator, 0, len(keySetsMap)) // Use the Creator interface for flexibility
		for _, keyName := range names {
			ks := keySetsMap[keyName]
			creator, err := broadcaster.NewUTXOCreator(
				logger.With(slog.String("address", ks.Address(!isTestnet)), slog.String("name", keyName)),
				client, ks, wocClient, isTestnet, broadcaster.WithFees(miningFeeSat),
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
		multiCreator.Start(outputs, uint64(satoshisPerOutput))

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

	Cmd.Flags().Int("outputs", 0, "Nr of requested outputs")
	err = viper.BindPFlag("outputs", Cmd.Flags().Lookup("outputs"))
	if err != nil {
		log.Fatal(err)
	}
}
