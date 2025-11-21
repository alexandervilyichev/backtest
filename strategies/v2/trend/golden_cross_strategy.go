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

// PredictNextSignal предсказывает ближайший сигнал Golden/Death Cross
func (sg *GoldenCrossSignalGenerator) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	gcConfig, ok := config.(*GoldenCrossConfig)
	if !ok {
		return nil
	}

	if err := gcConfig.Validate(); err != nil {
		return nil
	}

	if len(candles) < gcConfig.SlowPeriod*2 {
		return nil
	}

	// Извлекаем цены закрытия
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// Рассчитываем EMA
	fastEMA := internal.CalculateEMAForValues(prices, gcConfig.FastPeriod)
	slowEMA := internal.CalculateEMAForValues(prices, gcConfig.SlowPeriod)

	if fastEMA == nil || slowEMA == nil {
		return nil
	}

	currentIdx := len(candles) - 1
	currFast := fastEMA[currentIdx]
	currSlow := slowEMA[currentIdx]

	// Определяем текущее состояние
	isFastAbove := currFast > currSlow

	// Вычисляем скорость изменения EMA (производная)
	lookback := 5
	if currentIdx < lookback {
		lookback = currentIdx
	}

	if lookback < 2 {
		return nil
	}

	// Вычисляем средние скорости изменения
	fastVelocity := (fastEMA[currentIdx] - fastEMA[currentIdx-lookback]) / float64(lookback)
	slowVelocity := (slowEMA[currentIdx] - slowEMA[currentIdx-lookback]) / float64(lookback)

	// Относительная скорость сближения/расхождения
	relativeVelocity := fastVelocity - slowVelocity

	// Если линии расходятся, предсказание невозможно
	// Расхождение происходит когда:
	// 1. Fast выше Slow и растет быстрее (relativeVelocity > 0)
	// 2. Fast ниже Slow и падает быстрее (relativeVelocity < 0)
	// Сближение происходит когда:
	// 1. Fast выше Slow но растет медленнее или падает быстрее (relativeVelocity < 0)
	// 2. Fast ниже Slow но растет быстрее или падает медленнее (relativeVelocity > 0)
	
	if isFastAbove && relativeVelocity > 0 {
		// Fast выше и продолжает расти быстрее - расхождение вверх
		return nil
	}
	if !isFastAbove && relativeVelocity < 0 {
		// Fast ниже и продолжает падать быстрее - расхождение вниз
		return nil
	}

	// Текущее расстояние между линиями
	distance := currFast - currSlow

	// Если скорость сближения слишком мала, предсказание ненадежно
	if internal.Abs(relativeVelocity) < 0.0001 {
		return nil
	}

	// Предсказываем количество свечей до пересечения
	candlesUntilCross := internal.Abs(distance / relativeVelocity)

	// Ограничиваем горизонт предсказания
	maxHorizon := float64(gcConfig.SlowPeriod)
	if candlesUntilCross > maxHorizon {
		return nil
	}

	if candlesUntilCross < 1 {
		candlesUntilCross = 1
	}

	// Округляем до целого числа свечей
	predictedCandles := int(candlesUntilCross + 0.5)

	// Экстраполируем цену в точке пересечения
	// Используем линейную экстраполяцию текущей цены
	priceVelocity := (prices[currentIdx] - prices[currentIdx-lookback]) / float64(lookback)
	predictedPrice := prices[currentIdx] + priceVelocity*float64(predictedCandles)

	// Определяем тип сигнала
	var signalType internal.SignalType
	if isFastAbove {
		// Fast выше, но сближается - ожидается Death Cross (SELL)
		signalType = internal.SELL
	} else {
		// Fast ниже, но сближается - ожидается Golden Cross (BUY)
		signalType = internal.BUY
	}

	// Вычисляем уверенность на основе:
	// 1. Скорости сближения (чем быстрее, тем увереннее)
	// 2. Расстояния до пересечения (чем ближе, тем увереннее)
	// 3. Стабильности скорости

	// Базовая уверенность от скорости сближения
	velocityConfidence := internal.Min(internal.Abs(relativeVelocity)*1000, 0.5)

	// Бонус за близость пересечения
	distanceConfidence := 0.0
	if candlesUntilCross < float64(gcConfig.FastPeriod) {
		distanceConfidence = 0.3
	} else if candlesUntilCross < float64(gcConfig.SlowPeriod)/2 {
		distanceConfidence = 0.2
	} else {
		distanceConfidence = 0.1
	}

	// Проверяем стабильность скорости (сравниваем с более ранним периодом)
	stabilityBonus := 0.0
	if currentIdx >= lookback*2 {
		prevFastVelocity := (fastEMA[currentIdx-lookback] - fastEMA[currentIdx-lookback*2]) / float64(lookback)
		prevSlowVelocity := (slowEMA[currentIdx-lookback] - slowEMA[currentIdx-lookback*2]) / float64(lookback)
		prevRelativeVelocity := prevFastVelocity - prevSlowVelocity

		// Если скорости одного знака и близки по величине - стабильно
		if relativeVelocity*prevRelativeVelocity > 0 {
			ratio := internal.Abs(relativeVelocity / prevRelativeVelocity)
			if ratio > 0.5 && ratio < 2.0 {
				stabilityBonus = 0.2
			}
		}
	}

	confidence := velocityConfidence + distanceConfidence + stabilityBonus

	// Ограничиваем диапазон
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.1 {
		confidence = 0.1
	}

	// Вычисляем дату сигнала
	if len(candles) < 2 {
		return nil
	}

	timeInterval := (candles[len(candles)-1].ToTime().Unix() - candles[0].ToTime().Unix()) / int64(len(candles)-1)
	lastTimestamp := candles[len(candles)-1].ToTime().Unix()
	futureTimestamp := lastTimestamp + timeInterval*int64(predictedCandles)

	return &internal.FutureSignal{
		SignalType: signalType,
		Date:       futureTimestamp,
		Price:      predictedPrice,
		Confidence: confidence,
	}
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
