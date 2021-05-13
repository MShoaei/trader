package cmd

import (
	"os"

	"github.com/MShoaei/trader/internals"
	"github.com/adshao/go-binance/v2"
	"github.com/spf13/cobra"
)

type Order struct {
	ID       string
	Side     binance.SideType
	Quantity string
	Price    string
}

func NewWatchCommand() *cobra.Command {
	var (
		symbol     string
		interval   string
		limit      int
		risk       float64
		commission float64
		leverage   int
		demo       bool
	)

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "watch watches the market and places order when the conditions are true",
		RunE: func(cmd *cobra.Command, args []string) error {
			interruptCh := make(chan os.Signal, 1)
			w := internals.Watchdog{
				Client:     client,
				Symbol:     symbol,
				Interval:   interval,
				Risk:       risk,
				Leverage:   leverage,
				Commission: commission,
				Demo:       demo,

				InterruptCh: interruptCh,
			}
			return w.Watch()
		},
	}
	f := cmd.Flags()
	f.SortFlags = false
	f.StringVarP(&symbol, "symbol", "s", "", "the symbol to query e.g. ETHUSDT")
	_ = cmd.MarkFlagRequired("symbol")
	f.Float64VarP(&risk, "risk", "r", 15.0, "total value of the position in USD including leverage. e.g. if the risk is 100$ and leverage is 25X the position would be 4$")
	_ = cmd.MarkFlagRequired("risk")
	f.StringVarP(&interval, "interval", "i", "", "the lower interval to query e.g. 1m")
	_ = cmd.MarkFlagRequired("interval")
	f.IntVar(&limit, "limit", 250, "number of candles to query")
	f.Float64VarP(&commission, "commission", "c", 0.1, "commission per trade in percent")
	f.IntVarP(&leverage, "leverage", "l", 1, "account leverage")
	f.BoolVar(&demo, "demo", false, "set to false to place real orders")

	return cmd
}

// watchCmd represents the watch command

func init() {
	rootCmd.AddCommand(NewWatchCommand())
}
