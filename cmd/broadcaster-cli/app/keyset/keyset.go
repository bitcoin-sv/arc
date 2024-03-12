package keyset

import (
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset/address"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset/balance"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset/dist"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset/new"
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset/topup"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "keyset",
	Short: "Function set for the keyset",
}

func init() {
	Cmd.AddCommand(address.Cmd)
	Cmd.AddCommand(balance.Cmd)
	Cmd.AddCommand(dist.Cmd)
	Cmd.AddCommand(new.Cmd)
	Cmd.AddCommand(topup.Cmd)
}
