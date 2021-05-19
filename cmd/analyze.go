package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"time"

	"github.com/MShoaei/techan"
	"github.com/MShoaei/trader/internal"
	"github.com/adshao/go-binance/v2"
	"github.com/sdcoffey/big"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// newAnalyzeCommand represents the analyze command
func newAnalyzeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "analyze strategy using either crypto currency market data",
		Args:  cobra.NoArgs,
	}
	return cmd
}

func newCryptoCommand() *cobra.Command {
	var (
		input      string
		fetch      bool
		strategy   int
		logFile    string
		symbol     string
		risk       float64
		commission float64
		leverage   int
		count      int
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
			candleC, err := cryptoCandleGenerator(file, count)
			if err != nil {
				return err
			}

			var record *techan.TradingRecord
			var series *techan.TimeSeries
			switch strategy {
			case 0:
				series, record = RunDynamicStrategy(internal.CreateBollingerStochStrategy, candleC, symbol, risk, leverage)
			case 1:
				series, record = RunDynamicStrategy(internal.CreateMACDStrategy, candleC, symbol, risk, leverage)
			case 2:
				series, record = RunDynamicStrategy(internal.CreateEMAStrategy, candleC, symbol, risk, leverage)
			case 3:
				series, record = RunDynamicStrategy(internal.CreateIchimokuStrategy, candleC, symbol, risk, leverage)
			default:
				return fmt.Errorf("invalid strategy")
			}

			totalProfit := techan.TotalProfitAnalysis{}.Analyze(record)
			commissionValue := internal.CommissionAnalysis{Commission: commission}.Analyze(record)
			openPL := internal.OpenPLAnalysis{LastCandle: series.LastCandle()}.Analyze(record)
			tradeCount := techan.NumTradesAnalysis{}.Analyze(record)
			profitableTradeCount := internal.ProfitableTradesAnalysis{}.Analyze(record)
			log.Infof("Total profit: %f, Commission: %f, PNL: %f",
				totalProfit,
				commissionValue,
				openPL,
			)
			log.Infof("Total trades: %d, Profitable trades: %d, Win rate: %f%%",
				int(tradeCount),
				int(profitableTradeCount),
				(profitableTradeCount/tradeCount)*100,
			)
			log.Infof("Win streak: %d, Lose streak: %d", int(internal.WinStreakAnalysis{}.Analyze(record)), int(internal.LoseStreakAnalysis{}.Analyze(record)))
			log.Infof("Max win: %f, Max loss: %f", internal.MaxWinAnalysis{}.Analyze(record), internal.MaxLossAnalysis{}.Analyze(record))
			log.Infof("Average win: %f, Average loss: %f", internal.AverageWinAnalysis{}.Analyze(record), internal.AverageLossAnalysis{}.Analyze(record))

			internal.LogTradesAnalysis{
				Writer: analysisFile,
			}.Analyze(record)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVarP(&input, "input", "i", "", "path to json file to read data from")
	f.BoolVarP(&fetch, "fetch", "f", false, "if data should be downloaded")
	f.IntVar(&strategy, "strategy", 0, "the strategy to use for analysis")
	_ = cmd.MarkFlagRequired("strategy")
	f.StringVarP(&logFile, "output", "o", "-", "path to file to write analysis data use '-' if you want to print to stdout")
	f.StringVarP(&symbol, "symbol", "s", "", "symbol of the test")
	_ = cmd.MarkFlagRequired("symbol")
	f.Float64VarP(&risk, "risk", "r", 25.0, "total value of the position in USD including leverage. e.g. if the risk is 100$ and leverage is 25X the position would be 4$")
	_ = cmd.MarkFlagRequired("risk")
	f.Float64VarP(&commission, "commission", "c", 0.04, "commission per trade in percent")
	f.IntVarP(&leverage, "leverage", "l", 1, "account leverage")
	f.IntVar(&count, "count", 0, "use the latest 'count' candles. 0 means all")
	return cmd
}

// RunDynamicStrategy runs the analysis using a strategy with dynamic exit rules.
// A exit rule with fixed stop loss and/or take profit price is not a dynamic strategy.
func RunDynamicStrategy(f internal.DynamicStrategyFunc, candleC chan *techan.Candle, symbol string, risk float64, leverage int) (*techan.TimeSeries, *techan.TradingRecord) {
	series := techan.NewTimeSeries()
	record := techan.NewTradingRecord()
	long, _ := f(series)
	index := 0
	for candle := range candleC {
		series.AddCandle(candle)

		if long.ShouldEnter(series.LastIndex(), record) {
			log.Debugf("entering long at price: %f", candle.ClosePrice.Float())
			log.Debugln(index, candle)
			record.Operate(techan.Order{
				Side:          techan.BUY,
				Security:      symbol,
				Price:         candle.ClosePrice,
				Amount:        CalculateAmount(big.NewDecimal(risk), candle.ClosePrice, big.NewFromInt(leverage)),
				ExecutionTime: candle.Period.Start,
			})
		} else if record.CurrentPosition().IsLong() && long.ShouldExit(series.LastIndex(), record) {
			log.Debugf("exiting at price: %f", candle.ClosePrice.Float())
			log.Debugln(index, candle)
			record.Operate(techan.Order{
				Side:          techan.SELL,
				Security:      symbol,
				Price:         candle.ClosePrice,
				Amount:        record.CurrentPosition().EntranceOrder().Amount,
				ExecutionTime: candle.Period.Start,
			})
		}
		index++
	}
	return series, record
}

func CalculateAmount(total big.Decimal, price big.Decimal, leverage big.Decimal) big.Decimal {
	amount := total.Div(price.Div(leverage)).Float()
	return big.NewDecimal(math.Floor(amount*1000) / 1000)
}

func cryptoCandleGenerator(input io.Reader, count int) (candleC chan *techan.Candle, err error) {
	candleC = make(chan *techan.Candle)
	b, err := ioutil.ReadAll(input)
	if err != nil {
		return nil, err
	}
	data := make([]*binance.Kline, 0)
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	data = data[len(data)-count:]
	go func() {
		defer close(candleC)

		c := 0
		for _, candle := range data {
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
			c++
		}
	}()
	return candleC, nil
}

func init() {
	ac := newAnalyzeCommand()
	rootCmd.AddCommand(ac)
	ac.AddCommand(newCryptoCommand())
}
