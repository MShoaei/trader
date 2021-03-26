package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	binance "github.com/adshao/go-binance/v2"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// newFetchCommand create a new fetch cmd
func newFetchCommand() *cobra.Command {
	var (
		symbol   string
		interval string
		limit    int
		output   string
	)
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "fetch and store data. it re-writes existing data",
		Long:  `fetch and store data`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if output == "" {
				output = path.Join("data", strings.ToLower(symbol)+".json")
			}
			data, err := os.Create(output)
			if err != nil {
				return err
			}
			klines, err := client.NewKlinesService().Symbol(symbol).Interval(interval).Limit(limit).Do(context.Background())
			if err != nil {
				return err
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

func fetchKlineData(symbol, interval string, limit int, file string) (klines []*binance.Kline, err error) {
	if exists, _ := afero.Exists(afero.NewOsFs(), file); !exists {
		log.Println("no history found. fetching data.")
		klines, err = client.NewKlinesService().Symbol(symbol).Interval(interval).Limit(limit).Do(context.Background())
		if err != nil {
			return nil, err
		}
	}
	f, _ := os.Open(file)
	dec := json.NewDecoder(f)
	_, err = dec.Token()
	if err != nil {
		return nil, err
	}
	for dec.More() {
		var candle binance.Kline
		err := dec.Decode(&candle)
		if err != nil {
			return nil, err
		}
		klines = append(klines, &candle)
	}
	if klines[len(klines)-1].CloseTime < time.Now().Unix()*1e3 {
		fmt.Println("Do something")
	}
	return klines, err
}

func init() {
	rootCmd.AddCommand(newFetchCommand())
}
