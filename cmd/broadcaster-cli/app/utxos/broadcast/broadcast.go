package broadcast

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/woc_client"
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
		fullStatusUpdates := viper.GetBool("fullStatusUpdates")
		limit := viper.GetInt("limit")

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
		keyFile, err := helper.GetString("keyFile")
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
		wocApiKey, err := helper.GetString("wocAPIKey")
		if err != nil {
			return err
		}
		keyFiles := strings.Split(keyFile, ",")

		logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelInfo}))

		client, err := helper.CreateClient(&broadcaster.Auth{
			Authorization: authorization,
		}, arcServer)
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}

		rbs := make([]*broadcaster.RateBroadcaster, len(keyFiles))

		wg := &sync.WaitGroup{}

		var resultsPath string
		if store {
			network := "mainnet"
			if isTestnet {
				network = "testnet"
			}
			resultsPath = filepath.Join(".", fmt.Sprintf("results/%s-%s-rate-%d-batchsize-%d", network, time.Now().Format(time.DateTime), rateTxsPerSecond, batchSize))
			err := os.MkdirAll(resultsPath, os.ModePerm)
			if err != nil {
				return err
			}
		}

		for i, kf := range keyFiles {

			wg.Add(1)

			fundingKeySet, receivingKeySet, err := helper.GetKeySetsKeyFile(kf)
			if err != nil {
				return fmt.Errorf("failed to get key sets: %v", err)
			}

			wocClient := woc_client.New(woc_client.WithAuth(wocApiKey))

			var writer io.Writer
			if store {

				_, keyFileName := filepath.Split(kf)

				file, err := os.Create(fmt.Sprintf("%s/%s.json", resultsPath, keyFileName))
				if err != nil {
					return err
				}

				writer = file

				defer file.Close()
			}

			rateBroadcaster, err := broadcaster.NewRateBroadcaster(logger, client, fundingKeySet, receivingKeySet, wocClient,
				broadcaster.WithFees(miningFeeSat),
				broadcaster.WithIsTestnet(isTestnet),
				broadcaster.WithCallback(callbackURL, callbackToken),
				broadcaster.WithFullstatusUpdates(fullStatusUpdates),
				broadcaster.WithBatchSize(batchSize),
				broadcaster.WithStoreWriter(writer, 50),
			)
			if err != nil {
				return fmt.Errorf("failed to create rate broadcaster: %v", err)
			}

			rbs[i] = rateBroadcaster

			err = rateBroadcaster.StartRateBroadcaster(rateTxsPerSecond, limit, wg)
			if err != nil {
				return fmt.Errorf("failed to start rate broadcaster: %v", err)
			}
		}

		go func() {
			signalChan := make(chan os.Signal, 1)
			signal.Notify(signalChan, os.Interrupt) // Signal from Ctrl+C
			<-signalChan
			for _, rb := range rbs {
				rb.Shutdown()
			}
		}()

		wg.Wait()

		return nil
	},
}

func init() {
	var err error

	Cmd.Flags().Int("rate", 10, "Transactions per second to be rate broad casted")
	err = viper.BindPFlag("rate", Cmd.Flags().Lookup("rate"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Int("batchsize", 10, "Size of batches to submit transactions")
	err = viper.BindPFlag("batchsize", Cmd.Flags().Lookup("batchsize"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Int("limit", 0, "Limit to number of transactions to be submitted after which broadcaster will stop, default: no limit")
	err = viper.BindPFlag("limit", Cmd.Flags().Lookup("limit"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Bool("store", false, "Store results in a json file instead of printing")
	err = viper.BindPFlag("store", Cmd.Flags().Lookup("store"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().BoolP("fullstatusupdates", "f", false, fmt.Sprintf("Send callbacks for %s or %s status", metamorph_api.Status_SEEN_ON_NETWORK.String(), metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL.String()))
	err = viper.BindPFlag("fullStatusUpdates", Cmd.Flags().Lookup("fullstatusupdates"))
	if err != nil {
		log.Fatal(err)
	}
}
