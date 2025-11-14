// strategy.go
package internal

import (
	"encoding/json"
	"log"
)

// StrategyConfig defines the interface for strategy configuration
type StrategyConfig interface {
	Validate() error
	DefaultConfigString() string
}

// New SOLID architecture interfaces - will replace StrategyParams eventually
type Strategy interface {
	Config

	Name() string
	GenerateSignalsWithConfig(candles []Candle, config StrategyConfig) []SignalType
	OptimizeWithConfig(candles []Candle) StrategyConfig
}

type Config interface {
	DefaultConfig() StrategyConfig
	LoadConfigFromMap(raw json.RawMessage) StrategyConfig
}

type BaseConfig struct {
	Config StrategyConfig
}

func (s *BaseConfig) DefaultConfig() StrategyConfig {
	return s.Config
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
