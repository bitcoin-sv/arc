package broadcast

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/woc_client"
)

var Cmd = &cobra.Command{
	Use:   "broadcast",
	Short: "Submit transactions to ARC",
	RunE: func(_ *cobra.Command, _ []string) error {
		rateTxsPerSecond, err := helper.GetInt("rate")
		if err != nil {
			return err
		}
		if rateTxsPerSecond == 0 {
			return errors.New("rate must be a value greater than 0")
		}

		waitForStatus, err := helper.GetInt("waitForStatus")
		if err != nil {
			return err
		}

		batchSize, err := helper.GetInt("batchsize")
		if err != nil {
			return err
		}
		if batchSize == 0 {
			return errors.New("batch size must be a value greater than 0")
		}

		limit, err := helper.GetInt64("limit")
		if err != nil {
			return err
		}

		fullStatusUpdates, err := helper.GetBool("fullStatusUpdates")
		if err != nil {
			return err
		}

		isTestnet, err := helper.GetBool("testnet")
		if err != nil {
			return err
		}

		callbackURL, err := helper.GetString("callback")
		if err != nil {
			return err
		}

		callbackToken, err := helper.GetString("callbackToken")
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

		if miningFeeSat == 0 {
			return errors.New("no mining fee was given")
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

		opReturn, err := helper.GetString("opReturn")
		if err != nil {
			return err
		}

		sizeJitterMax, err := helper.GetInt("sizeJitter")
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

		wocClient := woc_client.New(!isTestnet, woc_client.WithAuth(wocAPIKey), woc_client.WithLogger(logger))

		opts := []func(p *broadcaster.Broadcaster){
			broadcaster.WithFees(miningFeeSat),
			broadcaster.WithCallback(callbackURL, callbackToken),
			broadcaster.WithFullstatusUpdates(fullStatusUpdates),
			broadcaster.WithBatchSize(batchSize),
			broadcaster.WithOpReturn(opReturn),
			broadcaster.WithSizeJitter(sizeJitterMax),
		}

		if waitForStatus > 0 {
			opts = append(opts, broadcaster.WithWaitForStatus(metamorph_api.Status(waitForStatus)))
		}

		rbs := make([]broadcaster.RateBroadcaster, 0, len(keySetsMap))
		for keyName, ks := range keySetsMap {
			rb, err := broadcaster.NewRateBroadcaster(logger.With(slog.String("address", ks.Address(!isTestnet)), slog.String("name", keyName)), client, ks, wocClient, isTestnet, rateTxsPerSecond, limit, opts...)
			if err != nil {
				return err
			}

			rbs = append(rbs, rb)
		}

		rateBroadcaster := broadcaster.NewMultiKeyRateBroadcaster(logger, rbs)

		doneChan := make(chan error) // Channel to signal the completion of Start
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt) // Listen for Ctrl+C

		go func() {
			// Start the broadcasting process
			err := rateBroadcaster.Start()
			logger.Info("Starting broadcaster", slog.Int("rate [txs/s]", rateTxsPerSecond), slog.Int("batch size", batchSize))
			doneChan <- err // Send the completion or error signal
		}()

		select {
		case <-signalChan:
			// If an interrupt signal is received
			logger.Info("Shutdown signal received. Shutting down the rate broadcaster.")
		case err := <-doneChan:
			if err != nil {
				logger.Error("Error during broadcasting", slog.String("err", err.Error()))
			}
		}

		// Shutdown the broadcaster in all cases
		rateBroadcaster.Shutdown()
		logger.Info("Broadcasting shutdown complete")
		return nil
	},
}

func init() {
	logger := helper.GetLogger()
	Cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		// Hide unused persistent flags
		err := command.Flags().MarkHidden("satoshis")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("err", err.Error()))
		}

		// Call parent help func
		command.Parent().HelpFunc()(command, strings)
	})

	var err error
	Cmd.Flags().Int("rate", 10, "Transactions per second to be rate broad casted per key set")
	err = viper.BindPFlag("rate", Cmd.Flags().Lookup("rate"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Int("batchsize", 10, "Size of batches to submit transactions")
	err = viper.BindPFlag("batchsize", Cmd.Flags().Lookup("batchsize"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Int("limit", 0, "Limit to number of transactions to be submitted after which broadcaster will stop per key set, default: no limit")
	err = viper.BindPFlag("limit", Cmd.Flags().Lookup("limit"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Int("waitForStatus", 0, "Transaction status for which broadcaster should wait")
	err = viper.BindPFlag("waitForStatus", Cmd.Flags().Lookup("waitForStatus"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().String("opReturn", "", "Text which will be added to an OP_RETURN output. If empty, no OP_RETURN output will be added")
	err = viper.BindPFlag("opReturn", Cmd.Flags().Lookup("opReturn"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Int("sizeJitter", 0, "Enable the option to randomise the transaction size by adding random data and using multiple inputs, the parameter specifies the maximum size of bytes that will be added to OP_RETURN (plus OP_RETURN header if specified)")
	err = viper.BindPFlag("sizeJitter", Cmd.Flags().Lookup("sizeJitter"))
	if err != nil {
		log.Fatal(err)
	}
}
