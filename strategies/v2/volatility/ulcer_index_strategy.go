// Ulcer Index Strategy V2
//
// Описание стратегии:
// Ulcer Index (UI) измеряет downside risk (риск снижения). Индекс растет по мере того,
// как цена уходит все дальше от недавнего максимума, и падает по мере достижения новых максимумов.
//
// Формула:
//   UI = √(Σ((Close - MaxHigh) / MaxHigh)² / period)
//
// Торговые правила:
// - Покупка (BUY): когда UI падает ниже порога (риск снижается)
// - Продажа (SELL): когда UI растет выше порога (риск увеличивается)
//
// Предсказание:
// - Анализирует скорость изменения UI
// - Экстраполирует движение индикатора
// - Предсказывает момент пересечения с пороговыми уровнями
// - Учитывает тренд максимумов и стабильность движения

package volatility

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
	"math"
)

type UlcerIndexConfig struct {
	Period        int     `json:"period"`
	BuyThreshold  float64 `json:"buy_threshold"`
	SellThreshold float64 `json:"sell_threshold"`
}

func (c *UlcerIndexConfig) Validate() error {
	if c.Period <= 0 {
		return errors.New("period must be positive")
	}
	if c.BuyThreshold <= 0 {
		return errors.New("buy threshold must be positive")
	}
	if c.SellThreshold <= c.BuyThreshold {
		return errors.New("sell threshold must be greater than buy threshold")
	}
	return nil
}

func (c *UlcerIndexConfig) String() string {
	return fmt.Sprintf("UlcerIndex(period=%d, buy=%.4f, sell=%.4f)",
		c.Period, c.BuyThreshold, c.SellThreshold)
}

type UlcerIndexSignalGenerator struct{}

func NewUlcerIndexSignalGenerator() *UlcerIndexSignalGenerator {
	return &UlcerIndexSignalGenerator{}
}

// calculateUlcerIndex рассчитывает Ulcer Index для заданного периода
func calculateUlcerIndex(candles []internal.Candle, period int) []float64 {
	if len(candles) < period {
		return nil
	}

	ulcerIndex := make([]float64, len(candles))

	// Первые period-1 значений не определены
	for i := 0; i < period-1; i++ {
		ulcerIndex[i] = 0
	}

	// Для каждого окна рассчитываем Ulcer Index
	for i := period - 1; i < len(candles); i++ {
		// Находим максимум в текущем окне
		maxHigh := candles[i-period+1].High.ToFloat64()
		for j := i - period + 2; j <= i; j++ {
			if candles[j].High.ToFloat64() > maxHigh {
				maxHigh = candles[j].High.ToFloat64()
			}
		}

		// Если максимум равен 0, пропускаем расчет
		if maxHigh == 0 {
			ulcerIndex[i] = 0
			continue
		}

		// Рассчитываем сумму квадратов drawdown'ов
		var sumSquaredDrawdown float64
		for j := i - period + 1; j <= i; j++ {
			currentPrice := candles[j].Close.ToFloat64()
			drawdown := (currentPrice - maxHigh) / maxHigh
			sumSquaredDrawdown += drawdown * drawdown
		}

		// Ulcer Index = квадратный корень от среднего квадрата drawdown'а
		ulcerIndex[i] = math.Sqrt(sumSquaredDrawdown / float64(period))
	}

	return ulcerIndex
}

func (sg *UlcerIndexSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	uiConfig, ok := config.(*UlcerIndexConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := uiConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	ulcerIndex := calculateUlcerIndex(candles, uiConfig.Period)
	if ulcerIndex == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := uiConfig.Period; i < len(candles); i++ {
		currentUI := ulcerIndex[i]

		// BUY сигнал: когда Ulcer Index падает ниже порога (риск снижается)
		if !inPosition && currentUI < uiConfig.BuyThreshold {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		// SELL сигнал: когда Ulcer Index растет выше порога (риск увеличивается)
		if inPosition && currentUI > uiConfig.SellThreshold {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

// PredictNextSignal предсказывает ближайший сигнал Ulcer Index
func (sg *UlcerIndexSignalGenerator) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	uiConfig, ok := config.(*UlcerIndexConfig)
	if !ok {
		return nil
	}

	if err := uiConfig.Validate(); err != nil {
		log.Printf("⚠️ Ошибка валидации конфигурации: %v", err)
		return nil
	}

	minCandles := uiConfig.Period * 2
	if len(candles) < minCandles {
		log.Printf("⚠️ Недостаточно данных для предсказания: получено %d свечей, требуется минимум %d", len(candles), minCandles)
		return nil
	}

	// Вычисляем Ulcer Index
	ulcerIndex := calculateUlcerIndex(candles, uiConfig.Period)
	if ulcerIndex == nil {
		log.Printf("⚠️ Не удалось вычислить Ulcer Index")
		return nil
	}

	currentIdx := len(candles) - 1
	currentUI := ulcerIndex[currentIdx]

	// Анализируем последние несколько значений для определения скорости изменения
	lookback := 5
	if lookback > uiConfig.Period/4 {
		lookback = uiConfig.Period / 4
	}
	if lookback < 3 {
		lookback = 3
	}

	if currentIdx < uiConfig.Period+lookback {
		log.Printf("⚠️ Недостаточно данных для анализа скорости")
		return nil
	}

	// Вычисляем среднюю скорость изменения UI
	uiVelocity := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		uiChange := ulcerIndex[idx] - ulcerIndex[prevIdx]
		uiVelocity += uiChange
	}
	uiVelocity /= float64(lookback - 1)

	// Определяем текущее положение и ожидаемый сигнал
	var targetSignal internal.SignalType
	var targetLevel float64
	var distanceToTarget float64

	// Определяем, к какому уровню движется UI
	if currentUI < uiConfig.BuyThreshold {
		// UI ниже порога покупки - уже можем покупать
		// Предсказываем следующий SELL сигнал
		targetSignal = internal.SELL
		targetLevel = uiConfig.SellThreshold
		distanceToTarget = targetLevel - currentUI

		// Если UI падает, до SELL далеко
		if uiVelocity <= 0 {
			log.Printf("⚠️ UI ниже порога покупки и продолжает падать")
			return nil
		}
	} else if currentUI > uiConfig.SellThreshold {
		// UI выше порога продажи - уже можем продавать
		// Предсказываем следующий BUY сигнал
		targetSignal = internal.BUY
		targetLevel = uiConfig.BuyThreshold
		distanceToTarget = currentUI - targetLevel

		// Если UI растет, до BUY далеко
		if uiVelocity >= 0 {
			log.Printf("⚠️ UI выше порога продажи и продолжает расти")
			return nil
		}
	} else {
		// UI в нейтральной зоне - определяем направление движения
		if uiVelocity > 0 {
			// UI растет - ожидаем достижения порога продажи (SELL)
			targetSignal = internal.SELL
			targetLevel = uiConfig.SellThreshold
			distanceToTarget = targetLevel - currentUI
		} else if uiVelocity < 0 {
			// UI падает - ожидаем достижения порога покупки (BUY)
			targetSignal = internal.BUY
			targetLevel = uiConfig.BuyThreshold
			distanceToTarget = currentUI - targetLevel
		} else {
			log.Printf("⚠️ UI не имеет направленного движения")
			return nil
		}
	}

	// Проверяем, достаточно ли сильное движение
	if math.Abs(uiVelocity) < 0.0001 {
		log.Printf("⚠️ Слишком слабое движение UI")
		return nil
	}

	// Вычисляем количество свечей до достижения целевого уровня
	candlesToTarget := int(math.Abs(distanceToTarget / uiVelocity))

	// Ограничиваем горизонт предсказания
	maxHorizon := uiConfig.Period
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
	confidence := sg.calculateConfidence(
		uiVelocity,
		distanceToTarget,
		currentUI,
		targetLevel,
		priceVelocity,
		candlesToTarget,
		maxHorizon,
		ulcerIndex,
		candles,
		currentIdx,
		lookback,
		uiConfig.Period,
	)

	// Минимальный порог уверенности
	if confidence < 0.30 {
		log.Printf("⚠️ Недостаточная уверенность в предсказании (%.3f < 0.30)", confidence)
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

// calculateConfidence вычисляет уверенность в предсказании
func (sg *UlcerIndexSignalGenerator) calculateConfidence(
	uiVelocity float64,
	distanceToTarget float64,
	currentUI float64,
	targetLevel float64,
	priceVelocity float64,
	candlesToTarget int,
	maxHorizon int,
	ulcerIndex []float64,
	candles []internal.Candle,
	currentIdx int,
	lookback int,
	period int,
) float64 {
	confidence := 0.5 // Базовая уверенность

	// Фактор 1: Сила движения UI (чем сильнее, тем лучше)
	velocityStrength := math.Abs(uiVelocity)
	if velocityStrength > 0.001 {
		confidence += 0.20
	} else if velocityStrength > 0.0005 {
		confidence += 0.15
	} else if velocityStrength > 0.0002 {
		confidence += 0.10
	}

	// Фактор 2: Близость к целевому уровню (чем ближе, тем выше уверенность)
	relativeDistance := distanceToTarget / targetLevel
	if relativeDistance < 0.1 {
		confidence += 0.20
	} else if relativeDistance < 0.2 {
		confidence += 0.15
	} else if relativeDistance < 0.5 {
		confidence += 0.10
	} else if relativeDistance > 1.0 {
		confidence -= 0.10
	}

	// Фактор 3: Стабильность скорости изменения UI
	if currentIdx >= lookback*2 {
		prevVelocity := 0.0
		for i := lookback; i < lookback*2-1; i++ {
			idx := currentIdx - i
			prevIdx := idx - 1
			uiChange := ulcerIndex[idx] - ulcerIndex[prevIdx]
			prevVelocity += uiChange
		}
		prevVelocity /= float64(lookback - 1)

		// Если скорости одного знака и близки по величине - стабильно
		if uiVelocity*prevVelocity > 0 {
			ratio := math.Abs(uiVelocity / prevVelocity)
			if ratio > 0.5 && ratio < 2.0 {
				confidence += 0.15
			} else if ratio > 0.3 && ratio < 3.0 {
				confidence += 0.10
			}
		}
	}

	// Фактор 4: Горизонт предсказания (чем дальше, тем менее уверены)
	horizonRatio := float64(candlesToTarget) / float64(maxHorizon)
	if horizonRatio < 0.25 {
		confidence += 0.10
	} else if horizonRatio > 0.75 {
		confidence -= 0.20
	}

	// Фактор 5: Анализ тренда максимумов
	// Если максимумы растут, UI должен падать (хороший знак для BUY)
	// Если максимумы падают, UI должен расти (хороший знак для SELL)
	if currentIdx >= period+lookback {
		recentMaxTrend := 0.0
		for i := 0; i < lookback-1; i++ {
			idx := currentIdx - i
			prevIdx := idx - 1

			// Находим максимумы в окнах
			maxCurrent := candles[idx].High.ToFloat64()
			maxPrev := candles[prevIdx].High.ToFloat64()
			for j := 1; j < period && idx-j >= 0; j++ {
				if candles[idx-j].High.ToFloat64() > maxCurrent {
					maxCurrent = candles[idx-j].High.ToFloat64()
				}
				if candles[prevIdx-j].High.ToFloat64() > maxPrev {
					maxPrev = candles[prevIdx-j].High.ToFloat64()
				}
			}

			recentMaxTrend += (maxCurrent - maxPrev)
		}
		recentMaxTrend /= float64(lookback - 1)

		// Проверяем согласованность
		if (recentMaxTrend > 0 && uiVelocity < 0) || (recentMaxTrend < 0 && uiVelocity > 0) {
			confidence += 0.15
		} else if math.Abs(recentMaxTrend) < 0.01 {
			// Максимумы стабильны - нейтрально
			confidence += 0.05
		}
	}

	// Фактор 6: Согласованность движения UI и цены
	// Обычно когда UI растет, цена падает (и наоборот)
	if (uiVelocity > 0 && priceVelocity < 0) || (uiVelocity < 0 && priceVelocity > 0) {
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

type UlcerIndexConfigGenerator struct{}

func NewUlcerIndexConfigGenerator() *UlcerIndexConfigGenerator {
	return &UlcerIndexConfigGenerator{}
}

func (g *UlcerIndexConfigGenerator) Generate() []internal.StrategyConfigV2 {
	// Используем те же параметры, что и в v1 для совместимости результатов
	configs := []internal.StrategyConfigV2{}

	for period := 300; period <= 400; period += 10 {
		for buyThreshold := 0.02; buyThreshold <= 0.03; buyThreshold += 0.002 {
			for sellThreshold := buyThreshold + 0.01; sellThreshold <= 0.07; sellThreshold += 0.002 {
				config := &UlcerIndexConfig{
					Period:        period,
					BuyThreshold:  buyThreshold,
					SellThreshold: sellThreshold,
				}
				if config.Validate() == nil {
					configs = append(configs, config)
				}
			}
		}
	}

	return configs
}

func NewUlcerIndexStrategyV2(slippage float64) internal.TradingStrategy {
	slippageProvider := internal.NewSlippageProvider(slippage)
	signalGenerator := NewUlcerIndexSignalGenerator()

	configManager := internal.NewConfigManager(
		&UlcerIndexConfig{
			Period:        300,
			BuyThreshold:  0.028,
			SellThreshold: 0.044,
		},
		func() internal.StrategyConfigV2 {
			return &UlcerIndexConfig{}
		},
	)

	configGenerator := NewUlcerIndexConfigGenerator()
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	return internal.NewStrategyBase(
		"ulcer_index_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewUlcerIndexStrategyV2(0.01)
	internal.RegisterStrategyV2(strategy)
}
