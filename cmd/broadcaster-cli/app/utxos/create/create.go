package create

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/lmittmann/tint"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:   "create",
	Short: "Create a UTXO set",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		keyFile, err := helper.GetString("keyFile")
		if err != nil {
			return err
		}
		if keyFile == "" {
			return errors.New("no key file was given")
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

		wocApiKey, err := helper.GetString("wocAPIKey")
		if err != nil {
			return err
		}

		logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelInfo}))

		client, err := helper.CreateClient(&broadcaster.Auth{
			Authorization: authorization,
		}, arcServer)
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}

		wocClient := woc_client.New(woc_client.WithAuth(wocApiKey))

		keyFiles := strings.Split(keyFile, ",")

		wg := &sync.WaitGroup{}

		for _, kf := range keyFiles {

			if wocApiKey == "" {
				time.Sleep(1 * time.Second)
			}

			wg.Add(1)

			go func(keyfile string, waitGroup *sync.WaitGroup) {
				defer waitGroup.Done()

				time.Sleep(500 * time.Millisecond)

				fundingKeySet, _, err := helper.GetKeySetsKeyFile(keyfile)
				if err != nil {
					logger.Error("failed to get key sets", slog.String("err", err.Error()))
					return
				}

				rateBroadcaster, _ := broadcaster.NewRateBroadcaster(logger, client, fundingKeySet, wocClient, broadcaster.WithFees(miningFeeSat), broadcaster.WithIsTestnet(isTestnet), broadcaster.WithCallback(callbackURL, callbackToken), broadcaster.WithFullstatusUpdates(fullStatusUpdates))

				err = rateBroadcaster.CreateUtxos(outputs, satoshisPerOutput)
				if err != nil {
					logger.Error("failed to create utxos", slog.String("address", fundingKeySet.Address(!isTestnet)), slog.String("err", err.Error()))
					return
				}
			}(kf, wg)
		}

		wg.Wait()

		return nil
	},
}

func init() {
	var err error

	Cmd.Flags().Int("outputs", 10, "Nr of requested outputs")
	err = viper.BindPFlag("outputs", Cmd.Flags().Lookup("outputs"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Int("satoshis", 1000, "Nr of satoshis per output outputs")
	err = viper.BindPFlag("satoshis", Cmd.Flags().Lookup("satoshis"))
	if err != nil {
		log.Fatal(err)
	}
}
