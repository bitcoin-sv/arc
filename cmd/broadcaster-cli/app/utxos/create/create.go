package create

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/bitcoin-sv/arc/pkg/keyset"
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

		logger := helper.GetLogger()

		client, err := helper.CreateClient(&broadcaster.Auth{
			Authorization: authorization,
		}, arcServer)
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}
		if err != nil {
			return fmt.Errorf("failed to create client: %v", err)
		}

		wocClient := woc_client.New(woc_client.WithAuth(wocApiKey), woc_client.WithLogger(logger))

		keyFiles := strings.Split(keyFile, ",")

		fundingKeySets := make([]*keyset.KeySet, len(keyFiles))

		for i, kf := range keyFiles {

			fundingKeySet, _, err := helper.GetKeySetsKeyFile(kf)
			if err != nil {
				return fmt.Errorf("failed to get key sets: %v", err)
			}

			fundingKeySets[i] = fundingKeySet
		}

		rateBroadcaster, err := broadcaster.NewUTXOCreator(logger, client, fundingKeySets, wocClient,
			broadcaster.WithFees(miningFeeSat),
			broadcaster.WithIsTestnet(isTestnet),
			broadcaster.WithCallback(callbackURL, callbackToken),
			broadcaster.WithFullstatusUpdates(fullStatusUpdates),
		)
		if err != nil {
			return fmt.Errorf("failed to create broadcaster: %v", err)
		}

		err = rateBroadcaster.CreateUtxos(outputs, satoshisPerOutput)
		if err != nil {
			return fmt.Errorf("failed to create utxos: %v", err)
		}

		return nil
	},
}

func init() {
	var err error

	Cmd.Flags().Int("outputs", 0, "Nr of requested outputs")
	err = viper.BindPFlag("outputs", Cmd.Flags().Lookup("outputs"))
	if err != nil {
		log.Fatal(err)
	}

	Cmd.Flags().Int("satoshis", 0, "Nr of satoshis per output outputs")
	err = viper.BindPFlag("satoshis", Cmd.Flags().Lookup("satoshis"))
	if err != nil {
		log.Fatal(err)
	}
}
