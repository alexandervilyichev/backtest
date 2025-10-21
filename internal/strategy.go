// strategy.go
package internal

import (
	"log"
)

// StrategyConfig defines the interface for strategy configuration
type StrategyConfig interface {
	Validate() error
	DefaultConfigString() string
}

// New SOLID architecture interfaces - will replace StrategyParams eventually
type Strategy interface {
	Name() string
	DefaultConfig() StrategyConfig
	GenerateSignalsWithConfig(candles []Candle, config StrategyConfig) []SignalType
	OptimizeWithConfig(candles []Candle) StrategyConfig
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
