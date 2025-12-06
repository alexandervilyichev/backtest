// Volatility Breakout Strategy V2
//
// Описание стратегии:
// Стратегия выявляет прорывы на основе волатильности. Использует Average True Range (ATR)
// для измерения текущей волатильности и генерирует сигналы при резком увеличении
// волатильности с последующим движением цены в направлении прорыва.
//
// Как работает:
// - Рассчитывается ATR для измерения волатильности
// - Определяется базовый уровень волатильности (средний ATR за период)
// - Покупка: резкое увеличение волатильности + движение цены вверх
// - Продажа: резкое увеличение волатильности + движение цены вниз
// - Фильтруется по минимальному размеру движения для избежания шума
//
// Параметры:
// - AtrPeriod: период расчета ATR (обычно 14)
// - VolatilityMultiplier: множитель для определения повышенной волатильности (обычно 1.5-2.0)
// - MinMoveFilter: минимальный размер движения в процентах (обычно 0.5-2.0%)
//
// Предсказание:
// - Анализирует тренд волатильности и цены
// - Экстраполирует движение цены при продолжении повышенной волатильности
// - Предсказывает момент следующего breakout сигнала

package volatility

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
	"math"
)

type VolatilityBreakoutConfig struct {
	AtrPeriod            int     `json:"atr_period"`
	VolatilityMultiplier float64 `json:"volatility_multiplier"`
	MinMoveFilter        float64 `json:"min_move_filter"`
}

func (c *VolatilityBreakoutConfig) Validate() error {
	if c.AtrPeriod <= 0 {
		return errors.New("ATR period must be positive")
	}
	if c.VolatilityMultiplier <= 1.0 {
		return errors.New("volatility multiplier must be greater than 1.0")
	}
	if c.MinMoveFilter <= 0 {
		return errors.New("minimum move filter must be positive")
	}
	return nil
}

func (c *VolatilityBreakoutConfig) String() string {
	return fmt.Sprintf("VolatilityBreakout(atr_period=%d, vol_mult=%.2f, min_move=%.3f)",
		c.AtrPeriod, c.VolatilityMultiplier, c.MinMoveFilter)
}

type VolatilityBreakoutSignalGenerator struct{}

func NewVolatilityBreakoutSignalGenerator() *VolatilityBreakoutSignalGenerator {
	return &VolatilityBreakoutSignalGenerator{}
}

// calculateATR рассчитывает Average True Range
func calculateATR(candles []internal.Candle, period int) []float64 {
	if len(candles) < period+1 {
		return nil
	}

	tr := make([]float64, len(candles))

	// Вычисляем True Range для каждой свечи
	for i := 1; i < len(candles); i++ {
		high := candles[i].High.ToFloat64()
		low := candles[i].Low.ToFloat64()
		prevClose := candles[i-1].Close.ToFloat64()

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		tr[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// Вычисляем ATR как SMA от True Range
	atr := make([]float64, len(candles))
	sum := 0.0

	// Первое значение ATR
	for i := 1; i <= period; i++ {
		sum += tr[i]
	}
	atr[period] = sum / float64(period)

	// Остальные значения ATR
	for i := period + 1; i < len(candles); i++ {
		atr[i] = (atr[i-1]*float64(period-1) + tr[i]) / float64(period)
	}

	return atr
}

func (sg *VolatilityBreakoutSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	vbConfig, ok := config.(*VolatilityBreakoutConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := vbConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < vbConfig.AtrPeriod*2 {
		log.Printf("⚠️ Недостаточно данных для volatility breakout: получено %d свечей, требуется минимум %d", len(candles), vbConfig.AtrPeriod*2)
		return make([]internal.SignalType, len(candles))
	}

	// Рассчитываем ATR
	atr := calculateATR(candles, vbConfig.AtrPeriod)
	if atr == nil {
		log.Printf("❌ Не удалось рассчитать ATR")
		return make([]internal.SignalType, len(candles))
	}

	// Рассчитываем средний ATR за более длительный период для определения базового уровня
	basePeriod := vbConfig.AtrPeriod * 2
	if basePeriod > len(candles)/2 {
		basePeriod = len(candles) / 2
	}
	// Гарантируем минимальный размер базового периода
	if basePeriod < vbConfig.AtrPeriod {
		basePeriod = vbConfig.AtrPeriod
	}
	if basePeriod < 1 {
		basePeriod = 1
	}

	signals := make([]internal.SignalType, len(candles))
	// Инициализируем все сигналы как HOLD
	for i := range signals {
		signals[i] = internal.HOLD
	}
	inPosition := false

	for i := basePeriod; i < len(candles); i++ {
		currentATR := atr[i]
		currentPrice := candles[i].Close.ToFloat64()
		currentHigh := candles[i].High.ToFloat64()
		currentLow := candles[i].Low.ToFloat64()

		// Вычисляем средний ATR за базовый период
		sumATR := 0.0
		for j := i - basePeriod; j < i; j++ {
			sumATR += atr[j]
		}
		avgATR := sumATR / float64(basePeriod)

		// Проверяем повышенную волатильность
		highVolatility := currentATR > avgATR*vbConfig.VolatilityMultiplier

		if !highVolatility {
			signals[i] = internal.HOLD
			continue
		}

		// Определяем направление движения
		prevPrice := candles[i-1].Close.ToFloat64()
		// Защита от деления на ноль
		if prevPrice == 0 {
			signals[i] = internal.HOLD
			continue
		}
		priceChange := (currentPrice - prevPrice) / prevPrice

		// Проверяем минимальный размер движения
		minMove := vbConfig.MinMoveFilter
		if math.Abs(priceChange) < minMove {
			signals[i] = internal.HOLD
			continue
		}

		// BUY: повышенная волатильность + движение вверх + пробой максимума предыдущей свечи
		if !inPosition && priceChange > 0 {
			prevHigh := candles[i-1].High.ToFloat64()
			breakoutUp := currentHigh > prevHigh

			if breakoutUp {
				signals[i] = internal.BUY
				inPosition = true
				continue
			}
		}

		// SELL: повышенная волатильность + движение вниз + пробой минимума предыдущей свечи
		if inPosition && priceChange < 0 {
			prevLow := candles[i-1].Low.ToFloat64()
			breakoutDown := currentLow < prevLow

			if breakoutDown {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

// PredictNextSignal предсказывает ближайший сигнал Volatility Breakout
func (sg *VolatilityBreakoutSignalGenerator) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	vbConfig, ok := config.(*VolatilityBreakoutConfig)
	if !ok {
		return nil
	}

	if err := vbConfig.Validate(); err != nil {
		log.Printf("⚠️ Ошибка валидации конфигурации: %v", err)
		return nil
	}

	minCandles := vbConfig.AtrPeriod * 3
	if len(candles) < minCandles {
		log.Printf("⚠️ Недостаточно данных для предсказания: получено %d свечей, требуется минимум %d", len(candles), minCandles)
		return nil
	}

	// Рассчитываем ATR
	atr := calculateATR(candles, vbConfig.AtrPeriod)
	if atr == nil {
		log.Printf("⚠️ Не удалось рассчитать ATR")
		return nil
	}

	currentIdx := len(candles) - 1
	currentATR := atr[currentIdx]
	currentPrice := candles[currentIdx].Close.ToFloat64()

	// Вычисляем базовый уровень волатильности
	basePeriod := vbConfig.AtrPeriod * 2
	if basePeriod > len(candles)/2 {
		basePeriod = len(candles) / 2
	}
	// Гарантируем минимальный размер базового периода
	if basePeriod < vbConfig.AtrPeriod {
		basePeriod = vbConfig.AtrPeriod
	}
	if basePeriod < 1 {
		basePeriod = 1
	}

	if currentIdx < basePeriod {
		log.Printf("⚠️ Недостаточно данных для анализа базовой волатильности")
		return nil
	}

	sumATR := 0.0
	for j := currentIdx - basePeriod; j < currentIdx; j++ {
		sumATR += atr[j]
	}
	avgATR := sumATR / float64(basePeriod)
	// Защита от нулевого avgATR
	if avgATR <= 0 {
		log.Printf("⚠️ Средний ATR равен нулю или отрицательный")
		return nil
	}

	// Анализируем последние несколько свечей для определения тренда
	lookback := 5
	if vbConfig.AtrPeriod > 0 {
		if lookback > vbConfig.AtrPeriod/2 {
			lookback = vbConfig.AtrPeriod / 2
		}
	}
	if lookback < 3 {
		lookback = 3
	}
	// Гарантируем, что lookback не превышает доступные данные
	if lookback > currentIdx {
		lookback = currentIdx
	}
	if lookback < 2 || currentIdx < lookback {
		log.Printf("⚠️ Недостаточно данных для анализа тренда (lookback=%d, currentIdx=%d)", lookback, currentIdx)
		return nil
	}

	// Вычисляем скорость изменения ATR
	atrVelocity := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		atrChange := atr[idx] - atr[prevIdx]
		atrVelocity += atrChange
	}
	atrVelocity /= float64(lookback - 1)

	// Вычисляем скорость изменения цены
	priceVelocity := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		priceChange := candles[idx].Close.ToFloat64() - candles[prevIdx].Close.ToFloat64()
		priceVelocity += priceChange
	}
	priceVelocity /= float64(lookback - 1)

	// Определяем текущую ситуацию
	currentHighVolatility := currentATR > avgATR*vbConfig.VolatilityMultiplier

	var targetSignal internal.SignalType
	var targetPrice float64
	var reason string

	if currentHighVolatility {
		// Уже повышенная волатильность - ждем продолжения движения
		if priceVelocity > 0 {
			// Цена движется вверх - ожидаем BUY при следующем импульсе
			targetSignal = internal.BUY
			// Предсказываем цену с учетом скорости движения
			targetPrice = currentPrice + math.Abs(priceVelocity)*float64(vbConfig.AtrPeriod)
			reason = "continuation of uptrend with high volatility"
		} else if priceVelocity < 0 {
			// Цена движется вниз - ожидаем SELL при следующем импульсе
			targetSignal = internal.SELL
			targetPrice = currentPrice - math.Abs(priceVelocity)*float64(vbConfig.AtrPeriod)
			reason = "continuation of downtrend with high volatility"
		} else {
			// Цена стабильна - анализируем ATR тренд
			if atrVelocity > 0 {
				// Волатильность растет - ждем breakout в направлении последнего движения
				lastMove := candles[currentIdx].Close.ToFloat64() - candles[currentIdx-1].Close.ToFloat64()
				if lastMove > 0 {
					targetSignal = internal.BUY
					targetPrice = currentPrice + currentATR*vbConfig.VolatilityMultiplier
					reason = "increasing volatility, expecting breakout up"
				} else {
					targetSignal = internal.SELL
					targetPrice = currentPrice - currentATR*vbConfig.VolatilityMultiplier
					reason = "increasing volatility, expecting breakout down"
				}
			} else {
				log.Printf("⚠️ Волатильность стабильна или снижается при высоком уровне")
				return nil
			}
		}
	} else {
		// Нормальная волатильность - ждем ее увеличения
		if atrVelocity > 0 {
			// Волатильность растет - скоро может быть breakout
			// Определяем направление по последнему движению
			lastMove := candles[currentIdx].Close.ToFloat64() - candles[currentIdx-1].Close.ToFloat64()
			if lastMove > 0 {
				targetSignal = internal.BUY
				targetPrice = currentPrice + (avgATR*vbConfig.VolatilityMultiplier-currentATR)*2
				reason = "volatility increasing, expecting breakout up"
			} else {
				targetSignal = internal.SELL
				targetPrice = currentPrice - (avgATR*vbConfig.VolatilityMultiplier-currentATR)*2
				reason = "volatility increasing, expecting breakout down"
			}
		} else {
			log.Printf("⚠️ Волатильность стабильна или снижается")
			return nil
		}
	}

	// Проверяем минимальный размер движения
	priceDiff := math.Abs(targetPrice - currentPrice)
	minMoveRequired := currentPrice * vbConfig.MinMoveFilter
	if priceDiff < minMoveRequired {
		log.Printf("⚠️ Предсказанное движение слишком мало: %.6f < %.6f", priceDiff, minMoveRequired)
		return nil
	}

	// Вычисляем количество свечей до сигнала
	// Оцениваем время нарастания волатильности до целевого уровня
	atrDiff := avgATR*vbConfig.VolatilityMultiplier - currentATR
	candlesToSignal := 1
	if atrDiff > 0 {
		// Защита от деления на ноль или очень маленькое значение
		absVelocity := math.Abs(atrVelocity)
		if absVelocity > 1e-10 {
			candlesToSignal = int(math.Ceil(atrDiff / absVelocity))
		} else {
			// Если скорость изменения ATR очень мала, используем консервативную оценку
			candlesToSignal = vbConfig.AtrPeriod
		}
	}

	// Ограничиваем горизонт предсказания
	maxHorizon := vbConfig.AtrPeriod * 3
	if candlesToSignal > maxHorizon {
		candlesToSignal = maxHorizon
	}
	if candlesToSignal < 1 {
		candlesToSignal = 1
	}

	// Вычисляем уверенность в предсказании
	confidence := sg.calculateConfidence(
		atrVelocity,
		priceVelocity,
		currentATR,
		avgATR,
		vbConfig.VolatilityMultiplier,
		candlesToSignal,
		maxHorizon,
		currentHighVolatility,
		reason,
	)

	// Минимальный порог уверенности
	if confidence < 0.30 {
		log.Printf("⚠️ Недостаточная уверенность в предсказании (%.3f < 0.30) для причины: %s", confidence, reason)
		return nil
	}

	// Вычисляем дату сигнала
	if len(candles) < 2 {
		return nil
	}

	timeInterval := (candles[len(candles)-1].ToTime().Unix() - candles[0].ToTime().Unix()) / int64(len(candles)-1)
	lastTimestamp := candles[len(candles)-1].ToTime().Unix()
	futureTimestamp := lastTimestamp + timeInterval*int64(candlesToSignal)

	log.Printf("🎯 Предсказан сигнал %s на %.4f через %d свечей (уверенность: %.3f) по причине: %s",
		targetSignal, targetPrice, candlesToSignal, confidence, reason)

	return &internal.FutureSignal{
		SignalType: targetSignal,
		Date:       futureTimestamp,
		Price:      targetPrice,
		Confidence: confidence,
	}
}

// calculateConfidence вычисляет уверенность в предсказании
func (sg *VolatilityBreakoutSignalGenerator) calculateConfidence(
	atrVelocity float64,
	priceVelocity float64,
	currentATR float64,
	avgATR float64,
	volatilityMultiplier float64,
	candlesToSignal int,
	maxHorizon int,
	currentHighVolatility bool,
	reason string,
) float64 {
	confidence := 0.5 // Базовая уверенность

	// Защита от деления на ноль
	if avgATR <= 0 {
		return 0.0
	}
	thresholdATR := avgATR * volatilityMultiplier
	if thresholdATR <= 0 {
		return 0.0
	}

	// Фактор 1: Сила тренда волатильности
	atrStrength := math.Abs(atrVelocity) / avgATR
	if atrStrength > 0.01 {
		confidence += 0.20
	} else if atrStrength > 0.005 {
		confidence += 0.15
	} else if atrStrength > 0.002 {
		confidence += 0.10
	}

	// Фактор 2: Текущий уровень волатильности относительно порога
	volatilityRatio := currentATR / thresholdATR
	if currentHighVolatility {
		// Уже высокая волатильность - хорошо для продолжения
		confidence += 0.15
		if volatilityRatio > 1.2 {
			confidence += 0.10
		}
	} else {
		// Волатильность растет к порогу
		if volatilityRatio > 0.8 {
			confidence += 0.10
		}
	}

	// Фактор 3: Направленность движения цены
	if math.Abs(priceVelocity) > 0.0001 {
		confidence += 0.10
	}

	// Фактор 4: Горизонт предсказания
	horizonRatio := float64(candlesToSignal) / float64(maxHorizon)
	if horizonRatio < 0.25 {
		confidence += 0.15
	} else if horizonRatio > 0.75 {
		confidence -= 0.20
	}

	// Фактор 5: Логичность причины
	switch reason {
	case "continuation of uptrend with high volatility":
		if priceVelocity > 0 && currentHighVolatility {
			confidence += 0.10
		}
	case "continuation of downtrend with high volatility":
		if priceVelocity < 0 && currentHighVolatility {
			confidence += 0.10
		}
	case "increasing volatility, expecting breakout up":
		if atrVelocity > 0 && priceVelocity >= 0 {
			confidence += 0.10
		}
	case "increasing volatility, expecting breakout down":
		if atrVelocity > 0 && priceVelocity <= 0 {
			confidence += 0.10
		}
	case "volatility increasing, expecting breakout up":
		if atrVelocity > 0 && priceVelocity >= 0 {
			confidence += 0.10
		}
	case "volatility increasing, expecting breakout down":
		if atrVelocity > 0 && priceVelocity <= 0 {
			confidence += 0.10
		}
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

type VolatilityBreakoutConfigGenerator struct{}

func NewVolatilityBreakoutConfigGenerator() *VolatilityBreakoutConfigGenerator {
	return &VolatilityBreakoutConfigGenerator{}
}

func (g *VolatilityBreakoutConfigGenerator) Generate() []internal.StrategyConfigV2 {
	configs := []internal.StrategyConfigV2{}

	// Используем тот же диапазон, что и в V1 для честного сравнения
	for atrPeriod := 10; atrPeriod <= 25; atrPeriod += 5 {
		for volatilityMultiplier := 1.2; volatilityMultiplier <= 2.5; volatilityMultiplier += 0.3 {
			for minMoveFilter := 0.005; minMoveFilter <= 0.025; minMoveFilter += 0.005 {
				configs = append(configs, &VolatilityBreakoutConfig{
					AtrPeriod:            atrPeriod,
					VolatilityMultiplier: volatilityMultiplier,
					MinMoveFilter:        minMoveFilter,
				})
			}
		}
	}

	return configs
}

type VolatilityBreakoutStrategy struct{}

func (s *VolatilityBreakoutStrategy) Name() string {
	return "volatility_breakout"
}

func NewVolatilityBreakoutStrategyV2(slippage float64) internal.TradingStrategy {
	slippageProvider := internal.NewSlippageProvider(slippage)
	signalGenerator := NewVolatilityBreakoutSignalGenerator()

	configManager := internal.NewConfigManager(
		&VolatilityBreakoutConfig{
			AtrPeriod:            14,
			VolatilityMultiplier: 1.8,
			MinMoveFilter:        0.01,
		},
		func() internal.StrategyConfigV2 {
			return &VolatilityBreakoutConfig{}
		},
	)

	configGenerator := NewVolatilityBreakoutConfigGenerator()
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	return internal.NewStrategyBase(
		"volatility_breakout_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewVolatilityBreakoutStrategyV2(0.01)
	internal.RegisterStrategyV2(strategy)
}
