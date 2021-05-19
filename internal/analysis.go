package internal

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

// TotalProfitAnalysis analyzes the trading record for total profit.
type TotalProfitAnalysis struct {
	Commission float64
}

// Analyze analyzes the trading record for total profit.
func (tps TotalProfitAnalysis) Analyze(record *techan.TradingRecord) float64 {
	totalProfit := big.NewDecimal(0)
	for _, trade := range record.Trades {
		if trade.IsClosed() {

			amount := trade.EntranceOrder().Amount
			realAmount := amount.Sub(amount.Mul(big.NewDecimal(tps.Commission * 0.01)))
			closeCommission := trade.ExitValue().Mul(big.NewDecimal(tps.Commission * 0.01))

			openValue := trade.CostBasis()
			closeValue := realAmount.Mul(trade.ExitOrder().Price).Sub(closeCommission)

			// costBasis := trade.CostBasis()
			// exitValue := trade.ExitValue()

			if trade.IsLong() {
				totalProfit = totalProfit.Add(closeValue.Sub(openValue))
			} else if trade.IsShort() {
				totalProfit = totalProfit.Sub(closeValue.Sub(openValue))
			}

		}
	}

	return totalProfit.Float()
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
type ProfitableTradesAnalysis struct {
	Commission float64
}

// Analyze returns the number of profitable trades in a trading record
func (pta ProfitableTradesAnalysis) Analyze(record *techan.TradingRecord) float64 {
	var profitableTrades int

	for _, trade := range record.Trades {
		amount := trade.EntranceOrder().Amount
		realAmount := amount.Sub(amount.Mul(big.NewDecimal(pta.Commission * 0.01)))
		closeCommission := realAmount.Mul(trade.ExitOrder().Price).Mul(big.NewDecimal(pta.Commission * 0.01))

		openValue := trade.CostBasis()
		closeValue := realAmount.Mul(trade.ExitOrder().Price).Sub(closeCommission)

		if (trade.IsLong() && closeValue.GT(openValue)) || (trade.IsShort() && closeValue.LT(openValue)) {
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
	total := big.NewDecimal(0)
	for _, trade := range record.Trades {
		amount := trade.EntranceOrder().Amount
		commissionAmount := amount.Mul(big.NewDecimal(ca.Commission * 0.01))
		total = total.Add(commissionAmount.Mul(trade.ExitOrder().Price))

		closeCommission := amount.Sub(commissionAmount).Mul(trade.ExitOrder().Price).Mul(big.NewDecimal(ca.Commission * 0.01))
		total = total.Add(closeCommission)
	}
	return total.Float()
}

type OpenPLAnalysis struct {
	LastCandle *techan.Candle
	Commission float64
}

func (o OpenPLAnalysis) Analyze(record *techan.TradingRecord) float64 {
	if record.CurrentPosition().IsNew() {
		return 0
	}
	var profit big.Decimal
	trade := record.CurrentPosition()
	amount := trade.EntranceOrder().Amount
	realAmount := amount.Sub(amount.Mul(big.NewDecimal(o.Commission * 0.01)))

	closeCommission := o.LastCandle.ClosePrice.Mul(realAmount).Mul(big.NewDecimal(o.Commission * 0.01))
	if trade.IsShort() {
		profit = o.LastCandle.ClosePrice.Mul(realAmount).Sub(trade.CostBasis()).Neg().Sub(closeCommission)
	} else if trade.IsLong() {
		profit = o.LastCandle.ClosePrice.Mul(realAmount).Sub(trade.CostBasis()).Sub(closeCommission)
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
