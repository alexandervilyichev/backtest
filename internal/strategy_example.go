// strategy_example.go
// Пример использования улучшенной архитектуры стратегий
package internal

import (
	"errors"
	"fmt"

	"github.com/samber/lo"
)

// ============================================================================
// ПРИМЕР: Simple Moving Average Strategy с новой архитектурой
// ============================================================================

// SMAConfigV2 - конфигурация SMA стратегии
type SMAConfigV2 struct {
	FastPeriod int `json:"fast_period"`
	SlowPeriod int `json:"slow_period"`
}

func (c *SMAConfigV2) Validate() error {
	if c.FastPeriod <= 0 {
		return errors.New("fast period must be positive")
	}
	if c.SlowPeriod <= 0 {
		return errors.New("slow period must be positive")
	}
	if c.FastPeriod >= c.SlowPeriod {
		return errors.New("fast period must be less than slow period")
	}
	return nil
}

func (c *SMAConfigV2) String() string {
	return fmt.Sprintf("SMA(fast=%d, slow=%d)", c.FastPeriod, c.SlowPeriod)
}

// ============================================================================
// SMASignalGenerator - генератор сигналов для SMA стратегии
// ============================================================================

type SMASignalGenerator struct{}

func NewSMASignalGenerator() *SMASignalGenerator {
	return &SMASignalGenerator{}
}

func (sg *SMASignalGenerator) GenerateSignals(candles []Candle, config StrategyConfigV2) []SignalType {
	smaConfig, ok := config.(*SMAConfigV2)
	if !ok {
		return make([]SignalType, len(candles))
	}

	if err := smaConfig.Validate(); err != nil {
		return make([]SignalType, len(candles))
	}

	fastSMA := CalculateSMACommon(candles, smaConfig.FastPeriod)
	slowSMA := CalculateSMACommon(candles, smaConfig.SlowPeriod)

	if fastSMA == nil || slowSMA == nil {
		return make([]SignalType, len(candles))
	}

	signals := make([]SignalType, len(candles))
	inPosition := false

	startIndex := smaConfig.SlowPeriod
	for i := startIndex; i < len(candles); i++ {
		if i > startIndex {
			prevFast := fastSMA[i-1]
			prevSlow := slowSMA[i-1]
			currFast := fastSMA[i]
			currSlow := slowSMA[i]

			// Golden cross - BUY
			if !inPosition && prevFast <= prevSlow && currFast > currSlow {
				signals[i] = BUY
				inPosition = true
				continue
			}

			// Death cross - SELL
			if inPosition && prevFast >= prevSlow && currFast < currSlow {
				signals[i] = SELL
				inPosition = false
				continue
			}
		}

		signals[i] = HOLD
	}

	return signals
}

// ============================================================================
// SMAConfigGenerator - генератор конфигураций для оптимизации
// ============================================================================

type SMAConfigGenerator struct {
	fastRange []int
	slowRange []int
}

func NewSMAConfigGenerator(fastRange, slowRange []int) *SMAConfigGenerator {
	return &SMAConfigGenerator{
		fastRange: fastRange,
		slowRange: slowRange,
	}
}

func (cg *SMAConfigGenerator) Generate() []StrategyConfigV2 {
	configs := lo.FlatMap(cg.fastRange, func(fast int, _ int) []StrategyConfigV2 {
		return lo.Map(cg.slowRange, func(slow int, _ int) StrategyConfigV2 {
			return &SMAConfigV2{
				FastPeriod: fast,
				SlowPeriod: slow,
			}
		})
	})
	return configs
}

// ============================================================================
// Фабричная функция для создания SMA стратегии
// ============================================================================

func NewSMAStrategy(slippage float64) TradingStrategy {
	// Создаем компоненты
	slippageProvider := NewSlippageProvider(slippage)
	
	signalGenerator := NewSMASignalGenerator()
	
	configManager := NewConfigManager(
		&SMAConfigV2{FastPeriod: 12, SlowPeriod: 26},
		func() StrategyConfigV2 { return &SMAConfigV2{} },
	)
	
	configGenerator := NewSMAConfigGenerator(
		lo.RangeWithSteps(5, 50, 5),   // fast: 5, 10, 15, ..., 45
		lo.RangeWithSteps(50, 200, 10), // slow: 50, 60, 70, ..., 190
	)
	
	optimizer := NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	// Собираем стратегию через композицию
	return NewStrategyBase(
		"sma_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

// ============================================================================
// ПРЕИМУЩЕСТВА НОВОЙ АРХИТЕКТУРЫ:
// ============================================================================

/*
1. SINGLE RESPONSIBILITY:
   - SMASignalGenerator отвечает ТОЛЬКО за генерацию сигналов
   - ConfigManager отвечает ТОЛЬКО за управление конфигурацией
   - GridSearchOptimizer отвечает ТОЛЬКО за оптимизацию
   - SlippageProvider отвечает ТОЛЬКО за проскальзывание

2. OPEN/CLOSED:
   - Можно добавить новый оптимизатор (например, GeneticOptimizer) без изменения существующего кода
   - Можно добавить новый генератор сигналов без изменения оптимизатора

3. DEPENDENCY INVERSION:
   - Все зависят от интерфейсов (SignalGenerator, ConfigOptimizer), а не от конкретных реализаций
   - Легко подменить реализацию для тестирования (mock objects)

4. INTERFACE SEGREGATION:
   - Маленькие, специализированные интерфейсы вместо одного большого
   - Каждый компонент реализует только то, что ему нужно

5. LISKOV SUBSTITUTION:
   - Любой SignalGenerator можно заменить другим без нарушения работы
   - Любой ConfigOptimizer можно заменить другим

6. ТЕСТИРУЕМОСТЬ:
   - Каждый компонент можно тестировать независимо
   - Легко создавать mock-объекты для unit-тестов

7. ПЕРЕИСПОЛЬЗОВАНИЕ:
   - GridSearchOptimizer можно использовать для ЛЮБОЙ стратегии
   - SlippageProvider можно использовать везде, где нужно проскальзывание

8. ГИБКОСТЬ:
   - Можно комбинировать разные генераторы сигналов с разными оптимизаторами
   - Можно добавлять новые компоненты (например, RiskManager) без изменения существующих
*/
