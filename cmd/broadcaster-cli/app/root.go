package app

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "broadcaster",
	Short: "cli tool to broadcast transactions to ARC",
}

func init() {

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().Bool("testnet", false, "send transactions to testnet")
	viper.BindPFlag("testnet", rootCmd.Flags().Lookup("testnet"))

	rootCmd.PersistentFlags().String("authorization", "", "Authorization header to use for the http api client")
	viper.BindPFlag("authorization", rootCmd.Flags().Lookup("authorization"))

	rootCmd.PersistentFlags().String("callback", "", "URL which will be called with ARC callbacks")
	viper.BindPFlag("callback", rootCmd.Flags().Lookup("callback"))

	rootCmd.PersistentFlags().String("keyfile", "", "private key from file (arc.key) to use for funding transactions")
	viper.BindPFlag("keyFile", rootCmd.Flags().Lookup("keyfile"))

	//rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "Display more verbose output in console output. (default: false)")
	//viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	//
	//rootCmd.PersistentFlags().BoolVarP(&Debug, "debug", "d", false, "Display debugging output in the console. (default: false)")
	//viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
}

func Execute() error {
	return rootCmd.Execute()
}

func initConfig() {

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("../../")
	viper.ReadInConfig()
	//if cfgFile != "" {
	//	// Use config file from the flag.
	//	viper.SetConfigFile(cfgFile)
	//} else {
	//	// Find home directory.
	//	home, err := os.UserHomeDir()
	//	cobra.CheckErr(err)
	//
	//	// Search config in home directory with name ".cobra" (without extension).
	//	viper.AddConfigPath(home)
	//	viper.SetConfigType("yaml")
	//	viper.SetConfigName(".cobra")
	//}
	//
	//viper.AutomaticEnv()
	//
	//if err := viper.ReadInConfig(); err == nil {
	//	fmt.Println("Using config file:", viper.ConfigFileUsed())
	//}
}
