// Momentum Breakout Strategy
//
// Описание стратегии:
// Стратегия сочетает анализ моментума (скорости изменения цены) с выявлением прорывов
// ключевых уровней поддержки/сопротивления. Входит в позиции только при сильном моментуме,
// подтвержденном повышенным объемом и достаточной волатильностью.
//
// Как работает:
// - Рассчитывается моментум как скорость изменения цены за заданный период
// - Определяются динамические уровни поддержки/сопротивления на основе локальных экстремумов
// - Покупка: прорыв уровня сопротивления вверх с сильным моментумом и повышенным объемом
// - Продажа: прорыв уровня поддержки вниз с сильным моментумом и повышенным объемом
// - Дополнительно фильтруется по минимальной волатильности для избежания ложных сигналов
//
// Параметры:
// - MomentumPeriod: период расчета моментума (5-20, по умолчанию 10)
// - BreakoutThreshold: порог прорыва уровня в процентах (0.5-2.0%, по умолчанию 1.0%)
// - VolumeMultiplier: множитель объема для подтверждения (1.2-2.0, по умолчанию 1.5)
// - VolatilityFilter: минимальная волатильность для активности (0.1-1.0%, по умолчанию 0.3%)
//
// Сильные стороны:
// - Фильтрует слабые движения, фокусируясь только на сильных трендах
// - Адаптивные уровни, подстраивающиеся под рыночные условия
// - Многофакторное подтверждение сигналов (моментум + объем + волатильность)
// - Хорошо работает на волатильных рынках с четкими трендами
//
// Слабые стороны:
// - Может пропускать медленные, но устойчивые движения
// - Требует достаточной волатильности для генерации сигналов
// - Зависит от качества данных объема
// - В периоды низкой волатильности может генерировать мало сигналов
//
// Лучшие условия для применения:
// - Волатильные рынки с выраженными трендами
// - Акции с хорошей ликвидностью
// - Периоды высокой рыночной активности
// - В качестве дополнения к долгосрочным стратегиям

package volatility

import (
	"bt/internal"
	"log"
	"math"
	"strconv"
)

// MomentumBreakoutStrategy представляет стратегию прорыва с моментумом
type MomentumBreakoutStrategy struct{}

// Name возвращает название стратегии
func (s *MomentumBreakoutStrategy) Name() string {
	return "momentum_breakout"
}

// calculateMomentum рассчитывает моментум как скорость изменения цены
func calculateMomentum(prices []float64, period int) []float64 {
	if len(prices) < period+1 {
		return nil
	}

	momentum := make([]float64, len(prices))
	for i := period; i < len(prices); i++ {
		// Моментум = (текущая цена - цена period периодов назад) / цена period периодов назад
		momentum[i] = (prices[i] - prices[i-period]) / prices[i-period]
	}

	return momentum
}

// findDynamicLevels находит динамические уровни поддержки/сопротивления
func findDynamicLevels(prices []float64, lookback int) (support, resistance []float64) {
	if len(prices) < lookback {
		return nil, nil
	}

	support = make([]float64, len(prices))
	resistance = make([]float64, len(prices))

	window := int(math.Min(float64(lookback), float64(len(prices))))

	for i := window; i < len(prices); i++ {
		windowStart := i - window
		windowPrices := prices[windowStart:i]

		// Находим локальные минимумы и максимумы в окне
		minPrice := windowPrices[0]
		maxPrice := windowPrices[0]

		for _, price := range windowPrices {
			if price < minPrice {
				minPrice = price
			}
			if price > maxPrice {
				maxPrice = price
			}
		}

		// Уровни с небольшим буфером для фильтрации шума
		buffer := (maxPrice - minPrice) * 0.1 // 10% буфер
		support[i] = minPrice - buffer
		resistance[i] = maxPrice + buffer
	}

	return support, resistance
}

// calculateVolatility рассчитывает волатильность как стандартное отклонение доходностей
func calculateVolatility(prices []float64, period int) []float64 {
	if len(prices) < period+1 {
		return nil
	}

	volatility := make([]float64, len(prices))

	for i := period; i < len(prices); i++ {
		// Рассчитываем доходности в окне
		windowStart := i - period
		windowPrices := prices[windowStart:i]

		// Средняя доходность
		sum := 0.0
		for j := 1; j < len(windowPrices); j++ {
			ret := (windowPrices[j] - windowPrices[j-1]) / windowPrices[j-1]
			sum += ret
		}
		meanReturn := sum / float64(len(windowPrices)-1)

		// Дисперсия доходностей
		variance := 0.0
		for j := 1; j < len(windowPrices); j++ {
			ret := (windowPrices[j] - windowPrices[j-1]) / windowPrices[j-1]
			variance += math.Pow(ret-meanReturn, 2)
		}
		variance /= float64(len(windowPrices) - 1)

		volatility[i] = math.Sqrt(variance)
	}

	return volatility
}

// GenerateSignals генерирует торговые сигналы на основе моментума и прорывов
func (s *MomentumBreakoutStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	if len(candles) < 50 {
		log.Printf("⚠️ Недостаточно данных для momentum breakout: получено %d свечей, требуется минимум 50", len(candles))
		return make([]internal.SignalType, len(candles))
	}

	// Извлекаем параметры с значениями по умолчанию
	momentumPeriod := params.MomentumPeriod
	if momentumPeriod == 0 {
		momentumPeriod = 10
	}

	breakoutThreshold := params.BreakoutThreshold
	if breakoutThreshold == 0 {
		breakoutThreshold = 0.01 // 1%
	}

	volumeMultiplier := params.VolumeMultiplier
	if volumeMultiplier == 0 {
		volumeMultiplier = 1.5
	}

	volatilityFilter := params.VolatilityFilter
	if volatilityFilter == 0 {
		volatilityFilter = 0.003 // 0.3%
	}

	// Извлекаем ценовые данные
	prices := make([]float64, len(candles))
	volumes := make([]float64, len(candles))

	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
		vol, err := strconv.ParseFloat(candle.Volume, 64)
		if err != nil {
			log.Printf("Предупреждение: некорректный объем на свече %d: %s, используем 0", i, candle.Volume)
			vol = 0
		}
		volumes[i] = vol
	}

	// Рассчитываем необходимые индикаторы
	momentum := calculateMomentum(prices, momentumPeriod)
	support, resistance := findDynamicLevels(prices, 20) // фиксированный lookback для уровней
	volatility := calculateVolatility(prices, 20)        // фиксированный период для волатильности

	if momentum == nil || support == nil || resistance == nil || volatility == nil {
		log.Println("❌ Ошибка расчета индикаторов для momentum breakout")
		return make([]internal.SignalType, len(candles))
	}

	log.Printf("🔍 Анализ momentum breakout: период=%d, порог=%.3f, объем=%.1f, волатильность=%.3f",
		momentumPeriod, breakoutThreshold, volumeMultiplier, volatilityFilter)

	// Генерируем сигналы
	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	// Начинаем анализ после достаточного количества данных
	startIdx := 50

	for i := startIdx; i < len(candles); i++ {
		currentPrice := prices[i]
		currentMomentum := momentum[i]
		currentVolatility := volatility[i]

		// Пропускаем если волатильность слишком низкая
		if currentVolatility < volatilityFilter {
			signals[i] = internal.HOLD
			continue
		}

		// Проверяем условия для BUY (прорыв сопротивления вверх)
		if !inPosition && resistance[i] > 0 {
			// Цена должна пробить уровень сопротивления
			breakoutUp := (currentPrice-resistance[i])/resistance[i] > breakoutThreshold

			// Моментум должен быть положительным и сильным
			strongUpMomentum := currentMomentum > breakoutThreshold*2

			// Объем должен быть повышенным
			avgVolume := 0.0
			volumeCount := 0
			for j := int(math.Max(0, float64(i-5))); j < i; j++ {
				avgVolume += volumes[j]
				volumeCount++
			}
			if volumeCount > 0 {
				avgVolume /= float64(volumeCount)
				highVolume := volumes[i] > avgVolume*volumeMultiplier

				// Все условия для BUY
				if breakoutUp && strongUpMomentum && highVolume {
					signals[i] = internal.BUY
					inPosition = true
					// log.Printf("   BUY сигнал на свече %d: цена=%.2f, сопротивление=%.2f, моментум=%.4f",
					//	i, currentPrice, resistance[i], currentMomentum)
					continue
				}
			}
		}

		// Проверяем условия для SELL (прорыв поддержки вниз)
		if inPosition && support[i] > 0 {
			// Цена должна пробить уровень поддержки
			breakoutDown := (support[i]-currentPrice)/support[i] > breakoutThreshold

			// Моментум должен быть отрицательным и сильным
			strongDownMomentum := currentMomentum < -breakoutThreshold*2

			// Объем должен быть повышенным
			avgVolume := 0.0
			volumeCount := 0
			for j := int(math.Max(0, float64(i-5))); j < i; j++ {
				avgVolume += volumes[j]
				volumeCount++
			}
			if volumeCount > 0 {
				avgVolume /= float64(volumeCount)
				highVolume := volumes[i] > avgVolume*volumeMultiplier

				// Все условия для SELL
				if breakoutDown && strongDownMomentum && highVolume {
					signals[i] = internal.SELL
					inPosition = false
					// log.Printf("   SELL сигнал на свече %d: цена=%.2f, поддержка=%.2f, моментум=%.4f",
					//	i, currentPrice, support[i], currentMomentum)
					continue
				}
			}
		}

		signals[i] = internal.HOLD
	}

	log.Printf("✅ Momentum breakout анализ завершен")
	return signals
}

// Optimize оптимизирует параметры стратегии
func (s *MomentumBreakoutStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{
		MomentumPeriod:    10,
		BreakoutThreshold: 0.01,
		VolumeMultiplier:  1.5,
		VolatilityFilter:  0.003,
	}
	bestProfit := -1.0

	generator := s.GenerateSignals

	// Grid search по параметрам
	for momentumPeriod := 5; momentumPeriod <= 20; momentumPeriod += 5 {
		for breakoutThreshold := 0.005; breakoutThreshold <= 0.025; breakoutThreshold += 0.005 {
			for volumeMultiplier := 1.2; volumeMultiplier <= 2.0; volumeMultiplier += 0.2 {
				for volatilityFilter := 0.001; volatilityFilter <= 0.005; volatilityFilter += 0.001 {
					params := internal.StrategyParams{
						MomentumPeriod:    momentumPeriod,
						BreakoutThreshold: breakoutThreshold,
						VolumeMultiplier:  volumeMultiplier,
						VolatilityFilter:  volatilityFilter,
					}

					signals := generator(candles, params)
					result := internal.Backtest(candles, signals, 0.01) // 0.01 units проскальзывание

					if result.TotalProfit > bestProfit {
						bestProfit = result.TotalProfit
						bestParams = params
					}
				}
			}
		}
	}

	log.Printf("Лучшие параметры momentum breakout: период=%d, порог=%.3f, объем=%.1f, волатильность=%.3f, прибыль=%.2f",
		bestParams.MomentumPeriod, bestParams.BreakoutThreshold, bestParams.VolumeMultiplier,
		bestParams.VolatilityFilter, bestProfit)

	return bestParams
}

func init() {
	internal.RegisterStrategy("momentum_breakout", &MomentumBreakoutStrategy{})
}
