package app

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "broadcaster",
	Short: "cli tool to broadcast transactions to ARC",
}

func init() {

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().Bool("testnet", false, "Use testnet")
	viper.BindPFlag("testnet", rootCmd.PersistentFlags().Lookup("testnet"))

	rootCmd.PersistentFlags().String("authorization", "", "Authorization header to use for the http api client")
	viper.BindPFlag("authorization", rootCmd.PersistentFlags().Lookup("authorization"))

	rootCmd.PersistentFlags().String("callback", "", "URL which will be called with ARC callbacks")
	viper.BindPFlag("callback", rootCmd.PersistentFlags().Lookup("callback"))

	rootCmd.PersistentFlags().String("keyfile", "", "private key from file (arc.key) to use for funding transactions")
	viper.BindPFlag("keyFile", rootCmd.PersistentFlags().Lookup("keyfile"))

}

func Execute() error {
	return rootCmd.Execute()
}

func initConfig() {

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("../../")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("failed to read config file: %v\n", err)
	}
}
