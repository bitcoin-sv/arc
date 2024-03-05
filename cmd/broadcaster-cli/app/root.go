package app

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"log"
)

var rootCmd = &cobra.Command{
	Use:   "broadcaster",
	Short: "cli tool to broadcast transactions to ARC",
}

func init() {
	var err error
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().Bool("testnet", false, "Use testnet")
	err = viper.BindPFlag("testnet", rootCmd.PersistentFlags().Lookup("testnet"))
	if err != nil {
		log.Fatal(err)
	}

	rootCmd.PersistentFlags().String("authorization", "", "Authorization header to use for the http api client")
	err = viper.BindPFlag("authorization", rootCmd.PersistentFlags().Lookup("authorization"))
	if err != nil {
		log.Fatal(err)
	}

	rootCmd.PersistentFlags().String("callback", "", "URL which will be called with ARC callbacks")
	err = viper.BindPFlag("callback", rootCmd.PersistentFlags().Lookup("callback"))
	if err != nil {
		log.Fatal(err)
	}

	rootCmd.PersistentFlags().String("keyfile", "", "private key from file (arc.key) to use for funding transactions")
	err = viper.BindPFlag("keyFile", rootCmd.PersistentFlags().Lookup("keyfile"))
	if err != nil {
		log.Fatal(err)
	}

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
