package cmd

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"

	binance "github.com/adshao/go-binance/v2"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	testNet     bool
	debug       bool
	client      *binance.Client
	proxy       string
	interruptCh = make(chan os.Signal, 1)
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
	log.SetLevel(log.DebugLevel)
	rootCmd.PersistentFlags().BoolVar(&testNet, "test", false, "set to use binance test network")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "set to enable debug output")
	rootCmd.PersistentFlags().StringVar(&proxy, "proxy", "", "set to enable proxy")

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
	var (
		key    string
		secret string
	)
	if testNet {
		key = viper.GetString("test.key")
		secret = viper.GetString("test.secret")
		if key == "" || secret == "" {
			log.Fatalln("test network API key or secret is empty")
		}
		client = binance.NewClient(key, secret)
	} else {
		key = viper.GetString("main.key")
		secret = viper.GetString("main.secret")
		if key == "" || secret == "" {
			log.Fatalln("main network API key or secret is empty")
		}
		client = binance.NewClient(key, secret)
	}
	binance.UseTestnet = testNet
	client.Debug = debug

	if proxy != "" {
		proxyURL, _ := url.Parse(proxy)
		client.HTTPClient.Transport = &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		websocket.DefaultDialer.Proxy = http.ProxyURL(proxyURL)
	}
	signal.Notify(interruptCh, os.Interrupt)
}
