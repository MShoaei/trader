/*
Copyright © 2021 Mohammad Shoaei <Mohammad.Shoaei@outlook.com>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package cmd

import (
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/sdcoffey/big"
	"github.com/sdcoffey/techan"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// testCmd represents the test command
func newAnalyzeCommand() *cobra.Command {
	var (
		input   string
		logFile string
		symbol  string
		amount  float64
	)
	var analysisFile io.Writer
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "analyze strategy with the provided data",
		Long:  `analyze strategy with the provided data`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
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
			candleC, err := candleGenerator(file)
			if err != nil {
				return err
			}

			series := techan.NewTimeSeries()
			record := techan.NewTradingRecord()

			closeIndicator := techan.NewClosePriceIndicator(series)

			strategy := createSingleStrategy(closeIndicator)

			index := 0
			for candle := range candleC {
				series.AddCandle(candle)
				if strategy.ShouldEnter(series.LastIndex(), record) {
					log.Infof("entering at price: %s", candle.ClosePrice.FormattedString(2))
					log.Debugln(index, candle)
					record.Operate(techan.Order{
						Side:          techan.BUY,
						Security:      symbol,
						Price:         candle.ClosePrice,
						Amount:        big.NewDecimal(amount).Sub(big.NewDecimal(amount).Mul(big.NewDecimal(0.001))),
						ExecutionTime: candle.Period.Start,
					})
				} else if strategy.ShouldExit(series.LastIndex(), record) {
					log.Infof("exiting at price: %s", candle.ClosePrice.FormattedString(2))
					log.Debugln(index, candle)
					record.Operate(techan.Order{
						Side:          techan.SELL,
						Security:      symbol,
						Price:         candle.ClosePrice,
						Amount:        record.CurrentPosition().EntranceOrder().Amount.Sub(record.CurrentPosition().EntranceOrder().Amount.Mul(big.NewDecimal(0.0001))),
						ExecutionTime: candle.Period.Start,
					})
				}
				log.Infof("MACD: %f, MACD histogram: %f", strategy.macd.Calculate(series.LastIndex()).Float(), strategy.macdHist.Calculate(series.LastIndex()).Float())
				index++
			}

			techan.LogTradesAnalysis{
				Writer: analysisFile,
			}.Analyze(record)
			log.Warnf("Total profit: %f, Profitable trades: %f",
				techan.TotalProfitAnalysis{}.Analyze(record),
				techan.ProfitableTradesAnalysis{}.Analyze(record))
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVarP(&input, "input", "i", "", "path to json file to read data from")
	_ = cmd.MarkFlagRequired("input")
	f.StringVarP(&logFile, "output", "o", "", "path to file to write analysis data")
	_ = cmd.MarkFlagRequired("input")
	f.StringVarP(&symbol, "symbol", "s", "", "symbol of the test")
	_ = cmd.MarkFlagRequired("symbol")
	f.Float64VarP(&amount, "amount", "a", 0.01, "amount of currency to use for trades")
	_ = cmd.MarkFlagRequired("amount")

	return cmd
}

func candleGenerator(input io.Reader) (candleC chan *techan.Candle, err error) {
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

func init() {
	rootCmd.AddCommand(newAnalyzeCommand())
}
