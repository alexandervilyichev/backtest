// strategy_refactored.go
// Улучшенная архитектура стратегий с использованием композиции и интерфейсов
package internal

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
)

// ============================================================================
// ИНТЕРФЕЙСЫ - определяют контракты, а не реализацию
// ============================================================================

// StrategyConfig - конфигурация стратегии
type StrategyConfigV2 interface {
	Validate() error
	String() string
}

// SignalGenerator - генератор торговых сигналов
type SignalGenerator interface {
	GenerateSignals(candles []Candle, config StrategyConfigV2) []SignalType
}

// ConfigOptimizer - оптимизатор конфигурации
type ConfigOptimizer interface {
	Optimize(candles []Candle, generator SignalGenerator) StrategyConfigV2
}

// ConfigManager - управление конфигурацией
type ConfigManager interface {
	DefaultConfig() StrategyConfigV2
	LoadFromJSON(raw json.RawMessage) (StrategyConfigV2, error)
}

// TradingStrategy - полная стратегия торговли
type TradingStrategy interface {
	Name() string
	SignalGenerator
	ConfigOptimizer
	ConfigManager
}

// ============================================================================
// КОМПОЗИЦИЯ - собираем функциональность из независимых компонентов
// ============================================================================

// SlippageProvider - провайдер проскальзывания
type SlippageProvider struct {
	slippage float64
}

func NewSlippageProvider(slippage float64) *SlippageProvider {
	return &SlippageProvider{slippage: slippage}
}

func (sp *SlippageProvider) GetSlippage() float64 {
	return sp.slippage
}

func (sp *SlippageProvider) SetSlippage(slippage float64) {
	sp.slippage = slippage
}

// ============================================================================
// ConfigManagerImpl - реализация управления конфигурацией
// ============================================================================

type ConfigManagerImpl struct {
	defaultConfig StrategyConfigV2
	configFactory func() StrategyConfigV2 // фабрика для создания новых экземпляров
}

func NewConfigManager(defaultConfig StrategyConfigV2, factory func() StrategyConfigV2) *ConfigManagerImpl {
	return &ConfigManagerImpl{
		defaultConfig: defaultConfig,
		configFactory: factory,
	}
}

func (cm *ConfigManagerImpl) DefaultConfig() StrategyConfigV2 {
	return cm.defaultConfig
}

func (cm *ConfigManagerImpl) LoadFromJSON(raw json.RawMessage) (StrategyConfigV2, error) {
	config := cm.configFactory()
	if err := json.Unmarshal(raw, config); err != nil {
		return nil, err
	}
	return config, nil
}

// ============================================================================
// GridSearchOptimizer - универсальный оптимизатор через grid search
// ============================================================================

type GridSearchOptimizer struct {
	slippageProvider *SlippageProvider
	configGenerator  func() []StrategyConfigV2 // генератор конфигураций для перебора
}

func NewGridSearchOptimizer(
	slippageProvider *SlippageProvider,
	configGenerator func() []StrategyConfigV2,
) *GridSearchOptimizer {
	return &GridSearchOptimizer{
		slippageProvider: slippageProvider,
		configGenerator:  configGenerator,
	}
}

func (gso *GridSearchOptimizer) Optimize(candles []Candle, generator SignalGenerator) StrategyConfigV2 {
	configs := gso.configGenerator()

	// Фильтруем только валидные конфигурации
	validConfigs := lo.Filter(configs, func(cfg StrategyConfigV2, _ int) bool {
		return cfg.Validate() == nil
	})

	if len(validConfigs) == 0 {
		log.Println("Warning: no valid configs for optimization")
		return nil
	}

	// Параллельно тестируем все конфигурации
	configsWithProfit := lop.Map(validConfigs, func(cfg StrategyConfigV2, _ int) lo.Tuple2[StrategyConfigV2, float64] {
		signals := generator.GenerateSignals(candles, cfg)
		result := Backtest(candles, signals, gso.slippageProvider.GetSlippage())
		return lo.Tuple2[StrategyConfigV2, float64]{A: cfg, B: result.TotalProfit}
	})

	// Находим лучшую конфигурацию
	best := lo.MaxBy(configsWithProfit, func(a, b lo.Tuple2[StrategyConfigV2, float64]) bool {
		return a.B > b.B
	})

	fmt.Printf("Best config found: %s with profit: %.4f\n", best.A.String(), best.B)
	return best.A
}

type StrategyBase struct {
	name             string
	signalGenerator  SignalGenerator
	configManager    ConfigManager
	configOptimizer  ConfigOptimizer
	slippageProvider *SlippageProvider
}

// NewStrategyBase - конструктор с явными зависимостями (Dependency Injection)
func NewStrategyBase(
	name string,
	signalGenerator SignalGenerator,
	configManager ConfigManager,
	configOptimizer ConfigOptimizer,
	slippageProvider *SlippageProvider,
) *StrategyBase {
	return &StrategyBase{
		name:             name,
		signalGenerator:  signalGenerator,
		configManager:    configManager,
		configOptimizer:  configOptimizer,
		slippageProvider: slippageProvider,
	}
}

func (sb *StrategyBase) Name() string {
	return sb.name
}

func (sb *StrategyBase) GenerateSignals(candles []Candle, config StrategyConfigV2) []SignalType {
	return sb.signalGenerator.GenerateSignals(candles, config)
}

func (sb *StrategyBase) Optimize(candles []Candle, generator SignalGenerator) StrategyConfigV2 {
	return sb.configOptimizer.Optimize(candles, generator)
}

func (sb *StrategyBase) DefaultConfig() StrategyConfigV2 {
	return sb.configManager.DefaultConfig()
}

func (sb *StrategyBase) LoadFromJSON(raw json.RawMessage) (StrategyConfigV2, error) {
	return sb.configManager.LoadFromJSON(raw)
}

func (sb *StrategyBase) GetSlippage() float64 {
	return sb.slippageProvider.GetSlippage()
}

func (sb *StrategyBase) SetSlippage(slippage float64) {
	sb.slippageProvider.SetSlippage(slippage)
}

var strategyRegistryV2 = make(map[string]TradingStrategy)

func RegisterStrategyV2(strategy TradingStrategy) {
	strategyRegistryV2[strategy.Name()] = strategy
}

func GetStrategyV2(name string) (TradingStrategy, bool) {
	strategy, ok := strategyRegistryV2[name]
	return strategy, ok
}

func GetStrategyNamesV2() []string {
	names := make([]string, 0, len(strategyRegistryV2))
	for name := range strategyRegistryV2 {
		names = append(names, name)
	}
	return names
}
