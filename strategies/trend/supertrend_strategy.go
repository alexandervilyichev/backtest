// strategies/supertrend_strategy.go

// SuperTrend Strategy
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
// - SuperTrendPeriod: период расчета ATR (обычно 10-14)
// - SuperTrendMultiplier: множитель для ATR (обычно 2.0-3.0)
//
// Сильные стороны:
// - Хорошо определяет направление тренда
// - Динамически адаптируется к волатильности
// - Четкие сигналы входа/выхода
// - Работает на разных временных интервалах
// - Минимизирует ложные сигналы в боковых рынках
//
// Слабые стороны:
// - Может запаздывать в начале сильных движений
// - В периоды низкой волатильности может давать ложные сигналы
// - Зависит от правильного выбора периода и множителя
// - Не подходит для скальпинга из-за задержки
//
// Лучшие условия для применения:
// - Трендовые рынки (бычий/медвежий)
// - Среднесрочная торговля
// - Активы с четко выраженными трендами
// - В комбинации с другими индикаторами для фильтрации

package trend

import (
	"bt/internal"
	"errors"
	"fmt"
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

func (c *SupertrendConfig) DefaultConfigString() string {
	return fmt.Sprintf("Supertrend(period=%d, mult=%.2f)",
		c.Period, c.Multiplier)
}

type SuperTrendStrategy struct{}

func (s *SuperTrendStrategy) Name() string {
	return "supertrend"
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

func (s *SuperTrendStrategy) DefaultConfig() internal.StrategyConfig {
	return &SupertrendConfig{
		Period:     10,
		Multiplier: 3.0,
	}
}

func (s *SuperTrendStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
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

func (s *SuperTrendStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := &SupertrendConfig{
		Period:     10,
		Multiplier: 3.0,
	}
	bestProfit := -1.0

	// Grid search по параметрам
	for period := 7; period <= 20; period += 1 {
		for multiplier := 1.5; multiplier <= 4.0; multiplier += 0.25 {
			config := &SupertrendConfig{
				Period:     period,
				Multiplier: multiplier,
			}
			if config.Validate() != nil {
				continue
			}

			signals := s.GenerateSignalsWithConfig(candles, config)
			result := internal.Backtest(candles, signals, 0.01) // 0.01 units проскальзывание
			if result.TotalProfit > bestProfit {
				bestProfit = result.TotalProfit
				bestConfig = config
			}
		}
	}

	fmt.Printf("Лучшие параметры SOLID Supertrend: period=%d, multiplier=%.2f, профит=%.4f\n",
		bestConfig.Period, bestConfig.Multiplier, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("supertrend", &SuperTrendStrategy{})
}
