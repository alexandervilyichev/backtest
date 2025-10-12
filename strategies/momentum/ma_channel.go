// strategies/ma_channel.go

// Moving Average Channel Strategy
//
// Описание стратегии:
// Стратегия использует коридор, построенный на основе двух скользящих средних с разными периодами.
// Создается канал, где верхняя граница находится выше медленной SMA, а нижняя - ниже.
// Стратегия торгует пробои этих границ как сигналы сильного momentum.
//
// Как работает:
// - Рассчитывается быстрая SMA (fastPeriod) и медленная SMA (slowPeriod)
// - Разница между SMA умножается на multiplier для создания ширины канала
// - Верхний канал = медленная SMA + (быстрая SMA - медленная SMA) × multiplier
// - Нижний канал = медленная SMA - (быстрая SMA - медленная SMA) × multiplier
// - Покупка: при пробое цены выше верхнего канала (бычий breakout)
// - Продажа: при пробое цены ниже нижнего канала (медвежий breakout)
//
// Параметры:
// - MAChannelFastPeriod: период быстрой SMA (обычно 10)
// - MAChannelSlowPeriod: период медленной SMA (обычно 20)
// - MAChannelMultiplier: множитель ширины канала (обычно 0.5-2.0)
//
// Сильные стороны:
// - Ловит сильные движения и пробои
// - Хорошо работает в трендовых рынках
// - Учитывает momentum через пробои канала
// - Может быть адаптирована под разные рыночные условия
//
// Слабые стороны:
// - Может давать много ложных сигналов в боковых рынках
// - Чувствительна к выбору ширины канала (multiplier)
// - В волатильных условиях может генерировать слишком много сигналов
// - Требует хорошего money management из-за потенциальных убытков
//
// Лучшие условия для применения:
// - Трендовые рынки с сильными движениями
// - Волатильные активы с четкими пробоями
// - Средне- и долгосрочная торговля
// - В сочетании с фильтрами объема или волатильности

package momentum

import (
	"bt/internal"
	"fmt"
)

type MAChannelStrategy struct{}

func (s *MAChannelStrategy) Name() string {
	return "ma_channel"
}

func (s *MAChannelStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	fastPeriod := params.MAChannelFastPeriod
	slowPeriod := params.MAChannelSlowPeriod
	multiplier := params.MAChannelMultiplier

	if fastPeriod == 0 {
		fastPeriod = 10 // default
	}
	if slowPeriod == 0 {
		slowPeriod = 20 // default
	}
	if multiplier == 0 {
		multiplier = 1.0 // default
	}

	upperChannel, lowerChannel := internal.CalculateMAChannel(candles, fastPeriod, slowPeriod, multiplier)
	if upperChannel == nil || lowerChannel == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := slowPeriod; i < len(candles); i++ {
		closePrice := candles[i].Close.ToFloat64()
		upper := upperChannel[i]
		lower := lowerChannel[i]

		if upper == 0 || lower == 0 {
			signals[i] = internal.HOLD
			continue
		}

		if !inPosition {
			// Buy when price breaks above upper channel
			if closePrice > upper {
				signals[i] = internal.BUY
				inPosition = true
				continue
			}
		} else {
			// Sell when price breaks below lower channel
			if closePrice < lower {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *MAChannelStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{
		MAChannelFastPeriod: 10,
		MAChannelSlowPeriod: 20,
		MAChannelMultiplier: 1.0,
	}
	bestProfit := -1.0

	generator := s.GenerateSignals

	// Grid search по параметрам
	for fast := 5; fast <= 15; fast += 2 {
		for slow := 15; slow <= 30; slow += 5 {
			for mult := 0.5; mult <= 2.0; mult += 0.25 {
				if fast >= slow {
					continue // fast period must be less than slow period
				}

				params := internal.StrategyParams{
					MAChannelFastPeriod: fast,
					MAChannelSlowPeriod: slow,
					MAChannelMultiplier: mult,
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

	fmt.Printf("Лучшие параметры MA Channel: fast=%d, slow=%d, multiplier=%.2f\n",
		bestParams.MAChannelFastPeriod, bestParams.MAChannelSlowPeriod, bestParams.MAChannelMultiplier)

	return bestParams
}

func init() {
	// internal.RegisterStrategy("ma_channel", &MAChannelStrategy{})
}
