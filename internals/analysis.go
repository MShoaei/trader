package internals

import (
	"fmt"
	"io"
	"math"
	"time"

	"github.com/MShoaei/techan"
	"github.com/sdcoffey/big"
)

// LogTradesAnalysis is a wrapper around an io.Writer, which logs every trade executed to that writer
type LogTradesAnalysis struct {
	io.Writer
}

// Analyze logs trades to provided io.Writer
func (lta LogTradesAnalysis) Analyze(record *techan.TradingRecord) float64 {
	var profit big.Decimal
	logOrder := func(trade *techan.Position) {
		if trade.IsShort() {
			fmt.Fprintf(lta.Writer, "%s - enter with sell %s (%s @ $%s)\n", trade.EntranceOrder().ExecutionTime.UTC().Format(time.RFC822), trade.EntranceOrder().Security, trade.EntranceOrder().Amount, trade.EntranceOrder().Price)
			fmt.Fprintf(lta.Writer, "%s - exit with buy %s (%s @ $%s)\n", trade.ExitOrder().ExecutionTime.UTC().Format(time.RFC822), trade.ExitOrder().Security, trade.ExitOrder().Amount, trade.ExitOrder().Price)
			profit = trade.ExitValue().Sub(trade.CostBasis()).Neg()
		} else {
			fmt.Fprintf(lta.Writer, "%s - enter with buy %s (%s @ $%s)\n", trade.EntranceOrder().ExecutionTime.UTC().Format(time.RFC822), trade.EntranceOrder().Security, trade.EntranceOrder().Amount, trade.EntranceOrder().Price)
			fmt.Fprintf(lta.Writer, "%s - exit with sell %s (%s @ $%s)\n", trade.ExitOrder().ExecutionTime.UTC().Format(time.RFC822), trade.ExitOrder().Security, trade.ExitOrder().Amount, trade.ExitOrder().Price)
			profit = trade.ExitValue().Sub(trade.CostBasis())
		}
		fmt.Fprintf(lta.Writer, "Profit: $%s\n", profit)
	}

	for _, trade := range record.Trades {
		if trade.IsClosed() {
			logOrder(trade)
		}
	}
	return 0.0
}

// ProfitableTradesAnalysis analyzes the trading record for the number of profitable trades
type ProfitableTradesAnalysis struct{}

// Analyze returns the number of profitable trades in a trading record
func (pta ProfitableTradesAnalysis) Analyze(record *techan.TradingRecord) float64 {
	var profitableTrades int
	for _, trade := range record.Trades {
		costBasis := trade.EntranceOrder().Amount.Mul(trade.EntranceOrder().Price)
		sellPrice := trade.ExitOrder().Amount.Mul(trade.ExitOrder().Price)

		if (trade.IsLong() && sellPrice.GT(costBasis)) || (trade.IsShort() && sellPrice.LT(costBasis)) {
			profitableTrades++
		}
	}

	return float64(profitableTrades)
}

type CommissionAnalysis struct {
	Commission float64
}

// Analyze analyzes the trading record for the total commission cost.
func (ca CommissionAnalysis) Analyze(record *techan.TradingRecord) float64 {
	total := big.ZERO
	for _, trade := range record.Trades {
		total = total.Add(trade.CostBasis().Mul(big.NewDecimal(ca.Commission * 0.01)))
		total = total.Add(trade.ExitValue().Mul(big.NewDecimal(ca.Commission * 0.01)))
	}
	return total.Float()
}

type OpenPLAnalysis struct {
	LastCandle *techan.Candle
}

func (o OpenPLAnalysis) Analyze(record *techan.TradingRecord) float64 {
	if record.CurrentPosition().IsNew() {
		return 0
	}
	var profit big.Decimal
	trade := record.CurrentPosition()
	amount := trade.EntranceOrder().Amount
	if trade.IsShort() {
		profit = o.LastCandle.ClosePrice.Mul(amount).Sub(trade.CostBasis()).Neg()
	} else if trade.IsLong() {
		profit = o.LastCandle.ClosePrice.Mul(amount).Sub(trade.CostBasis())
	}
	return profit.Float()
}

func isProfitable(trade *techan.Position) bool {
	return (trade.IsLong() && trade.ExitOrder().Price.GT(trade.EntranceOrder().Price)) || (trade.IsShort() && trade.ExitOrder().Price.LT(trade.EntranceOrder().Price))
}

type WinStreakAnalysis struct{}

func (w WinStreakAnalysis) Analyze(record *techan.TradingRecord) float64 {
	max := 0
	currentStreak := 0
	for _, trade := range record.Trades {
		if !isProfitable(trade) {
			max = techan.Max(max, currentStreak)
			currentStreak = 0
			continue
		}
		currentStreak++
	}
	return math.Max(float64(max), float64(currentStreak))
}

type LoseStreakAnalysis struct{}

func (l LoseStreakAnalysis) Analyze(record *techan.TradingRecord) float64 {
	max := 0
	currentStreak := 0
	for _, trade := range record.Trades {
		if isProfitable(trade) {
			max = techan.Max(max, currentStreak)
			currentStreak = 0
			continue
		}
		currentStreak++
	}
	return math.Max(float64(max), float64(currentStreak))
}

type MaxWinAnalysis struct{}

func (m MaxWinAnalysis) Analyze(record *techan.TradingRecord) float64 {
	maxProfit := big.ZERO
	for _, trade := range record.Trades {
		if !isProfitable(trade) {
			continue
		}
		if trade.IsShort() {
			maxProfit = big.MaxSlice(maxProfit, trade.ExitValue().Sub(trade.CostBasis()).Neg())
		} else {
			maxProfit = big.MaxSlice(maxProfit, trade.ExitValue().Sub(trade.CostBasis()))
		}
	}
	return maxProfit.Float()
}

type MaxLossAnalysis struct{}

func (m MaxLossAnalysis) Analyze(record *techan.TradingRecord) float64 {
	minProfit := big.ZERO
	for _, trade := range record.Trades {
		if isProfitable(trade) {
			continue
		}
		if trade.IsShort() {
			minProfit = big.MinSlice(minProfit, trade.ExitValue().Sub(trade.CostBasis()).Neg())
		} else {
			minProfit = big.MinSlice(minProfit, trade.ExitValue().Sub(trade.CostBasis()))
		}
	}
	return minProfit.Float()
}

type AverageWinAnalysis struct{}

func (a AverageWinAnalysis) Analyze(record *techan.TradingRecord) float64 {
	win := big.ZERO
	count := len(record.Trades)
	for _, trade := range record.Trades {
		if !isProfitable(trade) {
			continue
		}
		count++
		if trade.IsShort() {
			win = win.Add(trade.ExitValue().Sub(trade.CostBasis()).Neg())
		} else {
			win = win.Add(trade.ExitValue().Sub(trade.CostBasis()))
		}
	}
	return win.Div(big.NewFromInt(count)).Float()
}

type AverageLossAnalysis struct{}

func (a AverageLossAnalysis) Analyze(record *techan.TradingRecord) float64 {
	loss := big.ZERO
	count := len(record.Trades)
	for _, trade := range record.Trades {
		if isProfitable(trade) {
			continue
		}
		if trade.IsShort() {
			loss = loss.Add(trade.ExitValue().Sub(trade.CostBasis()).Neg())
		} else {
			loss = loss.Add(trade.ExitValue().Sub(trade.CostBasis()))
		}
	}
	return loss.Div(big.NewFromInt(count)).Float()
}
