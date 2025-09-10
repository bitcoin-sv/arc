package utxos

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos/broadcast"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos/consolidate"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos/create"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/utxos/split"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
)

var Cmd = &cobra.Command{
	Use:   "utxos",
	Short: "Create UTXO set to be used with broadcaster",
}

func init() {
	var err error

	logger := helper.NewLogger("INFO", "tint")

	Cmd.PersistentFlags().String("apiURL", "", "URL of ARC api")
	err = viper.BindPFlag("apiURL", Cmd.PersistentFlags().Lookup("apiURL"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "apiURL"), slog.String("err", err.Error()))
		return
	}

	Cmd.PersistentFlags().String("authorization", "", "Authorization header to use for the http api client")
	err = viper.BindPFlag("authorization", Cmd.PersistentFlags().Lookup("authorization"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "authorization"), slog.String("err", err.Error()))
		return
	}

	Cmd.PersistentFlags().String("callback", "", "URL which will be called with ARC callbacks")
	err = viper.BindPFlag("callback", Cmd.PersistentFlags().Lookup("callback")) // Todo: Change to callbackURL
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "callback"), slog.String("err", err.Error()))
		return
	}

	Cmd.PersistentFlags().String("callbackToken", "", "Token used as authentication header to be sent with ARC callbacks")
	err = viper.BindPFlag("callbackToken", Cmd.PersistentFlags().Lookup("callbackToken"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "callbackToken"), slog.String("err", err.Error()))
		return
	}

	Cmd.PersistentFlags().Int("miningFeeSatPerKb", 1, "Mining fee offered in transactions [sat/kb]")
	err = viper.BindPFlag("miningFeeSatPerKb", Cmd.PersistentFlags().Lookup("miningFeeSatPerKb"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "miningFeeSatPerKb"), slog.String("err", err.Error()))
		return
	}

	Cmd.PersistentFlags().BoolP("fullStatusUpdates", "f", false, fmt.Sprintf("Send callbacks for %s or %s status", metamorph_api.Status_SEEN_ON_NETWORK.String(), metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL.String()))
	err = viper.BindPFlag("fullStatusUpdates", Cmd.PersistentFlags().Lookup("fullStatusUpdates"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "fullStatusUpdates"), slog.String("err", err.Error()))
		return
	}

	Cmd.PersistentFlags().Int("satoshis", 0, "Nr of satoshis per output outputs")
	err = viper.BindPFlag("satoshis", Cmd.PersistentFlags().Lookup("satoshis"))
	if err != nil {
		logger.Error("failed to bind flag", slog.String("flag", "satoshis"), slog.String("err", err.Error()))
		return
	}

	Cmd.AddCommand(create.Cmd)
	Cmd.AddCommand(broadcast.Cmd)
	Cmd.AddCommand(consolidate.Cmd)
	Cmd.AddCommand(split.Cmd)
}
