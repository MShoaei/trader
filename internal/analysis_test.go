package internal

import (
	"testing"
	"time"

	"github.com/MShoaei/techan"
	"github.com/sdcoffey/big"
)

func TestCommissionAnalysis_Analyze(t *testing.T) {
	record := techan.NewTradingRecord()
	ca := CommissionAnalysis{
		Commission: 1,
	}
	t.Run("empty", func(t *testing.T) {
		got := ca.Analyze(record)
		if got != 0 {
			t.Errorf("something is very wrong")
		}
	})

	record.Operate(techan.Order{
		Side:          techan.BUY,
		Security:      "WOW",
		Price:         big.NewDecimal(200),
		Amount:        big.NewDecimal(100),
		ExecutionTime: time.Now(),
	})
	record.Operate(techan.Order{
		Side:          techan.SELL,
		Security:      "WOW",
		Price:         big.NewDecimal(200),
		Amount:        big.NewDecimal(99),
		ExecutionTime: time.Now(),
	})
	t.Run("commission 1%", func(t *testing.T) {
		expect := 398.0
		got := ca.Analyze(record)
		if got != expect {
			t.Errorf("expected %f, got %f", expect, got)
		}
	})
}
