// strategies/buy_and_hold.go
package simple

import (
	"bt/internal"
	"encoding/json"
)

type BuyAndHoldConfig struct{}

func (c *BuyAndHoldConfig) Validate() error {
	return nil
}

func (c *BuyAndHoldConfig) DefaultConfigString() string {
	return "BuyAndHold()"
}

type BuyAndHoldStrategy struct{}

func (s *BuyAndHoldStrategy) Name() string {
	return "buy_and_hold"
}

func (s *BuyAndHoldStrategy) DefaultConfig() internal.StrategyConfig {
	return &BuyAndHoldConfig{}
}

func (s *BuyAndHoldStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	bhConfig, ok := config.(*BuyAndHoldConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := bhConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

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

func (s *BuyAndHoldStrategy) LoadConfigFromMap(raw json.RawMessage) internal.StrategyConfig {
	config := s.DefaultConfig()
	if err := json.Unmarshal(raw, config); err != nil {
		return nil
	}
	return config
}

func (s *BuyAndHoldStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	// Нет параметров для оптимизации, возврат оптимизированного конфига
	return &BuyAndHoldConfig{}
}

func init() {
	internal.RegisterStrategy("buy_and_hold", &BuyAndHoldStrategy{})
}
