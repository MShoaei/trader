package cmd

import (
	"github.com/MShoaei/techan"
	"github.com/sdcoffey/big"
	"math"
)

type CustomStrategy struct {
	techan.RuleStrategy
	macd     techan.Indicator
	macdHist techan.Indicator
}

type Kind int

const (
	BollingerStoch Kind = iota
	MACD
)

func (k Kind) Strategy(series *techan.TimeSeries) techan.Strategy {
	switch k {
	case BollingerStoch:
		return createBollingerStochStrategy(series)
	case MACD:
		return createMACDStrategy(series)
	default:
		return nil
	}
}

type BollingerStochStrategy struct {
}

func createBollingerStochStrategy(series *techan.TimeSeries) techan.Strategy {
	closePrice := techan.NewClosePriceIndicator(series)

	bbUpper := techan.NewBollingerUpperBandIndicator(closePrice, 20, 2)
	bbLower := techan.NewBollingerLowerBandIndicator(closePrice, 20, 2)

	stoch := techan.NewSlowStochasticIndicator(techan.NewFastStochasticIndicator(series, 14), 3)
	stopLoss := techan.NewStopLossRule(series, -0.1)

	sma := techan.NewSimpleMovingAverage(closePrice, 60)
	trend := techan.NewTrendlineIndicator(sma, 10)

	buySignal := techan.And(techan.UnderIndicatorRule{First: stoch, Second: techan.NewConstantIndicator(20)}, techan.NewCrossUpIndicatorRule(bbLower, closePrice))
	sellSignal := techan.Or(
		stopLoss,
		techan.Or(
			techan.UnderIndicatorRule{First: trend, Second: techan.NewConstantIndicator(0)},
			techan.Or(
				techan.OverIndicatorRule{First: stoch, Second: techan.NewConstantIndicator(80)},
				techan.OverIndicatorRule{First: closePrice, Second: bbUpper},
			),
		),
	)
	return techan.RuleStrategy{
		EntryRule:      buySignal,
		ExitRule:       sellSignal,
		UnstablePeriod: 100,
	}
}

func createMACDStrategy(series *techan.TimeSeries) techan.Strategy {
	closePrice := techan.NewClosePriceIndicator(series)
	macd := techan.NewMACDIndicator(closePrice, 12, 26)
	macdHist := techan.NewMACDHistogramIndicator(macd, 9)

	stopLoss := techan.NewStopLossRule(series, -0.05)

	macdSellSignal := techan.Or(
		stopLoss,
		techan.Or(
			techan.UnderIndicatorRule{First: macdHist, Second: techan.NewConstantIndicator(0)},
			techan.UnderIndicatorRule{First: macd, Second: macdHist},
		),
	)
	macdBuySignal := techan.And(techan.OverIndicatorRule{First: macd, Second: macdHist}, techan.Not(macdSellSignal))
	return techan.RuleStrategy{
		EntryRule:      macdBuySignal,
		ExitRule:       macdSellSignal,
		UnstablePeriod: 100,
	}
}

func createIchimokuStrategy(series *techan.TimeSeries) techan.RuleStrategy {
	closePrice := techan.NewClosePriceIndicator(series)
	conv := NewConversionLineIndicator(series, 9)
	base := NewBaseLineIndicator(series, 26)
	spanA := NewLeadingSpanAIndicator(conv.(conversionLineIndicator), base.(baseLineIndicator))
	spanB := NewLeadingSpanBIndicator(series, 52)
	laggingSpan := NewLaggingSpanIndicator(series)

	rule1 := techan.And(
		techan.OverIndicatorRule{First: closePrice, Second: NewDispositionIndicator(spanA, -26)},
		techan.OverIndicatorRule{First: closePrice, Second: NewDispositionIndicator(spanB, -26)},
	)
	rule2 := techan.OverIndicatorRule{First: spanA, Second: spanB}
	rule3 := techan.OverIndicatorRule{First: conv, Second: base}
	rule4 := techan.And(
		techan.OverIndicatorRule{First: laggingSpan, Second: NewDispositionIndicator(spanA, -52)},
		techan.OverIndicatorRule{First: laggingSpan, Second: NewDispositionIndicator(spanB, -52)},
	)
	buySignal := techan.And(
		rule1,
		techan.And(rule2,
			techan.And(rule3, rule4),
		),
	)
	sellSignal := techan.Or(
		techan.UnderIndicatorRule{First: laggingSpan, Second: NewDispositionIndicator(closePrice, -26)},
		techan.UnderIndicatorRule{
			First: closePrice,
			Second: NewMinimumIndicator(
				NewDispositionIndicator(spanA, -26),
				NewDispositionIndicator(spanB, -26)),
		})
	return techan.RuleStrategy{
		EntryRule:      buySignal,
		ExitRule:       sellSignal,
		UnstablePeriod: 100,
	}
}

type EMAStochATRStrategy struct {
	EMA50         techan.Indicator
	EMA14         techan.Indicator
	EMA8          techan.Indicator
	StochasticRSI techan.Indicator
	ATR           techan.Indicator
}

func createEMAStochATRStrategy(series *techan.TimeSeries) (long, short techan.RuleStrategy, atr techan.Indicator) {
	closePrice := techan.NewClosePriceIndicator(series)
	ema50 := techan.NewEMAIndicator(closePrice, 50)
	ema14 := techan.NewEMAIndicator(closePrice, 14)
	ema8 := techan.NewEMAIndicator(closePrice, 8)
	stochRSI := NewStochasticRSI(series, 14)
	atr = techan.NewAverageTrueRangeIndicator(series, 14)
	longRule1 := techan.And(
		techan.OverIndicatorRule{
			First:  ema8,
			Second: ema14,
		}, techan.And(
			techan.OverIndicatorRule{
				First:  ema14,
				Second: ema50,
			}, techan.OverIndicatorRule{
				First:  ema8,
				Second: ema50,
			}),
	)
	longRule2 := techan.OverIndicatorRule{First: closePrice, Second: ema8}
	longRule3 := techan.NewCrossUpIndicatorRule(stochRSI.StochD, stochRSI.StochK)

	long = techan.RuleStrategy{
		EntryRule:      techan.And(longRule1, techan.And(longRule2, longRule3)),
		ExitRule:       FalseRule{},
		UnstablePeriod: 100,
	}

	shortRule1 := techan.And(
		techan.UnderIndicatorRule{
			First:  ema8,
			Second: ema14,
		}, techan.And(
			techan.UnderIndicatorRule{
				First:  ema14,
				Second: ema50,
			}, techan.UnderIndicatorRule{
				First:  ema8,
				Second: ema50,
			}),
	)
	shortRule2 := techan.UnderIndicatorRule{First: closePrice, Second: ema8}
	shortRule3 := techan.NewCrossDownIndicatorRule(stochRSI.StochD, stochRSI.StochK)

	short = techan.RuleStrategy{
		EntryRule:      techan.And(shortRule1, techan.And(shortRule2, shortRule3)),
		ExitRule:       FalseRule{},
		UnstablePeriod: 100,
	}
	return long, short, atr
}

type FalseRule struct{}

func (r FalseRule) IsSatisfied(_ int, _ *techan.TradingRecord) bool {
	return false
}

type kIndicator struct {
	closePrice techan.Indicator
	minValue   techan.Indicator
	maxValue   techan.Indicator
	window     int
}

func NewFastStochasticIndicator(closePrice techan.Indicator, series *techan.TimeSeries, timeframe int) techan.Indicator {
	return kIndicator{
		closePrice: closePrice,
		minValue:   techan.NewMinimumValueIndicator(techan.NewLowPriceIndicator(series), timeframe),
		maxValue:   techan.NewMaximumValueIndicator(techan.NewHighPriceIndicator(series), timeframe),
		window:     timeframe,
	}
}

func (k kIndicator) Calculate(index int) big.Decimal {
	closeVal := k.closePrice.Calculate(index)
	minVal := k.minValue.Calculate(index)
	maxVal := k.maxValue.Calculate(index)

	if minVal.EQ(maxVal) {
		return big.NewDecimal(math.Inf(1))
	}

	return closeVal.Sub(minVal).Div(maxVal.Sub(minVal)).Mul(big.NewDecimal(100))
}

type StochasticRSI struct {
	StochK techan.Indicator
	StochD techan.Indicator
}

func NewStochasticRSI(series *techan.TimeSeries, window int) StochasticRSI {
	rsi := techan.NewRelativeStrengthIndexIndicator(techan.NewClosePriceIndicator(series), window)
	stochK := NewFastStochasticIndicator(rsi, series, 14)
	stochD := techan.NewSlowStochasticIndicator(stochK, 14)
	return StochasticRSI{
		StochK: stochK,
		StochD: stochD,
	}
}
