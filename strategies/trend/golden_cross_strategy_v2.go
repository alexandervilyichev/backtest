// strategies/trend/golden_cross_strategy_v2.go
// Пример миграции Golden Cross Strategy на новую архитектуру

package trend

import (
	"bt/internal"
	"errors"
	"fmt"

	"github.com/samber/lo"
)

// ============================================================================
// КОНФИГУРАЦИЯ (без изменений, только переименование интерфейса)
// ============================================================================

type GoldenCrossConfigV2 struct {
	FastPeriod int `json:"fast_period"`
	SlowPeriod int `json:"slow_period"`
}

func (c *GoldenCrossConfigV2) Validate() error {
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

func (c *GoldenCrossConfigV2) String() string {
	return fmt.Sprintf("GoldenCross(fast=%d, slow=%d)", c.FastPeriod, c.SlowPeriod)
}

// ============================================================================
// ГЕНЕРАТОР СИГНАЛОВ (выделен в отдельный компонент)
// ============================================================================

type GoldenCrossSignalGenerator struct{}

func NewGoldenCrossSignalGenerator() *GoldenCrossSignalGenerator {
	return &GoldenCrossSignalGenerator{}
}

// GenerateSignals - ТОЛЬКО генерация сигналов, никакой другой логики
func (sg *GoldenCrossSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	gcConfig, ok := config.(*GoldenCrossConfigV2)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := gcConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	// Извлекаем цены закрытия для расчета EMA
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// Рассчитываем экспоненциальные скользящие средние
	fastEMA := internal.CalculateEMAForValues(prices, gcConfig.FastPeriod)
	slowEMA := internal.CalculateEMAForValues(prices, gcConfig.SlowPeriod)

	if fastEMA == nil || slowEMA == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	startIndex := gcConfig.SlowPeriod - 1
	if gcConfig.FastPeriod > gcConfig.SlowPeriod {
		startIndex = gcConfig.FastPeriod - 1
	}

	for i := startIndex; i < len(candles); i++ {
		if i > startIndex {
			prevFast := fastEMA[i-1]
			prevSlow := slowEMA[i-1]
			currFast := fastEMA[i]
			currSlow := slowEMA[i]

			// Golden cross - BUY
			if !inPosition && prevFast <= prevSlow && currFast > currSlow {
				signals[i] = internal.BUY
				inPosition = true
				continue
			}

			// Death cross - SELL
			if inPosition && prevFast >= prevSlow && currFast < currSlow {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

// ============================================================================
// ГЕНЕРАТОР КОНФИГУРАЦИЙ (выделен в отдельный компонент)
// ============================================================================

type GoldenCrossConfigGenerator struct {
	fastMin, fastMax, fastStep int
	slowMin, slowMax, slowStep int
}

func NewGoldenCrossConfigGenerator(
	fastMin, fastMax, fastStep int,
	slowMin, slowMax, slowStep int,
) *GoldenCrossConfigGenerator {
	return &GoldenCrossConfigGenerator{
		fastMin: fastMin, fastMax: fastMax, fastStep: fastStep,
		slowMin: slowMin, slowMax: slowMax, slowStep: slowStep,
	}
}

func (cg *GoldenCrossConfigGenerator) Generate() []internal.StrategyConfigV2 {
	fastRange := lo.RangeWithSteps(cg.fastMin, cg.fastMax, cg.fastStep)
	slowRange := lo.RangeWithSteps(cg.slowMin, cg.slowMax, cg.slowStep)

	configs := lo.FlatMap(fastRange, func(fast int, _ int) []internal.StrategyConfigV2 {
		return lo.Map(slowRange, func(slow int, _ int) internal.StrategyConfigV2 {
			return &GoldenCrossConfigV2{
				FastPeriod: fast,
				SlowPeriod: slow,
			}
		})
	})

	return configs
}

// ============================================================================
// ФАБРИЧНАЯ ФУНКЦИЯ (создание стратегии через композицию)
// ============================================================================

func NewGoldenCrossStrategyV2(slippage float64) internal.TradingStrategy {
	// 1. Создаем провайдер проскальзывания
	slippageProvider := internal.NewSlippageProvider(slippage)

	// 2. Создаем генератор сигналов
	signalGenerator := NewGoldenCrossSignalGenerator()

	// 3. Создаем менеджер конфигурации
	configManager := internal.NewConfigManager(
		&GoldenCrossConfigV2{FastPeriod: 12, SlowPeriod: 26}, // default config
		func() internal.StrategyConfigV2 { return &GoldenCrossConfigV2{} }, // factory
	)

	// 4. Создаем генератор конфигураций для оптимизации
	configGenerator := NewGoldenCrossConfigGenerator(
		5, 240, 5,    // fast: от 5 до 240 с шагом 5
		100, 340, 15, // slow: от 100 до 340 с шагом 15
	)

	// 5. Создаем оптимизатор (переиспользуем универсальный GridSearchOptimizer!)
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	// 6. Собираем всё вместе через композицию
	return internal.NewStrategyBase(
		"golden_cross_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

// ============================================================================
// РЕГИСТРАЦИЯ СТРАТЕГИИ
// ============================================================================

func init() {
	strategy := NewGoldenCrossStrategyV2(0.01) // default slippage 0.01
	internal.RegisterStrategyV2(strategy)
}

// ============================================================================
// СРАВНЕНИЕ: ДО И ПОСЛЕ
// ============================================================================

/*
ДО (старая архитектура):
-----------------------
✗ 150+ строк кода
✗ Дублирование OptimizeWithConfig (~50 строк)
✗ Жесткая связь с BaseStrategy
✗ Невозможно протестировать генерацию сигналов отдельно от оптимизации
✗ Невозможно заменить алгоритм оптимизации без изменения кода стратегии
✗ Нарушение SRP: одна структура делает всё

ПОСЛЕ (новая архитектура):
-------------------------
✓ 100 строк кода (только уникальная логика)
✓ Нет дублирования (GridSearchOptimizer переиспользуется)
✓ Слабая связанность через интерфейсы
✓ Каждый компонент тестируется независимо
✓ Можно легко заменить оптимизатор: NewGeneticOptimizer(), NewBayesianOptimizer()
✓ Соблюдение SRP: каждый компонент отвечает за одну вещь

ПРЕИМУЩЕСТВА:
------------
1. Гибкость: можно комбинировать разные генераторы сигналов с разными оптимизаторами
2. Тестируемость: каждый компонент тестируется отдельно
3. Переиспользование: GridSearchOptimizer работает для ВСЕХ стратегий
4. Расширяемость: добавление нового оптимизатора не требует изменения стратегии
5. Читаемость: явные зависимости, понятная структура

ПРИМЕР ИСПОЛЬЗОВАНИЯ:
--------------------
// Создание стратегии с дефолтными параметрами
strategy := NewGoldenCrossStrategyV2(0.01)

// Генерация сигналов
config := strategy.DefaultConfig()
signals := strategy.GenerateSignals(candles, config)

// Оптимизация
bestConfig := strategy.Optimize(candles, strategy)

// Легко заменить оптимизатор
customOptimizer := NewGeneticOptimizer(...)
customStrategy := internal.NewStrategyBase(
    "golden_cross_genetic",
    NewGoldenCrossSignalGenerator(),
    configManager,
    customOptimizer,  // <-- другой оптимизатор!
    slippageProvider,
)
*/
