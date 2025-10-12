// strategies/stochastic_oscillator.go

// Stochastic Oscillator Strategy
//
// Описание стратегии:
// Стратегия использует стохастический осциллятор - momentum индикатор, который сравнивает closing price
// с диапазоном цен за определенный период. Состоит из двух линий: %K (быстрая) и %D (сигнальная).
//
// Как работает:
// - Рассчитывается %K: 100 * (close - lowest_low) / (highest_high - lowest_low)
// - Рассчитывается %D: SMA от %K за сигнальный период
// - Покупка: когда %K пересекает %D снизу вверх, и обе линии ниже уровня перепроданности
// - Продажа: когда %K пересекает %D сверху вниз, и обе линии выше уровня перекупленности
//
// Параметры:
// - StochasticKPeriod: период для расчета %K (обычно 14)
// - StochasticDPeriod: период smoothing для %D (обычно 3)
// - StochasticBuyLevel: уровень перепроданности для покупки (обычно 20)
// - StochasticSellLevel: уровень перекупленности для продажи (обычно 80)
//
// Сильные стороны:
// - Хорошо определяет перекупленность/перепроданность
// - Учитывает momentum и скорость движения цены
// - Реагирует быстрее RSI на изменения
// - Эффективен в ranging рынках
//
// Слабые стороны:
// - Может давать много ложных сигналов в трендовых рынках
// - Чувствителен к выбору периодов
// - В волатильных условиях может генерировать whipsaws
// - Не учитывает общий тренд рынка
//
// Лучшие условия для применения:
// - Боковые/осциллирующие рынки
// - Кратко- и среднесрочная торговля
// - В сочетании с трендовыми индикаторами
// - На активах с четкими циклами

package oscillators

import (
	"bt/internal"
	"fmt"
)

type StochasticOscillatorStrategy struct{}

func (s *StochasticOscillatorStrategy) Name() string {
	return "stochastic_oscillator"
}

func (s *StochasticOscillatorStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	kPeriod := params.StochasticKPeriod
	dPeriod := params.StochasticDPeriod
	if kPeriod == 0 {
		kPeriod = 14 // default
	}
	if dPeriod == 0 {
		dPeriod = 3 // default
	}

	kValues, dValues := internal.CalculateStochastic(candles, kPeriod, dPeriod)
	if kValues == nil || dValues == nil {
		return make([]internal.SignalType, len(candles))
	}

	buyLevel := params.StochasticBuyLevel
	sellLevel := params.StochasticSellLevel
	if buyLevel == 0 {
		buyLevel = 20 // default oversold level
	}
	if sellLevel == 0 {
		sellLevel = 80 // default overbought level
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := kPeriod + dPeriod - 1; i < len(candles); i++ {
		k := kValues[i]
		d := dValues[i]
		kPrev := kValues[i-1]
		dPrev := dValues[i-1]

		if !inPosition {
			// Buy when %K crosses above %D and both are below buy level
			if kPrev <= dPrev && k > d && k < buyLevel && d < buyLevel {
				signals[i] = internal.BUY
				inPosition = true
				continue
			}
		} else {
			// Sell when %K crosses below %D and both are above sell level
			if kPrev >= dPrev && k < d && k > sellLevel && d > sellLevel {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *StochasticOscillatorStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{
		StochasticKPeriod:   14,
		StochasticDPeriod:   3,
		StochasticBuyLevel:  20,
		StochasticSellLevel: 80,
	}
	bestProfit := -1.0

	generator := s.GenerateSignals

	// Grid search по параметрам
	for kPeriod := 10; kPeriod <= 20; kPeriod += 2 {
		for dPeriod := 2; dPeriod <= 5; dPeriod++ {
			for buyLevel := 15.0; buyLevel <= 30.0; buyLevel += 5 {
				for sellLevel := 70.0; sellLevel <= 85.0; sellLevel += 5 {
					params := internal.StrategyParams{
						StochasticKPeriod:   kPeriod,
						StochasticDPeriod:   dPeriod,
						StochasticBuyLevel:  buyLevel,
						StochasticSellLevel: sellLevel,
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

	fmt.Printf("Лучшие параметры stochastic: kPeriod=%d, dPeriod=%d, buyLevel=%.1f, sellLevel=%.1f\n",
		bestParams.StochasticKPeriod, bestParams.StochasticDPeriod,
		bestParams.StochasticBuyLevel, bestParams.StochasticSellLevel)

	return bestParams
}

func init() {
	internal.RegisterStrategy("stochastic_oscillator", &StochasticOscillatorStrategy{})
}
