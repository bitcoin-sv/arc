package keyset

import (
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/keyset/address"
	"github.com/spf13/cobra"
)

var WalletCmd = &cobra.Command{
	Use:   "keyset",
	Short: "function set for the keyset",
}

func init() {
	WalletCmd.AddCommand(address.AddrCmd)
}
