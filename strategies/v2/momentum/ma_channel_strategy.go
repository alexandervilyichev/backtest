package momentum

import (
	"bt/internal"
	"errors"
	"fmt"

	"github.com/samber/lo"
)

type MAChannelConfig struct {
	FastPeriod int     `json:"fast_period"`
	SlowPeriod int     `json:"slow_period"`
	Multiplier float64 `json:"multiplier"`
}

func (c *MAChannelConfig) Validate() error {
	if c.FastPeriod <= 0 {
		return errors.New("fast period must be positive")
	}
	if c.SlowPeriod <= 0 {
		return errors.New("slow period must be positive")
	}
	if c.Multiplier <= 0 {
		return errors.New("multiplier must be positive")
	}
	if c.FastPeriod >= c.SlowPeriod {
		return errors.New("fast period must be less than slow period")
	}
	return nil
}

func (c *MAChannelConfig) String() string {
	return fmt.Sprintf("MAChannel(fast=%d, slow=%d, mult=%.2f)", c.FastPeriod, c.SlowPeriod, c.Multiplier)
}

type MAChannelSignalGenerator struct{}

func NewMAChannelSignalGenerator() *MAChannelSignalGenerator {
	return &MAChannelSignalGenerator{}
}

// calculateChannels вычисляет верхний и нижний каналы на основе EMA
func (sg *MAChannelSignalGenerator) calculateChannels(prices []float64, fastPeriod, slowPeriod int, multiplier float64) ([]float64, []float64, []float64, []float64) {
	fastEMA := internal.CalculateEMAForValues(prices, fastPeriod)
	slowEMA := internal.CalculateEMAForValues(prices, slowPeriod)

	if fastEMA == nil || slowEMA == nil {
		return nil, nil, nil, nil
	}

	upperChannel := make([]float64, len(prices))
	lowerChannel := make([]float64, len(prices))

	for i := range prices {
		diff := internal.Abs(fastEMA[i] - slowEMA[i])
		upperChannel[i] = slowEMA[i] + diff*multiplier
		lowerChannel[i] = slowEMA[i] - diff*multiplier
	}

	return upperChannel, lowerChannel, fastEMA, slowEMA
}

// PredictNextSignal предсказывает ближайший пробой канала
func (sg *MAChannelSignalGenerator) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	maConfig, ok := config.(*MAChannelConfig)
	if !ok {
		return nil
	}

	if err := maConfig.Validate(); err != nil {
		return nil
	}

	if len(candles) < maConfig.SlowPeriod*2 {
		return nil
	}

	// Извлекаем цены
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// Рассчитываем каналы
	upperChannel, lowerChannel, fastEMA, slowEMA := sg.calculateChannels(prices, maConfig.FastPeriod, maConfig.SlowPeriod, maConfig.Multiplier)
	if upperChannel == nil {
		return nil
	}

	currentIdx := len(candles) - 1
	currentPrice := prices[currentIdx]
	currentUpper := upperChannel[currentIdx]
	currentLower := lowerChannel[currentIdx]

	// Вычисляем скорость изменения цены и каналов
	lookback := 5
	if currentIdx < lookback {
		lookback = currentIdx
	}
	if lookback < 2 {
		return nil
	}

	// Скорость изменения цены
	priceVelocity := (prices[currentIdx] - prices[currentIdx-lookback]) / float64(lookback)

	// Скорость изменения каналов
	upperVelocity := (upperChannel[currentIdx] - upperChannel[currentIdx-lookback]) / float64(lookback)
	lowerVelocity := (lowerChannel[currentIdx] - lowerChannel[currentIdx-lookback]) / float64(lookback)

	// Расстояния до границ канала
	distanceToUpper := currentUpper - currentPrice
	distanceToLower := currentPrice - currentLower

	// Относительные скорости сближения
	relativeUpperVelocity := priceVelocity - upperVelocity
	relativeLowerVelocity := priceVelocity - lowerVelocity

	var predictedSignal internal.SignalType
	var candlesUntilBreakout float64
	var targetPrice float64
	var confidence float64

	// Проверяем возможность пробоя верхнего канала (BUY)
	if relativeUpperVelocity > 0 && distanceToUpper > 0 {
		// Цена движется к верхнему каналу
		candlesUntilBreakout = distanceToUpper / relativeUpperVelocity

		if candlesUntilBreakout > 0 && candlesUntilBreakout <= float64(maConfig.SlowPeriod) {
			predictedSignal = internal.BUY
			targetPrice = currentPrice + priceVelocity*candlesUntilBreakout

			// Уверенность зависит от:
			// 1. Скорости движения к границе
			// 2. Близости к границе
			// 3. Силы тренда (расстояние между fast и slow EMA)

			velocityConfidence := internal.Min(internal.Abs(relativeUpperVelocity)*100, 0.4)
			distanceConfidence := internal.Max(0.1, 0.3*(1.0-distanceToUpper/currentUpper))

			// Сила тренда
			trendStrength := internal.Abs(fastEMA[currentIdx]-slowEMA[currentIdx]) / slowEMA[currentIdx]
			trendConfidence := internal.Min(trendStrength*10, 0.3)

			confidence = velocityConfidence + distanceConfidence + trendConfidence
		}
	}

	// Проверяем возможность пробоя нижнего канала (SELL)
	if relativeLowerVelocity < 0 && distanceToLower > 0 {
		// Цена движется к нижнему каналу
		candlesUntilBreakout = distanceToLower / internal.Abs(relativeLowerVelocity)

		if candlesUntilBreakout > 0 && candlesUntilBreakout <= float64(maConfig.SlowPeriod) {
			// Если уже есть предсказание BUY, выбираем ближайшее
			if predictedSignal == internal.BUY {
				existingBreakout := distanceToUpper / relativeUpperVelocity
				if candlesUntilBreakout < existingBreakout {
					predictedSignal = internal.SELL
					targetPrice = currentPrice + priceVelocity*candlesUntilBreakout
				} else {
					// Оставляем BUY
					return nil
				}
			} else {
				predictedSignal = internal.SELL
				targetPrice = currentPrice + priceVelocity*candlesUntilBreakout
			}

			velocityConfidence := internal.Min(internal.Abs(relativeLowerVelocity)*100, 0.4)
			distanceConfidence := internal.Max(0.1, 0.3*(1.0-distanceToLower/currentPrice))

			trendStrength := internal.Abs(fastEMA[currentIdx]-slowEMA[currentIdx]) / slowEMA[currentIdx]
			trendConfidence := internal.Min(trendStrength*10, 0.3)

			confidence = velocityConfidence + distanceConfidence + trendConfidence
		}
	}

	// Если не нашли подходящего сигнала
	if predictedSignal == internal.HOLD || candlesUntilBreakout <= 0 {
		return nil
	}

	// Ограничиваем уверенность
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.1 {
		confidence = 0.1
	}

	// Округляем до целого числа свечей
	predictedCandles := int(candlesUntilBreakout + 0.5)
	if predictedCandles < 1 {
		predictedCandles = 1
	}

	// Вычисляем временную метку
	if len(candles) < 2 {
		return nil
	}

	timeInterval := (candles[len(candles)-1].ToTime().Unix() - candles[0].ToTime().Unix()) / int64(len(candles)-1)
	lastTimestamp := candles[len(candles)-1].ToTime().Unix()
	futureTimestamp := lastTimestamp + timeInterval*int64(predictedCandles)

	return &internal.FutureSignal{
		SignalType: predictedSignal,
		Date:       futureTimestamp,
		Price:      targetPrice,
		Confidence: confidence,
	}
}

func (sg *MAChannelSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	maConfig, ok := config.(*MAChannelConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := maConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	// Извлекаем цены
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// Рассчитываем каналы
	upperChannel, lowerChannel, fastEMA, slowEMA := sg.calculateChannels(prices, maConfig.FastPeriod, maConfig.SlowPeriod, maConfig.Multiplier)
	if upperChannel == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	startIndex := maConfig.SlowPeriod - 1

	for i := startIndex; i < len(candles); i++ {
		closePrice := prices[i]
		upper := upperChannel[i]
		lower := lowerChannel[i]

		if !inPosition {
			// Покупка при отскоке от нижнего канала (цена была ниже, вернулась внутрь)
			// Это означает, что цена нашла поддержку и может начать расти
			if i > startIndex {
				prevPrice := prices[i-1]
				// Цена была ниже нижнего канала и вернулась внутрь + fast EMA выше slow EMA (восходящий тренд)
				if prevPrice < lowerChannel[i-1] && closePrice >= lower && fastEMA[i] > slowEMA[i] {
					signals[i] = internal.BUY
					inPosition = true
					continue
				}
			}
		} else {
			// Продажа при отскоке от верхнего канала (цена была выше, вернулась внутрь)
			// Или при пробое нижнего канала (стоп-лосс)
			if i > startIndex {
				prevPrice := prices[i-1]
				// Цена была выше верхнего канала и вернулась внутрь + fast EMA ниже slow EMA (нисходящий тренд)
				if (prevPrice > upperChannel[i-1] && closePrice <= upper && fastEMA[i] < slowEMA[i]) || closePrice < lower {
					signals[i] = internal.SELL
					inPosition = false
					continue
				}
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

type MAChannelConfigGenerator struct {
	fastMin, fastMax, fastStep int
	slowMin, slowMax, slowStep int
	multMin, multMax, multStep float64
}

func NewMAChannelConfigGenerator(
	fastMin, fastMax, fastStep int,
	slowMin, slowMax, slowStep int,
	multMin, multMax, multStep float64,
) *MAChannelConfigGenerator {
	return &MAChannelConfigGenerator{
		fastMin: fastMin, fastMax: fastMax, fastStep: fastStep,
		slowMin: slowMin, slowMax: slowMax, slowStep: slowStep,
		multMin: multMin, multMax: multMax, multStep: multStep,
	}
}

func (cg *MAChannelConfigGenerator) Generate() []internal.StrategyConfigV2 {
	fastRange := lo.RangeWithSteps(cg.fastMin, cg.fastMax, cg.fastStep)
	slowRange := lo.RangeWithSteps(cg.slowMin, cg.slowMax, cg.slowStep)

	var multRange []float64
	for m := cg.multMin; m <= cg.multMax; m += cg.multStep {
		multRange = append(multRange, m)
	}

	var configs []internal.StrategyConfigV2
	for _, fast := range fastRange {
		for _, slow := range slowRange {
			for _, mult := range multRange {
				if fast < slow {
					configs = append(configs, &MAChannelConfig{
						FastPeriod: fast,
						SlowPeriod: slow,
						Multiplier: mult,
					})
				}
			}
		}
	}

	return configs
}

func NewMAChannelStrategyV2(slippage float64) internal.TradingStrategy {
	// 1. Создаем провайдер проскальзывания
	slippageProvider := internal.NewSlippageProvider(slippage)

	// 2. Создаем генератор сигналов
	signalGenerator := NewMAChannelSignalGenerator()

	// 3. Создаем менеджер конфигурации
	configManager := internal.NewConfigManager(
		&MAChannelConfig{FastPeriod: 10, SlowPeriod: 20, Multiplier: 1.0}, // default config
		func() internal.StrategyConfigV2 { return &MAChannelConfig{} },    // factory
	)

	// 4. Создаем генератор конфигураций для оптимизации
	configGenerator := NewMAChannelConfigGenerator(
		5, 20, 3, // fast: от 5 до 20 с шагом 3
		15, 50, 5, // slow: от 15 до 50 с шагом 5
		0.5, 2.5, 0.5, // multiplier: от 0.5 до 2.5 с шагом 0.5
	)

	// 5. Создаем оптимизатор
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	// 6. Собираем всё вместе через композицию
	return internal.NewStrategyBase(
		"ma_channel_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewMAChannelStrategyV2(0.01) // default slippage 0.01
	internal.RegisterStrategyV2(strategy)
}
