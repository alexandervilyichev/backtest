// Awesome Oscillator Strategy V2
//
// Описание стратегии:
// Awesome Oscillator (AO) — индикатор, измеряющий изменение рыночной энергии.
// Он рассчитывается как разница между двумя простыми скользящими средними (SMA)
// медианной цены (High + Low) / 2:
//
//   AO(t) = SMA(MedianPrice, 5) - SMA(MedianPrice, 34)
//
// Торговые правила:
// - Покупка (BUY): AO пересекает ноль снизу вверх
// - Продажа (SELL): AO пересекает ноль сверху вниз
//
// Предсказание:
// - Анализирует скорость изменения AO (производная)
// - Экстраполирует движение AO
// - Предсказывает момент пересечения с нулевой линией
// - Учитывает импульс и стабильность движения

package oscillators

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
	"math"
)

type AOConfig struct {
	FastPeriod          int  `json:"fast_period"`
	SlowPeriod          int  `json:"slow_period"`
	ConfirmByTwoCandles bool `json:"confirm_by_two_candles"`
}

func (c *AOConfig) Validate() error {
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

func (c *AOConfig) String() string {
	return fmt.Sprintf("AO(fast=%d, slow=%d, confirm=%t)",
		c.FastPeriod, c.SlowPeriod, c.ConfirmByTwoCandles)
}

type AOSignalGenerator struct{}

func NewAOSignalGenerator() *AOSignalGenerator {
	return &AOSignalGenerator{}
}

// calculateMedianPrice возвращает медианную цену для одной свечи: (High + Low) / 2
func calculateMedianPrice(c internal.Candle) float64 {
	h := c.High.ToFloat64()
	l := c.Low.ToFloat64()
	return (h + l) / 2.0
}

// calculateAO вычисляет значения Awesome Oscillator для массива свечей
func calculateAO(candles []internal.Candle, fastPeriod, slowPeriod int) []float64 {
	if len(candles) < slowPeriod {
		return nil
	}

	ao := make([]float64, len(candles))

	// Вычисляем медианные цены
	medians := make([]float64, len(candles))
	for i := range candles {
		medians[i] = calculateMedianPrice(candles[i])
	}

	// Вычисляем SMA для быстрого и медленного периода
	smaFast := make([]float64, len(candles))
	smaSlow := make([]float64, len(candles))

	// Расчет SMA (простое скользящее среднее)
	for i := 0; i < len(candles); i++ {
		if i < fastPeriod-1 {
			smaFast[i] = 0
		} else {
			var sum float64
			for j := i - fastPeriod + 1; j <= i; j++ {
				sum += medians[j]
			}
			smaFast[i] = sum / float64(fastPeriod)
		}

		if i < slowPeriod-1 {
			smaSlow[i] = 0
		} else {
			var sum float64
			for j := i - slowPeriod + 1; j <= i; j++ {
				sum += medians[j]
			}
			smaSlow[i] = sum / float64(slowPeriod)
		}
	}

	// AO = SMA_fast - SMA_slow
	for i := 0; i < len(candles); i++ {
		if smaFast[i] == 0 || smaSlow[i] == 0 {
			ao[i] = 0
		} else {
			ao[i] = smaFast[i] - smaSlow[i]
		}
	}

	return ao
}

func (sg *AOSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	aoConfig, ok := config.(*AOConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := aoConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	aoValues := calculateAO(candles, aoConfig.FastPeriod, aoConfig.SlowPeriod)
	if aoValues == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	// Начинаем с slowPeriod, так как до этого AO не определён
	for i := aoConfig.SlowPeriod; i < len(candles); i++ {
		prevAo := aoValues[i-1]
		currAo := aoValues[i]

		// Проверка на пересечение нуля
		isBuySignal := prevAo < 0 && currAo >= 0
		isSellSignal := prevAo > 0 && currAo <= 0

		// Подтверждение двумя свечами (опционально)
		confirmCondition := true
		if aoConfig.ConfirmByTwoCandles && i >= 2 {
			medPrev2 := calculateMedianPrice(candles[i-2])
			medPrev1 := calculateMedianPrice(candles[i-1])
			medCurr := calculateMedianPrice(candles[i])

			if isBuySignal {
				// Две подряд растущие медианные цены
				confirmCondition = medPrev1 > medPrev2 && medCurr > medPrev1
			} else if isSellSignal {
				// Две подряд падающие медианные цены
				confirmCondition = medPrev1 < medPrev2 && medCurr < medPrev1
			}
		}

		// Генерация сигнала
		if isBuySignal && confirmCondition && !inPosition {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		if isSellSignal && confirmCondition && inPosition {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

// PredictNextSignal предсказывает ближайший сигнал AO
func (sg *AOSignalGenerator) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	aoConfig, ok := config.(*AOConfig)
	if !ok {
		return nil
	}

	if err := aoConfig.Validate(); err != nil {
		log.Printf("⚠️ Ошибка валидации конфигурации: %v", err)
		return nil
	}

	if len(candles) < aoConfig.SlowPeriod*2 {
		log.Printf("⚠️ Недостаточно данных для предсказания: получено %d свечей, требуется минимум %d", len(candles), aoConfig.SlowPeriod*2)
		return nil
	}

	// Вычисляем AO
	aoValues := calculateAO(candles, aoConfig.FastPeriod, aoConfig.SlowPeriod)
	if aoValues == nil {
		log.Printf("⚠️ Не удалось вычислить AO")
		return nil
	}

	currentIdx := len(candles) - 1
	currentAO := aoValues[currentIdx]

	// Анализируем последние несколько значений для определения скорости изменения
	lookback := 5
	if lookback > aoConfig.FastPeriod {
		lookback = aoConfig.FastPeriod
	}
	if lookback < 3 {
		lookback = 3
	}

	if currentIdx < aoConfig.SlowPeriod+lookback {
		log.Printf("⚠️ Недостаточно данных для анализа скорости")
		return nil
	}

	// Вычисляем среднюю скорость изменения AO (производная)
	aoVelocity := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		aoChange := aoValues[idx] - aoValues[prevIdx]
		aoVelocity += aoChange
	}
	aoVelocity /= float64(lookback - 1)

	// Определяем текущее положение относительно нуля
	isAboveZero := currentAO > 0

	// Определяем ожидаемый сигнал
	var targetSignal internal.SignalType
	var distanceToZero float64

	if isAboveZero {
		// AO выше нуля - ожидаем SELL сигнал (пересечение вниз)
		targetSignal = internal.SELL
		distanceToZero = currentAO

		// Если AO растет, пересечения не будет
		if aoVelocity >= 0 {
			log.Printf("⚠️ AO выше нуля и растет, пересечение маловероятно")
			return nil
		}
	} else {
		// AO ниже нуля - ожидаем BUY сигнал (пересечение вверх)
		targetSignal = internal.BUY
		distanceToZero = -currentAO

		// Если AO падает, пересечения не будет
		if aoVelocity <= 0 {
			log.Printf("⚠️ AO ниже нуля и падает, пересечение маловероятно")
			return nil
		}
	}

	// Проверяем, достаточно ли сильное движение
	if math.Abs(aoVelocity) < 0.00001 {
		log.Printf("⚠️ Слишком слабое движение AO")
		return nil
	}

	// Вычисляем количество свечей до пересечения нуля
	candlesToCrossing := int(math.Abs(distanceToZero / aoVelocity))

	// Ограничиваем горизонт предсказания
	maxHorizon := aoConfig.SlowPeriod
	if candlesToCrossing > maxHorizon {
		candlesToCrossing = maxHorizon
	}
	if candlesToCrossing < 1 {
		candlesToCrossing = 1
	}

	// Вычисляем скорость изменения медианной цены
	medianVelocity := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		medianChange := calculateMedianPrice(candles[idx]) - calculateMedianPrice(candles[prevIdx])
		medianVelocity += medianChange
	}
	medianVelocity /= float64(lookback - 1)

	// Экстраполируем медианную цену в будущее
	currentMedian := calculateMedianPrice(candles[currentIdx])
	futureMedian := currentMedian + medianVelocity*float64(candlesToCrossing)

	// Предсказанная цена - используем медианную цену
	targetPrice := futureMedian

	// Вычисляем уверенность в предсказании
	confidence := sg.calculateConfidence(
		aoVelocity,
		distanceToZero,
		currentAO,
		medianVelocity,
		candlesToCrossing,
		maxHorizon,
		aoValues,
		currentIdx,
		lookback,
	)

	// Минимальный порог уверенности
	if confidence < 0.30 {
		log.Printf("⚠️ Недостаточная уверенность в предсказании (%.3f < 0.30)", confidence)
		return nil
	}

	// Вычисляем дату сигнала
	if len(candles) < 2 {
		return nil
	}

	timeInterval := (candles[len(candles)-1].ToTime().Unix() - candles[0].ToTime().Unix()) / int64(len(candles)-1)
	lastTimestamp := candles[len(candles)-1].ToTime().Unix()
	futureTimestamp := lastTimestamp + timeInterval*int64(candlesToCrossing)

	return &internal.FutureSignal{
		SignalType: targetSignal,
		Date:       futureTimestamp,
		Price:      targetPrice,
		Confidence: confidence,
	}
}

// calculateConfidence вычисляет уверенность в предсказании
func (sg *AOSignalGenerator) calculateConfidence(
	aoVelocity float64,
	distanceToZero float64,
	currentAO float64,
	medianVelocity float64,
	candlesToCrossing int,
	maxHorizon int,
	aoValues []float64,
	currentIdx int,
	lookback int,
) float64 {
	confidence := 0.5 // Базовая уверенность

	// Фактор 1: Сила движения AO (чем сильнее, тем лучше)
	velocityStrength := math.Abs(aoVelocity)
	if velocityStrength > 0.01 {
		confidence += 0.20
	} else if velocityStrength > 0.005 {
		confidence += 0.15
	} else if velocityStrength > 0.002 {
		confidence += 0.10
	}

	// Фактор 2: Близость к нулевой линии (чем ближе, тем выше уверенность)
	if math.Abs(currentAO) < 0.05 {
		confidence += 0.20
	} else if math.Abs(currentAO) < 0.10 {
		confidence += 0.15
	} else if math.Abs(currentAO) < 0.20 {
		confidence += 0.10
	} else if math.Abs(currentAO) > 0.50 {
		confidence -= 0.10
	}

	// Фактор 3: Стабильность скорости изменения AO
	// Проверяем, насколько стабильна скорость на более раннем периоде
	if currentIdx >= lookback*2 {
		prevVelocity := 0.0
		for i := lookback; i < lookback*2-1; i++ {
			idx := currentIdx - i
			prevIdx := idx - 1
			aoChange := aoValues[idx] - aoValues[prevIdx]
			prevVelocity += aoChange
		}
		prevVelocity /= float64(lookback - 1)

		// Если скорости одного знака и близки по величине - стабильно
		if aoVelocity*prevVelocity > 0 {
			ratio := math.Abs(aoVelocity / prevVelocity)
			if ratio > 0.5 && ratio < 2.0 {
				confidence += 0.15
			} else if ratio > 0.3 && ratio < 3.0 {
				confidence += 0.10
			}
		}
	}

	// Фактор 4: Горизонт предсказания (чем дальше, тем менее уверены)
	horizonRatio := float64(candlesToCrossing) / float64(maxHorizon)
	if horizonRatio < 0.25 {
		confidence += 0.10
	} else if horizonRatio > 0.75 {
		confidence -= 0.20
	}

	// Фактор 5: Согласованность движения AO и медианной цены
	// Если AO растет и медианная цена растет - хорошо
	// Если AO падает и медианная цена падает - хорошо
	if (aoVelocity > 0 && medianVelocity > 0) || (aoVelocity < 0 && medianVelocity < 0) {
		confidence += 0.10
	} else {
		confidence -= 0.05
	}

	// Ограничиваем диапазон
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0 {
		confidence = 0
	}

	return confidence
}

type AOConfigGenerator struct{}

func NewAOConfigGenerator() *AOConfigGenerator {
	return &AOConfigGenerator{}
}

func (g *AOConfigGenerator) Generate() []internal.StrategyConfigV2 {
	configs := []internal.StrategyConfigV2{}

	// Перебираем параметры
	fastOptions := []int{3, 5, 7}
	slowOptions := []int{21, 34, 55}
	confirmOptions := []bool{false, true}

	for _, fast := range fastOptions {
		for _, slow := range slowOptions {
			// Исключаем некорректные пары
			if fast >= slow {
				continue
			}
			for _, confirm := range confirmOptions {
				configs = append(configs, &AOConfig{
					FastPeriod:          fast,
					SlowPeriod:          slow,
					ConfirmByTwoCandles: confirm,
				})
			}
		}
	}

	return configs
}

func NewAOStrategyV2(slippage float64) internal.TradingStrategy {
	slippageProvider := internal.NewSlippageProvider(slippage)
	signalGenerator := NewAOSignalGenerator()

	configManager := internal.NewConfigManager(
		&AOConfig{
			FastPeriod:          5,
			SlowPeriod:          34,
			ConfirmByTwoCandles: false,
		},
		func() internal.StrategyConfigV2 {
			return &AOConfig{}
		},
	)

	configGenerator := NewAOConfigGenerator()
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	return internal.NewStrategyBase(
		"awesome_oscillator_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewAOStrategyV2(0.01)
	internal.RegisterStrategyV2(strategy)
}
