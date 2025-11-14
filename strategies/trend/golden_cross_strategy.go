// strategies/trend/golden_cross_strategy.go

// GoldenCrossStrategy - стратегия золотого пересечения
//
// Описание стратегии:
// Классическая стратегия пересечения экспоненциальных скользящих средних (EMA).
// Использует две EMA с разными периодами для выявления трендовых сигналов.
//
// Как работает:
// - Рассчитывается быстрая EMA (короткий период) и медленная EMA (длинный период)
// - Покупка: когда быстрая EMA пересекает медленную EMA снизу вверх (золотое пересечение)
// - Продажа: когда быстрая EMA пересекает медленную EMA сверху вниз (смертельное пересечение)
// - Стратегия следует тренду: покупает при начале восходящего тренда, продает при начале нисходящего
//
// Параметры:
// - Быстрая EMA период (обычно 5-15): реагирует на краткосрочные изменения цены
// - Медленная EMA период (обычно 15-30): отражает долгосрочный тренд
//
// Преимущества EMA над SMA:
// - EMA более чувствительна к недавним изменениям цены
// - Снижает лаг и дает более timely сигналы
// - Лучше работает в волатильных рыночных условиях
// - Более точно отражает текущую рыночную ситуацию
//
// Сильные стороны:
// - Простота и понятность логики
// - Хорошо работает в трендовых рынках
// - Быстрое реагирование на изменения тренда
// - Минимизирует влияние старых данных
//
// Слабые стороны:
// - Может генерировать ложные сигналы в боковых рынках
// - Чувствительна к рыночному шуму при малых периодах
// - Требует правильного подбора периодов для каждого актива
//
// Лучшие условия для применения:
// - Трендовые рынки с четким направлением движения
// - Среднесрочная и долгосрочная торговля
// - Активы с высокой волатильностью
// - В периоды сильных рыночных движений

package trend

import (
	"bt/internal"
	"errors"
	"fmt"
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

func (c *GoldenCrossConfig) DefaultConfigString() string {
	return fmt.Sprintf("GoldenCross(fast=%d, slow=%d)",
		c.FastPeriod, c.SlowPeriod)
}

type GoldenCrossStrategy struct{ internal.BaseConfig }

func (s *GoldenCrossStrategy) Name() string {
	return "golden_cross"
}

func (s *GoldenCrossStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
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

	// Начинаем анализ с максимального из двух периодов
	startIndex := gcConfig.SlowPeriod - 1
	if gcConfig.FastPeriod > gcConfig.SlowPeriod {
		startIndex = gcConfig.FastPeriod - 1
	}

	for i := startIndex; i < len(candles); i++ {
		// Проверяем пересечение EMA начиная со следующего индекса после стартового
		if i > startIndex {
			prevFast := fastEMA[i-1]
			prevSlow := slowEMA[i-1]
			currFast := fastEMA[i]
			currSlow := slowEMA[i]

			// Быстрая EMA пересекает медленную EMA снизу вверх - сигнал на покупку (золотое пересечение)
			if !inPosition && prevFast <= prevSlow && currFast > currSlow {
				signals[i] = internal.BUY
				inPosition = true
				continue
			}

			// Быстрая EMA пересекает медленную EMA сверху вниз - сигнал на продажу (смертельное пересечение)
			if inPosition && prevFast >= prevSlow && currFast < currSlow {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		// Если нет сигнала на покупку или продажу, то HOLD
		if signals[i] == 0 {
			signals[i] = internal.HOLD
		}
	}

	return signals
}

func (s *GoldenCrossStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := s.DefaultConfig().(*GoldenCrossConfig)
	bestProfit := -1.0

	// Оптимизируем периоды EMA для лучших результатов

	for fast := 2; fast < 240; fast += 10 {
		for slow := 100; slow < 340; slow += 5 {

			config := &GoldenCrossConfig{
				FastPeriod: fast,
				SlowPeriod: slow,
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

	fmt.Printf("Лучшие параметры Golden Cross: fast=%d, slow=%d, профит=%.4f\n",
		bestConfig.FastPeriod, bestConfig.SlowPeriod, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("golden_cross", &GoldenCrossStrategy{
		BaseConfig: internal.BaseConfig{
			Config: &GoldenCrossConfig{
				FastPeriod: 12,
				SlowPeriod: 26,
			},
		},
	})
}
