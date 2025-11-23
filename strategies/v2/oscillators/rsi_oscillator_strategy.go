// RSI Oscillator Strategy V2
//
// Описание стратегии:
// Стратегия использует индекс относительной силы (RSI) для определения
// перекупленных и перепроданных уровней рынка.
// RSI измеряет скорость и изменение ценовых движений на шкале от 0 до 100.
//
// Как работает:
// - Рассчитывается RSI с заданным периодом (по умолчанию 14)
// - Покупка: когда RSI опускается ниже уровня перепроданности (по умолчанию 30)
// - Продажа: когда RSI поднимается выше уровня перекупленности (по умолчанию 70)
//
// Предсказание:
// - Анализирует скорость изменения RSI (производная)
// - Экстраполирует движение RSI
// - Предсказывает момент пересечения с пороговыми уровнями
// - Учитывает импульс и стабильность движения

package oscillators

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
	"math"

	"github.com/samber/lo"
)

type RSIConfig struct {
	Period        int     `json:"period"`
	BuyThreshold  float64 `json:"buy_threshold"`
	SellThreshold float64 `json:"sell_threshold"`
}

func (c *RSIConfig) Validate() error {
	if c.Period <= 0 {
		return errors.New("period must be positive")
	}
	if c.BuyThreshold >= c.SellThreshold {
		return errors.New("buy threshold must be less than sell threshold")
	}
	if c.BuyThreshold < 0 || c.BuyThreshold > 100 {
		return errors.New("buy threshold must be between 0 and 100")
	}
	if c.SellThreshold < 0 || c.SellThreshold > 100 {
		return errors.New("sell threshold must be between 0 and 100")
	}
	return nil
}

func (c *RSIConfig) String() string {
	return fmt.Sprintf("RSI(period=%d, buy=%.1f, sell=%.1f)",
		c.Period, c.BuyThreshold, c.SellThreshold)
}

type RSISignalGenerator struct{}

func NewRSISignalGenerator() *RSISignalGenerator {
	return &RSISignalGenerator{}
}

func (sg *RSISignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	rsiConfig, ok := config.(*RSIConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := rsiConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	rsiValues := internal.CalculateRSICommon(candles, rsiConfig.Period)
	if rsiValues == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := rsiConfig.Period; i < len(candles); i++ {
		rsi := rsiValues[i]

		if !inPosition && rsi < rsiConfig.BuyThreshold {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		if inPosition && rsi > rsiConfig.SellThreshold {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

// PredictNextSignal предсказывает ближайший сигнал RSI
func (sg *RSISignalGenerator) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	rsiConfig, ok := config.(*RSIConfig)
	if !ok {
		return nil
	}

	if err := rsiConfig.Validate(); err != nil {
		log.Printf("⚠️ Ошибка валидации конфигурации: %v", err)
		return nil
	}

	if len(candles) < rsiConfig.Period*2 {
		log.Printf("⚠️ Недостаточно данных для предсказания: получено %d свечей, требуется минимум %d", len(candles), rsiConfig.Period*2)
		return nil
	}

	// Вычисляем RSI
	rsiValues := internal.CalculateRSICommon(candles, rsiConfig.Period)
	if rsiValues == nil {
		log.Printf("⚠️ Не удалось вычислить RSI")
		return nil
	}

	currentIdx := len(candles) - 1
	currentRSI := rsiValues[currentIdx]

	// Анализируем последние несколько значений для определения скорости изменения
	lookback := 5
	if lookback > rsiConfig.Period/2 {
		lookback = rsiConfig.Period / 2
	}
	if lookback < 3 {
		lookback = 3
	}

	if currentIdx < rsiConfig.Period+lookback {
		log.Printf("⚠️ Недостаточно данных для анализа скорости")
		return nil
	}

	// Вычисляем среднюю скорость изменения RSI (производная)
	rsiVelocity := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		rsiChange := rsiValues[idx] - rsiValues[prevIdx]
		rsiVelocity += rsiChange
	}
	rsiVelocity /= float64(lookback - 1)

	// Определяем текущее положение RSI и ожидаемый сигнал
	var targetSignal internal.SignalType
	var targetLevel float64
	var distanceToTarget float64

	// Определяем, к какому уровню движется RSI
	if currentRSI < rsiConfig.BuyThreshold {
		// RSI в зоне перепроданности - уже можем покупать
		// Но предсказываем следующий SELL сигнал
		targetSignal = internal.SELL
		targetLevel = rsiConfig.SellThreshold
		distanceToTarget = targetLevel - currentRSI

		// Если RSI падает, до SELL далеко
		if rsiVelocity <= 0 {
			log.Printf("⚠️ RSI в зоне перепроданности и продолжает падать")
			return nil
		}
	} else if currentRSI > rsiConfig.SellThreshold {
		// RSI в зоне перекупленности - уже можем продавать
		// Но предсказываем следующий BUY сигнал
		targetSignal = internal.BUY
		targetLevel = rsiConfig.BuyThreshold
		distanceToTarget = currentRSI - targetLevel

		// Если RSI растет, до BUY далеко
		if rsiVelocity >= 0 {
			log.Printf("⚠️ RSI в зоне перекупленности и продолжает расти")
			return nil
		}
	} else {
		// RSI в нейтральной зоне - определяем направление движения
		if rsiVelocity > 0 {
			// RSI растет - ожидаем достижения уровня перекупленности (SELL)
			targetSignal = internal.SELL
			targetLevel = rsiConfig.SellThreshold
			distanceToTarget = targetLevel - currentRSI
		} else if rsiVelocity < 0 {
			// RSI падает - ожидаем достижения уровня перепроданности (BUY)
			targetSignal = internal.BUY
			targetLevel = rsiConfig.BuyThreshold
			distanceToTarget = currentRSI - targetLevel
		} else {
			log.Printf("⚠️ RSI не имеет направленного движения")
			return nil
		}
	}

	// Проверяем, достаточно ли сильное движение
	if math.Abs(rsiVelocity) < 0.1 {
		log.Printf("⚠️ Слишком слабое движение RSI")
		return nil
	}

	// Вычисляем количество свечей до достижения целевого уровня
	candlesToTarget := int(math.Abs(distanceToTarget / rsiVelocity))

	// Ограничиваем горизонт предсказания
	maxHorizon := rsiConfig.Period * 2
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
		rsiVelocity,
		distanceToTarget,
		currentRSI,
		targetLevel,
		priceVelocity,
		candlesToTarget,
		maxHorizon,
		rsiValues,
		currentIdx,
		lookback,
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
func (sg *RSISignalGenerator) calculateConfidence(
	rsiVelocity float64,
	distanceToTarget float64,
	currentRSI float64,
	targetLevel float64,
	priceVelocity float64,
	candlesToTarget int,
	maxHorizon int,
	rsiValues []float64,
	currentIdx int,
	lookback int,
) float64 {
	confidence := 0.5 // Базовая уверенность

	// Фактор 1: Сила движения RSI (чем сильнее, тем лучше)
	velocityStrength := math.Abs(rsiVelocity)
	if velocityStrength > 2.0 {
		confidence += 0.20
	} else if velocityStrength > 1.0 {
		confidence += 0.15
	} else if velocityStrength > 0.5 {
		confidence += 0.10
	}

	// Фактор 2: Близость к целевому уровню (чем ближе, тем выше уверенность)
	if distanceToTarget < 5 {
		confidence += 0.20
	} else if distanceToTarget < 10 {
		confidence += 0.15
	} else if distanceToTarget < 20 {
		confidence += 0.10
	} else if distanceToTarget > 40 {
		confidence -= 0.10
	}

	// Фактор 3: Стабильность скорости изменения RSI
	// Проверяем, насколько стабильна скорость на более раннем периоде
	if currentIdx >= lookback*2 {
		prevVelocity := 0.0
		for i := lookback; i < lookback*2-1; i++ {
			idx := currentIdx - i
			prevIdx := idx - 1
			rsiChange := rsiValues[idx] - rsiValues[prevIdx]
			prevVelocity += rsiChange
		}
		prevVelocity /= float64(lookback - 1)

		// Если скорости одного знака и близки по величине - стабильно
		if rsiVelocity*prevVelocity > 0 {
			ratio := math.Abs(rsiVelocity / prevVelocity)
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

	// Фактор 5: Согласованность движения RSI и цены
	// Обычно RSI и цена движутся в одном направлении
	if (rsiVelocity > 0 && priceVelocity > 0) || (rsiVelocity < 0 && priceVelocity < 0) {
		confidence += 0.10
	} else {
		// Дивергенция - может быть сигналом разворота, но снижает уверенность в линейном предсказании
		confidence -= 0.05
	}

	// Фактор 6: Положение RSI относительно экстремальных зон
	// Если RSI уже в экстремальной зоне, вероятность разворота выше
	if currentRSI < 20 || currentRSI > 80 {
		confidence += 0.10
	} else if currentRSI < 10 || currentRSI > 90 {
		confidence += 0.15
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

type RSIConfigGenerator struct{}

func NewRSIConfigGenerator() *RSIConfigGenerator {
	return &RSIConfigGenerator{}
}

func (g *RSIConfigGenerator) Generate() []internal.StrategyConfigV2 {
	configs := lo.CrossJoinBy3(
		lo.RangeWithSteps[int](10, 20, 1),
		lo.RangeWithSteps[float64](10, 35, 1),
		lo.RangeWithSteps[float64](65, 86, 1),
		func(period int, buyThresh float64, sellThresh float64) internal.StrategyConfigV2 {
			return &RSIConfig{
				Period:        period,
				BuyThreshold:  buyThresh,
				SellThreshold: sellThresh,
			}
		})

	return configs
}

func NewRSIStrategyV2(slippage float64) internal.TradingStrategy {
	slippageProvider := internal.NewSlippageProvider(slippage)
	signalGenerator := NewRSISignalGenerator()

	configManager := internal.NewConfigManager(
		&RSIConfig{
			Period:        14,
			BuyThreshold:  30.0,
			SellThreshold: 70.0,
		},
		func() internal.StrategyConfigV2 {
			return &RSIConfig{}
		},
	)

	configGenerator := NewRSIConfigGenerator()
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	return internal.NewStrategyBase(
		"rsi_oscillator_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewRSIStrategyV2(0.01)
	internal.RegisterStrategyV2(strategy)
}
