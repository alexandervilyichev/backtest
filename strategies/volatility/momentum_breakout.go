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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
)

type MomentumBreakoutConfig struct {
	MomentumPeriod    int     `json:"momentum_period"`
	BreakoutThreshold float64 `json:"breakout_threshold"`
	VolumeMultiplier  float64 `json:"volume_multiplier"`
	VolatilityFilter  float64 `json:"volatility_filter"`
}

func (c *MomentumBreakoutConfig) Validate() error {
	if c.MomentumPeriod <= 0 {
		return errors.New("momentum period must be positive")
	}
	if c.BreakoutThreshold <= 0 {
		return errors.New("breakout threshold must be positive")
	}
	if c.VolumeMultiplier <= 1.0 {
		return errors.New("volume multiplier must be greater than 1.0")
	}
	if c.VolatilityFilter < 0 {
		return errors.New("volatility filter must be non-negative")
	}
	return nil
}

func (c *MomentumBreakoutConfig) DefaultConfigString() string {
	return fmt.Sprintf("MomentumBreakout(period=%d, threshold=%.3f, vol_mult=%.1f, vol_filt=%.3f)",
		c.MomentumPeriod, c.BreakoutThreshold, c.VolumeMultiplier, c.VolatilityFilter)
}

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

// Optimize оптимизирует параметры стратегии
func (s *MomentumBreakoutStrategy) DefaultConfig() internal.StrategyConfig {
	return &MomentumBreakoutConfig{
		MomentumPeriod:    10,
		BreakoutThreshold: 0.01,
		VolumeMultiplier:  1.5,
		VolatilityFilter:  0.003,
	}
}

func (s *MomentumBreakoutStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	mbConfig, ok := config.(*MomentumBreakoutConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := mbConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < 50 {
		log.Printf("⚠️ Недостаточно данных для momentum breakout: получено %d свечей, требуется минимум 50", len(candles))
		return make([]internal.SignalType, len(candles))
	}

	// Извлекаем ценовые данные
	prices := make([]float64, len(candles))
	volumes := make([]float64, len(candles))

	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
		volumes[i] = candle.VolumeFloat // используем предвычисленное значение
	}

	// Рассчитываем необходимые индикаторы
	momentum := calculateMomentum(prices, mbConfig.MomentumPeriod)
	support, resistance := findDynamicLevels(prices, 20)               // фиксированный lookback для уровней
	volatility := internal.CalculateRollingStdDevOfReturns(prices, 20) // фиксированный период для волатильности

	if momentum == nil || support == nil || resistance == nil {
		log.Println("❌ Ошибка расчета индикаторов для momentum breakout")
		return make([]internal.SignalType, len(candles))
	}

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
		if currentVolatility < mbConfig.VolatilityFilter {
			signals[i] = internal.HOLD
			continue
		}

		// Проверяем условия для BUY (прорыв сопротивления вверх)
		if !inPosition && resistance[i] > 0 {
			// Цена должна пробить уровень сопротивления
			breakoutUp := (currentPrice-resistance[i])/resistance[i] > mbConfig.BreakoutThreshold

			// Моментум должен быть положительным и сильным
			strongUpMomentum := currentMomentum > mbConfig.BreakoutThreshold*2

			// Объем должен быть повышенным
			avgVolume := 0.0
			volumeCount := 0
			for j := int(math.Max(0, float64(i-5))); j < i; j++ {
				avgVolume += volumes[j]
				volumeCount++
			}
			if volumeCount > 0 {
				avgVolume /= float64(volumeCount)
				highVolume := volumes[i] > avgVolume*mbConfig.VolumeMultiplier

				// Все условия для BUY
				if breakoutUp && strongUpMomentum && highVolume {
					signals[i] = internal.BUY
					inPosition = true
					continue
				}
			}
		}

		// Проверяем условия для SELL (прорыв поддержки вниз)
		if inPosition && support[i] > 0 {
			// Цена должна пробить уровень поддержки
			breakoutDown := (support[i]-currentPrice)/support[i] > mbConfig.BreakoutThreshold

			// Моментум должен быть отрицательным и сильным
			strongDownMomentum := currentMomentum < -mbConfig.BreakoutThreshold*2

			// Объем должен быть повышенным
			avgVolume := 0.0
			volumeCount := 0
			for j := int(math.Max(0, float64(i-5))); j < i; j++ {
				avgVolume += volumes[j]
				volumeCount++
			}
			if volumeCount > 0 {
				avgVolume /= float64(volumeCount)
				highVolume := volumes[i] > avgVolume*mbConfig.VolumeMultiplier

				// Все условия для SELL
				if breakoutDown && strongDownMomentum && highVolume {
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

func (s *MomentumBreakoutStrategy) LoadConfigFromMap(raw json.RawMessage) internal.StrategyConfig {
	config := s.DefaultConfig()
	if err := json.Unmarshal(raw, config); err != nil {
		return nil
	}
	return config
}

func (s *MomentumBreakoutStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := &MomentumBreakoutConfig{
		MomentumPeriod:    10,
		BreakoutThreshold: 0.01,
		VolumeMultiplier:  1.5,
		VolatilityFilter:  0.003,
	}
	bestProfit := -1.0

	// Grid search по параметрам
	for momentumPeriod := 5; momentumPeriod <= 20; momentumPeriod += 5 {
		for breakoutThreshold := 0.005; breakoutThreshold <= 0.025; breakoutThreshold += 0.005 {
			for volumeMultiplier := 1.2; volumeMultiplier <= 2.0; volumeMultiplier += 0.2 {
				for volatilityFilter := 0.001; volatilityFilter <= 0.005; volatilityFilter += 0.001 {
					config := &MomentumBreakoutConfig{
						MomentumPeriod:    momentumPeriod,
						BreakoutThreshold: breakoutThreshold,
						VolumeMultiplier:  volumeMultiplier,
						VolatilityFilter:  volatilityFilter,
					}
					if config.Validate() != nil {
						continue
					}

					signals := s.GenerateSignalsWithConfig(candles, config)
					result := internal.Backtest(candles, signals, 0.01) // 0.01 units проскальзывание

					if result.TotalProfit >= bestProfit {
						bestProfit = result.TotalProfit
						bestConfig = config
					}
				}
			}
		}
	}

	fmt.Printf("Лучшие параметры Momentum Breakout: period=%d, threshold=%.3f, vol_mult=%.1f, vol_filt=%.3f, профит=%.4f\n",
		bestConfig.MomentumPeriod, bestConfig.BreakoutThreshold, bestConfig.VolumeMultiplier,
		bestConfig.VolatilityFilter, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("momentum_breakout", &MomentumBreakoutStrategy{})
}
