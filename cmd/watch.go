package cmd

import (
	"context"
	"time"

	binance "github.com/adshao/go-binance/v2"
	"github.com/sdcoffey/big"
	"github.com/sdcoffey/techan"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newWatchCommand() *cobra.Command {
	var (
		symbol       string
		lowInterval  string
		highInterval string
		limit        int
	)
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "watch watches the market and places order when the conditions are true",
		RunE: func(cmd *cobra.Command, args []string) error {
			klinesLowTimeFrame, err := client.NewKlinesService().Symbol(symbol).Interval(lowInterval).Limit(limit).Do(context.Background())
			if err != nil {
				return err
			}

			klinesHighTimeFrame, err := client.NewKlinesService().Symbol(symbol).Interval(highInterval).Limit(limit).Do(context.Background())
			if err != nil {
				return err
			}

			lowTimeFrameSeries, highTimeFrameSeries := createTimeSeries(klinesLowTimeFrame, klinesHighTimeFrame)
			closeLowTimeFrameIndicator := techan.NewClosePriceIndicator(lowTimeFrameSeries)
			closeHighTimeFrameIndicator := techan.NewClosePriceIndicator(highTimeFrameSeries)
			buyClose := defaultBuyIndicators(closeLowTimeFrameIndicator)
			// buyLowTimeFrameRules, buyHighTimeFrameRules, sellLowTimeFrameRules, sellHighTimeFrameRules := createNewRules(klinesLowTimeFrame, klinesHighTimeFrame)
			buyLowTimeFrameRules, _, sellLowTimeFrameRules, _ := createNewRules(closeLowTimeFrameIndicator, closeHighTimeFrameIndicator, buyClose, nil)

			// rsi := techan.NewRelativeStrengthIndexIndicator(closeIndicator, 14)
			// stoch := techan.NewFastStochasticIndicator(series, 14)
			// macd := techan.NewMACDIndicator(closeIndicator, 12, 26)
			// diff := techan.NewMACDHistogramIndicator(macd, 9)
			// upper := techan.NewBollingerUpperBandIndicator(closeIndicator, 20, 2)
			// lower := techan.NewBollingerLowerBandIndicator(closeIndicator, 20, 2)

			// record trades on this object
			record := techan.NewTradingRecord()

			strategies := make([]techan.RuleStrategy, 0, 20)
			for i := 0; i < len(buyLowTimeFrameRules); i++ {
				strategies = append(strategies, techan.RuleStrategy{
					EntryRule:      buyLowTimeFrameRules[i],
					ExitRule:       sellLowTimeFrameRules[i],
					UnstablePeriod: 5,
				})
			}

			newCandle := lowTimeFrameSeries.LastCandle()

			wsKlineHandler := func(event *binance.WsKlineEvent) {

				if event.Kline.IsFinal {
					newCandle.OpenPrice = big.NewFromString(event.Kline.Open)
					newCandle.ClosePrice = big.NewFromString(event.Kline.Close)
					newCandle.MaxPrice = big.NewFromString(event.Kline.High)
					newCandle.MinPrice = big.NewFromString(event.Kline.Low)
					newCandle.Volume = big.NewFromString(event.Kline.Volume)
					newCandle.TradeCount = uint(event.Kline.TradeNum)
					b := countBuys(lowTimeFrameSeries.LastIndex(), record, strategies)
					s := countSells(lowTimeFrameSeries.LastIndex(), record, strategies)
					if b-s > 3 {
						log.Infof("entering at price: %s", newCandle.ClosePrice.FormattedString(2))
						record.Operate(techan.Order{
							Side:          techan.BUY,
							Security:      symbol,
							Price:         newCandle.ClosePrice,
							Amount:        big.ONE,
							ExecutionTime: time.Now(),
						})
					} else if b-s < -2 {
						log.Infof("exiting at price: %s", newCandle.ClosePrice.FormattedString(2))
						record.Operate(techan.Order{
							Side:          techan.SELL,
							Security:      symbol,
							Price:         newCandle.ClosePrice,
							Amount:        big.ONE,
							ExecutionTime: time.Now(),
						})
					}
					for i, indicator := range buyClose {
						log.Debugf("index %d: %s", i, indicator.Calculate(lowTimeFrameSeries.LastIndex()))
					}
					newCandle = techan.NewCandle(newCandle.Period.Advance(1))
					if success := lowTimeFrameSeries.AddCandle(newCandle); !success {
						log.Errorln("failed to add candle")
					}
					log.Debugln(event)
					return
				}

				log.Info(event.Kline.Close)
			}
			errHandler := func(err error) {
				log.Info(err)
			}
			doneC, _, err := binance.WsKlineServe(symbol, lowInterval, wsKlineHandler, errHandler)
			if err != nil {
				return err
			}
			ticker := time.NewTicker(30 * time.Minute)
			select {
			case <-doneC:
			case <-ticker.C:
				log.Infof("Total profit: %f", techan.TotalProfitAnalysis{}.Analyze(record))
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.SortFlags = false
	f.StringVarP(&symbol, "symbol", "s", "", "the symbol to query e.g. ETHUSDT")
	_ = cmd.MarkFlagRequired("symbol")
	f.StringVar(&lowInterval, "low", "", "the lower interval to query e.g. 1m")
	_ = cmd.MarkFlagRequired("low")
	f.StringVar(&highInterval, "high", "", "the higher interval to query e.g. 15m")
	_ = cmd.MarkFlagRequired("high")
	f.IntVar(&limit, "limit", 25, "number of candles to query")

	return cmd
}

func countBuys(index int, record *techan.TradingRecord, s []techan.RuleStrategy) (count int) {
	for i := 0; i < len(s); i++ {
		if s[i].ShouldEnter(index, record) {
			count++
		}
	}
	log.Debugf("Buy count: %d", count)
	return count
}

func countSells(index int, record *techan.TradingRecord, s []techan.RuleStrategy) (count int) {
	for i := 0; i < len(s); i++ {
		if s[i].ShouldExit(index, record) {
			count++
		}
	}
	log.Debugf("Sell count: %d", count)
	return count
}

func defaultBuyIndicators(base techan.Indicator) []techan.Indicator {
	return []techan.Indicator{
		techan.NewEMAIndicator(base, 10),
		techan.NewSimpleMovingAverage(base, 10),
		techan.NewEMAIndicator(base, 20),
		techan.NewSimpleMovingAverage(base, 20),
		techan.NewEMAIndicator(base, 30),
		techan.NewSimpleMovingAverage(base, 30),
		techan.NewEMAIndicator(base, 50),
		techan.NewSimpleMovingAverage(base, 50),
		techan.NewEMAIndicator(base, 100),
		techan.NewSimpleMovingAverage(base, 100),
		techan.NewEMAIndicator(base, 200),
		techan.NewSimpleMovingAverage(base, 200),
	}
}

func createNewRules(closeLowTimeFrameIndicator techan.Indicator, closeHighTimeFrameIndicator techan.Indicator, maLow []techan.Indicator, maHigh []techan.Indicator) ([]techan.Rule, []techan.Rule, []techan.Rule, []techan.Rule) {
	buyLowTimeFrameRules := []techan.Rule{
		techan.OverIndicatorRule{
			First:  maLow[0],
			Second: closeLowTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  maLow[1],
			Second: closeLowTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  maLow[2],
			Second: closeLowTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  maLow[3],
			Second: closeLowTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  maLow[4],
			Second: closeLowTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  maLow[5],
			Second: closeLowTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  maLow[6],
			Second: closeLowTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  maLow[7],
			Second: closeLowTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  maLow[8],
			Second: closeLowTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  maLow[9],
			Second: closeLowTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  maLow[10],
			Second: closeLowTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  maLow[11],
			Second: closeLowTimeFrameIndicator,
		},
	}
	buyHighTimeFrameRules := []techan.Rule{
		techan.OverIndicatorRule{
			First:  techan.NewEMAIndicator(closeHighTimeFrameIndicator, 10),
			Second: closeHighTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeHighTimeFrameIndicator, 10),
			Second: closeHighTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  techan.NewEMAIndicator(closeHighTimeFrameIndicator, 20),
			Second: closeHighTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeHighTimeFrameIndicator, 20),
			Second: closeHighTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  techan.NewEMAIndicator(closeHighTimeFrameIndicator, 30),
			Second: closeHighTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeHighTimeFrameIndicator, 30),
			Second: closeHighTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  techan.NewEMAIndicator(closeHighTimeFrameIndicator, 50),
			Second: closeHighTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeHighTimeFrameIndicator, 50),
			Second: closeHighTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  techan.NewEMAIndicator(closeHighTimeFrameIndicator, 100),
			Second: closeHighTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeHighTimeFrameIndicator, 100),
			Second: closeHighTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  techan.NewEMAIndicator(closeHighTimeFrameIndicator, 200),
			Second: closeHighTimeFrameIndicator,
		},
		techan.OverIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeHighTimeFrameIndicator, 200),
			Second: closeHighTimeFrameIndicator,
		},
	}

	sellLowTimeFrameRules := []techan.Rule{
		techan.UnderIndicatorRule{
			First:  techan.NewEMAIndicator(closeLowTimeFrameIndicator, 10),
			Second: closeLowTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeLowTimeFrameIndicator, 10),
			Second: closeLowTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewEMAIndicator(closeLowTimeFrameIndicator, 20),
			Second: closeLowTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeLowTimeFrameIndicator, 20),
			Second: closeLowTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewEMAIndicator(closeLowTimeFrameIndicator, 30),
			Second: closeLowTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeLowTimeFrameIndicator, 30),
			Second: closeLowTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewEMAIndicator(closeLowTimeFrameIndicator, 50),
			Second: closeLowTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeLowTimeFrameIndicator, 50),
			Second: closeLowTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewEMAIndicator(closeLowTimeFrameIndicator, 100),
			Second: closeLowTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeLowTimeFrameIndicator, 100),
			Second: closeLowTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewEMAIndicator(closeLowTimeFrameIndicator, 200),
			Second: closeLowTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeLowTimeFrameIndicator, 200),
			Second: closeLowTimeFrameIndicator,
		},
	}
	sellHighTimeFrameRules := []techan.Rule{
		techan.UnderIndicatorRule{
			First:  techan.NewEMAIndicator(closeHighTimeFrameIndicator, 10),
			Second: closeHighTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeHighTimeFrameIndicator, 10),
			Second: closeHighTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewEMAIndicator(closeHighTimeFrameIndicator, 20),
			Second: closeHighTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeHighTimeFrameIndicator, 20),
			Second: closeHighTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewEMAIndicator(closeHighTimeFrameIndicator, 30),
			Second: closeHighTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeHighTimeFrameIndicator, 30),
			Second: closeHighTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewEMAIndicator(closeHighTimeFrameIndicator, 50),
			Second: closeHighTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeHighTimeFrameIndicator, 50),
			Second: closeHighTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewEMAIndicator(closeHighTimeFrameIndicator, 100),
			Second: closeHighTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeHighTimeFrameIndicator, 100),
			Second: closeHighTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewEMAIndicator(closeHighTimeFrameIndicator, 200),
			Second: closeHighTimeFrameIndicator,
		},
		techan.UnderIndicatorRule{
			First:  techan.NewSimpleMovingAverage(closeHighTimeFrameIndicator, 200),
			Second: closeHighTimeFrameIndicator,
		},
	}

	return buyLowTimeFrameRules, buyHighTimeFrameRules, sellLowTimeFrameRules, sellHighTimeFrameRules
}

func createTimeSeries(klinesLowTimeFrame []*binance.Kline, klinesHighTimeFrame []*binance.Kline) (*techan.TimeSeries, *techan.TimeSeries) {
	lowTimeFrameSeries := techan.NewTimeSeries()
	highTimeFrameSeries := techan.NewTimeSeries()
	for i := 0; i < len(klinesLowTimeFrame); i++ {
		candle := techan.Candle{
			Period: techan.TimePeriod{
				Start: time.Unix(klinesLowTimeFrame[i].OpenTime/1e3, (klinesLowTimeFrame[i].OpenTime%1e3)*1e3),
				End:   time.Unix(klinesLowTimeFrame[i].CloseTime/1e3, (klinesLowTimeFrame[i].CloseTime%1e3)*1e3),
			},
			OpenPrice:  big.NewFromString(klinesLowTimeFrame[i].Open),
			ClosePrice: big.NewFromString(klinesLowTimeFrame[i].Close),
			MaxPrice:   big.NewFromString(klinesLowTimeFrame[i].High),
			MinPrice:   big.NewFromString(klinesLowTimeFrame[i].Low),
			Volume:     big.NewFromString(klinesLowTimeFrame[i].Volume),
			TradeCount: uint(klinesLowTimeFrame[i].TradeNum),
		}
		lowTimeFrameSeries.AddCandle(&candle)
	}
	for i := 0; i < len(klinesHighTimeFrame); i++ {
		candle := techan.Candle{
			Period: techan.TimePeriod{
				Start: time.Unix(klinesHighTimeFrame[i].OpenTime/1e3, (klinesHighTimeFrame[i].OpenTime%1e3)*1e3),
				End:   time.Unix(klinesHighTimeFrame[i].CloseTime/1e3, (klinesHighTimeFrame[i].CloseTime%1e3)*1e3),
			},
			OpenPrice:  big.NewFromString(klinesHighTimeFrame[i].Open),
			ClosePrice: big.NewFromString(klinesHighTimeFrame[i].Close),
			MaxPrice:   big.NewFromString(klinesHighTimeFrame[i].High),
			MinPrice:   big.NewFromString(klinesHighTimeFrame[i].Low),
			Volume:     big.NewFromString(klinesHighTimeFrame[i].Volume),
			TradeCount: uint(klinesHighTimeFrame[i].TradeNum),
		}
		highTimeFrameSeries.AddCandle(&candle)
	}

	return lowTimeFrameSeries, highTimeFrameSeries
}

// watchCmd represents the watch command

func init() {
	rootCmd.AddCommand(newWatchCommand())
}
