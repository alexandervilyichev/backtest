// Qstick Oscillator Strategy - Улучшенная версия с архитектурой
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
// - архитектура с типизированным конфигом
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

	"github.com/samber/lo"
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

func (c *QStickConfig) String() string {
	return fmt.Sprintf("QStick(period=%d, buy_thresh=%.2f, sell_thresh=%.2f, sl=%.1f%%, tp=%.1f%%, vol_filt=%.4f) ",
		c.Period, c.BuyThreshold, c.SellThreshold, c.StopLossPercent, c.TakeProfitPercent, c.VolatilityFilter)
}

type QStickSignalGenerator struct{}

func NewQStickSignalGenerator() *QStickSignalGenerator {
	return &QStickSignalGenerator{}
}

func (s *QStickSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
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

// PredictNextSignal предсказывает ближайший сигнал QStick
func (s *QStickSignalGenerator) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	qstickConfig, ok := config.(*QStickConfig)
	if !ok {
		return nil
	}

	if err := qstickConfig.Validate(); err != nil {
		return nil
	}

	minCandles := qstickConfig.Period * 3
	if len(candles) < minCandles {
		return nil
	}

	// Вычисляем QStick
	qstickValues := calculateQstickValues(candles, qstickConfig.Period)
	if qstickValues == nil {
		return nil
	}

	// Вычисляем тренд
	trendValues := calculateTrendDirection(candles, qstickConfig.Period*2)
	if trendValues == nil {
		return nil
	}

	currentIdx := len(candles) - 1
	currentQstick := qstickValues[currentIdx]
	currentTrend := trendValues[currentIdx]

	// Анализируем последние несколько значений для определения скорости изменения
	lookback := 5
	if lookback > qstickConfig.Period/2 {
		lookback = qstickConfig.Period / 2
	}
	if lookback < 3 {
		lookback = 3
	}

	if currentIdx < qstickConfig.Period+lookback {
		return nil
	}

	// Вычисляем среднюю скорость изменения QStick
	qstickVelocity := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		qstickChange := qstickValues[idx] - qstickValues[prevIdx]
		qstickVelocity += qstickChange
	}
	qstickVelocity /= float64(lookback - 1)

	// Определяем текущее положение и ожидаемый сигнал
	var targetSignal internal.SignalType
	var targetLevel float64
	var distanceToTarget float64

	// Определяем, к какому уровню движется QStick
	if currentQstick < qstickConfig.BuyThreshold {
		// QStick ниже порога покупки
		if qstickVelocity <= 0 {
			// Продолжает падать - сигнал не ожидается скоро
			return nil
		}
		// Растет к порогу покупки
		targetSignal = internal.BUY
		targetLevel = qstickConfig.BuyThreshold
		distanceToTarget = targetLevel - currentQstick
	} else if currentQstick > qstickConfig.SellThreshold {
		// QStick выше порога продажи
		if qstickVelocity >= 0 {
			// Продолжает расти - сигнал не ожидается скоро
			return nil
		}
		// Падает к порогу продажи
		targetSignal = internal.SELL
		targetLevel = qstickConfig.SellThreshold
		distanceToTarget = currentQstick - targetLevel
	} else {
		// QStick в нейтральной зоне - определяем направление движения
		if qstickVelocity > 0 && currentTrend > 0 {
			// QStick растет и тренд положительный - ожидаем BUY
			targetSignal = internal.BUY
			targetLevel = qstickConfig.BuyThreshold
			distanceToTarget = targetLevel - currentQstick
			if distanceToTarget < 0 {
				distanceToTarget = 0.1 // Уже выше порога, скоро сигнал
			}
		} else if qstickVelocity < 0 && currentTrend < 0 {
			// QStick падает и тренд отрицательный - ожидаем SELL
			targetSignal = internal.SELL
			targetLevel = qstickConfig.SellThreshold
			distanceToTarget = currentQstick - targetLevel
			if distanceToTarget < 0 {
				distanceToTarget = 0.1 // Уже ниже порога, скоро сигнал
			}
		} else {
			// Нет четкого направления
			return nil
		}
	}

	// Проверяем, достаточно ли сильное движение
	if qstickVelocity == 0 || distanceToTarget < 0 {
		return nil
	}

	// Вычисляем количество свечей до достижения целевого уровня
	candlesToTarget := int(distanceToTarget / (qstickVelocity + 0.0001))
	if candlesToTarget < 0 {
		candlesToTarget = -candlesToTarget
	}

	// Ограничиваем горизонт предсказания
	maxHorizon := qstickConfig.Period * 2
	if candlesToTarget > maxHorizon {
		candlesToTarget = maxHorizon
	}
	if candlesToTarget < 1 {
		candlesToTarget = 1
	}

	// Вычисляем скорость изменения цены
	priceVelocity := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		priceChange := candles[idx].Close.ToFloat64() - candles[prevIdx].Close.ToFloat64()
		priceVelocity += priceChange
	}
	priceVelocity /= float64(lookback - 1)

	// Экстраполируем цену в будущее
	currentPrice := candles[currentIdx].Close.ToFloat64()
	futurePrice := currentPrice + priceVelocity*float64(candlesToTarget)

	// Вычисляем уверенность в предсказании
	confidence := calculateQstickConfidence(
		qstickVelocity,
		distanceToTarget,
		currentQstick,
		targetLevel,
		currentTrend,
		priceVelocity,
		candlesToTarget,
		maxHorizon,
		qstickValues,
		trendValues,
		currentIdx,
		lookback,
	)

	// Минимальный порог уверенности
	if confidence < 0.30 {
		return nil
	}

	// Вычисляем дату сигнала
	if len(candles) < 2 {
		return nil
	}

	timeInterval := (candles[len(candles)-1].ToTime().Unix() - candles[0].ToTime().Unix()) / int64(len(candles)-1)
	lastTimestamp := candles[len(candles)-1].ToTime().Unix()
	futureTimestamp := lastTimestamp + timeInterval*int64(candlesToTarget)

	return &internal.FutureSignal{
		SignalType: targetSignal,
		Date:       futureTimestamp,
		Price:      futurePrice,
		Confidence: confidence,
	}
}

// calculateQstickConfidence вычисляет уверенность в предсказании
func calculateQstickConfidence(
	qstickVelocity float64,
	distanceToTarget float64,
	currentQstick float64,
	targetLevel float64,
	currentTrend float64,
	priceVelocity float64,
	candlesToTarget int,
	maxHorizon int,
	qstickValues []float64,
	trendValues []float64,
	currentIdx int,
	lookback int,
) float64 {
	confidence := 0.5 // Базовая уверенность

	// Фактор 1: Сила движения QStick (чем сильнее, тем лучше)
	velocityStrength := qstickVelocity
	if velocityStrength < 0 {
		velocityStrength = -velocityStrength
	}
	if velocityStrength > 0.1 {
		confidence += 0.20
	} else if velocityStrength > 0.05 {
		confidence += 0.15
	} else if velocityStrength > 0.02 {
		confidence += 0.10
	}

	// Фактор 2: Близость к целевому уровню (чем ближе, тем выше уверенность)
	if distanceToTarget < 0.2 {
		confidence += 0.20
	} else if distanceToTarget < 0.5 {
		confidence += 0.15
	} else if distanceToTarget < 1.0 {
		confidence += 0.10
	} else if distanceToTarget > 2.0 {
		confidence -= 0.10
	}

	// Фактор 3: Согласованность с трендом
	trendStrength := currentTrend
	if trendStrength < 0 {
		trendStrength = -trendStrength
	}
	if (qstickVelocity > 0 && currentTrend > 0) || (qstickVelocity < 0 && currentTrend < 0) {
		// QStick и тренд в одном направлении
		if trendStrength > 0.01 {
			confidence += 0.20
		} else if trendStrength > 0.005 {
			confidence += 0.15
		} else {
			confidence += 0.10
		}
	} else {
		// Дивергенция - снижаем уверенность
		confidence -= 0.10
	}

	// Фактор 4: Стабильность скорости изменения QStick
	if currentIdx >= lookback*2 {
		prevVelocity := 0.0
		for i := lookback; i < lookback*2-1; i++ {
			idx := currentIdx - i
			prevIdx := idx - 1
			qstickChange := qstickValues[idx] - qstickValues[prevIdx]
			prevVelocity += qstickChange
		}
		prevVelocity /= float64(lookback - 1)

		// Если скорости одного знака и близки по величине - стабильно
		if qstickVelocity*prevVelocity > 0 {
			ratio := qstickVelocity / prevVelocity
			if ratio < 0 {
				ratio = -ratio
			}
			if ratio > 0.5 && ratio < 2.0 {
				confidence += 0.15
			} else if ratio > 0.3 && ratio < 3.0 {
				confidence += 0.10
			}
		}
	}

	// Фактор 5: Горизонт предсказания (чем дальше, тем менее уверены)
	horizonRatio := float64(candlesToTarget) / float64(maxHorizon)
	if horizonRatio < 0.25 {
		confidence += 0.10
	} else if horizonRatio > 0.75 {
		confidence -= 0.20
	}

	// Фактор 6: Согласованность движения QStick и цены
	if (qstickVelocity > 0 && priceVelocity > 0) || (qstickVelocity < 0 && priceVelocity < 0) {
		confidence += 0.10
	} else {
		confidence -= 0.05
	}

	// Ограничиваем диапазон
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0 {
		confidence = 0
	}

	return confidence
}

type QstickConfigGenerator struct{}

func NewQstickConfigGenerator() *QstickConfigGenerator {
	return &QstickConfigGenerator{}
}

func (s *QstickConfigGenerator) Generate() []internal.StrategyConfigV2 {

	configs := lo.CrossJoinBy6(
		lo.RangeWithSteps[int](12, 19, 1),
		lo.RangeWithSteps[float64](-2, -1, 0.2),
		lo.RangeWithSteps[float64](0.2, 1, 0.2),
		lo.RangeWithSteps[float64](1, 3, 1),
		lo.RangeWithSteps[float64](4, 9, 1),
		lo.RangeWithSteps[float64](0.003, 0.005, 0.0003),
		func(period int, buyThreshold float64, sellThreshold float64, stopLoss float64, takeProfit float64, volatilityFilter float64) internal.StrategyConfigV2 {
			return &QStickConfig{
				Period:            period,
				BuyThreshold:      buyThreshold,
				SellThreshold:     sellThreshold,
				StopLossPercent:   stopLoss,
				TakeProfitPercent: takeProfit,
				VolatilityFilter:  volatilityFilter,
			}
		})

	return configs
}

func NewQstickStrategyV2(slippage float64) internal.TradingStrategy {
	// 1. Создаем провайдер проскальзывания
	slippageProvider := internal.NewSlippageProvider(slippage)

	// 2. Создаем генератор сигналов
	signalGenerator := NewQStickSignalGenerator()

	// 3. Создаем менеджер конфигурации
	configManager := internal.NewConfigManager(
		&QStickConfig{
			Period:            12,
			BuyThreshold:      -1.5,
			SellThreshold:     0.2,
			StopLossPercent:   1.0,
			TakeProfitPercent: 6.0,
			VolatilityFilter:  0.0045,
		},
		func() internal.StrategyConfigV2 {
			return &QStickConfig{}
		},
	)

	// 4. Создаем генератор конфигураций для оптимизации
	configGenerator := NewQstickConfigGenerator()

	// 5. Создаем оптимизатор (переиспользуем универсальный GridSearchOptimizer!)
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	// 6. Собираем всё вместе через композицию
	return internal.NewStrategyBase(
		"qstick_oscillator_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewQstickStrategyV2(0.01) // default slippage 0.01
	internal.RegisterStrategyV2(strategy)
}

// =======================================================

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
