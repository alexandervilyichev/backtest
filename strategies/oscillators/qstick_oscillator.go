// Qstick Oscillator Strategy - Улучшенная версия с SOLID архитектурой
//
// Описание стратегии:
// Qstick - индикатор момента, который определяет тренд актива путем расчета SMA разницы между ценой закрытия и открытия.
// Qstick показывает давление покупателей и продавцов на основе внутридневных изменений цены.
//
// УЛУЧШЕНИЯ В ЭТОЙ ВЕРСИИ:
// - Добавлены фильтры ложных сигналов
// - Проверка направления тренда перед входом
// - Механизм стоп-лосса и тейк-профита
// - Более консервативный подход к входу в позиции
// - Улучшенная оптимизация параметров
// - Фильтрация по волатильности
// - SOLID архитектура с типизированным конфигом
//
// Как работает:
// - Рассчитывается разница между ценой закрытия и открытия для каждой свечи (Close - Open)
// - Вычисляется SMA этой разницы за заданный период
// - Qstick выше нуля указывает на растущее давление покупателей
// - Qstick ниже нуля указывает на растущее давление продавцов
// - Покупка: когда Qstick поднимается выше уровня покупки И подтверждается трендом
// - Продажа: когда Qstick опускается ниже уровня продажи И подтверждается трендом
//
// Параметры (QStickConfig):
// - Period: период расчета SMA разницы (обычно 8-21)
// - BuyThreshold: уровень для покупки (обычно -0.5 до 0)
// - SellThreshold: уровень для продажи (обычно 0.5 до 1.5)
// - StopLossPercent: процент стоп-лосса (обычно 2-5%)
// - TakeProfitPercent: процент тейк-профита (обычно 3-8%)
// - VolatilityFilter: минимальная волатильность для входа (0.001-0.01)
//
// Сильные стороны:
// - Простота расчета и понимания
// - Хорошо показывает давление покупателей/продавцов
// - Работает на разных таймфреймах
// - Не требует сложных расчетов
// - Хорошо фильтрует рыночный шум через SMA
// - Улучшенная фильтрация ложных сигналов
//
// Слабые стороны:
// - Может запаздывать в быстрых движениях рынка
// - Зависит от правильного выбора периода
// - Не учитывает объем торгов
//
// Лучшие условия для применения:
// - Трендовые рынки с четким направлением
// - Среднесрочная торговля
// - Комбинация с объемными индикаторами
// - На активах с хорошей ликвидностью и волатильностью

package oscillators

import (
	"bt/internal"
	"errors"
	"fmt"
)

type QStickConfig struct {
	Period            int     `json:"period"`
	BuyThreshold      float64 `json:"buy_threshold"`
	SellThreshold     float64 `json:"sell_threshold"`
	StopLossPercent   float64 `json:"stop_loss_percent"`
	TakeProfitPercent float64 `json:"take_profit_percent"`
	VolatilityFilter  float64 `json:"volatility_filter"`
}

func (c *QStickConfig) Validate() error {
	if c.Period <= 0 {
		return errors.New("period must be positive")
	}
	if c.BuyThreshold >= c.SellThreshold {
		return errors.New("buy threshold must be less than sell threshold")
	}
	if c.StopLossPercent <= 0 {
		return errors.New("stop loss percent must be positive")
	}
	if c.TakeProfitPercent <= 0 {
		return errors.New("take profit percent must be positive")
	}
	if c.VolatilityFilter < 0 {
		return errors.New("volatility filter must be non-negative")
	}
	return nil
}

func (c *QStickConfig) DefaultConfigString() string {
	return fmt.Sprintf("QStick(period=%d, buy_thresh=%.2f, sell_thresh=%.2f, sl=%.1f%%, tp=%.1f%%, vol_filt=%.4f)",
		c.Period, c.BuyThreshold, c.SellThreshold, c.StopLossPercent, c.TakeProfitPercent, c.VolatilityFilter)
}

type QstickOscillatorStrategy struct{}

func (s *QstickOscillatorStrategy) Name() string {
	return "qstick_oscillator"
}

// calculateQstickValues рассчитывает значения Qstick индикатора
// Qstick = SMA(Close - Open) за период
func calculateQstickValues(candles []internal.Candle, period int) []float64 {
	if len(candles) < period {
		return nil
	}

	qstick := make([]float64, len(candles))

	// Первые period-1 значений — не определены
	for i := 0; i < period-1; i++ {
		qstick[i] = 0
	}

	// Рассчитываем Qstick для каждой свечи начиная с позиции period-1
	for i := period - 1; i < len(candles); i++ {
		var sum float64

		// Суммируем разницы (Close - Open) за период
		for j := i - period + 1; j <= i; j++ {
			close := candles[j].Close.ToFloat64()
			open := candles[j].Open.ToFloat64()
			sum += (close - open)
		}

		// Qstick = SMA разницы
		qstick[i] = sum / float64(period)
	}

	return qstick
}

// calculateTrendDirection определяет направление тренда с помощью линейной регрессии
func calculateTrendDirection(candles []internal.Candle, period int) []float64 {
	if len(candles) < period {
		return nil
	}

	trend := make([]float64, len(candles))

	// Первые period-1 значений — не определены
	for i := 0; i < period-1; i++ {
		trend[i] = 0
	}

	for i := period - 1; i < len(candles); i++ {
		var sumX, sumY, sumXY, sumXX float64
		n := float64(period)

		// Рассчитываем линейную регрессию
		for j := i - period + 1; j <= i; j++ {
			x := float64(j - (i - period + 1))
			y := candles[j].Close.ToFloat64()

			sumX += x
			sumY += y
			sumXY += x * y
			sumXX += x * x
		}

		// Наклон линии тренда (slope)
		denominator := n*sumXX - sumX*sumX
		if denominator == 0 {
			trend[i] = 0
		} else {
			slope := (n*sumXY - sumX*sumY) / denominator
			trend[i] = slope
		}
	}

	return trend
}

func (s *QstickOscillatorStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	period := params.QstickPeriod
	if period == 0 {
		period = 8 // стандартный период Qstick
	}

	buyThreshold := params.QstickBuyThreshold
	sellThreshold := params.QstickSellThreshold
	if buyThreshold == 0 {
		buyThreshold = -0.5 // стандартный уровень покупки
	}
	if sellThreshold == 0 {
		sellThreshold = 0.5 // стандартный уровень продажи
	}

	// Параметры фильтрации
	stopLossPercent := params.StopLossPercent
	takeProfitPercent := params.TakeProfitPercent
	volatilityFilter := params.VolatilityFilter

	if stopLossPercent == 0 {
		stopLossPercent = 3.0 // 3% стоп-лосс по умолчанию
	}
	if takeProfitPercent == 0 {
		takeProfitPercent = 6.0 // 6% тейк-профит по умолчанию
	}
	if volatilityFilter == 0 {
		volatilityFilter = 0.001 // минимальная волатильность по умолчанию
	}

	qstickValues := calculateQstickValues(candles, period)
	if qstickValues == nil {
		return make([]internal.SignalType, len(candles))
	}

	// Дополнительные индикаторы для фильтрации
	volatilityValues := internal.CalculateVolatilityQstick(candles, period)
	trendValues := calculateTrendDirection(candles, period*2) // Более длинный период для тренда

	signals := make([]internal.SignalType, len(candles))
	inPosition := false
	entryPrice := 0.0
	stopLossPrice := 0.0
	takeProfitPrice := 0.0

	for i := period; i < len(candles); i++ {
		currentPrice := candles[i].Close.ToFloat64()
		qstick := qstickValues[i]

		// Проверяем условия выхода из позиции (стоп-лосс/тейк-профит)
		if inPosition {
			// Обновляем стоп-лосс и тейк-профит на основе текущей цены для trailing stop
			newStopLoss := entryPrice * (1.0 - stopLossPercent/100.0)
			newTakeProfit := entryPrice * (1.0 + takeProfitPercent/100.0)

			// Trailing stop: улучшаем стоп-лосс если цена выросла
			if currentPrice > entryPrice {
				trailingStopLoss := currentPrice * (1.0 - stopLossPercent/100.0)
				if trailingStopLoss > stopLossPrice {
					newStopLoss = trailingStopLoss
				}
			}

			if currentPrice <= newStopLoss || currentPrice >= newTakeProfit {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}

			// Обновляем уровни для следующей итерации
			stopLossPrice = newStopLoss
			_ = takeProfitPrice // Используем переменную для избежания ошибки компиляции
		}

		// Пропускаем если волатильность слишком низкая
		if volatilityValues[i] < volatilityFilter {
			signals[i] = internal.HOLD
			continue
		}

		// BUY: Улучшенная логика с фильтрами
		if !inPosition && qstick > buyThreshold {
			// Дополнительные фильтры для подтверждения сигнала
			trendConfirmed := i > 0 && trendValues[i] > 0                          // Положительный тренд
			priceGrowing := i > 0 && currentPrice > candles[i-1].Close.ToFloat64() // Цена растет
			qstickGrowing := i > period && qstick > qstickValues[i-1]              // Qstick растет

			// Входим в позицию только если все условия соблюдены
			if trendConfirmed && priceGrowing && qstickGrowing {
				signals[i] = internal.BUY
				inPosition = true
				entryPrice = currentPrice
				stopLossPrice = currentPrice * (1.0 - stopLossPercent/100.0)
				takeProfitPrice = currentPrice * (1.0 + takeProfitPercent/100.0)
				continue
			}
		}

		// SELL: Улучшенная логика с фильтрами
		if inPosition && qstick < sellThreshold {
			// Дополнительные фильтры для подтверждения сигнала
			trendConfirmed := i > 0 && trendValues[i] < 0                          // Отрицательный тренд
			priceFalling := i > 0 && currentPrice < candles[i-1].Close.ToFloat64() // Цена падает
			qstickFalling := i > period && qstick < qstickValues[i-1]              // Qstick падает

			// Выходим из позиции только если есть подтверждение
			if trendConfirmed && priceFalling && qstickFalling {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *QstickOscillatorStrategy) DefaultConfig() internal.StrategyConfig {
	return &QStickConfig{
		Period:            12,
		BuyThreshold:      -1.5,
		SellThreshold:     0.2,
		StopLossPercent:   1.0,
		TakeProfitPercent: 6.0,
		VolatilityFilter:  0.0045,
	}
}

func (s *QstickOscillatorStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	qstickConfig, ok := config.(*QStickConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := qstickConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	qstickValues := calculateQstickValues(candles, qstickConfig.Period)
	if qstickValues == nil {
		return make([]internal.SignalType, len(candles))
	}

	// Дополнительные индикаторы для фильтрации
	volatilityValues := internal.CalculateVolatilityQstick(candles, qstickConfig.Period)
	trendValues := calculateTrendDirection(candles, qstickConfig.Period*2) // Более длинный период для тренда

	signals := make([]internal.SignalType, len(candles))
	inPosition := false
	entryPrice := 0.0
	stopLossPrice := 0.0
	takeProfitPrice := 0.0

	for i := qstickConfig.Period; i < len(candles); i++ {
		currentPrice := candles[i].Close.ToFloat64()
		qstick := qstickValues[i]

		// Проверяем условия выхода из позиции (стоп-лосс/тейк-профит)
		if inPosition {
			// Обновляем стоп-лосс и тейк-профит на основе текущей цены для trailing stop
			newStopLoss := entryPrice * (1.0 - qstickConfig.StopLossPercent/100.0)
			newTakeProfit := entryPrice * (1.0 + qstickConfig.TakeProfitPercent/100.0)

			// Trailing stop: улучшаем стоп-лосс если цена выросла
			if currentPrice > entryPrice {
				trailingStopLoss := currentPrice * (1.0 - qstickConfig.StopLossPercent/100.0)
				if trailingStopLoss > stopLossPrice {
					newStopLoss = trailingStopLoss
				}
			}

			if currentPrice <= newStopLoss || currentPrice >= newTakeProfit {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}

			// Обновляем уровни для следующей итерации
			stopLossPrice = newStopLoss
			_ = takeProfitPrice // Используем переменную для избежания ошибки компиляции
		}

		// Пропускаем если волатильность слишком низкая
		if volatilityValues[i] < qstickConfig.VolatilityFilter {
			signals[i] = internal.HOLD
			continue
		}

		// BUY: Улучшенная логика с фильтрами
		if !inPosition && qstick > qstickConfig.BuyThreshold {
			// Дополнительные фильтры для подтверждения сигнала
			trendConfirmed := i > 0 && trendValues[i] > 0                          // Положительный тренд
			priceGrowing := i > 0 && currentPrice > candles[i-1].Close.ToFloat64() // Цена растет
			qstickGrowing := i > qstickConfig.Period && qstick > qstickValues[i-1] // Qstick растет

			// Входим в позицию только если все условия соблюдены
			if trendConfirmed && priceGrowing && qstickGrowing {
				signals[i] = internal.BUY
				inPosition = true
				entryPrice = currentPrice
				stopLossPrice = currentPrice * (1.0 - qstickConfig.StopLossPercent/100.0)
				takeProfitPrice = currentPrice * (1.0 + qstickConfig.TakeProfitPercent/100.0)
				continue
			}
		}

		// SELL: Улучшенная логика с фильтрами
		if inPosition && qstick < qstickConfig.SellThreshold {
			// Дополнительные фильтры для подтверждения сигнала
			trendConfirmed := i > 0 && trendValues[i] < 0                          // Отрицательный тренд
			priceFalling := i > 0 && currentPrice < candles[i-1].Close.ToFloat64() // Цена падает
			qstickFalling := i > qstickConfig.Period && qstick < qstickValues[i-1] // Qstick падает

			// Выходим из позиции только если есть подтверждение
			if trendConfirmed && priceFalling && qstickFalling {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *QstickOscillatorStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{
		QstickPeriod:        12,
		QstickBuyThreshold:  -1.5,
		QstickSellThreshold: 0.2,
		StopLossPercent:     1.0,
		TakeProfitPercent:   6.0,
		VolatilityFilter:    0.0045,
	}
	bestProfit := -1.0

	/*generator := s.GenerateSignals

	// Расширенный grid search с новыми параметрами
	for period := 12; period <= 19; period += 1 {
		for buyThreshold := -2.0; buyThreshold <= -1.0; buyThreshold += 0.2 {
			for sellThreshold := 0.2; sellThreshold <= 1.0; sellThreshold += 0.2 {
				for stopLoss := 1.0; stopLoss <= 3.0; stopLoss += 1.0 {
					for takeProfit := 4.0; takeProfit <= 9.0; takeProfit += 1.0 {
						for volatilityFilter := 0.003; volatilityFilter <= 0.005; volatilityFilter += 0.0005 {
							params := internal.StrategyParams{
								QstickPeriod:        period,
								QstickBuyThreshold:  buyThreshold,
								QstickSellThreshold: sellThreshold,
								StopLossPercent:     stopLoss,
								TakeProfitPercent:   takeProfit,
								VolatilityFilter:    volatilityFilter,
							}
							signals := generator(candles, params)
							result := internal.Backtest(candles, signals, 0.01) // 0.01 units проскальзывание

							// Учитываем не только прибыль, но и количество сделок
							// Стратегия должна быть прибыльной И эффективной (не слишком много сделок)
							efficiency := result.TotalProfit
							if result.TradeCount > 1000 {
								efficiency *= 0.8 // Штраф за слишком частую торговлю
							}
							if result.TradeCount < 10 {
								efficiency *= 0.9 // Штраф за слишком редкую торговлю
							}

							if efficiency > bestProfit {
								bestProfit = efficiency
								bestParams = params
							}
						}
					}
				}
			}
		}
	}
	*/

	fmt.Printf("Лучшие параметры Qstick:\n")
	fmt.Printf("  Период: %d\n", bestParams.QstickPeriod)
	fmt.Printf("  Порог покупки: %.2f\n", bestParams.QstickBuyThreshold)
	fmt.Printf("  Порог продажи: %.2f\n", bestParams.QstickSellThreshold)
	fmt.Printf("  Стоп-лосс: %.1f%%\n", bestParams.StopLossPercent)
	fmt.Printf("  Тейк-профит: %.1f%%\n", bestParams.TakeProfitPercent)
	fmt.Printf("  Фильтр волатильности: %.4f\n", bestParams.VolatilityFilter)
	fmt.Printf("  Ожидаемый профит: %.4f\n", bestProfit)

	return bestParams
}

func (s *QstickOscillatorStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := &QStickConfig{
		Period:            12,
		BuyThreshold:      -1.5,
		SellThreshold:     0.2,
		StopLossPercent:   1.0,
		TakeProfitPercent: 6.0,
		VolatilityFilter:  0.0045,
	}
	bestProfit := -1.0

	// Расширенный grid search с новыми параметрами
	for period := 12; period <= 19; period += 1 {
		for buyThreshold := -2.0; buyThreshold <= -1.0; buyThreshold += 0.2 {
			for sellThreshold := 0.2; sellThreshold <= 1.0; sellThreshold += 0.2 {
				for stopLoss := 1.0; stopLoss <= 3.0; stopLoss += 1.0 {
					for takeProfit := 4.0; takeProfit <= 9.0; takeProfit += 1.0 {
						for volatilityFilter := 0.003; volatilityFilter <= 0.005; volatilityFilter += 0.0005 {
							config := &QStickConfig{
								Period:            period,
								BuyThreshold:      buyThreshold,
								SellThreshold:     sellThreshold,
								StopLossPercent:   stopLoss,
								TakeProfitPercent: takeProfit,
								VolatilityFilter:  volatilityFilter,
							}
							if config.Validate() != nil {
								continue
							}

							signals := s.GenerateSignalsWithConfig(candles, config)
							result := internal.Backtest(candles, signals, 0.01) // 0.01 units проскальзывание

							// Учитываем не только прибыль, но и количество сделок
							// Стратегия должна быть прибыльной И эффективной (не слишком много сделок)
							efficiency := result.TotalProfit
							if result.TradeCount > 1000 {
								efficiency *= 0.8 // Штраф за слишком частую торговлю
							}
							if result.TradeCount < 10 {
								efficiency *= 0.9 // Штраф за слишком редкую торговлю
							}

							if efficiency > bestProfit {
								bestProfit = efficiency
								bestConfig = config
							}
						}
					}
				}
			}
		}
	}

	fmt.Printf("Лучшие параметры Qstick (SOLID):\n")
	fmt.Printf("  Период: %d\n", bestConfig.Period)
	fmt.Printf("  Порог покупки: %.2f\n", bestConfig.BuyThreshold)
	fmt.Printf("  Порог продажи: %.2f\n", bestConfig.SellThreshold)
	fmt.Printf("  Стоп-лосс: %.1f%%\n", bestConfig.StopLossPercent)
	fmt.Printf("  Тейк-профит: %.1f%%\n", bestConfig.TakeProfitPercent)
	fmt.Printf("  Фильтр волатильности: %.4f\n", bestConfig.VolatilityFilter)
	fmt.Printf("  Ожидаемый профит: %.4f\n", bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("qstick_oscillator", &QstickOscillatorStrategy{})
}
