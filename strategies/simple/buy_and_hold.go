// strategies/buy_and_hold.go
package simple

import "bt/internal"

type BuyAndHoldStrategy struct{}

func (s *BuyAndHoldStrategy) Name() string {
	return "buy_and_hold"
}

func (s *BuyAndHoldStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	signals := make([]internal.SignalType, len(candles))
	if len(candles) == 0 {
		return signals
	}

	// Покупаем на первой свече
	signals[0] = internal.BUY

	// Никогда не продаем
	for i := 1; i < len(signals); i++ {
		signals[i] = internal.HOLD
	}

	return signals
}

func (s *BuyAndHoldStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	// Нет параметров для оптимизации
	return internal.StrategyParams{}
}

func init() {
	internal.RegisterStrategy("buy_and_hold", &BuyAndHoldStrategy{})
}
