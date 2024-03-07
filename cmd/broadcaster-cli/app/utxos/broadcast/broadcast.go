package broadcast

import (
	"fmt"
	"github.com/bitcoin-sv/arc/broadcaster"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/bitcoin-sv/arc/lib/woc_client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var Cmd = &cobra.Command{
	Use:   "broadcast",
	Short: "Submit transactions to ARC",
	RunE: func(cmd *cobra.Command, args []string) error {

		rateTxsPerSecond := viper.GetInt("rate")
		batchSize := viper.GetInt("batchsize")
		store := viper.GetBool("store")

		isTestnet := viper.GetBool("testnet")
		callbackURL := viper.GetString("callback")
		authorization := viper.GetString("authorization")
		keyFile := viper.GetString("keyFile")
		miningFeeSat := viper.GetInt("broadcaster.miningFeeSatPerKb")

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

		var client broadcaster.ArcClient
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

		wocClient := woc_client.New()

		var writer io.Writer
		if store {
			writer, err = os.Open(fmt.Sprintf("results/responses-%s.json", time.Now().Format(time.DateTime)))
			if err != nil {
				return err
			}
		}

		preparer, err := broadcaster.NewRateBroadcaster(logger, client, fundingKeySet, receivingKeySet, &wocClient,
			broadcaster.WithFees(miningFeeSat),
			broadcaster.WithIsTestnet(isTestnet),
			broadcaster.WithCallbackURL(callbackURL),
			broadcaster.WithBatchSize(batchSize),
			broadcaster.WithStoreWriter(writer, 50),
		)
		if err != nil {
			return fmt.Errorf("failed to create rate broadcaster: %v", err)
		}

		shutdown := make(chan struct{})
		shutdownComplete := make(chan struct{})

		go func() {
			err = preparer.Broadcast(rateTxsPerSecond, shutdown, shutdownComplete)
			if err != nil {
				logger.Error("failed to broadcast back txs", slog.String("err", err.Error()))
				return
			}
		}()

		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGTERM)

		// wait for termination of program
		<-signalChan

		shutdown <- struct{}{}

		// wait for graceful shutdown
		<-shutdownComplete

		return nil
	},
}

func init() {
	var err error

	Cmd.Flags().Int("rate", 10, "transactions per second to be rate broadcasted")
	err = viper.BindPFlag("rate", Cmd.Flags().Lookup("rate"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Int("batchsize", 10, "size of batches to submit transactions")
	err = viper.BindPFlag("batchsize", Cmd.Flags().Lookup("batchsize"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Bool("store", false, "Store results in a json file instead of printing")
	err = viper.BindPFlag("store", Cmd.Flags().Lookup("store"))
	if err != nil {
		log.Fatal(err)
	}

}
