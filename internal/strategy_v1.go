// strategy.go
package internal

import (
	"encoding/json"
	"log"

	"github.com/samber/lo"

	lop "github.com/samber/lo/parallel"
)

// StrategyConfig defines the interface for strategy configuration
type StrategyConfig interface {
	Validate() error
	DefaultConfigString() string
}

type Strategy interface {
	Config

	Name() string
	GenerateSignalsWithConfig(candles []Candle, config StrategyConfig) []SignalType
	OptimizeWithConfig(candles []Candle) StrategyConfig
}

type InternalStrategy interface {
	GenerateSignalsWithConfig(candles []Candle, config StrategyConfig) []SignalType
}

type BaseStrategy struct {
	BaseConfig
}

type Config interface {
	DefaultConfig() StrategyConfig
	GetSlippage() float64
	SetSlippage(slippage float64)
	LoadConfigFromMap(raw json.RawMessage) StrategyConfig
}

type BaseConfig struct {
	Config   StrategyConfig
	slippage float64
}

func (s *BaseConfig) DefaultConfig() StrategyConfig {
	return s.Config
}

func (s *BaseConfig) GetSlippage() float64 {
	return s.slippage
}

func (s *BaseConfig) SetSlippage(slippage float64) {
	s.slippage = slippage
}

func (s *BaseConfig) LoadConfigFromMap(raw json.RawMessage) StrategyConfig {
	config := s.Config
	if err := json.Unmarshal(raw, config); err != nil {
		return nil
	}
	return config
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

func (b *BaseStrategy) ProcessConfigs(cc InternalStrategy, candles []Candle, configs []StrategyConfig) lo.Tuple2[StrategyConfig, float64] {
	configs = lo.Filter(configs, func(x StrategyConfig, index int) bool {
		return x.Validate() == nil
	})

	configsWithProfit := lop.Map(configs, func(c StrategyConfig, index int) lo.Tuple2[StrategyConfig, float64] {

		signals := cc.GenerateSignalsWithConfig(candles, c)
		result := Backtest(candles, signals, b.GetSlippage())
		return lo.Tuple2[StrategyConfig, float64]{A: c, B: result.TotalProfit}
	})

	max := lo.MaxBy(configsWithProfit, func(
		x lo.Tuple2[StrategyConfig, float64],
		y lo.Tuple2[StrategyConfig, float64]) bool {

		return x.B > y.B
	})
	return max
}
