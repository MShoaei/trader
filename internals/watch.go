package internals

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/MShoaei/techan"
	"github.com/adshao/go-binance/v2"
	"github.com/sdcoffey/big"
	log "github.com/sirupsen/logrus"
)

type ID string

func NewID(symbol string, interval string) ID {
	return ID(symbol + "-" + interval)
}

func (id ID) GetSymbol() string {
	return strings.Split(string(id), "-")[0]
}

func (id ID) GetInterval() string {
	return strings.Split(string(id), "-")[1]
}

var watchdogs = make(map[ID]*Watchdog, 20)

func createTimeSeries(klines []*binance.Kline) (series *techan.TimeSeries) {
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

func getKlines(symbol, interval string, limit int) (*techan.TimeSeries, error) {
	klines, err := binance.NewClient("", "").NewKlinesService().
		Symbol(symbol).
		Interval(interval).
		Limit(limit).
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	return createTimeSeries(klines), nil
}

func calculateAmount(total big.Decimal, price big.Decimal, leverage big.Decimal) big.Decimal {
	amount := total.Div(price.Div(leverage)).Float()
	return big.NewDecimal(math.Floor(amount*1000) / 1000)
}

type Watchdog struct {
	Client     *binance.Client `json:"-"`
	Symbol     string
	Interval   string
	Risk       float64
	Commission float64
	Leverage   int
	Demo       bool

	series  *techan.TimeSeries
	records *techan.TradingRecord

	StopC       chan struct{}
	InterruptCh chan os.Signal
}

func (w *Watchdog) Watch() error {
	record := techan.NewTradingRecord()
	w.records = record

	series, err := getKlines(w.Symbol, w.Interval, 1000)
	if err != nil {
		return err
	}
	w.series = series

	long, _ := CreateEMAStrategy(series)

	newCandle := series.LastCandle()

	index := 0
	wsKlineHandler := func(event *binance.WsKlineEvent) {
		log.Debugln(event)
		if !event.Kline.IsFinal {
			return
		}
		newCandle.OpenPrice = big.NewFromString(event.Kline.Open)
		newCandle.ClosePrice = big.NewFromString(event.Kline.Close)
		newCandle.MaxPrice = big.NewFromString(event.Kline.High)
		newCandle.MinPrice = big.NewFromString(event.Kline.Low)
		newCandle.Volume = big.NewFromString(event.Kline.Volume)
		newCandle.TradeCount = uint(event.Kline.TradeNum)
		if long.ShouldEnter(series.LastIndex(), record) {
			log.Infof("entering at price: %s", newCandle.ClosePrice.FormattedString(2))

			log.Debugln(index, newCandle)
			quantity := calculateAmount(big.NewDecimal(w.Risk), newCandle.ClosePrice, big.NewFromInt(w.Leverage))
			record.Operate(techan.Order{
				Side:          techan.BUY,
				Security:      w.Symbol,
				Price:         newCandle.ClosePrice,
				Amount:        quantity,
				ExecutionTime: time.Now(),
			})
			if !w.Demo {
				resp, err := w.Client.NewCreateOrderService().
					Symbol(w.Symbol).
					Side(binance.SideTypeBuy).
					Type(binance.OrderTypeLimit).
					Quantity(quantity.String()).
					Price(newCandle.ClosePrice.String()).Do(context.Background())
				if err != nil {
					log.Fatalf("sell failed: %v", err)
				}
				log.Info(resp)
			}
		} else if long.ShouldExit(series.LastIndex(), record) {
			log.Infof("exiting at price: %s", newCandle.ClosePrice.FormattedString(2))

			log.Debugln(index, newCandle)
			quantity := calculateAmount(big.NewDecimal(w.Risk), newCandle.ClosePrice, big.NewFromInt(w.Leverage))
			record.Operate(techan.Order{
				Side:          techan.SELL,
				Security:      w.Symbol,
				Price:         newCandle.ClosePrice,
				Amount:        record.CurrentPosition().EntranceOrder().Amount.Sub(record.CurrentPosition().EntranceOrder().Amount.Mul(big.NewDecimal(0.0001))),
				ExecutionTime: time.Now(),
			})
			if !w.Demo {
				resp, err := w.Client.NewCreateOrderService().
					Symbol(w.Symbol).
					Side(binance.SideTypeSell).
					Type(binance.OrderTypeLimit).
					Quantity(quantity.String()).
					Price(newCandle.ClosePrice.String()).Do(context.Background())
				if err != nil {
					log.Fatalf("sell failed: %v", err)
				}
				log.Info(resp)
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
	doneC, stopC, err := binance.WsKlineServe(w.Symbol, w.Interval, wsKlineHandler, errHandler)
	if err != nil {
		return err
	}
	w.StopC = stopC

	id := NewID(w.Symbol, w.Interval)
	watchdogs[id] = w

	select {
	case <-doneC:
		delete(watchdogs, id)
	case <-w.InterruptCh:
		w.StopC <- struct{}{}
		delete(watchdogs, id)
	}
	return nil
}

type Report struct {
	TotalProfit          float64 `json:"totalProfit"`
	CommissionValue      float64 `json:"commissionValue"`
	OpenProfit           float64 `json:"openProfit"`
	TradeCount           float64 `json:"tradeCount"`
	ProfitableTradeCount float64 `json:"profitableTradeCount"`
}

func (w *Watchdog) Report() Report {
	return Report{
		TotalProfit:          techan.TotalProfitAnalysis{}.Analyze(w.records),
		CommissionValue:      CommissionAnalysis{w.Commission}.Analyze(w.records),
		OpenProfit:           OpenPLAnalysis{w.series.LastCandle()}.Analyze(w.records),
		TradeCount:           techan.NumTradesAnalysis{}.Analyze(w.records),
		ProfitableTradeCount: ProfitableTradesAnalysis{}.Analyze(w.records),
	}
}

func GetWatchdog(symbol string, interval string) (*Watchdog, bool) {
	id := NewID(symbol, interval)
	w, ok := watchdogs[id]
	return w, ok
}

func DeleteWatchdog(symbol string, interval string) (bool, error) {
	id := NewID(symbol, interval)
	w, ok := watchdogs[id]
	if !ok {
		return false, nil
	}
	t := time.NewTimer(2 * time.Second)
	select {
	case w.StopC <- struct{}{}:
		break
	case <-t.C:
		if !t.Stop() {
			<-t.C
		}
		return false, fmt.Errorf("failed to stop the watchdog")
	}
	delete(watchdogs, id)
	return true, nil
}

func (w Watchdog) MarshalJSON() ([]byte, error) {
	aux := struct {
		Symbol     string
		Interval   string
		Risk       float64
		Commission float64
		Leverage   int
		Demo       bool
		LastPrice  float64
		Position   *struct {
			Open float64
			TP   float64 `json:"tp"`
			SL   float64 `json:"sl"`
		} `json:",omitempty"`
	}{
		Symbol:     w.Symbol,
		Interval:   w.Interval,
		Risk:       w.Risk,
		Commission: w.Commission,
		Leverage:   w.Leverage,
		Demo:       w.Demo,
		LastPrice:  w.series.LastCandle().ClosePrice.Float(),
	}
	if w.records.CurrentPosition().IsOpen() {
		aux.Position = &struct {
			Open float64
			TP   float64 `json:"tp"`
			SL   float64 `json:"sl"`
		}{
			Open: w.records.CurrentPosition().EntranceOrder().Price.Float(),
			// TP:   w.records.CurrentPosition().GetTakeProfit(),
			TP: 0,
			// SL:   w.records.CurrentPosition().GetStopLoss(),
			SL: 0,
		}
	}
	return json.Marshal(&aux)
}