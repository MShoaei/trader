package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/MShoaei/techan"
	"github.com/adshao/go-binance/v2"
	"github.com/sdcoffey/big"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// testCmd represents the test command
func newAnalyzeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "analyze strategy using either crypto currency market data or forex data",
		Args:  cobra.NoArgs,
	}
	return cmd
}

func newCryptoCommand() *cobra.Command {
	var (
		input   string
		fetch   bool
		logFile string
		symbol  string
		amount  float64
	)
	var analysisFile io.Writer
	cmd := &cobra.Command{
		Use:   "crypto",
		Short: "analyze crypto data using the strategy",
		Long:  "analyze crypto data using the strategy",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			if logFile == "-" {
				log.SetOutput(os.Stdout)
				return nil
			}

			analysisFile, err = os.Create(logFile)
			if err != nil {
				return err
			}
			log.SetOutput(analysisFile)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := os.Open(input)
			if err != nil {
				return err
			}
			candleC, err := cryptoCandleGenerator(file)

			series := techan.NewTimeSeries()
			record := techan.NewTradingRecord()

			closePrice := techan.NewClosePriceIndicator(series)
			long, short, atr := createEMAStochATRStrategy(series)

			var (
				stopLoss   techan.Rule = FalseRule{}
				takeProfit techan.Rule = FalseRule{}
			)

			index := 0

			for candleLow := range candleC {
				series.AddCandle(candleLow)
				if long.ShouldEnter(series.LastIndex(), record) && short.ShouldEnter(series.LastIndex(), record) {
					log.Panicln("this should not happen")
					continue
				}

				if long.ShouldEnter(series.LastIndex(), record) {
					log.Infof("entering long at price: %f", candleLow.ClosePrice.Float())
					log.Debugln(index, candleLow)
					record.Operate(techan.Order{
						Side:          techan.BUY,
						Security:      symbol,
						Price:         candleLow.ClosePrice,
						Amount:        big.NewDecimal(amount),
						ExecutionTime: candleLow.Period.Start,
					})
					stopLoss = techan.UnderIndicatorRule{
						First:  closePrice,
						Second: techan.NewConstantIndicator(candleLow.ClosePrice.Sub(atr.Calculate(series.LastIndex()).Mul(big.NewDecimal(3.0))).Float()),
					}
					takeProfit = techan.OverIndicatorRule{
						First:  closePrice,
						Second: techan.NewConstantIndicator(candleLow.ClosePrice.Add(atr.Calculate(series.LastIndex()).Mul(big.NewDecimal(2.0))).Float()),
					}
				} else if short.ShouldEnter(series.LastIndex(), record) {
					log.Infof("entering short at price: %f", candleLow.ClosePrice.Float())
					log.Debugln(index, candleLow)
					record.Operate(techan.Order{
						Side:          techan.SELL,
						Security:      symbol,
						Price:         candleLow.ClosePrice,
						Amount:        big.NewDecimal(amount),
						ExecutionTime: candleLow.Period.Start,
					})
					stopLoss = techan.OverIndicatorRule{
						First:  closePrice,
						Second: techan.NewConstantIndicator(candleLow.ClosePrice.Add(atr.Calculate(series.LastIndex()).Mul(big.NewDecimal(3.0))).Float()),
					}
					takeProfit = techan.UnderIndicatorRule{
						First:  closePrice,
						Second: techan.NewConstantIndicator(candleLow.ClosePrice.Sub(atr.Calculate(series.LastIndex()).Mul(big.NewDecimal(2.0))).Float()),
					}
				} else if takeProfit.IsSatisfied(series.LastIndex(), record) || stopLoss.IsSatisfied(series.LastIndex(), record) {
					var side techan.OrderSide
					if record.CurrentPosition().IsShort() {
						side = techan.BUY
					} else {
						side = techan.SELL
					}
					log.Infof("exiting at price: %f", candleLow.ClosePrice.Float())
					log.Debugln(index, candleLow)
					record.Operate(techan.Order{
						Side:          side,
						Security:      symbol,
						Price:         candleLow.ClosePrice,
						Amount:        record.CurrentPosition().EntranceOrder().Amount,
						ExecutionTime: candleLow.Period.Start,
					})
					stopLoss = FalseRule{}
					takeProfit = FalseRule{}
				}
				index++
			}

			log.Infof("Total profit: %f, Total trades: %f, Profitable trades: %f",
				techan.TotalProfitAnalysis{}.Analyze(record),
				techan.NumTradesAnalysis{}.Analyze(record),
				techan.ProfitableTradesAnalysis{}.Analyze(record))
			LogTradesAnalysis{
				Writer: analysisFile,
			}.Analyze(record)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVarP(&input, "input", "i", "", "path to json file to read data from")

	f.BoolVarP(&fetch, "fetch", "f", false, "if data should be downloaded")
	f.StringVarP(&logFile, "output", "o", "-", "path to file to write analysis data use '-' if you want to print to stdout")
	f.StringVarP(&symbol, "symbol", "s", "", "symbol of the test")
	_ = cmd.MarkFlagRequired("symbol")
	f.Float64VarP(&amount, "amount", "a", 0.01, "amount of currency to use for trades")
	_ = cmd.MarkFlagRequired("amount")
	return cmd
}

func cryptoCandleGenerator(input io.Reader) (candleC chan *techan.Candle, err error) {
	candleC = make(chan *techan.Candle)
	dec := json.NewDecoder(input)
	_, err = dec.Token()
	if err != nil {
		return nil, err
	}
	go func() {
		defer close(candleC)

		for dec.More() {
			var candle binance.Kline
			err := dec.Decode(&candle)
			if err != nil {
				return
			}
			data := &techan.Candle{
				Period: techan.TimePeriod{
					Start: time.Unix(candle.OpenTime/1e3, (candle.OpenTime%1e3)*1e3),
					End:   time.Unix(candle.CloseTime/1e3, (candle.CloseTime%1e3)*1e3),
				},
				OpenPrice:  big.NewFromString(candle.Open),
				ClosePrice: big.NewFromString(candle.Close),
				MaxPrice:   big.NewFromString(candle.High),
				MinPrice:   big.NewFromString(candle.Low),
				Volume:     big.NewFromString(candle.Volume),
				TradeCount: uint(candle.TradeNum),
			}

			// ticker := time.NewTicker(2 * time.Second)
			// <-ticker.C
			candleC <- data
		}
	}()
	return candleC, nil
}

// LogTradesAnalysis is a wrapper around an io.Writer, which logs every trade executed to that writer
type LogTradesAnalysis struct {
	io.Writer
}

// Analyze logs trades to provided io.Writer
func (lta LogTradesAnalysis) Analyze(record *techan.TradingRecord) float64 {
	var profit big.Decimal
	logOrder := func(trade *techan.Position) {
		if trade.IsShort() {
			fmt.Fprintln(lta.Writer, fmt.Sprintf("%s - enter with sell %s (%s @ $%s)", trade.EntranceOrder().ExecutionTime.UTC().Format(time.RFC822), trade.EntranceOrder().Security, trade.EntranceOrder().Amount, trade.EntranceOrder().Price))
			fmt.Fprintln(lta.Writer, fmt.Sprintf("%s - exit with buy %s (%s @ $%s)", trade.ExitOrder().ExecutionTime.UTC().Format(time.RFC822), trade.ExitOrder().Security, trade.ExitOrder().Amount, trade.ExitOrder().Price))
			profit = trade.ExitValue().Sub(trade.CostBasis()).Neg()
		} else {
			fmt.Fprintln(lta.Writer, fmt.Sprintf("%s - enter with buy %s (%s @ $%s)", trade.EntranceOrder().ExecutionTime.UTC().Format(time.RFC822), trade.EntranceOrder().Security, trade.EntranceOrder().Amount, trade.EntranceOrder().Price))
			fmt.Fprintln(lta.Writer, fmt.Sprintf("%s - exit with sell %s (%s @ $%s)", trade.ExitOrder().ExecutionTime.UTC().Format(time.RFC822), trade.ExitOrder().Security, trade.ExitOrder().Amount, trade.ExitOrder().Price))
			profit = trade.ExitValue().Sub(trade.CostBasis())
		}
		fmt.Fprintln(lta.Writer, fmt.Sprintf("Profit: $%s", profit))
	}

	for _, trade := range record.Trades {
		if trade.IsClosed() {
			logOrder(trade)
		}
	}
	return 0.0
}

func init() {
	ac := newAnalyzeCommand()
	rootCmd.AddCommand(ac)
	ac.AddCommand(newCryptoCommand())
}
