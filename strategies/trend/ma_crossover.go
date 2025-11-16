// strategies/ma_crossover.go

// Moving Average Crossover Strategy
//
// Описание стратегии:
// Классическая стратегия пересечения скользящих средних - один из фундаментальных подходов
// технического анализа. Стратегия использует две скользящие средние с разными периодами:
// быструю (короткий период) и медленную (длинный период).
//
// Как работает:
// - Рассчитывается быстрая SMA (короткий период) и медленная SMA (длинный период)
// - Покупка: когда быстрая MA пересекает медленную MA снизу вверх (золотой crossover)
// - Продажа: когда быстрая MA пересекает медленную MA сверху вниз (смертельный crossover)
// - Стратегия следует тренду: покупает при начале восходящего тренда, продает при начале нисходящего
//
// Параметры:
// - Быстрая MA период (обычно 5-15): реагирует на краткосрочные изменения
// - Медленная MA период (обычно 15-30): отражает долгосрочный тренд
//
// Сильные стороны:
// - Простота и понятность логики
// - Хорошо работает в трендовых рынках
// - Классический проверенный подход
// - Минимизирует влияние рыночного шума
//
// Слабые стороны:
// - Генерирует много ложных сигналов в боковых рынках (whipsaws)
// - Значительное запаздывание сигнала
// - Не определяет силу тренда
// - Может давать сигналы на вершинах/днищах трендов
//
// Лучшие условия для применения:
// - Трендовые рынки с четким направлением
// - Долгосрочная и среднесрочная торговля
// - В сочетании с фильтрами объема или волатильности
// - На активах с хорошей трендовой характеристикой

package trend

import (
	"bt/internal"
	"errors"
	"fmt"
)

type MACrossoverConfig struct {
	FastPeriod int `json:"fast_period"`
	SlowPeriod int `json:"slow_period"`
}

func (c *MACrossoverConfig) Validate() error {
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

func (c *MACrossoverConfig) DefaultConfigString() string {
	return fmt.Sprintf("MACrossover(fast=%d, slow=%d)",
		c.FastPeriod, c.SlowPeriod)
}

type MACrossoverStrategy struct{ internal.BaseConfig }

func (s *MACrossoverStrategy) Name() string {
	return "ma_crossover"
}

func (s *MACrossoverStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	maConfig, ok := config.(*MACrossoverConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := maConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	// Рассчитываем скользящие средние
	fastMA := internal.CalculateSMACommon(candles, maConfig.FastPeriod)
	slowMA := internal.CalculateSMACommon(candles, maConfig.SlowPeriod)

	if fastMA == nil || slowMA == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	// Начинаем анализ с максимального из двух периодов
	startIndex := maConfig.SlowPeriod - 1
	if maConfig.FastPeriod > maConfig.SlowPeriod {
		startIndex = maConfig.FastPeriod - 1
	}

	for i := startIndex; i < len(candles); i++ {
		// Проверяем пересечение скользящих средних
		if i > startIndex {
			prevFast := fastMA[i-1]
			prevSlow := slowMA[i-1]
			currFast := fastMA[i]
			currSlow := slowMA[i]

			// Быстрая MA пересекает медленную MA снизу вверх - сигнал на покупку
			if !inPosition && prevFast <= prevSlow && currFast > currSlow {
				signals[i] = internal.BUY
				inPosition = true
				continue
			}

			// Быстрая MA пересекает медленную MA сверху вниз - сигнал на продажу
			if inPosition && prevFast >= prevSlow && currFast < currSlow {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *MACrossoverStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := s.DefaultConfig().(*MACrossoverConfig)
	bestProfit := -1.0

	// Оптимизируем периоды скользящих средних
	for fast := 5; fast <= 15; fast += 2 {
		for slow := fast + 5; slow <= 30; slow += 5 {
			config := &MACrossoverConfig{
				FastPeriod: fast,
				SlowPeriod: slow,
			}
			if config.Validate() != nil {
				continue
			}

			signals := s.GenerateSignalsWithConfig(candles, config)
			result := internal.Backtest(candles, signals, s.GetSlippage()) // проскальзывание
			if result.TotalProfit >= bestProfit {
				bestProfit = result.TotalProfit
				bestConfig = config
			}
		}
	}

	fmt.Printf("Лучшие параметры MA Crossover: fast=%d, slow=%d, профит=%.4f\n",
		bestConfig.FastPeriod, bestConfig.SlowPeriod, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("ma_crossover", &MACrossoverStrategy{
		BaseConfig: internal.BaseConfig{
			Config: &MACrossoverConfig{
				FastPeriod: 10,
				SlowPeriod: 20,
			},
		},
	})
}
