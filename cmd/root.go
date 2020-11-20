package cmd

import (
	"fmt"
	"github.com/adshao/go-binance"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	client *binance.Client
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "trader",
	Short: "trader is a bot to automate crypto trading on binance.com",
	Long:  `trader is a bot to automate crypto trading on binance.com`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetConfigType("yaml")
	viper.SetConfigType("yml")
	viper.SetConfigName("secret")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("failed to read API keys: %v", err)
	}
	if viper.GetString("key") == "" || viper.GetString("secret") == "" {
		log.Fatalln("API key or secret is empty")
	}
	client = binance.NewClient(viper.GetString("key"), viper.GetString("secret"))
}
