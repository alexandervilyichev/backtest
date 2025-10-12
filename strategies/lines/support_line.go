// strategies/support_line.go

// Support Line Strategy
//
// Описание стратегии:
// Стратегия основана на концепции уровней поддержки - ценовых уровнях, где спрос превышает предложение,
// что приводит к развороту цены вверх. Стратегия ищет моменты, когда цена приближается к поддержке,
// и открывает длинные позиции с расчетом на отскок.
//
// Как работает:
// - Рассчитывается скользящий минимум (support level) за заданный период lookback
// - Покупка: когда цена закрытия находится вблизи уровня поддержки (ниже support * (1 + buyThreshold))
// - Продажа: когда цена пробивает уровень поддержки снизу (ниже support * (1 - sellThreshold))
// - Дополнительно: фиксация прибыли при росте на 3% от цены входа
//
// Параметры:
// - SupportLookbackPeriod: период для расчета скользящего минимума (обычно 10-30)
// - SupportBuyThreshold: порог приближения к поддержке для покупки (обычно 0.005 = 0.5%)
// - SupportSellThreshold: порог пробоя поддержки для продажи (обычно 0.01 = 1%)
//
// Сильные стороны:
// - Логичная идея: покупка у поддержки с ожиданием отскока
// - Хорошо работает в трендовых рынках с коррекциями
// - Учитывает рыночную психологию (поддержка как уровень спроса)
// - Может быть эффективна в комбинации с другими индикаторами
//
// Слабые стороны:
// - Поддержка может не сработать, особенно в сильных нисходящих трендах
// - Зависит от правильного определения периода lookback
// - Может давать ложные сигналы при пробое поддержки
// - Требует хорошего risk management из-за потенциальных стоп-лоссов
//
// Лучшие условия для применения:
// - Трендовые рынки с коррекциями
// - Средне- и долгосрочная торговля
// - Волатильные активы с четкими уровнями поддержки/сопротивления
// - В сочетании с объемом или momentum индикаторами

package lines

import (
	"bt/internal"
	"fmt"
)

type SupportLineStrategy struct{}

func (s *SupportLineStrategy) Name() string {
	return "support_line"
}

func (s *SupportLineStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	lookback := params.SupportLookbackPeriod
	if lookback == 0 {
		lookback = 20 // default
	}

	supportLevels := internal.CalculateRollingMin(candles, lookback)
	if supportLevels == nil {
		return make([]internal.SignalType, len(candles))
	}

	buyThreshold := params.SupportBuyThreshold
	sellThreshold := params.SupportSellThreshold
	if buyThreshold == 0 {
		buyThreshold = 0.005 // 0.5%
	}
	if sellThreshold == 0 {
		sellThreshold = 0.01 // 1%
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false
	var entryPrice float64

	for i := lookback; i < len(candles); i++ {
		support := supportLevels[i]
		closePrice := candles[i].Close.ToFloat64()

		if !inPosition && closePrice <= support*(1+buyThreshold) {
			signals[i] = internal.BUY
			inPosition = true
			entryPrice = closePrice
			continue
		}

		if inPosition {
			// Sell if price breaks below support
			if closePrice <= support*(1-sellThreshold) {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
			// Take profit if price rises 3% above entry
			if closePrice >= entryPrice*1.03 {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *SupportLineStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{
		SupportLookbackPeriod: 20,
		SupportBuyThreshold:   0.005,
		SupportSellThreshold:  0.01,
	}
	bestProfit := -1.0

	generator := s.GenerateSignals

	// Grid search по параметрам
	for lookback := 10; lookback <= 50; lookback += 5 {
		for buyThresh := 0.001; buyThresh <= 0.02; buyThresh += 0.002 {
			for sellThresh := 0.005; sellThresh <= 0.05; sellThresh += 0.005 {
				params := internal.StrategyParams{
					SupportLookbackPeriod: lookback,
					SupportBuyThreshold:   buyThresh,
					SupportSellThreshold:  sellThresh,
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

	fmt.Printf("Лучшие параметры support: lookback=%d, buyThresh=%.4f, sellThresh=%.4f\n",
		bestParams.SupportLookbackPeriod, bestParams.SupportBuyThreshold, bestParams.SupportSellThreshold)

	return bestParams
}

func init() {
	internal.RegisterStrategy("support_line", &SupportLineStrategy{})
}
