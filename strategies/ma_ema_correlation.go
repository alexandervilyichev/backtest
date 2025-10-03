// strategies/ma_ema_correlation.go

// MA-EMA Correlation Strategy
//
// Описание стратегии:
// Стратегия основана на анализе корреляции между простой скользящей средней (MA/SMA)
// и экспоненциальной скользящей средней (EMA). Корреляция рассчитывается в скользящем окне,
// и сигналы генерируются на основе пороговых значений корреляции.
//
// Как работает:
// - Рассчитывается MA и EMA на ценах закрытия
// - Вычисляется скользящая корреляция между MA и EMA за определенный период
// - BUY: когда корреляция превышает верхний порог (высокая положительная корреляция)
// - SELL: когда корреляция опускается ниже нижнего порога (отрицательная корреляция)
// - HOLD: в остальных случаях
//
// Параметры:
// - MA период: период для простой скользящей средней
// - EMA период: период для экспоненциальной скользящей средней
// - Lookback период: окно для расчета скользящей корреляции
// - Threshold: порог корреляции для генерации сигналов
//
// Сильные стороны:
// - Учитывает взаимосвязь между разными типами скользящих средних
// - Может выявлять периоды сильного тренда или неопределенности
// - Гибкая настройка параметров
//
// Слабые стороны:
// - Требует тщательной оптимизации параметров
// - Может быть чувствительна к выбору периодов MA и EMA
// - Корреляция не всегда является надежным индикатором направления
//
// Лучшие условия для применения:
// - Рынки с четкими трендами
// - В комбинации с другими индикаторами
// - Для фильтрации сигналов других стратегий

package strategies

import (
	"bt/internal"
	"fmt"
)

type MaEmaCorrelationStrategy struct{}

func (s *MaEmaCorrelationStrategy) Name() string {
	return "ma_ema_correlation"
}

func (s *MaEmaCorrelationStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	maPeriod := params.MaEmaCorrelationMAPeriod
	emaPeriod := params.MaEmaCorrelationEMAPeriod
	lookback := params.MaEmaCorrelationLookback
	threshold := params.MaEmaCorrelationThreshold

	// Значения по умолчанию
	if maPeriod == 0 {
		maPeriod = 20
	}
	if emaPeriod == 0 {
		emaPeriod = 20
	}
	if lookback == 0 {
		lookback = 10
	}
	if threshold == 0 {
		threshold = 0.7
	}

	// Получаем цены закрытия
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// Рассчитываем MA и EMA
	ma := calculateSMACommonForValues(prices, maPeriod)
	ema := calculateEMA(prices, emaPeriod)

	if ma == nil || ema == nil {
		return make([]internal.SignalType, len(candles))
	}

	// Рассчитываем скользящую корреляцию между MA и EMA
	correlations := calculateRollingCorrelation(ma, ema, lookback)
	if correlations == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	// Начинаем анализ после достаточного количества данных
	startIndex := maPeriod + emaPeriod + lookback - 3 // приблизительно

	for i := startIndex; i < len(candles); i++ {
		corr := correlations[i]

		// BUY: высокая положительная корреляция
		if !inPosition && corr > threshold {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		// SELL: отрицательная корреляция
		if inPosition && corr < -threshold {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *MaEmaCorrelationStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{
		MaEmaCorrelationMAPeriod:  20,
		MaEmaCorrelationEMAPeriod: 20,
		MaEmaCorrelationLookback:  10,
		MaEmaCorrelationThreshold: 0.7,
	}
	bestProfit := -1.0

	generator := s.GenerateSignals

	// Оптимизируем параметры
	for maPeriod := 10; maPeriod <= 30; maPeriod += 5 {
		for emaPeriod := 10; emaPeriod <= 30; emaPeriod += 5 {
			for lookback := 5; lookback <= 15; lookback += 5 {
				for threshold := 0.5; threshold <= 0.9; threshold += 0.1 {
					params := internal.StrategyParams{
						MaEmaCorrelationMAPeriod:  maPeriod,
						MaEmaCorrelationEMAPeriod: emaPeriod,
						MaEmaCorrelationLookback:  lookback,
						MaEmaCorrelationThreshold: threshold,
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

	fmt.Printf("🔍 Лучшие параметры ma_ema: emaPeriod=%d, maPeriod=%d, lookBack=%d → threshold=%.2f%%\n",
		bestParams.MaEmaCorrelationEMAPeriod, bestParams.MaEmaCorrelationMAPeriod,
		bestParams.MaEmaCorrelationLookback, bestParams.MaEmaCorrelationThreshold)

	return bestParams
}

func init() {
	internal.RegisterStrategy("ma_ema_correlation", &MaEmaCorrelationStrategy{})
}
