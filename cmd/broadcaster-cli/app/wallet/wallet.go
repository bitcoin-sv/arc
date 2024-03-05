package wallet

import (
	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/app/wallet/address"
	"github.com/spf13/cobra"
)

var WalletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "function set for the wallet",
}

func init() {
	WalletCmd.AddCommand(address.AddrCmd)
}
