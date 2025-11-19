package trend

import (
	"bt/internal"
	"errors"
	"fmt"

	"github.com/samber/lo"
)

type GoldenCrossConfig struct {
	FastPeriod int `json:"fast_period"`
	SlowPeriod int `json:"slow_period"`
}

func (c *GoldenCrossConfig) Validate() error {
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

func (c *GoldenCrossConfig) String() string {
	return fmt.Sprintf("GoldenCross(fast=%d, slow=%d) ", c.FastPeriod, c.SlowPeriod)
}

type GoldenCrossSignalGenerator struct{}

func NewGoldenCrossSignalGenerator() *GoldenCrossSignalGenerator {
	return &GoldenCrossSignalGenerator{}
}

func (sg *GoldenCrossSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	gcConfig, ok := config.(*GoldenCrossConfig)
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

	configs := lo.CrossJoinBy2(
		fastRange,
		slowRange,
		func(fast int, slow int) internal.StrategyConfigV2 {
			return &GoldenCrossConfig{
				FastPeriod: fast,
				SlowPeriod: slow,
			}
		})

	return configs
}

func NewGoldenCrossStrategyV2(slippage float64) internal.TradingStrategy {
	// 1. Создаем провайдер проскальзывания
	slippageProvider := internal.NewSlippageProvider(slippage)

	// 2. Создаем генератор сигналов
	signalGenerator := NewGoldenCrossSignalGenerator()

	// 3. Создаем менеджер конфигурации
	configManager := internal.NewConfigManager(
		&GoldenCrossConfig{FastPeriod: 12, SlowPeriod: 26},               // default config
		func() internal.StrategyConfigV2 { return &GoldenCrossConfig{} }, // factory
	)

	// 4. Создаем генератор конфигураций для оптимизации
	configGenerator := NewGoldenCrossConfigGenerator(
		5, 240, 5, // fast: от 5 до 240 с шагом 5
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

func init() {
	strategy := NewGoldenCrossStrategyV2(0.01) // default slippage 0.01
	internal.RegisterStrategyV2(strategy)
}
