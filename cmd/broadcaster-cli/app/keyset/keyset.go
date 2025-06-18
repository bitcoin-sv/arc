package keyset

import (
	"github.com/spf13/cobra"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset/address"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset/balance"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset/new"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset/topup"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset/utxos"
)

var Cmd = &cobra.Command{
	Use:   "keyset",
	Short: "Function set for the keyset",
}

func init() {
	Cmd.AddCommand(address.Cmd)
	Cmd.AddCommand(balance.Cmd)
	Cmd.AddCommand(new.Cmd)
	Cmd.AddCommand(topup.Cmd)
	Cmd.AddCommand(utxos.Cmd)
}
