package internal_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/MShoaei/techan"
	"github.com/adshao/go-binance/v2"
)

func TestWatchdog_Watch(t *testing.T) {
	type fields struct {
		Symbol      string
		Interval    string
		Risk        float64
		Commission  float64
		Leverage    int
		Demo        bool
		SymbolInfo  binance.Symbol
		series      *techan.TimeSeries
		records     *techan.TradingRecord
		StopC       chan struct{}
		InterruptCh chan os.Signal
	}
	type args struct {
		client *binance.Client
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    binance.WsKlineHandler
		want1   binance.ErrHandler
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Watchdog{
				Symbol:      tt.fields.Symbol,
				Interval:    tt.fields.Interval,
				Risk:        tt.fields.Risk,
				Commission:  tt.fields.Commission,
				Leverage:    tt.fields.Leverage,
				Demo:        tt.fields.Demo,
				SymbolInfo:  tt.fields.SymbolInfo,
				series:      tt.fields.series,
				records:     tt.fields.records,
				StopC:       tt.fields.StopC,
				InterruptCh: tt.fields.InterruptCh,
			}
			got, got1, err := w.Watch(tt.args.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("Watchdog.Watch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Watchdog.Watch() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Watchdog.Watch() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
