package app

import (
	"github.com/spf13/cobra"
)

var walletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "function set for the wallet",
}

func init() {
	rootCmd.AddCommand(walletCmd)

}
