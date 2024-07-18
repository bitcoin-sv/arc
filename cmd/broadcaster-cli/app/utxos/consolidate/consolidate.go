package consolidate

import (
	"errors"
	"fmt"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "consolidate",
	Short: "Consolidate UTXO set to 1 output",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		wocClient := woc_client.New(woc_client.WithAuth(wocApiKey), woc_client.WithLogger(logger))

		rateBroadcaster, err := broadcaster.NewUTXOConsolidator(logger, client, keySets, wocClient, isTestnet,
			broadcaster.WithFees(miningFeeSat),
			broadcaster.WithCallback(callbackURL, callbackToken),
			broadcaster.WithFullstatusUpdates(fullStatusUpdates),
		)
		if err != nil {
			return fmt.Errorf("failed to create broadcaster: %v", err)
		}

		err = rateBroadcaster.Consolidate()
		if err != nil {
			return fmt.Errorf("failed to consolidate utxos: %v", err)
		}

		return nil
	},
}
