package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2"
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
			elemCount := 0
			klines := make([]*binance.Kline, limit)

			fmt.Println(time.Now().Add(-30 * time.Minute).Round(30 * time.Minute))
			newKlines, err := client.NewKlinesService().
				Symbol(symbol).
				EndTime(int64(time.Now().Add(-30*time.Minute).Round(30*time.Minute).Unix()*1e3 - 1)).
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
