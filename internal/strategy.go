// strategy.go
package internal

import "log"

type StrategyParams struct {
	VolumeMultiplier          float64
	PullbackSensitivity       int
	RsiPeriod                 int
	RsiBuyThreshold           float64
	RsiSellThreshold          float64
	CciPeriod                 int
	CciBuyLevel               float64
	CciSellLevel              float64
	AoFastPeriod              int
	AoSlowPeriod              int
	AoConfirmByTwoCandles     bool
	SupportLookbackPeriod     int
	SupportBuyThreshold       float64
	SupportSellThreshold      float64
	StochasticKPeriod         int
	StochasticDPeriod         int
	StochasticBuyLevel        float64
	StochasticSellLevel       float64
	MACDFastPeriod            int
	MACDSlowPeriod            int
	MACDSignalPeriod          int
	MAChannelFastPeriod       int
	MAChannelSlowPeriod       int
	MAChannelMultiplier       float64
	MinExtremaDistance        int
	LookbackWindow            int
	ConfidenceThreshold       float64
	MaEmaCorrelationMAPeriod  int
	MaEmaCorrelationEMAPeriod int
	MaEmaCorrelationLookback  int
	MaEmaCorrelationThreshold float64
	QuantizationEnabled       bool
	QuantizationLevels        int
	QuantizationPriceStep     float64
	OBVPeriod                 int
	OBVMultiplier             float64
	UseDivergence             bool
	MomentumPeriod            int
	BreakoutThreshold         float64
	VolatilityFilter          float64
	MinWaveLength             int
	MaxWaveLength             int
	FibonacciThreshold        float64
	TrendStrength             float64
}

type Strategy interface {
	Name() string
	GenerateSignals(candles []Candle, params StrategyParams) []SignalType
	Optimize(candles []Candle) StrategyParams
}

var strategies = make(map[string]Strategy)

func RegisterStrategy(name string, s Strategy) {
	strategies[name] = s
}

func GetStrategy(name string) Strategy {
	s, ok := strategies[name]
	if !ok {
		log.Fatal("Неизвестная стратегия:", name)
	}
	return s
}

func GetStrategyNames() []string {
	names := make([]string, 0, len(strategies))
	for name := range strategies {
		names = append(names, name)
	}
	return names
}
