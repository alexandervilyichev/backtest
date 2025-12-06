// Dollar Cost Averaging Strategy V2
//
// Описание стратегии:
// Стратегия усреднения позиции (dollar cost averaging) с частичными покупками при падении
// и частичными продажами при росте. Используется для снижения средневзвешенной цены входа
// при падении и фиксации прибыли при росте.
//
// Как работает:
// - При падении цены относительно последней покупки: покупает частично (усредняет убытки)
// - При росте цены относительно последней покупки: продает частично (фиксирует прибыль)
// - Использует пороги падения/роста для генерации сигналов
// - Отслеживает средневзвешенную цену входа для расчета прибыли/убытка
//
// Параметры:
// - DropThreshold: порог падения цены для частичной покупки (обычно 0.01-0.05, т.е. 1-5%)
// - RiseThreshold: порог роста цены для частичной продажи (обычно 0.01-0.05, т.е. 1-5%)
// - MaxPositionSize: максимальный размер позиции в долях от капитала (обычно 1.0-2.0)
// - BuyRatio: доля капитала для частичной покупки (обычно 0.2-0.5, т.е. 20-50%)
// - SellRatio: доля позиции для частичной продажи (обычно 0.2-0.5, т.е. 20-50%)
//
// Предсказание:
// - Анализирует текущий тренд и волатильность
// - Предсказывает момент следующего сигнала на основе скорости изменения цены

package trend

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
	"math"
)

type DollarCostAveragingConfig struct {
	DropThreshold   float64 `json:"drop_threshold"`    // Порог падения для покупки (например, 0.02 = 2%)
	RiseThreshold   float64 `json:"rise_threshold"`    // Порог роста для продажи (например, 0.02 = 2%)
	MaxPositionSize float64 `json:"max_position_size"` // Максимальный размер позиции (1.0 = 100% капитала)
	BuyRatio        float64 `json:"buy_ratio"`         // Доля капитала для покупки (0.3 = 30%)
	SellRatio       float64 `json:"sell_ratio"`        // Доля позиции для продажи (0.3 = 30%)
	MinHoldPeriod   int     `json:"min_hold_period"`   // Минимальный период удержания между сделками
}

func (c *DollarCostAveragingConfig) Validate() error {
	if c.DropThreshold <= 0 || c.DropThreshold >= 1.0 {
		return errors.New("drop threshold must be between 0 and 1")
	}
	if c.RiseThreshold <= 0 || c.RiseThreshold >= 1.0 {
		return errors.New("rise threshold must be between 0 and 1")
	}
	if c.MaxPositionSize <= 0 {
		return errors.New("max position size must be positive")
	}
	if c.BuyRatio <= 0 || c.BuyRatio > 1.0 {
		return errors.New("buy ratio must be between 0 and 1")
	}
	if c.SellRatio <= 0 || c.SellRatio > 1.0 {
		return errors.New("sell ratio must be between 0 and 1")
	}
	if c.MinHoldPeriod < 0 {
		return errors.New("min hold period must be non-negative")
	}
	return nil
}

func (c *DollarCostAveragingConfig) String() string {
	return fmt.Sprintf("DollarCostAveraging(drop=%.3f, rise=%.3f, max_pos=%.2f, buy_ratio=%.2f, sell_ratio=%.2f, min_hold=%d)",
		c.DropThreshold, c.RiseThreshold, c.MaxPositionSize, c.BuyRatio, c.SellRatio, c.MinHoldPeriod)
}

type DollarCostAveragingSignalGenerator struct{}

func NewDollarCostAveragingSignalGenerator() *DollarCostAveragingSignalGenerator {
	return &DollarCostAveragingSignalGenerator{}
}

func (sg *DollarCostAveragingSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	dcaConfig, ok := config.(*DollarCostAveragingConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := dcaConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < 2 {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	// Инициализируем все сигналы как HOLD
	for i := range signals {
		signals[i] = internal.HOLD
	}

	// Отслеживаем состояние позиции
	inPosition := false
	var lastTradeIndex int = -1
	var averageEntryPrice float64
	var buyCount int // Количество покупок для отслеживания размера позиции

	for i := 1; i < len(candles); i++ {
		currentPrice := candles[i].Close.ToFloat64()
		prevPrice := candles[i-1].Close.ToFloat64()

		// Защита от деления на ноль
		if prevPrice == 0 || currentPrice == 0 {
			signals[i] = internal.HOLD
			continue
		}

		// Проверяем минимальный период удержания
		if lastTradeIndex >= 0 && i-lastTradeIndex < dcaConfig.MinHoldPeriod {
			signals[i] = internal.HOLD
			continue
		}

		priceChange := (currentPrice - prevPrice) / prevPrice

		if !inPosition {
			// Нет позиции: ищем возможность для первой покупки
			// Покупаем при падении цены (более мягкое условие для входа)
			if priceChange < -dcaConfig.DropThreshold {
				signals[i] = internal.BUY
				inPosition = true
				lastTradeIndex = i
				averageEntryPrice = currentPrice
				buyCount = 1
				continue
			}
		} else {
			// Есть позиция: проверяем условия для усреднения или фиксации прибыли
			priceChangeFromEntry := (currentPrice - averageEntryPrice) / averageEntryPrice

			// Усреднение при падении: покупаем еще при падении относительно средней цены входа
			// Проверяем максимальный размер позиции через количество покупок
			// Предполагаем, что каждая покупка использует buyRatio от доступных средств
			// Максимальное количество покупок примерно = log(maxPositionSize) / log(1 + buyRatio)
			maxBuyCount := int(math.Ceil(math.Log(dcaConfig.MaxPositionSize) / math.Log(1.0+dcaConfig.BuyRatio)))
			if maxBuyCount < 2 {
				maxBuyCount = 2
			}

			if priceChangeFromEntry < -dcaConfig.DropThreshold && priceChange < 0 && buyCount < maxBuyCount {
				signals[i] = internal.BUY
				lastTradeIndex = i
				// Обновляем средневзвешенную цену входа
				// Упрощенная модель: считаем среднее арифметическое цен покупок
				averageEntryPrice = (averageEntryPrice*float64(buyCount) + currentPrice) / float64(buyCount+1)
				buyCount++
				continue
			}

			// Продажа при росте: продаем при росте относительно средней цены входа
			// Используем более гибкую логику - продаем при достижении порога прибыли
			if priceChangeFromEntry > dcaConfig.RiseThreshold {
				signals[i] = internal.SELL
				lastTradeIndex = i
				// После продажи уменьшаем количество покупок пропорционально
				// Если продали большую часть, считаем позицию закрытой
				if dcaConfig.SellRatio >= 0.7 || buyCount <= 1 {
					inPosition = false
					buyCount = 0
					averageEntryPrice = 0
				} else {
					// Уменьшаем количество покупок пропорционально проданной доле
					buyCount = int(math.Max(1, float64(buyCount)*(1.0-dcaConfig.SellRatio)))
					// Средняя цена входа остается той же (продаем по текущей цене)
				}
				continue
			}

			// Дополнительная защита: продаем при сильном падении от входа (стоп-лосс)
			// Это предотвращает большие убытки
			if priceChangeFromEntry < -dcaConfig.DropThreshold*2 {
				signals[i] = internal.SELL
				lastTradeIndex = i
				inPosition = false
				buyCount = 0
				averageEntryPrice = 0
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

// PredictNextSignal предсказывает ближайший сигнал Dollar Cost Averaging
func (sg *DollarCostAveragingSignalGenerator) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	dcaConfig, ok := config.(*DollarCostAveragingConfig)
	if !ok {
		return nil
	}

	if err := dcaConfig.Validate(); err != nil {
		log.Printf("⚠️ Ошибка валидации конфигурации: %v", err)
		return nil
	}

	if len(candles) < 2 {
		return nil
	}

	currentIdx := len(candles) - 1
	currentPrice := candles[currentIdx].Close.ToFloat64()

	// Анализируем последние несколько свечей для определения тренда
	lookback := 10
	if lookback > currentIdx {
		lookback = currentIdx
	}
	if lookback < 2 {
		return nil
	}

	// Вычисляем скорость изменения цены
	priceVelocity := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		if prevIdx < 0 {
			break
		}
		prevPrice := candles[prevIdx].Close.ToFloat64()
		if prevPrice == 0 {
			continue
		}
		priceChange := (candles[idx].Close.ToFloat64() - prevPrice) / prevPrice
		priceVelocity += priceChange
	}
	priceVelocity /= float64(lookback - 1)

	// Вычисляем волатильность
	volatility := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		if prevIdx < 0 {
			break
		}
		prevPrice := candles[prevIdx].Close.ToFloat64()
		if prevPrice == 0 {
			continue
		}
		priceChange := math.Abs((candles[idx].Close.ToFloat64() - prevPrice) / prevPrice)
		volatility += priceChange
	}
	volatility /= float64(lookback - 1)

	// Определяем текущую ситуацию
	// Упрощенная модель: предполагаем, что мы в позиции, если цена падала недавно
	recentDrop := false
	recentRise := false
	for i := 0; i < 3 && currentIdx-i > 0; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		prevPrice := candles[prevIdx].Close.ToFloat64()
		if prevPrice == 0 {
			continue
		}
		change := (candles[idx].Close.ToFloat64() - prevPrice) / prevPrice
		if change < -dcaConfig.DropThreshold*0.5 {
			recentDrop = true
		}
		if change > dcaConfig.RiseThreshold*0.5 {
			recentRise = true
		}
	}

	var targetSignal internal.SignalType
	var targetPrice float64
	var reason string
	var candlesToSignal int

	if recentDrop || priceVelocity < -dcaConfig.DropThreshold*0.5 {
		// Ожидаем покупку при дальнейшем падении
		targetSignal = internal.BUY
		targetPrice = currentPrice * (1.0 - dcaConfig.DropThreshold)
		reason = "expecting price drop for averaging"
		// Оцениваем количество свечей до сигнала на основе скорости падения
		if priceVelocity < 0 {
			candlesToSignal = int(math.Ceil(dcaConfig.DropThreshold / math.Abs(priceVelocity)))
		} else {
			candlesToSignal = 5 // Консервативная оценка
		}
	} else if recentRise || priceVelocity > dcaConfig.RiseThreshold*0.5 {
		// Ожидаем продажу при дальнейшем росте
		targetSignal = internal.SELL
		targetPrice = currentPrice * (1.0 + dcaConfig.RiseThreshold)
		reason = "expecting price rise for profit taking"
		// Оцениваем количество свечей до сигнала на основе скорости роста
		if priceVelocity > 0 {
			candlesToSignal = int(math.Ceil(dcaConfig.RiseThreshold / priceVelocity))
		} else {
			candlesToSignal = 5 // Консервативная оценка
		}
	} else {
		// Нейтральная ситуация - предсказание затруднено
		return nil
	}

	// Ограничиваем горизонт предсказания
	maxHorizon := 20
	if candlesToSignal > maxHorizon {
		candlesToSignal = maxHorizon
	}
	if candlesToSignal < 1 {
		candlesToSignal = 1
	}

	// Вычисляем уверенность в предсказании
	confidence := sg.calculateConfidence(
		priceVelocity,
		volatility,
		dcaConfig.DropThreshold,
		dcaConfig.RiseThreshold,
		candlesToSignal,
		maxHorizon,
		reason,
	)

	// Минимальный порог уверенности
	if confidence < 0.30 {
		log.Printf("⚠️ Недостаточная уверенность в предсказании (%.3f < 0.30) для причины: %s", confidence, reason)
		return nil
	}

	// Вычисляем дату сигнала
	if len(candles) < 2 {
		return nil
	}

	timeInterval := (candles[len(candles)-1].ToTime().Unix() - candles[0].ToTime().Unix()) / int64(len(candles)-1)
	lastTimestamp := candles[len(candles)-1].ToTime().Unix()
	futureTimestamp := lastTimestamp + timeInterval*int64(candlesToSignal)

	log.Printf("🎯 Предсказан сигнал %s на %.4f через %d свечей (уверенность: %.3f) по причине: %s",
		targetSignal, targetPrice, candlesToSignal, confidence, reason)

	return &internal.FutureSignal{
		SignalType: targetSignal,
		Date:       futureTimestamp,
		Price:      targetPrice,
		Confidence: confidence,
	}
}

// calculateConfidence вычисляет уверенность в предсказании
func (sg *DollarCostAveragingSignalGenerator) calculateConfidence(
	priceVelocity float64,
	volatility float64,
	dropThreshold float64,
	riseThreshold float64,
	candlesToSignal int,
	maxHorizon int,
	reason string,
) float64 {
	confidence := 0.5 // Базовая уверенность

	// Фактор 1: Сила тренда
	velocityStrength := math.Abs(priceVelocity)
	if velocityStrength > dropThreshold*2 || velocityStrength > riseThreshold*2 {
		confidence += 0.20
	} else if velocityStrength > dropThreshold || velocityStrength > riseThreshold {
		confidence += 0.15
	} else if velocityStrength > dropThreshold*0.5 || velocityStrength > riseThreshold*0.5 {
		confidence += 0.10
	}

	// Фактор 2: Волатильность (умеренная волатильность лучше для предсказания)
	if volatility > 0.01 && volatility < 0.05 {
		confidence += 0.10
	} else if volatility > 0.005 && volatility < 0.10 {
		confidence += 0.05
	}

	// Фактор 3: Горизонт предсказания
	horizonRatio := float64(candlesToSignal) / float64(maxHorizon)
	if horizonRatio < 0.25 {
		confidence += 0.15
	} else if horizonRatio > 0.75 {
		confidence -= 0.20
	}

	// Фактор 4: Логичность причины
	switch reason {
	case "expecting price drop for averaging":
		if priceVelocity < 0 {
			confidence += 0.10
		}
	case "expecting price rise for profit taking":
		if priceVelocity > 0 {
			confidence += 0.10
		}
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

type DollarCostAveragingConfigGenerator struct{}

func NewDollarCostAveragingConfigGenerator() *DollarCostAveragingConfigGenerator {
	return &DollarCostAveragingConfigGenerator{}
}

func (g *DollarCostAveragingConfigGenerator) Generate() []internal.StrategyConfigV2 {
	configs := []internal.StrategyConfigV2{}

	// Оптимизированные диапазоны параметров с фокусом на результативность
	// Пороги падения/роста: более узкий диапазон для лучшей фильтрации
	dropThresholds := []float64{0.005, 0.01, 0.015, 0.02, 0.025, 0.03}
	riseThresholds := []float64{0.005, 0.01, 0.015, 0.02, 0.025, 0.03, 0.04}

	// Максимальный размер позиции: более консервативные значения
	maxPositionSizes := []float64{1.0, 1.2, 1.5, 1.8}

	// Доли для покупки/продажи: более широкий диапазон
	buyRatios := []float64{0.2, 0.3, 0.4, 0.5, 0.6}
	sellRatios := []float64{0.2, 0.3, 0.4, 0.5, 0.6, 0.7}

	// Минимальный период удержания: предотвращает излишнюю торговлю
	minHoldPeriods := []int{0, 1, 2, 3, 5}

	// Генерируем комбинации с приоритетом на разумные соотношения
	for _, dropThresh := range dropThresholds {
		for _, riseThresh := range riseThresholds {
			// Пропускаем нелогичные комбинации (слишком маленький rise относительно drop)
			if riseThresh < dropThresh*0.8 {
				continue
			}

			for _, maxPos := range maxPositionSizes {
				for _, buyRatio := range buyRatios {
					for _, sellRatio := range sellRatios {
						// Пропускаем комбинации где sellRatio слишком мал относительно buyRatio
						// (чтобы не накапливать позицию бесконечно)
						if sellRatio < buyRatio*0.5 {
							continue
						}

						for _, minHold := range minHoldPeriods {
							configs = append(configs, &DollarCostAveragingConfig{
								DropThreshold:   dropThresh,
								RiseThreshold:   riseThresh,
								MaxPositionSize: maxPos,
								BuyRatio:        buyRatio,
								SellRatio:       sellRatio,
								MinHoldPeriod:   minHold,
							})
						}
					}
				}
			}
		}
	}

	return configs
}

type DollarCostAveragingStrategy struct{}

func (s *DollarCostAveragingStrategy) Name() string {
	return "dollar_cost_averaging"
}

func NewDollarCostAveragingStrategyV2(slippage float64) internal.TradingStrategy {
	slippageProvider := internal.NewSlippageProvider(slippage)
	signalGenerator := NewDollarCostAveragingSignalGenerator()

	configManager := internal.NewConfigManager(
		&DollarCostAveragingConfig{
			DropThreshold:   0.015, // 1.5% падение для покупки
			RiseThreshold:   0.02,  // 2% рост для продажи
			MaxPositionSize: 1.5,   // Максимум 150% капитала
			BuyRatio:        0.4,   // Покупаем на 40% доступных средств
			SellRatio:       0.5,   // Продаем 50% позиции
			MinHoldPeriod:   2,     // Минимум 2 свечи между сделками
		},
		func() internal.StrategyConfigV2 {
			return &DollarCostAveragingConfig{}
		},
	)

	configGenerator := NewDollarCostAveragingConfigGenerator()
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	return internal.NewStrategyBase(
		"dollar_cost_averaging_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewDollarCostAveragingStrategyV2(0.01)
	internal.RegisterStrategyV2(strategy)
}
