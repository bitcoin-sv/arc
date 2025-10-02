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

type Config struct {
	RateTxsPerSecond    int
	WaitForStatus       int
	CallbackURL         string
	CallbackToken       string
	ArcServer           string
	IsTestnet           bool
	BatchSize           int
	FullStatusUpdates   bool
	Limit               int64
	OpReturn            string
	RampUpTickerEnabled bool
	SizeJitterMax       int64
	Authorization       string
	WocAPIKey           string
	MiningFeeSat        uint64
	LogLevel            string
	LogFormat           string
	MinFeeSat           uint64
}

func (c Config) Log(logger *slog.Logger) {
	logger.Info("config",
		"rate", c.RateTxsPerSecond,
		"batchSize", c.BatchSize,
		"waitForStatus", c.WaitForStatus,
		"callbackURL", c.CallbackURL,
		"callbackToken", c.CallbackToken,
		"arcServer", c.ArcServer,
		"isTestnet", c.IsTestnet,
		"fullStatusUpdates", c.FullStatusUpdates,
		"limit", c.Limit,
		"opReturn", c.OpReturn,
		"rampUpTickerEnabled", c.RampUpTickerEnabled,
		"sizeJitterMax", c.SizeJitterMax,
		"authorization", c.Authorization,
		"wocAPIKey", c.WocAPIKey,
		"miningFeeSat", c.MiningFeeSat,
		"logLevel", c.LogLevel,
		"logFormat", c.LogFormat,
		"minFeeSat", c.MinFeeSat,
	)
}

var Cmd = &cobra.Command{
	Use:   "broadcast",
	Short: "Submit transactions to ARC",
	RunE: func(_ *cobra.Command, _ []string) error {
		cfg := Config{}

		cfg.RateTxsPerSecond = helper.GetInt("rate")
		if cfg.RateTxsPerSecond == 0 {
			return errors.New("rate must be a value greater than 0")
		}

		cfg.WaitForStatus = helper.GetInt("waitForStatus")

		cfg.BatchSize = helper.GetInt("batchsize")
		if cfg.BatchSize == 0 {
			return errors.New("batch size must be a value greater than 0")
		}

		cfg.Limit = helper.GetInt64("limit")

		cfg.FullStatusUpdates = helper.GetBool("fullStatusUpdates")

		cfg.IsTestnet = helper.GetBool("testnet")

		cfg.CallbackURL = helper.GetString("callbackURL")

		cfg.CallbackToken = helper.GetString("callbackToken")

		cfg.Authorization = helper.GetString("authorization")

		keySetsMap, err := helper.GetSelectedKeySets()
		if err != nil {
			return err
		}

		cfg.MiningFeeSat = helper.GetUint64("miningFeeSatPerKb")

		if cfg.MiningFeeSat == 0 {
			return errors.New("no mining fee was given")
		}

		cfg.ArcServer = helper.GetString("apiURL")
		if cfg.ArcServer == "" {
			return errors.New("no api URL was given")
		}

		cfg.WocAPIKey = helper.GetString("wocAPIKey")

		cfg.OpReturn = helper.GetString("opReturn")

		cfg.RampUpTickerEnabled = helper.GetBool("rampUpTickerEnabled")

		cfg.SizeJitterMax = helper.GetInt64("sizeJitter")

		cfg.LogLevel = helper.GetString("logLevel")
		cfg.LogFormat = helper.GetString("logFormat")
		logger := helper.NewLogger(cfg.LogLevel, cfg.LogFormat)

		client, err := helper.CreateClient(&broadcaster.Auth{Authorization: cfg.Authorization}, cfg.ArcServer, logger)
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}

		wocClient := woc_client.New(!cfg.IsTestnet, woc_client.WithAuth(cfg.WocAPIKey), woc_client.WithLogger(logger))

		opts := []func(p *broadcaster.Broadcaster){
			broadcaster.WithFees(cfg.MiningFeeSat),
			broadcaster.WithCallback(cfg.CallbackURL, cfg.CallbackToken),
			broadcaster.WithFullstatusUpdates(cfg.FullStatusUpdates),
			broadcaster.WithBatchSize(cfg.BatchSize),
			broadcaster.WithOpReturn(cfg.OpReturn),
			broadcaster.WithSizeJitter(cfg.SizeJitterMax),
			broadcaster.WithIsTestnet(cfg.IsTestnet),
		}

		if cfg.WaitForStatus > 0 {
			opts = append(opts, broadcaster.WithWaitForStatus(metamorph_api.Status(cfg.WaitForStatus)))
		}

		submitBatchesPerSecond := float64(cfg.RateTxsPerSecond) / float64(cfg.BatchSize)

		if submitBatchesPerSecond > millisecondsPerSecond {
			return errors.Join(ErrTooHighSubmissionRate, fmt.Errorf("submission rate %d [txs/s] and batch size %d [txs] result in submission frequency %.2f greater than 1000 [/s]", cfg.RateTxsPerSecond, cfg.BatchSize, submitBatchesPerSecond))
		}
		submitBatchInterval := time.Duration(millisecondsPerSecond/float64(submitBatchesPerSecond)) * time.Millisecond

		rbs := make([]broadcaster.RateBroadcaster, 0, len(keySetsMap))
		for keyName, ks := range keySetsMap {
			var submitBatchTicker broadcaster.Ticker
			submitBatchTicker = broadcaster.NewConstantTicker(submitBatchInterval)
			if cfg.RampUpTickerEnabled {
				submitBatchTicker, err = broadcaster.NewRampUpTicker(5*time.Second+submitBatchInterval, submitBatchInterval, 10)
				if err != nil {
					return err
				}
			}
			rb, err := broadcaster.NewRateBroadcaster(logger.With(slog.String("address", ks.Address(!cfg.IsTestnet)), slog.String("name", keyName)), client, ks, wocClient, cfg.Limit, submitBatchTicker, opts...)
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
			cfg.Log(logger)
			keyNames := helper.GetOrderedKeys(keySetsMap)
			logger.Info("keys", slog.Any("names", keyNames))
			logger.Info("submit batch interval", slog.String("interval", submitBatchInterval.String()))

			projectedTimeSeconds := float64(cfg.Limit) / float64(cfg.RateTxsPerSecond)
			const delayMargin = 1.2
			timeout := time.Duration(projectedTimeSeconds*delayMargin) * time.Second // Add 20% to account for potential delays
			// Start the broadcasting process
			err := rateBroadcaster.Start(timeout)
			doneChan <- err // Send the completion or error signal
		}()

		select {
		case <-signalChan:
			// If an interrupt signal is received
			logger.Info("Shutdown signal received")
		case err := <-doneChan:
			if err != nil {
				logger.Error("Error during broadcasting", slog.String("err", err.Error()))
			}
		}

		// Shutdown the broadcaster in all cases
		rateBroadcaster.Shutdown()
		logger.Info("Shutdown complete")
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
