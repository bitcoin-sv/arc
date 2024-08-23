package create

import (
	"errors"
	"fmt"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	"log"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/woc_client"
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

		keySets, err := helper.GetKeySets()
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

		wocClient := woc_client.New(!isTestnet, woc_client.WithAuth(wocApiKey), woc_client.WithLogger(logger))

		ks := make([]*keyset.KeySet, len(keySets))
		counter := 0
		for _, keySet := range keySets {
			ks[counter] = keySet
			counter++
		}
		rateBroadcaster, err := broadcaster.NewUTXOCreator(logger, client, ks, wocClient, isTestnet,
			broadcaster.WithFees(miningFeeSat),
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
}
