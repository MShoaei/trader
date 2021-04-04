package cmd

import (
	"context"
	"github.com/adshao/go-binance/v2/futures"
	"os"
	"time"

	"github.com/MShoaei/techan"
	"github.com/sdcoffey/big"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type Order struct {
	ID       string
	Side     futures.PositionSideType
	Quantity string
	Price    string
}

func newWatchCommand() *cobra.Command {
	var (
		symbol       string
		amount       float64
		lowInterval  string
		highInterval string
		limit        int
		demo         bool
	)

	// record trades on this object
	record := techan.NewTradingRecord()
	var series *techan.TimeSeries
	//orderIDs := map[string]Order{}

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "watch watches the market and places order when the conditions are true",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			klines, err := client.NewKlinesService().
				Symbol(symbol).
				EndTime(time.Now().Add(15*time.Minute).Round(30*time.Minute).Unix()*1e3 - 1).
				Interval(lowInterval).
				Limit(limit).
				Do(context.Background())
			if err != nil {
				return err
			}
			series = createTimeSeries(klines)

			key, err := client.NewStartUserStreamService().Do(context.Background())
			if err != nil {
				return err
			}
			doneC, _, _ := futures.WsUserDataServe(key, func(event *futures.WsUserDataEvent) {
				if event.Event == futures.UserDataEventTypeOrderTradeUpdate && event.OrderTradeUpdate.Status == futures.OrderStatusTypeFilled {
					log.Infoln(event)
				}
			}, func(err error) {
				log.Errorln(err)
			})

			<-doneC

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			strategy := createIchimokuStrategy(series)

			closePrice := techan.NewClosePriceIndicator(series)
			conv := NewConversionLineIndicator(series, 9)
			base := NewBaseLineIndicator(series, 26)
			spanA := NewLeadingSpanAIndicator(conv.(conversionLineIndicator), base.(baseLineIndicator))
			spanB := NewLeadingSpanBIndicator(series, 52)
			laggingSpan := NewLaggingSpanIndicator(series)

			newCandle := series.LastCandle()

			index := 0
			wsKlineHandler := func(event *futures.WsKlineEvent) {
				if !event.Kline.IsFinal {
					return
				}
				newCandle.OpenPrice = big.NewFromString(event.Kline.Open)
				newCandle.ClosePrice = big.NewFromString(event.Kline.Close)
				newCandle.MaxPrice = big.NewFromString(event.Kline.High)
				newCandle.MinPrice = big.NewFromString(event.Kline.Low)
				newCandle.Volume = big.NewFromString(event.Kline.Volume)
				newCandle.TradeCount = uint(event.Kline.TradeNum)
				if strategy.ShouldEnter(series.LastIndex(), record) {
					log.Infof("entering at price: %s", newCandle.ClosePrice.FormattedString(2))

					log.Infof("close: %s", closePrice.Calculate(series.LastIndex()).String())
					log.Infof("conv: %s", conv.Calculate(series.LastIndex()).String())
					log.Infof("base: %s", base.Calculate(series.LastIndex()).String())
					log.Infof("spanA: %s", spanA.Calculate(series.LastIndex()).String())
					log.Infof("spanB: %s", spanB.Calculate(series.LastIndex()).String())
					log.Infof("laggingSpan: %s", laggingSpan.Calculate(series.LastIndex()).String())

					log.Debugln(index, newCandle)
					record.Operate(techan.Order{
						Side:          techan.BUY,
						Security:      symbol,
						Price:         newCandle.ClosePrice,
						Amount:        big.NewDecimal(amount).Sub(big.NewDecimal(amount).Mul(big.NewDecimal(0.001))),
						ExecutionTime: time.Now(),
					})
					if !demo {
						//order, err := client.NewCreateOrderService().Symbol(symbol).
						//	Side(binance.SideTypeBuy).Type(binance.OrderTypeLimit).
						//	TimeInForce(binance.TimeInForceTypeGTC).Quantity(strconv.FormatFloat(amount, 'f', 7, 64)).
						//	Price(newCandle.ClosePrice.FormattedString(7)).Do(context.Background())
						//if err != nil {
						//	log.Panic(err)
						//}
						//log.Traceln(order)
					}
				} else if strategy.ShouldExit(series.LastIndex(), record) {
					log.Infof("exiting at price: %s", newCandle.ClosePrice.FormattedString(2))

					log.Infof("close: %s", closePrice.Calculate(series.LastIndex()).String())
					log.Infof("conv: %s", conv.Calculate(series.LastIndex()).String())
					log.Infof("base: %s", base.Calculate(series.LastIndex()).String())
					log.Infof("spanA: %s", spanA.Calculate(series.LastIndex()).String())
					log.Infof("spanB: %s", spanB.Calculate(series.LastIndex()).String())
					log.Infof("laggingSpan: %s", laggingSpan.Calculate(series.LastIndex()).String())

					log.Debugln(index, newCandle)
					record.Operate(techan.Order{
						Side:          techan.SELL,
						Security:      symbol,
						Price:         newCandle.ClosePrice,
						Amount:        record.CurrentPosition().EntranceOrder().Amount.Sub(record.CurrentPosition().EntranceOrder().Amount.Mul(big.NewDecimal(0.0001))),
						ExecutionTime: time.Now(),
					})
					if !demo {
						//order, err := client.NewCreateOrderService().Symbol(symbol).
						//	Side(binance.SideTypeSell).Type(binance.OrderTypeLimit).
						//	TimeInForce(binance.TimeInForceTypeGTC).Quantity(strconv.FormatFloat(amount, 'f', 7, 64)).
						//	Price(newCandle.ClosePrice.FormattedString(7)).Do(context.Background())
						//if err != nil {
						//	log.Panic(err)
						//}
						//log.Traceln(order)
					}
				}
				newCandle = techan.NewCandle(newCandle.Period.Advance(1))
				if success := series.AddCandle(newCandle); !success {
					log.Fatalln("failed to add candle")
				}
				log.Debugln(event)
				index++
			}
			errHandler := func(err error) {
				log.Info(err)
			}
			doneC, _, err := futures.WsKlineServe(symbol, lowInterval, wsKlineHandler, errHandler)
			if err != nil {
				return err
			}

			select {
			case <-doneC:
			case <-interruptCh:
				recordLogFile, _ := os.Create(symbol + "-" + time.Now().Format("2006_01_02T15:04:05Z07:00") + ".log")
				techan.LogTradesAnalysis{Writer: recordLogFile}.Analyze(record)
				log.Infof("Total profit: %f", techan.TotalProfitAnalysis{}.Analyze(record))
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.SortFlags = false
	f.StringVarP(&symbol, "symbol", "s", "", "the symbol to query e.g. ETHUSDT")
	_ = cmd.MarkFlagRequired("symbol")
	f.Float64VarP(&amount, "amount", "a", 0.01, "amount of currency to use for trades")
	_ = cmd.MarkFlagRequired("amount")
	f.StringVar(&lowInterval, "low", "", "the lower interval to query e.g. 1m")
	_ = cmd.MarkFlagRequired("low")
	f.StringVar(&highInterval, "high", "", "the higher interval to query e.g. 15m")
	_ = cmd.MarkFlagRequired("high")
	f.IntVar(&limit, "limit", 25, "number of candles to query")
	f.BoolVar(&demo, "demo", false, "set to false to place real orders")

	return cmd
}

func createTimeSeries(klines []*futures.Kline) (series *techan.TimeSeries) {
	series = techan.NewTimeSeries()
	for i := 0; i < len(klines); i++ {
		candle := techan.Candle{
			Period: techan.TimePeriod{
				Start: time.Unix(klines[i].OpenTime/1e3, (klines[i].OpenTime%1e3)*1e3),
				End:   time.Unix(klines[i].CloseTime/1e3, (klines[i].CloseTime%1e3)*1e3),
			},
			OpenPrice:  big.NewFromString(klines[i].Open),
			ClosePrice: big.NewFromString(klines[i].Close),
			MaxPrice:   big.NewFromString(klines[i].High),
			MinPrice:   big.NewFromString(klines[i].Low),
			Volume:     big.NewFromString(klines[i].Volume),
			TradeCount: uint(klines[i].TradeNum),
		}
		series.AddCandle(&candle)
	}
	return series
}

// watchCmd represents the watch command

func init() {
	rootCmd.AddCommand(newWatchCommand())
}
