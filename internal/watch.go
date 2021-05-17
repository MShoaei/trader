package internal

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/MShoaei/techan"
	"github.com/adshao/go-binance/v2"
	"github.com/sdcoffey/big"
	log "github.com/sirupsen/logrus"
)

// var watchdogs = make(map[ID]*Watchdog, 20)

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

func calculateAmount(total big.Decimal, price big.Decimal, leverage big.Decimal, filter *binance.LotSizeFilter) big.Decimal {
	amount := total.Div(price.Div(leverage))
	str := amount.FormattedString(8)

	return big.NewFromString(str[:len(str)-(8-precision(filter.StepSize))])
}

func precision(number string) int {
	if number[0] == '1' {
		return 0
	}
	return strings.IndexByte(number, '1') - 1
}

type Watchdog struct {
	Symbol     string
	Interval   string
	Risk       float64
	Commission float64
	Leverage   int
	Demo       bool
	SymbolInfo binance.Symbol

	series  *techan.TimeSeries
	records *techan.TradingRecord

	StopC       chan struct{}
	InterruptCh chan os.Signal
}

func (w *Watchdog) Watch(client *binance.Client) (binance.WsKlineHandler, binance.ErrHandler, error) {
	record := techan.NewTradingRecord()
	w.records = record

	series, err := getKlines(w.Symbol, w.Interval, 1000)
	if err != nil {
		return nil, nil, err
	}
	w.series = series

	long, _ := CreateEMAStrategy(series)

	newCandle := series.LastCandle()

	wsKlineHandler := func(event *binance.WsKlineEvent) {
		newCandle.OpenPrice = big.NewFromString(event.Kline.Open)
		newCandle.ClosePrice = big.NewFromString(event.Kline.Close)
		newCandle.MaxPrice = big.NewFromString(event.Kline.High)
		newCandle.MinPrice = big.NewFromString(event.Kline.Low)
		newCandle.Volume = big.NewFromString(event.Kline.Volume)
		newCandle.TradeCount = uint(event.Kline.TradeNum)
		log.Debugln(event)
		if !event.Kline.IsFinal {
			return
		}
		if long.ShouldEnter(series.LastIndex(), record) {
			var (
				// resp *binance.CreateOrderResponse
				err error
			)
			quantity := calculateAmount(big.NewDecimal(w.Risk), newCandle.ClosePrice, big.NewFromInt(w.Leverage), w.SymbolInfo.LotSizeFilter())
			price := newCandle.ClosePrice.FormattedString(w.SymbolInfo.QuotePrecision)
			if !w.Demo {
				_, err = client.NewCreateOrderService().
					Symbol(w.Symbol).
					Side(binance.SideTypeBuy).
					Type(binance.OrderTypeLimit).
					Quantity(quantity.String()).
					TimeInForce(binance.TimeInForceTypeGTC).
					Price(price).
					Do(context.Background())
			} else {
				err = client.NewCreateOrderService().
					Symbol(w.Symbol).
					Side(binance.SideTypeBuy).
					Type(binance.OrderTypeLimit).
					Quantity(quantity.String()).
					TimeInForce(binance.TimeInForceTypeGTC).
					Price(price).
					Test(context.Background())
				log.Infof("%s buy: %v", w.Symbol, err)
			}
			if err != nil {
				log.Errorf("%s, Qty: %s, Price: %s", w.Symbol, quantity.String(), price)
			}
			record.Operate(techan.Order{
				Side:          techan.BUY,
				Security:      w.Symbol,
				Price:         newCandle.ClosePrice,
				Amount:        quantity.Sub(quantity.Mul(big.NewDecimal(w.Commission))),
				ExecutionTime: time.Now(),
			})
			log.Infof("%s entering at price: %s", w.Symbol, price)
		} else if long.ShouldExit(series.LastIndex(), record) {
			var (
				// resp *binance.CreateOrderResponse
				err error
			)
			quantity := calculateAmount(big.NewDecimal(w.Risk), newCandle.ClosePrice, big.NewFromInt(w.Leverage), w.SymbolInfo.LotSizeFilter())
			price := newCandle.ClosePrice.FormattedString(w.SymbolInfo.QuotePrecision)
			if !w.Demo {
				_, err = client.NewCreateOrderService().
					Symbol(w.Symbol).
					Side(binance.SideTypeSell).
					Type(binance.OrderTypeLimit).
					Quantity(quantity.String()).
					TimeInForce(binance.TimeInForceTypeGTC).
					Price(price).
					Do(context.Background())
			} else {
				err = client.NewCreateOrderService().
					Symbol(w.Symbol).
					Side(binance.SideTypeSell).
					Type(binance.OrderTypeLimit).
					Quantity(quantity.String()).
					TimeInForce(binance.TimeInForceTypeGTC).
					Price(price).
					Test(context.Background())
			}
			log.Infof("%s sell: %v", w.Symbol, err)
			if err != nil {
				log.Errorf("%s, Qty: %s, Price: %s", w.Symbol, quantity.String(), price)
				return
			}
			record.Operate(techan.Order{
				Side:          techan.SELL,
				Security:      w.Symbol,
				Price:         newCandle.ClosePrice,
				Amount:        quantity,
				ExecutionTime: time.Now(),
			})
			log.Infof("%s exiting at price: %s", w.Symbol, price)
		}
		newCandle = techan.NewCandle(newCandle.Period.Advance(1))
		if success := series.AddCandle(newCandle); !success {
			log.Errorln("failed to add candle")
		}
		log.Debugln(event)
	}
	errHandler := func(err error) {
		log.Error(err)
	}
	return wsKlineHandler, errHandler, nil
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
