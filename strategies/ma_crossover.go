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

package strategies

import "bt/internal"

type MovingAverageCrossoverStrategy struct{}

func (s *MovingAverageCrossoverStrategy) Name() string {
	return "moving_average_crossover"
}

func (s *MovingAverageCrossoverStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	// Применяем квантизацию если включена
	quantizedCandles := ApplyQuantizationToCandles(candles, params)

	// Используем параметры для быстрой и медленной скользящих средних
	// Временно используем существующие поля StrategyParams, пока не добавлены специальные
	fastPeriod := params.AoFastPeriod // Используем AoFastPeriod для быстрой MA
	slowPeriod := params.AoSlowPeriod // Используем AoSlowPeriod для медленной MA

	// Значения по умолчанию
	if fastPeriod == 0 {
		fastPeriod = 10
	}
	if slowPeriod == 0 {
		slowPeriod = 20
	}

	// Рассчитываем скользящие средние
	fastMA := calculateSMACommon(quantizedCandles, fastPeriod)
	slowMA := calculateSMACommon(quantizedCandles, slowPeriod)

	if fastMA == nil || slowMA == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	// Начинаем анализ с максимального из двух периодов
	startIndex := slowPeriod - 1
	if fastPeriod > slowPeriod {
		startIndex = fastPeriod - 1
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

func (s *MovingAverageCrossoverStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{
		AoFastPeriod: 10, // быстрая MA
		AoSlowPeriod: 20, // медленная MA
	}
	bestProfit := -1.0

	generator := s.GenerateSignals

	// Оптимизируем периоды скользящих средних
	for fast := 5; fast <= 15; fast += 2 {
		for slow := fast + 5; slow <= 30; slow += 5 {
			params := internal.StrategyParams{
				AoFastPeriod: fast,
				AoSlowPeriod: slow,
			}
			signals := generator(candles, params)
			result := internal.Backtest(candles, signals, 0.01) // 0.01 units проскальзывание
			if result.TotalProfit > bestProfit {
				bestProfit = result.TotalProfit
				bestParams = params
			}
		}
	}

	return bestParams
}

func init() {
	internal.RegisterStrategy("ma_crossover", &MovingAverageCrossoverStrategy{})
}
