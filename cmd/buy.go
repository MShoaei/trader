package cmd

import (
	"github.com/spf13/cobra"
)

// buyCmd represents the buy command
func newBuyCommand() *cobra.Command {

	var (
		symbol string
	)

	cmd := &cobra.Command{
		Use:   "buy",
		Short: "A brief description of your command",
		Long: `A longer description that spans multiple lines and likely contains examples
	and usage of using your command. For example:
	
	Cobra is a CLI library for Go that empowers applications.
	This application is a tool to generate the needed files
	to quickly create a Cobra application.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			//order, err := client.NewCreateOrderService().Symbol(symbol).
			//	Side(binance.SideTypeSell).Type(binance.OrderTypeLimit).
			//	TimeInForce(binance.TimeInForceTypeGTC).Quantity("0.01").
			//	Price("1720.00").Do(context.Background())
			//if err != nil {
			//	return err
			//}
			//res, err := client.NewGetOrderService().OrderID(order.OrderID).Symbol(symbol).Do(context.Background())
			//if err != nil {
			//	return err
			//}
			//for res.Status != binance.OrderStatusTypeFilled {
			//	res, err = client.NewGetOrderService().OrderID(order.OrderID).Symbol(symbol).Do(context.Background())
			//	if err != nil {
			//		return err
			//	}
			//	time.Sleep(5 * time.Second)
			//}
			return nil
		},
	}

	f := cmd.Flags()
	f.SortFlags = false
	f.StringVarP(&symbol, "symbol", "s", "", "the symbol to query e.g. ETHUSDT")
	_ = cmd.MarkFlagRequired("symbol")

	return cmd
}

func init() {
	rootCmd.AddCommand(newBuyCommand())
}
