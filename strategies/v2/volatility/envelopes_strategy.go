// Envelopes Strategy V2
//
// Описание стратегии:
// Envelopes - канал вокруг скользящей средней с фиксированным процентом отклонения.
//
// Как работает:
// - Рассчитывается простая скользящая средняя (SMA) за заданный период
// - Верхняя полоса = SMA * (1 + процент)
// - Нижняя полоса = SMA * (1 - процент)
// - Покупка: когда цена закрывается выше верхней полосы (breakout above)
// - Продажа: когда цена закрывается ниже нижней полосы (breakout below)
//
// Параметры:
// - Period: период расчета SMA (обычно 20)
// - Percentage: процент отклонения (обычно 0.02 - 0.05)
//
// Предсказание:
// - Анализирует скорость движения цены относительно полос
// - Экстраполирует движение цены
// - Предсказывает момент пересечения с верхней/нижней полосой
// - Учитывает волатильность и импульс движения

package volatility

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
	"math"
)

type EnvelopesConfig struct {
	Period     int     `json:"period"`
	Percentage float64 `json:"percentage"`
}

func (c *EnvelopesConfig) Validate() error {
	if c.Period <= 0 {
		return errors.New("period must be positive")
	}
	if c.Percentage <= 0 || c.Percentage >= 0.5 {
		return errors.New("percentage must be between 0 and 0.5")
	}
	return nil
}

func (c *EnvelopesConfig) String() string {
	return fmt.Sprintf("Envelopes(period=%d, pct=%.3f)",
		c.Period, c.Percentage)
}

type EnvelopesSignalGenerator struct{}

func NewEnvelopesSignalGenerator() *EnvelopesSignalGenerator {
	return &EnvelopesSignalGenerator{}
}

// calculateEnvelopes вычисляет верхнюю, среднюю и нижнюю полосы Envelopes
func calculateEnvelopes(candles []internal.Candle, period int, percentage float64) (upper []float64, middle []float64, lower []float64) {
	middle = internal.CalculateSMACommon(candles, period)
	if middle == nil {
		return nil, nil, nil
	}

	length := len(candles)
	upper = make([]float64, length)
	lower = make([]float64, length)

	for i := 0; i < length; i++ {
		if middle[i] == 0 {
			upper[i] = 0
			lower[i] = 0
		} else {
			upper[i] = middle[i] * (1 + percentage)
			lower[i] = middle[i] * (1 - percentage)
		}
	}

	return upper, middle, lower
}

func (sg *EnvelopesSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	envConfig, ok := config.(*EnvelopesConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := envConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	upper, _, lower := calculateEnvelopes(candles, envConfig.Period, envConfig.Percentage)
	if upper == nil || lower == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := envConfig.Period; i < len(candles); i++ {
		currentPrice := candles[i].Close.ToFloat64()
		currentLower := lower[i]
		currentUpper := upper[i]

		// BUY: breakout above upper envelope
		if !inPosition && currentPrice > currentUpper {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		// SELL: breakout below lower envelope
		if inPosition && currentPrice < currentLower {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

// PredictNextSignal предсказывает ближайший сигнал в будущем
func (sg *EnvelopesSignalGenerator) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	envConfig, ok := config.(*EnvelopesConfig)
	if !ok {
		return nil
	}

	if err := envConfig.Validate(); err != nil {
		log.Printf("⚠️ Ошибка валидации конфигурации: %v", err)
		return nil
	}

	if len(candles) < envConfig.Period*2 {
		log.Printf("⚠️ Недостаточно данных для предсказания: получено %d свечей, требуется минимум %d", len(candles), envConfig.Period*2)
		return nil
	}

	// Вычисляем полосы
	upper, middle, lower := calculateEnvelopes(candles, envConfig.Period, envConfig.Percentage)
	if upper == nil || middle == nil || lower == nil {
		log.Printf("⚠️ Не удалось вычислить полосы Envelopes")
		return nil
	}

	currentIdx := len(candles) - 1
	currentPrice := candles[currentIdx].Close.ToFloat64()
	currentUpper := upper[currentIdx]
	currentLower := lower[currentIdx]
	currentMiddle := middle[currentIdx]

	// Анализируем последние несколько свечей для определения скорости движения
	lookback := 5
	if lookback > envConfig.Period/2 {
		lookback = envConfig.Period / 2
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

	// Вычисляем скорость изменения средней линии (тренд)
	middleVelocity := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		if middle[idx] != 0 && middle[prevIdx] != 0 {
			middleChange := middle[idx] - middle[prevIdx]
			middleVelocity += middleChange
		}
	}
	middleVelocity /= float64(lookback - 1)

	// Определяем расстояние до полос
	distanceToUpper := currentUpper - currentPrice
	distanceToLower := currentPrice - currentLower

	// Определяем, к какой полосе движется цена
	var targetSignal internal.SignalType
	var targetPrice float64
	var distanceToTarget float64
	var targetBand string

	if priceVelocity > 0 {
		// Цена движется вверх - ожидаем пробой верхней полосы (BUY сигнал)
		targetSignal = internal.BUY
		targetPrice = currentUpper
		distanceToTarget = distanceToUpper
		targetBand = "upper"
	} else if priceVelocity < 0 {
		// Цена движется вниз - ожидаем пробой нижней полосы (SELL сигнал)
		targetSignal = internal.SELL
		targetPrice = currentLower
		distanceToTarget = distanceToLower
		targetBand = "lower"
	} else {
		// Цена не движется - нет предсказания
		log.Printf("⚠️ Цена не имеет направленного движения")
		return nil
	}

	// Проверяем, достаточно ли сильное движение
	if math.Abs(priceVelocity) < 0.0001 {
		log.Printf("⚠️ Слишком слабое движение цены")
		return nil
	}

	// Вычисляем количество свечей до пересечения
	candlesToCrossing := int(math.Abs(distanceToTarget / priceVelocity))

	// Ограничиваем горизонт предсказания
	maxHorizon := envConfig.Period * 2
	if candlesToCrossing > maxHorizon {
		candlesToCrossing = maxHorizon
	}
	if candlesToCrossing < 1 {
		candlesToCrossing = 1
	}

	// Экстраполируем цену и полосы в будущее
	futureCandleIdx := candlesToCrossing
	futureMiddle := currentMiddle + middleVelocity*float64(futureCandleIdx)
	futureUpper := futureMiddle * (1 + envConfig.Percentage)
	futureLower := futureMiddle * (1 - envConfig.Percentage)

	// Корректируем предсказанную цену с учетом движения полос
	if targetBand == "upper" {
		targetPrice = futureUpper
	} else {
		targetPrice = futureLower
	}

	// Вычисляем уверенность в предсказании
	confidence := sg.calculateConfidence(
		priceVelocity,
		middleVelocity,
		distanceToTarget,
		currentPrice,
		currentMiddle,
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
func (sg *EnvelopesSignalGenerator) calculateConfidence(
	priceVelocity float64,
	middleVelocity float64,
	distanceToTarget float64,
	currentPrice float64,
	currentMiddle float64,
	candlesToCrossing int,
	maxHorizon int,
) float64 {
	confidence := 0.5 // Базовая уверенность

	// Фактор 1: Сила движения цены (чем сильнее, тем лучше)
	velocityStrength := math.Abs(priceVelocity) / currentPrice
	if velocityStrength > 0.01 {
		confidence += 0.20
	} else if velocityStrength > 0.005 {
		confidence += 0.15
	} else if velocityStrength > 0.002 {
		confidence += 0.10
	}

	// Фактор 2: Близость к цели (чем ближе, тем выше уверенность)
	distanceRatio := distanceToTarget / currentPrice
	if distanceRatio < 0.01 {
		confidence += 0.20
	} else if distanceRatio < 0.02 {
		confidence += 0.15
	} else if distanceRatio < 0.03 {
		confidence += 0.10
	} else if distanceRatio > 0.05 {
		confidence -= 0.10
	}

	// Фактор 3: Согласованность движения цены и средней
	if (priceVelocity > 0 && middleVelocity > 0) || (priceVelocity < 0 && middleVelocity < 0) {
		// Цена и средняя движутся в одном направлении - хорошо
		confidence += 0.10
	} else {
		// Расхождение - снижаем уверенность
		confidence -= 0.10
	}

	// Фактор 4: Горизонт предсказания (чем дальше, тем менее уверены)
	horizonRatio := float64(candlesToCrossing) / float64(maxHorizon)
	if horizonRatio < 0.25 {
		confidence += 0.10
	} else if horizonRatio > 0.75 {
		confidence -= 0.20
	}

	// Фактор 5: Положение цены относительно средней
	pricePosition := (currentPrice - currentMiddle) / currentMiddle
	if math.Abs(pricePosition) > 0.02 {
		// Цена далеко от средней - сильное отклонение
		confidence += 0.10
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

type EnvelopesConfigGenerator struct{}

func NewEnvelopesConfigGenerator() *EnvelopesConfigGenerator {
	return &EnvelopesConfigGenerator{}
}

func (g *EnvelopesConfigGenerator) Generate() []internal.StrategyConfigV2 {
	configs := []internal.StrategyConfigV2{}

	// Используем тот же диапазон, что и в V1 для честного сравнения
	for period := 5; period <= 90; period += 5 {
		for percentage := 0.01; percentage <= 0.04; percentage += 0.0005 {
			configs = append(configs, &EnvelopesConfig{
				Period:     period,
				Percentage: percentage,
			})
		}
	}

	return configs
}

type EnvelopesStrategy struct{}

func (s *EnvelopesStrategy) Name() string {
	return "envelopes"
}

func NewEnvelopesStrategyV2(slippage float64) internal.TradingStrategy {
	slippageProvider := internal.NewSlippageProvider(slippage)
	signalGenerator := NewEnvelopesSignalGenerator()

	configManager := internal.NewConfigManager(
		&EnvelopesConfig{
			Period:     20,
			Percentage: 0.02,
		},
		func() internal.StrategyConfigV2 {
			return &EnvelopesConfig{}
		},
	)

	configGenerator := NewEnvelopesConfigGenerator()
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	return internal.NewStrategyBase(
		"envelopes_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewEnvelopesStrategyV2(0.01)
	internal.RegisterStrategyV2(strategy)
}
