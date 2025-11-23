// SuperTrend Strategy V2
//
// Описание стратегии:
// SuperTrend - трендовый индикатор, сочетающий в себе средние цены и волатильность (ATR).
// Показывает направление тренда и уровни поддержки/сопротивления.
//
// Как работает:
// - Рассчитывается ATR (Average True Range) за заданный период
// - Вычисляется базовая линия как среднее между максимумом и минимумом
// - Верхняя линия = базовая линия + (множитель × ATR)
// - Нижняя линия = базовая линия - (множитель × ATR)
// - SuperTrend следует за трендом: зеленая линия выше цены - бычий тренд, красная ниже - медвежий
// - Покупка: когда цена закрытия пересекает SuperTrend снизу вверх
// - Продажа: когда цена закрытия пересекает SuperTrend сверху вниз
//
// Параметры:
// - Period: период расчета ATR (обычно 10-14)
// - Multiplier: множитель для ATR (обычно 2.0-3.0)
//
// Предсказание:
// - Анализирует скорость движения цены и линии SuperTrend
// - Экстраполирует движение цены и ATR
// - Предсказывает момент пересечения цены с линией SuperTrend
// - Учитывает волатильность и направление тренда

package trend

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
	"math"
)

type SupertrendConfig struct {
	Period     int     `json:"period"`
	Multiplier float64 `json:"multiplier"`
}

func (c *SupertrendConfig) Validate() error {
	if c.Period <= 0 {
		return errors.New("period must be positive")
	}
	if c.Multiplier <= 0 {
		return errors.New("multiplier must be positive")
	}
	return nil
}

func (c *SupertrendConfig) String() string {
	return fmt.Sprintf("Supertrend(period=%d, mult=%.2f)",
		c.Period, c.Multiplier)
}

type SupertrendSignalGenerator struct{}

func NewSupertrendSignalGenerator() *SupertrendSignalGenerator {
	return &SupertrendSignalGenerator{}
}

// calculateATR рассчитывает Average True Range
func calculateATR(candles []internal.Candle, period int) []float64 {
	if len(candles) < period+1 {
		return nil
	}

	atr := make([]float64, len(candles))

	// Первые period+1 значений рассчитываем простым средним
	for i := 1; i < len(candles); i++ {
		if i < period+1 {
			high := candles[i].High.ToFloat64()
			low := candles[i].Low.ToFloat64()
			prevClose := candles[i-1].Close.ToFloat64()

			tr := math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
			atr[i] = tr
		} else {
			break
		}
	}

	// Рассчитываем первое ATR как простое среднее
	sum := 0.0
	for i := 1; i < period+1 && i < len(candles); i++ {
		sum += atr[i]
	}
	atr[period] = sum / float64(period)

	// Рассчитываем остальные ATR с использованием smoothing
	for i := period + 1; i < len(candles); i++ {
		high := candles[i].High.ToFloat64()
		low := candles[i].Low.ToFloat64()
		prevClose := candles[i-1].Close.ToFloat64()

		tr := math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
		atr[i] = (atr[i-1]*float64(period-1) + tr) / float64(period)
	}

	return atr
}

// calculateSuperTrend рассчитывает значения SuperTrend
func calculateSuperTrend(candles []internal.Candle, period int, multiplier float64) ([]float64, []bool) {
	atr := calculateATR(candles, period)
	if atr == nil {
		return nil, nil
	}

	superTrend := make([]float64, len(candles))
	upTrend := make([]bool, len(candles)) // true для восходящего тренда, false для нисходящего

	// Инициализируем первые значения
	if len(candles) > period {
		// Базовая линия для первого значения
		hl2 := (candles[period].High.ToFloat64() + candles[period].Low.ToFloat64()) / 2
		superTrend[period] = hl2 + multiplier*atr[period]
		upTrend[period] = true // начинаем с восходящего тренда

		// Рассчитываем остальные значения
		for i := period + 1; i < len(candles); i++ {
			hl2 := (candles[i].High.ToFloat64() + candles[i].Low.ToFloat64()) / 2

			// Верхняя линия
			upperBand := hl2 + multiplier*atr[i]
			// Нижняя линия
			lowerBand := hl2 - multiplier*atr[i]

			// Определяем тренд на основе предыдущего значения
			prevTrend := upTrend[i-1]
			prevSuperTrend := superTrend[i-1]

			var currentSuperTrend float64
			var currentTrend bool

			if prevTrend {
				// Предыдущий тренд восходящий
				if candles[i].Close.ToFloat64() <= prevSuperTrend {
					// Тренд меняется на нисходящий
					currentTrend = false
					currentSuperTrend = upperBand
				} else {
					// Тренд остается восходящим
					currentTrend = true
					currentSuperTrend = math.Max(lowerBand, prevSuperTrend)
				}
			} else {
				// Предыдущий тренд нисходящий
				if candles[i].Close.ToFloat64() >= prevSuperTrend {
					// Тренд меняется на восходящий
					currentTrend = true
					currentSuperTrend = lowerBand
				} else {
					// Тренд остается нисходящим
					currentTrend = false
					currentSuperTrend = math.Min(upperBand, prevSuperTrend)
				}
			}

			superTrend[i] = currentSuperTrend
			upTrend[i] = currentTrend
		}
	}

	return superTrend, upTrend
}

func (sg *SupertrendSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	stConfig, ok := config.(*SupertrendConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := stConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	superTrend, upTrend := calculateSuperTrend(candles, stConfig.Period, stConfig.Multiplier)
	if superTrend == nil || upTrend == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := stConfig.Period + 1; i < len(candles); i++ {
		currentPrice := candles[i].Close.ToFloat64()
		currentSuperTrend := superTrend[i]
		currentTrend := upTrend[i]

		prevPrice := candles[i-1].Close.ToFloat64()
		prevSuperTrend := superTrend[i-1]
		prevTrend := upTrend[i-1]

		// BUY сигнал: цена пересекает SuperTrend снизу вверх
		// Это происходит когда тренд меняется с нисходящего на восходящий
		if !inPosition && !prevTrend && currentTrend && prevPrice <= prevSuperTrend && currentPrice > currentSuperTrend {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		// SELL сигнал: цена пересекает SuperTrend сверху вниз
		// Это происходит когда тренд меняется с восходящего на нисходящий
		if inPosition && prevTrend && !currentTrend && prevPrice >= prevSuperTrend && currentPrice < currentSuperTrend {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

// PredictNextSignal предсказывает ближайший сигнал SuperTrend
func (sg *SupertrendSignalGenerator) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	stConfig, ok := config.(*SupertrendConfig)
	if !ok {
		return nil
	}

	if err := stConfig.Validate(); err != nil {
		log.Printf("⚠️ Ошибка валидации конфигурации: %v", err)
		return nil
	}

	if len(candles) < stConfig.Period*2 {
		log.Printf("⚠️ Недостаточно данных для предсказания: получено %d свечей, требуется минимум %d", len(candles), stConfig.Period*2)
		return nil
	}

	// Вычисляем SuperTrend и ATR
	superTrend, upTrend := calculateSuperTrend(candles, stConfig.Period, stConfig.Multiplier)
	atr := calculateATR(candles, stConfig.Period)

	if superTrend == nil || upTrend == nil || atr == nil {
		log.Printf("⚠️ Не удалось вычислить SuperTrend")
		return nil
	}

	currentIdx := len(candles) - 1
	currentPrice := candles[currentIdx].Close.ToFloat64()
	currentSuperTrend := superTrend[currentIdx]
	currentTrend := upTrend[currentIdx]
	currentATR := atr[currentIdx]

	// Анализируем последние несколько свечей для определения скорости движения
	lookback := 5
	if lookback > stConfig.Period/2 {
		lookback = stConfig.Period / 2
	}
	if lookback < 3 {
		lookback = 3
	}

	if currentIdx < lookback {
		log.Printf("⚠️ Недостаточно данных для анализа скорости")
		return nil
	}

	// Вычисляем среднюю скорость изменения цены
	priceVelocity := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		priceChange := candles[idx].Close.ToFloat64() - candles[prevIdx].Close.ToFloat64()
		priceVelocity += priceChange
	}
	priceVelocity /= float64(lookback - 1)

	// Вычисляем скорость изменения SuperTrend
	supertrendVelocity := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		if superTrend[idx] != 0 && superTrend[prevIdx] != 0 {
			stChange := superTrend[idx] - superTrend[prevIdx]
			supertrendVelocity += stChange
		}
	}
	supertrendVelocity /= float64(lookback - 1)

	// Вычисляем скорость изменения ATR (волатильность)
	atrVelocity := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		if atr[idx] != 0 && atr[prevIdx] != 0 {
			atrChange := atr[idx] - atr[prevIdx]
			atrVelocity += atrChange
		}
	}
	atrVelocity /= float64(lookback - 1)

	// Определяем расстояние до линии SuperTrend
	distanceToSuperTrend := currentPrice - currentSuperTrend

	// Определяем ожидаемый сигнал на основе текущего тренда
	var targetSignal internal.SignalType
	var targetPrice float64
	var relativeVelocity float64

	if currentTrend {
		// Восходящий тренд - ожидаем SELL сигнал (пересечение вниз)
		targetSignal = internal.SELL
		// Цена должна пересечь SuperTrend сверху вниз
		relativeVelocity = priceVelocity - supertrendVelocity

		// Если цена движется вверх быстрее чем SuperTrend, пересечения не будет
		if relativeVelocity >= 0 {
			log.Printf("⚠️ Цена движется вверх быстрее SuperTrend, пересечение маловероятно")
			return nil
		}
	} else {
		// Нисходящий тренд - ожидаем BUY сигнал (пересечение вверх)
		targetSignal = internal.BUY
		// Цена должна пересечь SuperTrend снизу вверх
		relativeVelocity = priceVelocity - supertrendVelocity

		// Если цена движется вниз быстрее чем SuperTrend, пересечения не будет
		if relativeVelocity <= 0 {
			log.Printf("⚠️ Цена движется вниз быстрее SuperTrend, пересечение маловероятно")
			return nil
		}
	}

	// Проверяем, достаточно ли сильное движение
	if math.Abs(relativeVelocity) < 0.0001 {
		log.Printf("⚠️ Слишком слабое относительное движение")
		return nil
	}

	// Вычисляем количество свечей до пересечения
	candlesToCrossing := int(math.Abs(distanceToSuperTrend / relativeVelocity))

	// Ограничиваем горизонт предсказания
	maxHorizon := stConfig.Period * 2
	if candlesToCrossing > maxHorizon {
		candlesToCrossing = maxHorizon
	}
	if candlesToCrossing < 1 {
		candlesToCrossing = 1
	}

	// Экстраполируем цену и SuperTrend в будущее
	futureCandleIdx := candlesToCrossing
	futurePrice := currentPrice + priceVelocity*float64(futureCandleIdx)
	futureSuperTrend := currentSuperTrend + supertrendVelocity*float64(futureCandleIdx)
	futureATR := currentATR + atrVelocity*float64(futureCandleIdx)

	// Корректируем предсказанную цену - она должна быть около линии SuperTrend
	targetPrice = (futurePrice + futureSuperTrend) / 2

	// Вычисляем уверенность в предсказании
	confidence := sg.calculateConfidence(
		priceVelocity,
		supertrendVelocity,
		relativeVelocity,
		distanceToSuperTrend,
		currentPrice,
		currentATR,
		futureATR,
		candlesToCrossing,
		maxHorizon,
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
func (sg *SupertrendSignalGenerator) calculateConfidence(
	priceVelocity float64,
	supertrendVelocity float64,
	relativeVelocity float64,
	distanceToSuperTrend float64,
	currentPrice float64,
	currentATR float64,
	futureATR float64,
	candlesToCrossing int,
	maxHorizon int,
) float64 {
	confidence := 0.5 // Базовая уверенность

	// Фактор 1: Сила относительного движения (чем сильнее, тем лучше)
	velocityStrength := math.Abs(relativeVelocity) / currentPrice
	if velocityStrength > 0.01 {
		confidence += 0.20
	} else if velocityStrength > 0.005 {
		confidence += 0.15
	} else if velocityStrength > 0.002 {
		confidence += 0.10
	}

	// Фактор 2: Близость к линии SuperTrend (чем ближе, тем выше уверенность)
	distanceRatio := math.Abs(distanceToSuperTrend) / currentPrice
	if distanceRatio < 0.01 {
		confidence += 0.20
	} else if distanceRatio < 0.02 {
		confidence += 0.15
	} else if distanceRatio < 0.03 {
		confidence += 0.10
	} else if distanceRatio > 0.05 {
		confidence -= 0.10
	}

	// Фактор 3: Стабильность волатильности (ATR)
	atrChangeRatio := math.Abs(futureATR-currentATR) / currentATR
	if atrChangeRatio < 0.1 {
		// Волатильность стабильна - хорошо
		confidence += 0.10
	} else if atrChangeRatio > 0.3 {
		// Волатильность сильно меняется - снижаем уверенность
		confidence -= 0.15
	}

	// Фактор 4: Горизонт предсказания (чем дальше, тем менее уверены)
	horizonRatio := float64(candlesToCrossing) / float64(maxHorizon)
	if horizonRatio < 0.25 {
		confidence += 0.10
	} else if horizonRatio > 0.75 {
		confidence -= 0.20
	}

	// Фактор 5: Соотношение расстояния к ATR
	// Если расстояние меньше ATR, пересечение более вероятно
	distanceToATRRatio := math.Abs(distanceToSuperTrend) / currentATR
	if distanceToATRRatio < 1.0 {
		confidence += 0.15
	} else if distanceToATRRatio < 2.0 {
		confidence += 0.10
	} else if distanceToATRRatio > 3.0 {
		confidence -= 0.10
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

type SupertrendConfigGenerator struct{}

func NewSupertrendConfigGenerator() *SupertrendConfigGenerator {
	return &SupertrendConfigGenerator{}
}

func (g *SupertrendConfigGenerator) Generate() []internal.StrategyConfigV2 {
	configs := []internal.StrategyConfigV2{}

	// Grid search по параметрам
	for period := 7; period <= 20; period += 1 {
		for multiplier := 1.5; multiplier <= 4.0; multiplier += 0.25 {
			configs = append(configs, &SupertrendConfig{
				Period:     period,
				Multiplier: multiplier,
			})
		}
	}

	return configs
}

func NewSupertrendStrategyV2(slippage float64) internal.TradingStrategy {
	slippageProvider := internal.NewSlippageProvider(slippage)
	signalGenerator := NewSupertrendSignalGenerator()

	configManager := internal.NewConfigManager(
		&SupertrendConfig{
			Period:     10,
			Multiplier: 3.0,
		},
		func() internal.StrategyConfigV2 {
			return &SupertrendConfig{}
		},
	)

	configGenerator := NewSupertrendConfigGenerator()
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	return internal.NewStrategyBase(
		"supertrend_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewSupertrendStrategyV2(0.01)
	internal.RegisterStrategyV2(strategy)
}
