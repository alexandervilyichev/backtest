// strategies/bollinger_bands.go

// Bollinger Bands Strategy
//
// Описание стратегии:
// Bollinger Bands - индикатор волатильности, состоящий из простой скользящей средней
// и двух стандартных отклонений выше и ниже SMA. Ширина полос указывает на волатильность.
//
// Как работает:
// - Рассчитывается SMA за заданный период (обычно 20)
// - Вычисляется стандартное отклонение цен за тот же период
// - Верхняя полоса = SMA + (множитель × std_dev) [обычно 2]
// - Нижняя полоса = SMA - (множитель × std_dev) [обычно 2]
// - Покупка: когда цена касается или пересекает нижнюю полосу снизу вверх
// - Продажа: когда цена касается или пересекает верхнюю полосу сверху вниз
//
// Параметры:
// - BollingerBandsPeriod: период расчета SMA (обычно 20)
// - BollingerBandsMultiplier: множитель стандартного отклонения (обычно 2.0)
//
// Сильные стороны:
// - Хорошо определяет уровни перекупленности/перепроданности
// - Учитывает волатильность рынка
// - Универсален для разных временных рамок
// - Хорошо работает в боковых рынках и для определния breakout'ов
//
// Слабые стороны:
// - В сильных трендах может генерировать много ложных сигналов
// - Зависит от правильного выбора периода и множителя
// - В периоды низкой волатильности полосы сжимаются, давая больше сигналов
// - Не является leading индикатором (запаздывает)
//
// Лучшие условия для применения:
// - Боковые рынки для поиска точек входа
// - Комбинация с трендовыми индикатторами для фильтрации
// - На активах со средней волатильностью
// - Для поиска точек разворота после экстремальных движений

package volatility

import (
	"bt/internal"
	"errors"
	"fmt"
	"math"
)

type BollingerBandsConfig struct {
	Period     int     `json:"period"`
	Multiplier float64 `json:"multiplier"`
}

func (c *BollingerBandsConfig) Validate() error {
	if c.Period <= 0 {
		return errors.New("period must be positive")
	}
	if c.Multiplier <= 0 {
		return errors.New("multiplier must be positive")
	}
	return nil
}

func (c *BollingerBandsConfig) DefaultConfigString() string {
	return fmt.Sprintf("BBands(period=%d, mult=%.2f)",
		c.Period, c.Multiplier)
}

type BollingerBandsStrategy struct{}

func (s *BollingerBandsStrategy) Name() string {
	return "bollinger_bands"
}

// calculateBBMiddle — рассчитывает среднюю линию Bollinger Bands (SMA)
func calculateBBMiddle(candles []internal.Candle, period int) []float64 {
	if len(candles) < period {
		return nil
	}

	middle := make([]float64, len(candles))

	// Первые period-1 значений — не определены
	for i := 0; i < period-1; i++ {
		middle[i] = 0
	}

	for i := period - 1; i < len(candles); i++ {
		var sum float64
		// Суммируем цены закрытия за период
		for j := i - period + 1; j <= i; j++ {
			sum += candles[j].Close.ToFloat64()
		}
		middle[i] = sum / float64(period)
	}

	return middle
}

// calculateBBStdDev — рассчитывает стандартное отклонение для Bollinger Bands
func calculateBBStdDev(candles []internal.Candle, middle []float64, period int) []float64 {
	if len(middle) == 0 || len(candles) < period {
		return nil
	}

	stdDev := make([]float64, len(candles))

	// Первые period-1 значений — не определены
	for i := 0; i < period-1; i++ {
		stdDev[i] = 0
	}

	for i := period - 1; i < len(candles); i++ {
		var sumSquares float64
		midValue := middle[i]

		// Суммируем квадраты отклонений от среднего
		for j := i - period + 1; j <= i; j++ {
			price := candles[j].Close.ToFloat64()
			diff := price - midValue
			sumSquares += diff * diff
		}

		// Стандартное отклонение
		stdDev[i] = math.Sqrt(sumSquares / float64(period))
	}

	return stdDev
}

// calculateBollingerBands — возвращает верхнюю, среднюю и нижнюю полосы
func calculateBollingerBands(candles []internal.Candle, period int, multiplier float64) (upper []float64, middle []float64, lower []float64) {
	middle = calculateBBMiddle(candles, period)
	if middle == nil {
		return nil, nil, nil
	}

	stdDev := calculateBBStdDev(candles, middle, period)
	if stdDev == nil {
		return nil, middle, nil
	}

	length := len(candles)
	upper = make([]float64, length)
	lower = make([]float64, length)

	for i := 0; i < period-1; i++ {
		upper[i] = 0
		lower[i] = 0
	}

	for i := period - 1; i < length; i++ {
		midValue := middle[i]
		dev := stdDev[i] * multiplier

		upper[i] = midValue + dev
		lower[i] = midValue - dev
	}

	return upper, middle, lower
}

func (s *BollingerBandsStrategy) DefaultConfig() internal.StrategyConfig {
	return &BollingerBandsConfig{
		Period:     20,
		Multiplier: 2.0,
	}
}

func (s *BollingerBandsStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	bbConfig, ok := config.(*BollingerBandsConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := bbConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	upper, _, lower := calculateBollingerBands(candles, bbConfig.Period, bbConfig.Multiplier)
	if upper == nil || lower == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := bbConfig.Period; i < len(candles); i++ {
		currentPrice := candles[i].Close.ToFloat64()
		currentLower := lower[i]
		currentUpper := upper[i]

		// Получаем предыдущие значения для обнаружения пересечений
		var prevPrice, prevLower, prevUpper float64
		if i > 0 {
			prevPrice = candles[i-1].Close.ToFloat64()
			if i-1 < len(lower) && i-1 < len(upper) {
				prevLower = lower[i-1]
				prevUpper = upper[i-1]
			}
		}

		// BUY: цена пересекает нижнюю полосу снизу вверх
		if !inPosition && prevPrice <= prevLower && currentPrice > currentLower {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		// SELL: цена пересекает верхнюю полосу сверху вниз
		if inPosition && prevPrice >= prevUpper && currentPrice < currentUpper {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *BollingerBandsStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := &BollingerBandsConfig{
		Period:     20,
		Multiplier: 2.0,
	}
	bestProfit := -1.0

	// Grid search по параметрам
	for period := 10; period <= 50; period += 5 {
		for multiplier := 1.5; multiplier <= 3.0; multiplier += 0.25 {
			config := &BollingerBandsConfig{
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

	fmt.Printf("Лучшие параметры Bollinger Bands: period=%d, multiplier=%.2f, профит=%.4f\n",
		bestConfig.Period, bestConfig.Multiplier, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("bollinger_bands", &BollingerBandsStrategy{})
}
