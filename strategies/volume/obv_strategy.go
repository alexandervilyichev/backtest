// On-Balance Volume (OBV) Strategy
//
// Описание стратегии:
// On-Balance Volume (OBV) - это кумулятивный индикатор, который измеряет давление покупок и продаж.
// OBV прибавляет объем на дни роста и вычитает объем на дни падения. Идея в том, что объем
// предшествует движению цены: если OBV растет, происходит накопление, если падает - распределение.
//
// Как работает:
// - Рассчитывается OBV как кумулятивная сумма объемов с учетом направления движения цены
// - Покупка: когда OBV растет и цена выше предыдущей (подтверждение тренда)
// - Продажа: когда OBV падает или происходит дивергенция с ценой
// - Дополнительно можно использовать дивергенции между OBV и ценой
//
// Параметры:
// - OBVPeriod: период для расчета OBV (обычно весь доступный период)
// - OBVMultiplier: множитель для определения значимых изменений OBV
// - UseDivergence: использовать ли дивергенции для сигналов
//
// Сильные стороны:
// - Учитывает объем как подтверждение силы движения
// - Хорошо работает в трендовых рынках
// - Может предсказывать развороты через дивергенции
// - Логичная идея: объем подтверждает цену
//
// Слабые стороны:
// - Может давать ложные сигналы в боковых рынках
// - Зависит от качества данных объема
// - Требует достаточной истории для расчета
// - Дивергенции не всегда приводят к развороту
//
// Лучшие условия для применения:
// - Трендовые рынки с четким направлением
// - Акции с хорошей ликвидностью
// - В сочетании с другими индикаторами
// - Среднесрочная торговля

package volume

import (
	"bt/internal"
	"log"
	"strconv"
)

type OBVStrategy struct{}

func (s *OBVStrategy) Name() string {
	return "obv_strategy"
}

// calculateOBV вычисляет On-Balance Volume
func calculateOBV(candles []internal.Candle) []float64 {
	if len(candles) < 2 {
		return nil
	}

	obv := make([]float64, len(candles))
	obv[0] = 0

	for i := 1; i < len(candles); i++ {
		currentVol, err := strconv.ParseFloat(candles[i].Volume, 64)
		if err != nil {
			log.Printf("Предупреждение: некорректный объем на свече %d: %s, используем 0", i, candles[i].Volume)
			currentVol = 0
		}

		currentClose := candles[i].Close.ToFloat64()
		previousClose := candles[i-1].Close.ToFloat64()

		if currentClose > previousClose {
			// Цена выросла - добавляем объем
			obv[i] = obv[i-1] + currentVol
		} else if currentClose < previousClose {
			// Цена упала - вычитаем объем
			obv[i] = obv[i-1] - currentVol
		} else {
			// Цена не изменилась - OBV не меняется
			obv[i] = obv[i-1]
		}
	}

	return obv
}

// detectOBVDivergence обнаруживает дивергенции между OBV и ценой
func detectOBVDivergence(candles []internal.Candle, obv []float64, lookback int) (bool, bool) {
	if len(candles) < lookback+2 || len(obv) < lookback+2 {
		return false, false
	}

	// Ищем последние максимумы/минимумы цены и OBV
	priceHigh := candles[len(candles)-1].Close.ToFloat64()
	priceLow := candles[len(candles)-1].Close.ToFloat64()
	obvHigh := obv[len(obv)-1]
	obvLow := obv[len(obv)-1]

	// Ищем экстремумы в lookback периоде
	for i := len(candles) - lookback; i < len(candles)-1; i++ {
		if candles[i].Close.ToFloat64() > priceHigh {
			priceHigh = candles[i].Close.ToFloat64()
		}
		if candles[i].Close.ToFloat64() < priceLow {
			priceLow = candles[i].Close.ToFloat64()
		}
		if obv[i] > obvHigh {
			obvHigh = obv[i]
		}
		if obv[i] < obvLow {
			obvLow = obv[i]
		}
	}

	currentPrice := candles[len(candles)-1].Close.ToFloat64()
	currentOBV := obv[len(obv)-1]

	// Медвежья дивергенция: цена делает новый максимум, OBV - нет
	bearishDivergence := currentPrice > priceHigh && currentOBV < obvHigh

	// Бычья дивергенция: цена делает новый минимум, OBV - нет
	bullishDivergence := currentPrice < priceLow && currentOBV > obvLow

	return bullishDivergence, bearishDivergence
}

func (s *OBVStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	// Устанавливаем значения по умолчанию
	obvPeriod := params.OBVPeriod
	if obvPeriod == 0 {
		obvPeriod = 20 // разумное значение по умолчанию
	}

	obvMultiplier := params.OBVMultiplier
	if obvMultiplier == 0 {
		obvMultiplier = 1.0 // значение по умолчанию
	}

	useDivergence := params.UseDivergence

	for i := range candles {
		if i < 2 {
			signals[i] = internal.HOLD
			continue
		}

		// Рассчитываем OBV для текущего момента
		obv := calculateOBV(candles[:i+1])
		if len(obv) < 2 {
			signals[i] = internal.HOLD
			continue
		}

		currentOBV := obv[len(obv)-1]
		previousOBV := obv[len(obv)-2]

		currentPrice := candles[i].Close.ToFloat64()
		previousPrice := candles[i-1].Close.ToFloat64()

		// Проверяем дивергенции если включены
		var bullishDiv, bearishDiv bool
		if useDivergence && i >= obvPeriod {
			bullishDiv, bearishDiv = detectOBVDivergence(candles[:i+1], obv, obvPeriod)
		}

		// BUY сигналы:
		// 1. OBV растет и цена растет (подтверждение тренда)
		// 2. Бычья дивергенция
		// 3. OBV значительно вырос (мультипликатор)
		buySignal := (!inPosition && currentOBV > previousOBV && currentPrice > previousPrice) ||
			(!inPosition && bullishDiv) ||
			(!inPosition && currentOBV > previousOBV*obvMultiplier)

		if buySignal {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		// SELL сигналы:
		// 1. OBV падает
		// 2. Медвежья дивергенция
		// 3. Цена падает значительно
		sellSignal := (inPosition && currentOBV < previousOBV) ||
			(inPosition && bearishDiv) ||
			(inPosition && currentPrice < previousPrice*0.98) // 2% падение цены

		if sellSignal {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *OBVStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{
		OBVPeriod:     10,
		OBVMultiplier: 1.0,
		UseDivergence: false,
	}
	bestProfit := -1.0

	generator := s.GenerateSignals

	// Оптимизируем период OBV
	for period := 5; period <= 50; period += 5 {
		// Оптимизируем мультипликатор
		for mult := 1.0; mult <= 2.0; mult += 0.1 {
			// Оптимизируем использование дивергенций
			for useDiv := 0; useDiv <= 1; useDiv++ {
				params := internal.StrategyParams{
					OBVPeriod:     period,
					OBVMultiplier: mult,
					UseDivergence: useDiv == 1,
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

	return bestParams
}

func init() {
	// internal.RegisterStrategy("obv_strategy", &OBVStrategy{})
}
