// strategies/cci_oscillator.go

// Commodity Channel Index (CCI) Strategy
//
// Описание стратегии:
// CCI - осциллятор, измеряющий отклонение цены от ее статистической средней.
// Индикатор показывает, насколько текущая цена отклоняется от средней цены за определенный период.
// CCI считается перекупленным выше +100 и перепроданным ниже -100.
//
// Как работает:
// - Рассчитывается типичная цена: (High + Low + Close) / 3
// - Вычисляется SMA типичных цен за период
// - Рассчитывается среднее отклонение от SMA
// - CCI = (Типичная цена - SMA) / (0.015 × Среднее отклонение)
// - Покупка: когда CCI опускается ниже уровня перепроданности и цена не падает
// - Продажа: когда CCI поднимается выше уровня перекупленности и цена не растет
//
// Параметры:
// - CciPeriod: период расчета CCI (обычно 14-20)
// - CciBuyLevel: уровень перепроданности для покупки (обычно -100)
// - CciSellLevel: уровень перекупленности для продажи (обычно +100)
//
// Сильные стороны:
// - Хорошо определяет экстремальные уровни перекупленности/перепроданности
// - Учитывает волатильность через среднее отклонение
// - Универсален для разных рынков и активов
// - Хорошо работает в трендовых рынках для поиска точек входа
//
// Слабые стороны:
// - Может давать ложные сигналы в боковых рынках
// - Зависит от правильного выбора периода
// - В очень волатильных условиях может генерировать много шума
// - Не является leading индикатором (запаздывает)
//
// Лучшие условия для применения:
// - Трендовые рынки с четкими циклами
// - Среднесрочная торговля
// - Комбинация с трендовыми индикаторами
// - На активах с хорошей волатильностью

package oscillators

import (
	"bt/internal"
	"fmt"
	"math"
)

type CciOscillatorStrategy struct{}

func (s *CciOscillatorStrategy) Name() string {
	return "cci_oscillator"
}

// calculateTypicalPrice — (High + Low + Close) / 3
func calculateTypicalPrice(c internal.Candle) float64 {
	h := c.High.ToFloat64()
	l := c.Low.ToFloat64()
	clo := c.Close.ToFloat64()
	return (h + l + clo) / 3.0
}

// calculateCCI — возвращает массив значений CCI
func calculateCCI(candles []internal.Candle, period int) []float64 {
	if len(candles) < period {
		return nil
	}

	cci := make([]float64, len(candles))

	// Первые period-1 значений — не определены
	for i := 0; i < period-1; i++ {
		cci[i] = 0
	}

	for i := period - 1; i < len(candles); i++ {
		// Берём окно из `period` свечей, заканчивающееся на i
		var tpSum float64
		typicalPrices := make([]float64, 0, period)

		for j := i - period + 1; j <= i; j++ {
			tp := calculateTypicalPrice(candles[j])
			typicalPrices = append(typicalPrices, tp)
			tpSum += tp
		}

		ma := tpSum / float64(period)

		// Рассчитываем Mean Deviation
		var mdSum float64
		for _, tp := range typicalPrices {
			mdSum += math.Abs(tp - ma)
		}
		meanDeviation := mdSum / float64(period)

		// CCI = (TP - MA) / (0.015 * Mean Deviation)
		currentTp := calculateTypicalPrice(candles[i])
		if meanDeviation == 0 {
			cci[i] = 0
		} else {
			cci[i] = (currentTp - ma) / (0.015 * meanDeviation)
		}
	}

	return cci
}

func (s *CciOscillatorStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	period := params.CciPeriod
	if period == 0 {
		period = 20 // стандартный период CCI
	}

	buyLevel := params.CciBuyLevel
	if buyLevel == 0 {
		buyLevel = -100.0
	}

	sellLevel := params.CciSellLevel
	if sellLevel == 0 {
		sellLevel = 100.0
	}

	cciValues := calculateCCI(candles, period)
	if cciValues == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := period; i < len(candles); i++ {
		cci := cciValues[i]

		// BUY: CCI ниже уровня перепроданности
		if !inPosition && cci <= buyLevel {
			// Дополнительная проверка: цена должна расти или быть в боковике
			if i > 0 && candles[i].Close.ToFloat64() >= candles[i-1].Close.ToFloat64() {
				signals[i] = internal.BUY
				inPosition = true
				continue
			}
		}

		// SELL: CCI выше уровня перекупленности
		if inPosition && cci >= sellLevel {
			// Дополнительная проверка: цена должна падать или быть в боковике
			if i > 0 && candles[i].Close.ToFloat64() <= candles[i-1].Close.ToFloat64() {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *CciOscillatorStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{
		CciPeriod:    20,
		CciBuyLevel:  -100,
		CciSellLevel: 100,
	}
	bestProfit := -1.0

	generator := s.GenerateSignals

	// Более широкий и детальный grid search
	for period := 5; period <= 10; period += 1 {
		for buy := -200.0; buy <= -150.0; buy += 5 {
			for sell := 150.0; sell <= 200.0; sell += 5 {
				params := internal.StrategyParams{
					CciPeriod:    period,
					CciBuyLevel:  buy,
					CciSellLevel: sell,
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

	fmt.Printf("Лучшие параметры CCI: период=%d, покупка=%.1f, продажа=%.1f, профит=%.4f\n",
		bestParams.CciPeriod, bestParams.CciBuyLevel, bestParams.CciSellLevel, bestProfit)

	return bestParams
}

func init() {
	internal.RegisterStrategy("cci_oscillator", &CciOscillatorStrategy{})
}
