package internals

import (
	"github.com/MShoaei/techan"
	"github.com/sdcoffey/big"
)

type conversionLineIndicator struct {
	ph, pl techan.Indicator
}

func NewConversionLineIndicator(series *techan.TimeSeries, window int) techan.Indicator {
	return conversionLineIndicator{
		ph: techan.NewMaximumValueIndicator(techan.NewHighPriceIndicator(series), window),
		pl: techan.NewMinimumValueIndicator(techan.NewLowPriceIndicator(series), window),
	}
}

func (cli conversionLineIndicator) Calculate(index int) big.Decimal {
	return cli.ph.Calculate(index).Add(cli.pl.Calculate(index)).Div(big.NewDecimal(2))
}

type baseLineIndicator struct {
	ph, pl techan.Indicator
}

func NewBaseLineIndicator(series *techan.TimeSeries, window int) techan.Indicator {
	return baseLineIndicator{
		ph: techan.NewMaximumValueIndicator(techan.NewHighPriceIndicator(series), window),
		pl: techan.NewMinimumValueIndicator(techan.NewLowPriceIndicator(series), window),
	}
}

func (cli baseLineIndicator) Calculate(index int) big.Decimal {
	return cli.ph.Calculate(index).Add(cli.pl.Calculate(index)).Div(big.NewDecimal(2))
}

type leadingSpanAIndicator struct {
	conv techan.Indicator
	base techan.Indicator
}

func NewLeadingSpanAIndicator(c conversionLineIndicator, b baseLineIndicator) techan.Indicator {
	return leadingSpanAIndicator{
		conv: c,
		base: b,
	}
}

func (lsa leadingSpanAIndicator) Calculate(index int) big.Decimal {
	return lsa.conv.Calculate(index).Add(lsa.base.Calculate(index)).Div(big.NewDecimal(2))
}

type leadingSpanBIndicator struct {
	ph, pl techan.Indicator
}

func NewLeadingSpanBIndicator(series *techan.TimeSeries, window int) techan.Indicator {
	return leadingSpanBIndicator{
		ph: techan.NewMaximumValueIndicator(techan.NewHighPriceIndicator(series), window),
		pl: techan.NewMinimumValueIndicator(techan.NewLowPriceIndicator(series), window),
	}
}

func (lsb leadingSpanBIndicator) Calculate(index int) big.Decimal {
	return lsb.ph.Calculate(index).Add(lsb.pl.Calculate(index)).Div(big.NewDecimal(2))
}

func NewLaggingSpanIndicator(series *techan.TimeSeries) techan.Indicator {
	return techan.NewClosePriceIndicator(series)
}

type dispositionIndicator struct {
	indicator   techan.Indicator
	disposition int
}

func NewDispositionIndicator(indicator techan.Indicator, disposition int) techan.Indicator {
	return dispositionIndicator{
		indicator:   indicator,
		disposition: disposition,
	}
}

func (di dispositionIndicator) Calculate(index int) big.Decimal {
	if index+di.disposition < 0 {
		return big.ZERO
	}
	return di.indicator.Calculate(index + di.disposition)
}

type minimumIndicator struct {
	ind1 techan.Indicator
	ind2 techan.Indicator
}

func NewMinimumIndicator(ind1, ind2 techan.Indicator) techan.Indicator {
	return minimumIndicator{
		ind1: ind1,
		ind2: ind2,
	}
}

func (m minimumIndicator) Calculate(index int) big.Decimal {
	if m.ind1.Calculate(index).LT(m.ind2.Calculate(index)) {
		return m.ind1.Calculate(index)
	}
	return m.ind2.Calculate(index)
}
