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
	"errors"
	"fmt"
)

type OBVConfig struct {
	Period             int     `json:"period"`
	Multiplier         float64 `json:"multiplier"`
	UseDivergence      bool    `json:"use_divergence"`
	DivergenceLookback int     `json:"divergence_lookback"`
	PriceDropThreshold float64 `json:"price_drop_threshold"`
	OBVDropMultiplier  float64 `json:"obv_drop_multiplier"`
}

func (c *OBVConfig) Validate() error {
	if c.Period <= 0 {
		return errors.New("period must be positive")
	}
	if c.Multiplier <= 0 {
		return errors.New("multiplier must be positive")
	}
	if c.DivergenceLookback <= 0 {
		return errors.New("divergence_lookback must be positive")
	}
	if c.PriceDropThreshold <= 0 {
		return errors.New("price_drop_threshold must be positive")
	}
	if c.OBVDropMultiplier <= 0 {
		return errors.New("obv_drop_multiplier must be positive")
	}
	return nil
}

func (c *OBVConfig) DefaultConfigString() string {
	return fmt.Sprintf("OBV(period=%d, mult=%.2f, div=%t, div_lookback=%d, price_drop=%.3f, obv_drop_mult=%.2f)",
		c.Period, c.Multiplier, c.UseDivergence, c.DivergenceLookback, c.PriceDropThreshold, c.OBVDropMultiplier)
}

type OBVStrategy struct{ internal.BaseConfig }

func (s *OBVStrategy) Name() string {
	return "obv_strategy"
}

// detectOBVDivergence обнаруживает дивергенции между OBV и ценой
func detectOBVDivergence(candles []internal.Candle, obv []float64, lookback int) (bool, bool) {
	if len(candles) < lookback+2 || len(obv) < lookback+2 {
		return false, false
	}

	// Ищем максимумы и минимумы цены в lookback периоде
	priceHigh := candles[len(candles)-lookback-1].Close.ToFloat64()
	priceLow := candles[len(candles)-lookback-1].Close.ToFloat64()

	// Ищем максимумы и минимумы OBV в lookback периоде
	obvHigh := obv[len(obv)-lookback-1]
	obvLow := obv[len(obv)-lookback-1]

	// Ищем экстремумы в lookback периоде (исключая текущую свечу)
	for i := len(candles) - lookback; i < len(candles)-1; i++ {
		price := candles[i].Close.ToFloat64()
		if price > priceHigh {
			priceHigh = price
		}
		if price < priceLow {
			priceLow = price
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

	// Медвежья дивергенция: цена делает новый максимум, но OBV НЕ делает новый максимум
	bearishDivergence := currentPrice > priceHigh && currentOBV < obvHigh

	// Бычья дивергенция: цена делает новый минимум, но OBV НЕ делает новый минимум
	bullishDivergence := currentPrice < priceLow && currentOBV > obvLow

	return bullishDivergence, bearishDivergence
}

func (s *OBVStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	obvConfig, ok := config.(*OBVConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := obvConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	// Precompute OBV and its deltas for the whole series (config version)
	obvAll := internal.CalculateOBV(candles)
	if len(obvAll) < 2 {
		return signals
	}
	dObv := make([]float64, len(candles))
	absDObv := make([]float64, len(candles))
	for i := 1; i < len(candles); i++ {
		d := obvAll[i] - obvAll[i-1]
		dObv[i] = d
		if d < 0 {
			absDObv[i] = -d
		} else {
			absDObv[i] = d
		}
	}
	avgAbsDObv := internal.CalculateSMACommonForValues(absDObv, obvConfig.Period)

	for i := range candles {
		if i < 2 {
			signals[i] = internal.HOLD
			continue
		}

		// Use precomputed OBV values
		deltaOBV := dObv[i]
		avgAbs := 0.0
		if avgAbsDObv != nil {
			avgAbs = avgAbsDObv[i]
		}

		currentPrice := candles[i].Close.ToFloat64()
		previousPrice := candles[i-1].Close.ToFloat64()

		// Проверяем дивергенции если включены
		var bullishDiv, bearishDiv bool
		if obvConfig.UseDivergence && i >= obvConfig.DivergenceLookback {
			bullishDiv, bearishDiv = detectOBVDivergence(candles[:i+1], obvAll[:i+1], obvConfig.DivergenceLookback)
		}

		// BUY сигналы (улучшенная логика):
		// 1. Сильный рост OBV + умеренный рост цены (подтверждение тренда)
		// 2. Бычья дивергенция
		// 3. Значительный рост OBV относительно среднего уровня

		// BUY: strong OBV thrust relative to recent average + price confirmation, or bullish divergence
		strongOBVUptrend := !inPosition && i >= obvConfig.Period-1 && avgAbs > 0 && deltaOBV > obvConfig.Multiplier*avgAbs && currentPrice > previousPrice
		buySignal := strongOBVUptrend || (!inPosition && bullishDiv)

		if buySignal {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		// SELL сигналы (улучшенная логика):
		// 1. Значительное падение OBV
		// 2. Медвежья дивергенция
		// 3. Цена падает значительно
		// 4. Слабый рост OBV при падении цены (отсутствие подтверждения)
		obvDrop := inPosition && i >= obvConfig.Period-1 && avgAbs > 0 && deltaOBV < -obvConfig.OBVDropMultiplier*avgAbs
		priceDrop := inPosition && previousPrice > 0 && (currentPrice-previousPrice)/previousPrice <= -obvConfig.PriceDropThreshold
		weakConfirmation := inPosition && currentPrice < previousPrice && deltaOBV <= 0 // Цена падает, OBV не подтверждает

		sellSignal := obvDrop || priceDrop || weakConfirmation || (inPosition && bearishDiv)

		if sellSignal {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *OBVStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := s.DefaultConfig().(*OBVConfig)
	bestProfit := -1.0

	// Оптимизируем период OBV
	for period := 25; period <= 90; period += 10 {
		// Оптимизируем мультипликатор для покупки
		for mult := 0.5; mult <= 2.5; mult += 0.2 {
			// Оптимизируем использование дивергенций
			for useDiv := 0; useDiv <= 0; useDiv++ {
				// Оптимизируем lookback для дивергенций
				for divLookback := 10; divLookback <= 50; divLookback += 10 {
					// Оптимизируем порог падения цены
					for priceDrop := 0.005; priceDrop <= 0.03; priceDrop += 0.005 {
						// Оптимизируем мультипликатор для падения OBV
						for obvDropMult := 0.5; obvDropMult <= 2.0; obvDropMult += 0.25 {
							config := &OBVConfig{
								Period:             period,
								Multiplier:         mult,
								UseDivergence:      useDiv == 1,
								DivergenceLookback: divLookback,
								PriceDropThreshold: priceDrop,
								OBVDropMultiplier:  obvDropMult,
							}
							if config.Validate() != nil {
								continue
							}

							signals := s.GenerateSignalsWithConfig(candles, config)
							result := internal.Backtest(candles, signals, 0.01) // 0.01 units проскальзывание

							if result.TotalProfit >= bestProfit {
								bestProfit = result.TotalProfit
								bestConfig = config
							}
						}
					}
				}
			}
		}
	}

	fmt.Printf("Лучшие параметры OBV: period=%d, multiplier=%.2f, use_div=%t, div_lookback=%d, price_drop=%.3f, obv_drop_mult=%.2f, профит=%.4f\n",
		bestConfig.Period, bestConfig.Multiplier, bestConfig.UseDivergence, bestConfig.DivergenceLookback, bestConfig.PriceDropThreshold, bestConfig.OBVDropMultiplier, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("obv_strategy", &OBVStrategy{
		BaseConfig: internal.BaseConfig{
			Config: &OBVConfig{
				Period:             20,
				Multiplier:         1.5,
				UseDivergence:      false,
				DivergenceLookback: 20,
				PriceDropThreshold: 0.02,
				OBVDropMultiplier:  1.5,
			},
		},
	})
}
