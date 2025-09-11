package broadcast

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/woc_client"
)

const (
	millisecondsPerSecond = 1000
)

var (
	ErrTooHighSubmissionRate = errors.New("submission rate is too high")
)

var Cmd = &cobra.Command{
	Use:   "broadcast",
	Short: "Submit transactions to ARC",
	RunE: func(_ *cobra.Command, _ []string) error {
		rateTxsPerSecond := helper.GetInt("rate")
		if rateTxsPerSecond == 0 {
			return errors.New("rate must be a value greater than 0")
		}

		waitForStatus := helper.GetInt("waitForStatus")

		batchSize := helper.GetInt("batchsize")
		if batchSize == 0 {
			return errors.New("batch size must be a value greater than 0")
		}

		limit := helper.GetInt64("limit")

		fullStatusUpdates := helper.GetBool("fullStatusUpdates")

		isTestnet := helper.GetBool("testnet")

		callbackURL := helper.GetString("callbackURL")

		callbackToken := helper.GetString("callbackToken")

		authorization := helper.GetString("authorization")

		keySetsMap, err := helper.GetSelectedKeySets()
		if err != nil {
			return err
		}

		miningFeeSat := helper.GetUint64("miningFeeSatPerKb")

		if miningFeeSat == 0 {
			return errors.New("no mining fee was given")
		}

		arcServer := helper.GetString("apiURL")
		if arcServer == "" {
			return errors.New("no api URL was given")
		}

		wocAPIKey := helper.GetString("wocAPIKey")

		opReturn := helper.GetString("opReturn")

		rampUpTickerEnabled := helper.GetBool("rampUpTickerEnabled")

		sizeJitterMax := helper.GetInt64("sizeJitter")

		logLevel := helper.GetString("logLevel")
		logFormat := helper.GetString("logFormat")
		logger := helper.NewLogger(logLevel, logFormat)

		client, err := helper.CreateClient(&broadcaster.Auth{Authorization: authorization}, arcServer, logger)
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
			broadcaster.WithIsTestnet(isTestnet),
		}

		if waitForStatus > 0 {
			opts = append(opts, broadcaster.WithWaitForStatus(metamorph_api.Status(waitForStatus)))
		}

		submitBatchesPerSecond := float64(rateTxsPerSecond) / float64(batchSize)

		if submitBatchesPerSecond > millisecondsPerSecond {
			return errors.Join(ErrTooHighSubmissionRate, fmt.Errorf("submission rate %d [txs/s] and batch size %d [txs] result in submission frequency %.2f greater than 1000 [/s]", rateTxsPerSecond, batchSize, submitBatchesPerSecond))
		}
		submitBatchInterval := time.Duration(millisecondsPerSecond/float64(submitBatchesPerSecond)) * time.Millisecond

		var submitBatchTicker broadcaster.Ticker
		submitBatchTicker = broadcaster.NewConstantTicker(submitBatchInterval)

		if rampUpTickerEnabled {
			submitBatchTicker, err = broadcaster.NewRampUpTicker(5*time.Second+submitBatchInterval, submitBatchInterval, 10)
			if err != nil {
				return err
			}
		}

		rbs := make([]broadcaster.RateBroadcaster, 0, len(keySetsMap))
		for keyName, ks := range keySetsMap {
			rb, err := broadcaster.NewRateBroadcaster(logger.With(slog.String("address", ks.Address(!isTestnet)), slog.String("name", keyName)), client, ks, wocClient, limit, submitBatchTicker, opts...)
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
			// Todo: print all settings

			// Todo: start all rate broadcasters at once

			// Start the broadcasting process
			logger.Info("Starting rate broadcaster", slog.Int("rate [txs/s]", rateTxsPerSecond), slog.Int("batch size", batchSize), slog.String("batch interval", submitBatchInterval.String()), slog.Int("parallel", rateBroadcaster.Len()))
			err := rateBroadcaster.Start()
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
	logger := helper.NewLogger("INFO", "tint")

	Cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		// Hide unused persistent flags
		err := command.Flags().MarkHidden("satoshis")
		if err != nil {
			logger.Error("failed to mark flag hidden", slog.String("flag", "satoshis"), slog.String("err", err.Error()))
		}

		// Call parent help func
		command.Parent().HelpFunc()(command, strings)
	})

	var err error
	Cmd.Flags().Int("rate", 10, "Transactions per second to be rate broad casted per key set")
	err = viper.BindPFlag("rate", Cmd.Flags().Lookup("rate"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "rate"), slog.String("err", err.Error()))
		return
	}

	Cmd.Flags().Int("batchsize", 10, "Size of batches to submit transactions")
	err = viper.BindPFlag("batchsize", Cmd.Flags().Lookup("batchsize"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "batchsize"), slog.String("err", err.Error()))
		return
	}

	Cmd.Flags().Int("limit", 0, "Limit to number of transactions to be submitted after which broadcaster will stop per key set, default: no limit")
	err = viper.BindPFlag("limit", Cmd.Flags().Lookup("limit"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "limit"), slog.String("err", err.Error()))
		return
	}

	Cmd.Flags().Int("waitForStatus", 0, "Transaction status for which broadcaster should wait")
	err = viper.BindPFlag("waitForStatus", Cmd.Flags().Lookup("waitForStatus"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "waitForStatus"), slog.String("err", err.Error()))
		return
	}

	Cmd.Flags().String("opReturn", "", "Text which will be added to an OP_RETURN output. If empty, no OP_RETURN output will be added")
	err = viper.BindPFlag("opReturn", Cmd.Flags().Lookup("opReturn"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "opReturn"), slog.String("err", err.Error()))
		return
	}

	Cmd.Flags().Bool("rampUpTickerEnabled", false, "If enabled, the ramp up ticker will start broadcasting the transaction rate slowly until it reaches the final rate")
	err = viper.BindPFlag("rampUpTickerEnabled", Cmd.Flags().Lookup("rampUpTickerEnabled"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "rampUpTickerEnabled"), slog.String("err", err.Error()))
		return
	}

	Cmd.Flags().Int("sizeJitter", 0, "Enable the option to randomise the transaction size by adding random data and using multiple inputs, the parameter specifies the maximum size of bytes that will be added to OP_RETURN (plus OP_RETURN header if specified)")
	err = viper.BindPFlag("sizeJitter", Cmd.Flags().Lookup("sizeJitter"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "sizeJitter"), slog.String("err", err.Error()))
		return
	}
}
