package internals

import (
	"math"

	"github.com/MShoaei/techan"
	"github.com/sdcoffey/big"
)

type DynamicStrategyFunc func(*techan.TimeSeries) (long, short techan.RuleStrategy)

//type StaticStrategyFunc func(*techan.TimeSeries) (long, short techan.Rule)

func CreateBollingerStochStrategy(series *techan.TimeSeries) (long, short techan.RuleStrategy) {
	closePrice := techan.NewClosePriceIndicator(series)

	bbUpper := techan.NewBollingerUpperBandIndicator(closePrice, 20, 2)
	bbLower := techan.NewBollingerLowerBandIndicator(closePrice, 20, 2)

	stoch := techan.NewSlowStochasticIndicator(techan.NewFastStochasticIndicator(series, 14), 3)

	longEntrySignal := techan.And(techan.UnderIndicatorRule{First: stoch, Second: techan.NewConstantIndicator(20)}, techan.NewCrossUpIndicatorRule(bbLower, closePrice))
	longExitSignal := techan.Or(
		techan.OverIndicatorRule{First: stoch, Second: techan.NewConstantIndicator(80)},
		techan.OverIndicatorRule{First: closePrice, Second: bbUpper},
	)
	long = techan.RuleStrategy{
		EntryRule:      longEntrySignal,
		ExitRule:       longExitSignal,
		UnstablePeriod: 100,
	}

	shortEntrySignal := techan.And(techan.OverIndicatorRule{First: stoch, Second: techan.NewConstantIndicator(80)}, techan.NewCrossDownIndicatorRule(closePrice, bbUpper))
	shortExitSignal := techan.Or(
		techan.OverIndicatorRule{First: stoch, Second: techan.NewConstantIndicator(80)},
		techan.OverIndicatorRule{First: closePrice, Second: bbUpper},
	)
	short = techan.RuleStrategy{
		EntryRule:      shortEntrySignal,
		ExitRule:       shortExitSignal,
		UnstablePeriod: 100,
	}
	return long, short
}

func CreateMACDStrategy(series *techan.TimeSeries) (long, short techan.RuleStrategy) {
	closePrice := techan.NewClosePriceIndicator(series)
	macd := techan.NewMACDIndicator(closePrice, 12, 26)
	macdHist := techan.NewMACDHistogramIndicator(macd, 9)

	long = techan.RuleStrategy{
		EntryRule:      techan.NewCrossUpIndicatorRule(techan.NewConstantIndicator(0), macdHist),
		ExitRule:       techan.NewCrossDownIndicatorRule(macdHist, techan.NewConstantIndicator(0)),
		UnstablePeriod: 100,
	}
	short = techan.RuleStrategy{
		EntryRule:      techan.NewCrossDownIndicatorRule(macdHist, techan.NewConstantIndicator(0)),
		ExitRule:       techan.NewCrossUpIndicatorRule(techan.NewConstantIndicator(0), macdHist),
		UnstablePeriod: 100,
	}
	return long, short
}

func CreateEMAStrategy(series *techan.TimeSeries) (long, short techan.RuleStrategy) {
	closePrice := techan.NewClosePriceIndicator(series)
	ema200 := techan.NewEMAIndicator(closePrice, 200)
	ema50 := techan.NewEMAIndicator(closePrice, 50)

	long = techan.RuleStrategy{
		EntryRule: techan.And(
			techan.OverIndicatorRule{First: closePrice, Second: ema50},
			techan.OverIndicatorRule{First: ema50, Second: ema200},
		),
		ExitRule:       techan.UnderIndicatorRule{First: closePrice, Second: ema200},
		UnstablePeriod: 200,
	}
	short = techan.RuleStrategy{
		EntryRule: techan.And(
			techan.UnderIndicatorRule{First: closePrice, Second: ema50},
			techan.UnderIndicatorRule{First: ema50, Second: ema200},
		),
		ExitRule:       techan.OverIndicatorRule{First: closePrice, Second: ema200},
		UnstablePeriod: 200,
	}
	return long, short
}

func CreateIchimokuStrategy(series *techan.TimeSeries) (long, short techan.RuleStrategy) {
	closePrice := techan.NewClosePriceIndicator(series)
	conv := NewConversionLineIndicator(series, 9)
	base := NewBaseLineIndicator(series, 26)
	spanA := NewLeadingSpanAIndicator(conv.(conversionLineIndicator), base.(baseLineIndicator))
	spanB := NewLeadingSpanBIndicator(series, 52)
	laggingSpan := NewLaggingSpanIndicator(series)

	longRule1 := techan.And(
		techan.OverIndicatorRule{First: closePrice, Second: NewDispositionIndicator(spanA, -26)},
		techan.OverIndicatorRule{First: closePrice, Second: NewDispositionIndicator(spanB, -26)},
	)
	longRule2 := techan.OverIndicatorRule{First: spanA, Second: spanB}
	longRule3 := techan.OverIndicatorRule{First: conv, Second: base}
	longRule4 := techan.And(
		techan.OverIndicatorRule{First: laggingSpan, Second: NewDispositionIndicator(spanA, -52)},
		techan.OverIndicatorRule{First: laggingSpan, Second: NewDispositionIndicator(spanB, -52)},
	)
	long = techan.RuleStrategy{
		EntryRule: techan.And(
			longRule1,
			techan.And(longRule2,
				techan.And(longRule3, longRule4),
			),
		),
		ExitRule: techan.Or(
			techan.UnderIndicatorRule{First: laggingSpan, Second: NewDispositionIndicator(closePrice, -26)},
			techan.UnderIndicatorRule{
				First: closePrice,
				Second: NewMinimumIndicator(
					NewDispositionIndicator(spanA, -26),
					NewDispositionIndicator(spanB, -26)),
			}),
		UnstablePeriod: 100,
	}
	shortRule1 := techan.And(
		techan.UnderIndicatorRule{First: closePrice, Second: NewDispositionIndicator(spanA, -26)},
		techan.UnderIndicatorRule{First: closePrice, Second: NewDispositionIndicator(spanB, -26)},
	)
	shortRule2 := techan.UnderIndicatorRule{First: spanA, Second: spanB}
	shortRule3 := techan.UnderIndicatorRule{First: conv, Second: base}
	shortRule4 := techan.And(
		techan.UnderIndicatorRule{First: laggingSpan, Second: NewDispositionIndicator(spanA, -52)},
		techan.UnderIndicatorRule{First: laggingSpan, Second: NewDispositionIndicator(spanB, -52)},
	)
	short = techan.RuleStrategy{
		EntryRule: techan.And(
			shortRule1,
			techan.And(shortRule2,
				techan.And(shortRule3, shortRule4),
			),
		),
		ExitRule: techan.Or(
			techan.OverIndicatorRule{First: laggingSpan, Second: NewDispositionIndicator(closePrice, -26)},
			techan.OverIndicatorRule{
				First: closePrice,
				Second: NewMinimumIndicator(
					NewDispositionIndicator(spanA, -26),
					NewDispositionIndicator(spanB, -26)),
			}),
		UnstablePeriod: 100,
	}
	return long, short
}

func CreateEMAStochATRStrategy(series *techan.TimeSeries) (long, short techan.RuleStrategy, atr techan.Indicator) {
	closePrice := techan.NewClosePriceIndicator(series)
	ema50 := techan.NewEMAIndicator(closePrice, 50) //100
	ema14 := techan.NewEMAIndicator(closePrice, 14) //19
	ema8 := techan.NewEMAIndicator(closePrice, 8)   //8
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
			},
		),
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
