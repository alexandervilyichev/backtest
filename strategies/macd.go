// strategies/macd.go

// MACD (Moving Average Convergence Divergence) Strategy
//
// Описание стратегии:
// MACD - это трендовый momentum индикатор, который показывает связь между двумя скользящими средними цены.
// Состоит из MACD линии (разность быстрой и медленной EMA), сигнальной линии (EMA от MACD) и гистограммы.
//
// Как работает:
// - Рассчитывается быстрая EMA (обычно 12 периодов) и медленная EMA (обычно 26 периодов)
// - MACD линия = быстрая EMA - медленная EMA
// - Сигнальная линия = EMA от MACD линии (обычно 9 периодов)
// - Покупка: когда MACD пересекает сигнальную линию снизу вверх (бычий crossover)
// - Продажа: когда MACD пересекает сигнальную линию сверху вниз (медвежий crossover)
//
// Параметры:
// - MACDFastPeriod: период быстрой EMA (обычно 12)
// - MACDSlowPeriod: период медленной EMA (обычно 26)
// - MACDSignalPeriod: период сигнальной линии (обычно 9)
//
// Сильные стороны:
// - Хорошо определяет изменения тренда и momentum
// - Классический и проверенный индикатор
// - Учитывает как направление, так и скорость движения цены
// - Гистограмма помогает визуально оценить силу сигнала
//
// Слабые стороны:
// - Может давать ложные сигналы в боковых рынках (whipsaws)
// - Запаздывает по сравнению с более быстрыми индикаторами
// - Чувствителен к выбору периодов EMA
// - В очень волатильных условиях может генерировать много шума
//
// Лучшие условия для применения:
// - Трендовые рынки с четкими направлениями
// - Средне- и долгосрочная торговля
// - В сочетании с другими индикаторами подтверждения
// - На активах с хорошей трендовой характеристикой

package strategies

import (
	"bt/internal"
	"fmt"
)

type MACDStrategy struct{}

func (s *MACDStrategy) Name() string {
	return "macd"
}

func (s *MACDStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	fastPeriod := params.MACDFastPeriod
	slowPeriod := params.MACDSlowPeriod
	signalPeriod := params.MACDSignalPeriod

	if fastPeriod == 0 {
		fastPeriod = 12 // default
	}
	if slowPeriod == 0 {
		slowPeriod = 26 // default
	}
	if signalPeriod == 0 {
		signalPeriod = 9 // default
	}

	macdLine, signalLine, _ := calculateMACD(candles, fastPeriod, slowPeriod, signalPeriod)
	if macdLine == nil || signalLine == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := slowPeriod + signalPeriod - 1; i < len(candles); i++ {
		macd := macdLine[i]
		signal := signalLine[i]
		macdPrev := macdLine[i-1]
		signalPrev := signalLine[i-1]

		if !inPosition {
			// Buy when MACD crosses above signal line
			if macdPrev <= signalPrev && macd > signal {
				signals[i] = internal.BUY
				inPosition = true
				continue
			}
		} else {
			// Sell when MACD crosses below signal line
			if macdPrev >= signalPrev && macd < signal {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *MACDStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{
		MACDFastPeriod:   12,
		MACDSlowPeriod:   26,
		MACDSignalPeriod: 9,
	}
	bestProfit := -1.0

	generator := s.GenerateSignals

	// Grid search по периодам
	for fast := 8; fast <= 16; fast += 2 {
		for slow := 20; slow <= 32; slow += 4 {
			for signal := 6; signal <= 12; signal += 2 {
				if fast >= slow {
					continue // fast period must be less than slow period
				}

				params := internal.StrategyParams{
					MACDFastPeriod:   fast,
					MACDSlowPeriod:   slow,
					MACDSignalPeriod: signal,
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

	fmt.Printf("Лучшие параметры MACD: fast=%d, slow=%d, signal=%d\n",
		bestParams.MACDFastPeriod, bestParams.MACDSlowPeriod, bestParams.MACDSignalPeriod)

	return bestParams
}

func init() {
	internal.RegisterStrategy("macd", &MACDStrategy{})
}
