package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// newFetchCommand create a new fetch cmd
func newFetchCommand() *cobra.Command {
	var (
		symbol   string
		interval string
		limit    int
		output   string
	)
	var client *binance.Client
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "fetch and store data. it re-writes existing data",
		Long:  `fetch and store data`,
		PreRunE: func(cmd *cobra.Command, args []string) error {

			key := viper.GetString("main.key")
			secret := viper.GetString("main.secret")
			if key == "" || secret == "" {
				log.Fatalln("main network API key or secret is empty")
			}
			client = binance.NewClient(key, secret)
			client.Debug = debug

			if proxy != "" {
				proxyURL, _ := url.Parse(proxy)
				client.HTTPClient.Transport = &http.Transport{
					Proxy:           http.ProxyURL(proxyURL),
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				}
				websocket.DefaultDialer.Proxy = http.ProxyURL(proxyURL)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if output == "" {
				output = path.Join("data", strings.ToLower(symbol)+".json")
			}
			data, err := os.Create(output)
			if err != nil {
				return err
			}
			elemCount := 0
			klines := make([]*binance.Kline, limit)

			fmt.Println(time.Now().Add(-30 * time.Minute).Round(30 * time.Minute))
			newKlines, err := client.NewKlinesService().
				Symbol(symbol).
				EndTime(time.Now().Add(-30*time.Minute).Round(30*time.Minute).Unix()*1e3 - 1).
				Interval(interval).
				Limit(limit).
				Do(context.Background())
			if err != nil {
				return err
			}

			for elemCount != len(klines) {
				start := limit - 1000 - elemCount
				for i := 0; i < len(newKlines); i++ {
					klines[start+i] = newKlines[i]
					elemCount++
				}
				newKlines, err = client.NewKlinesService().
					Symbol(symbol).
					EndTime(newKlines[0].OpenTime).
					Interval(interval).
					Limit(limit - elemCount).
					Do(context.Background())
				if err != nil {
					return err
				}
			}
			if err := json.NewEncoder(data).Encode(klines); err != nil {
				return err
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.SortFlags = false
	f.StringVarP(&symbol, "symbol", "s", "", "the symbol to query e.g. ETHUSDT")
	_ = cmd.MarkFlagRequired("symbol")
	f.StringVarP(&interval, "interval", "i", "", "the interval to query e.g. 30m")
	_ = cmd.MarkFlagRequired("interval")
	f.IntVarP(&limit, "limit", "l", 25, "number of candles to query")
	f.StringVarP(&output, "output", "o", "", "file to store the data")

	return cmd
}

func init() {
	rootCmd.AddCommand(newFetchCommand())
}
