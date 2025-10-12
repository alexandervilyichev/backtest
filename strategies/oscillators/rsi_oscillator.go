// strategies/rsi_oscillator.go

// RSI Oscillator Strategy
//
// Описание стратегии:
// Стратегия использует индекс относительной силы (RSI) для определения перекупленных и перепроданных уровней рынка.
// RSI измеряет скорость и изменение ценовых движений на шкале от 0 до 100.
//
// Как работает:
// - Рассчитывается RSI с заданным периодом (по умолчанию 14)
// - Покупка: когда RSI опускается ниже уровня перепроданности (по умолчанию 30)
// - Продажа: когда RSI поднимается выше уровня перекупленности (по умолчанию 70)
// - Стратегия удерживает позицию до противоположного сигнала
//
// Параметры:
// - RsiPeriod: период расчета RSI (обычно 14)
// - RsiBuyThreshold: уровень перепроданности для покупки (обычно 30)
// - RsiSellThreshold: уровень перекупленности для продажи (обычно 70)
//
// Сильные стороны:
// - Простота реализации и понимания
// - Хорошо работает в oscillating рынках
// - RSI является стандартным и проверенным индикатором
// - Хорошо фильтрует шум на рынке
//
// Слабые стороны:
// - Может давать ложные сигналы в сильных трендах
// - Не учитывает направление тренда
// - В боковых движениях может генерировать много сделок
// - Задержка сигнала из-за smoothing
//
// Лучшие условия для применения:
// - Боковые/осциллирующие рынки
// - Среднесрочная торговля
// - В сочетании с трендовыми фильтрами
// - На волатильных активах

package oscillators

import (
	"bt/internal"
	"fmt"
)

type RsiOscillatorStrategy struct{}

func (s *RsiOscillatorStrategy) Name() string {
	return "rsi_oscillator"
}

func (s *RsiOscillatorStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {

	period := params.RsiPeriod
	if period == 0 {
		period = 14 // default
	}

	rsiValues := internal.CalculateRSICommon(candles, period)
	if rsiValues == nil {
		return make([]internal.SignalType, len(candles))
	}

	buyThreshold := params.RsiBuyThreshold
	sellThreshold := params.RsiSellThreshold
	if buyThreshold == 0 {
		buyThreshold = 30
	}
	if sellThreshold == 0 {
		sellThreshold = 70
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := period; i < len(candles); i++ {
		rsi := rsiValues[i]

		if !inPosition && rsi < buyThreshold {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		if inPosition && rsi > sellThreshold {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *RsiOscillatorStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{
		RsiPeriod:        14,
		RsiBuyThreshold:  30,
		RsiSellThreshold: 70,
	}
	bestProfit := -1.0

	generator := s.GenerateSignals

	// Простой grid search по порогам
	for rsip := 10; rsip <= 20; rsip += 1 {
		for buy := 10.0; buy <= 35.0; buy += 1 {
			for sell := 65.0; sell <= 80.0; sell += 1 {
				params := internal.StrategyParams{
					RsiPeriod:        rsip,
					RsiBuyThreshold:  buy,
					RsiSellThreshold: sell,
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

	fmt.Printf("Лучшие параметры %d %f %f\n", bestParams.RsiPeriod, bestParams.RsiBuyThreshold, bestParams.RsiSellThreshold)

	return bestParams
}

func init() {
	internal.RegisterStrategy("rsi_oscillator", &RsiOscillatorStrategy{})
}
