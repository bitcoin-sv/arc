package broadcast

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bitcoin-sv/arc/broadcaster"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/bitcoin-sv/arc/lib/woc_client"
	"github.com/lmittmann/tint"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:   "broadcast",
	Short: "Submit transactions to ARC",
	RunE: func(cmd *cobra.Command, args []string) error {

		store := viper.GetBool("store")
		rateTxsPerSecond := viper.GetInt("rate")
		batchSize := viper.GetInt("batchsize")

		isTestnet := helper.GetBool("testnet")
		callbackURL := helper.GetString("callback")
		callbackToken := helper.GetString("callbackToken")
		authorization := helper.GetString("authorization")
		keyFile := helper.GetString("keyFile")
		miningFeeSat := helper.GetInt("miningFeeSatPerKb")
		arcServer := helper.GetString("apiURL")

		logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelInfo}))

		var client broadcaster.ArcClient

		client, err := helper.CreateClient(&broadcaster.Auth{
			Authorization: authorization,
		}, arcServer)
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
			resultsPath := filepath.Join(".", "results")
			err := os.MkdirAll(resultsPath, os.ModePerm)
			if err != nil {
				return err
			}

			network := "mainnet"
			if isTestnet {
				network = "testnet"
			}
			writer, err = os.Create(fmt.Sprintf("results/%s-batchsize-%d-rate-%d-%s.json", network, batchSize, rateTxsPerSecond, time.Now().Format(time.DateTime)))
			if err != nil {
				return err
			}
		}

		rateBroadcaster, err := broadcaster.NewRateBroadcaster(logger, client, fundingKeySet, receivingKeySet, &wocClient,
			broadcaster.WithFees(miningFeeSat),
			broadcaster.WithIsTestnet(isTestnet),
			broadcaster.WithCallback(callbackURL, callbackToken),
			broadcaster.WithBatchSize(batchSize),
			broadcaster.WithStoreWriter(writer, 50),
		)
		if err != nil {
			return fmt.Errorf("failed to create rate broadcaster: %v", err)
		}

		err = rateBroadcaster.StartRateBroadcaster(rateTxsPerSecond)
		if err != nil {
			return fmt.Errorf("failed to start rate broadcaster: %v", err)
		}

		defer rateBroadcaster.Shutdown()

		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGTERM)
		signal.Notify(signalChan, os.Interrupt) // Signal from Ctrl+C

		// wait for termination of program
		<-signalChan

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
