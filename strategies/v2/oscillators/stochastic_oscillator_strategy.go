// Stochastic Oscillator Strategy V2
//
// Описание стратегии:
// Стохастический осциллятор — индикатор импульса, который сравнивает текущую цену закрытия
// с диапазоном цен за определенный период времени.
//
// Формулы:
//   %K = 100 * (Close - Lowest Low) / (Highest High - Lowest Low)
//   %D = SMA(%K, dPeriod)
//
// Торговые правила:
// - Покупка (BUY): %K пересекает %D снизу вверх в зоне перепроданности (< buyThreshold)
// - Продажа (SELL): %K пересекает %D сверху вниз в зоне перекупленности (> sellThreshold)
//
// Предсказание:
// - Анализирует скорость изменения %K и %D
// - Экстраполирует движение обеих линий
// - Предсказывает момент пересечения в экстремальных зонах
// - Учитывает импульс, стабильность и положение относительно пороговых уровней

package oscillators

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
	"math"

	"github.com/samber/lo"
)

type StochasticConfig struct {
	KPeriod       int     `json:"k_period"`
	DPeriod       int     `json:"d_period"`
	BuyThreshold  float64 `json:"buy_threshold"`
	SellThreshold float64 `json:"sell_threshold"`
}

func (c *StochasticConfig) Validate() error {
	if c.KPeriod <= 0 {
		return errors.New("k_period must be positive")
	}
	if c.DPeriod <= 0 {
		return errors.New("d_period must be positive")
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

func (c *StochasticConfig) String() string {
	return fmt.Sprintf("Stochastic(K=%d, D=%d, buy=%.1f, sell=%.1f)",
		c.KPeriod, c.DPeriod, c.BuyThreshold, c.SellThreshold)
}

type StochasticSignalGenerator struct{}

func NewStochasticSignalGenerator() *StochasticSignalGenerator {
	return &StochasticSignalGenerator{}
}

func (sg *StochasticSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	stochConfig, ok := config.(*StochasticConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := stochConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	kValues, dValues := internal.CalculateStochastic(candles, stochConfig.KPeriod, stochConfig.DPeriod)
	if kValues == nil || dValues == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	startIdx := stochConfig.KPeriod + stochConfig.DPeriod - 1
	if startIdx >= len(candles) {
		return signals
	}

	for i := startIdx; i < len(candles); i++ {
		prevK := kValues[i-1]
		currK := kValues[i]
		prevD := dValues[i-1]
		currD := dValues[i]

		// Пересечение %K и %D
		kCrossedAboveD := prevK <= prevD && currK > currD
		kCrossedBelowD := prevK >= prevD && currK < currD

		// Покупка: %K пересекает %D снизу вверх в зоне перепроданности
		if !inPosition && kCrossedAboveD && currK < stochConfig.BuyThreshold {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		// Продажа: %K пересекает %D сверху вниз в зоне перекупленности
		if inPosition && kCrossedBelowD && currK > stochConfig.SellThreshold {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

// PredictNextSignal предсказывает ближайший сигнал Stochastic
func (sg *StochasticSignalGenerator) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	stochConfig, ok := config.(*StochasticConfig)
	if !ok {
		return nil
	}

	if err := stochConfig.Validate(); err != nil {
		log.Printf("⚠️ Ошибка валидации конфигурации: %v", err)
		return nil
	}

	minCandles := stochConfig.KPeriod + stochConfig.DPeriod + 10
	if len(candles) < minCandles {
		log.Printf("⚠️ Недостаточно данных для предсказания: получено %d свечей, требуется минимум %d", len(candles), minCandles)
		return nil
	}

	// Вычисляем %K и %D
	kValues, dValues := internal.CalculateStochastic(candles, stochConfig.KPeriod, stochConfig.DPeriod)
	if kValues == nil || dValues == nil {
		log.Printf("⚠️ Не удалось вычислить Stochastic")
		return nil
	}

	currentIdx := len(candles) - 1
	currentK := kValues[currentIdx]
	currentD := dValues[currentIdx]

	// Анализируем последние несколько значений для определения скорости изменения
	lookback := 5
	if lookback > stochConfig.KPeriod/2 {
		lookback = stochConfig.KPeriod / 2
	}
	if lookback < 3 {
		lookback = 3
	}

	startIdx := stochConfig.KPeriod + stochConfig.DPeriod - 1
	if currentIdx < startIdx+lookback {
		log.Printf("⚠️ Недостаточно данных для анализа скорости")
		return nil
	}

	// Вычисляем среднюю скорость изменения %K и %D
	kVelocity := 0.0
	dVelocity := 0.0
	for i := 0; i < lookback-1; i++ {
		idx := currentIdx - i
		prevIdx := idx - 1
		kChange := kValues[idx] - kValues[prevIdx]
		dChange := dValues[idx] - dValues[prevIdx]
		kVelocity += kChange
		dVelocity += dChange
	}
	kVelocity /= float64(lookback - 1)
	dVelocity /= float64(lookback - 1)

	// Определяем текущее положение и ожидаемый сигнал
	var targetSignal internal.SignalType
	var candlesToSignal int
	var futurePrice float64

	// Относительная скорость сближения линий
	relativeVelocity := kVelocity - dVelocity
	distanceBetweenLines := currentK - currentD

	// Проверяем, движутся ли линии навстречу друг другу
	if math.Abs(relativeVelocity) < 0.1 {
		log.Printf("⚠️ Слишком слабое относительное движение линий")
		return nil
	}

	// Определяем, какой сигнал ожидается
	if distanceBetweenLines < 0 && relativeVelocity > 0 {
		// %K ниже %D и движется вверх - возможно пересечение снизу вверх (BUY)
		// Проверяем, что мы в зоне перепроданности или движемся к ней
		if currentK > stochConfig.BuyThreshold+10 {
			log.Printf("⚠️ %%K слишком высоко для BUY сигнала")
			return nil
		}
		targetSignal = internal.BUY

		// Вычисляем количество свечей до пересечения
		candlesToCrossing := int(math.Abs(distanceBetweenLines / relativeVelocity))
		candlesToSignal = candlesToCrossing

	} else if distanceBetweenLines > 0 && relativeVelocity < 0 {
		// %K выше %D и движется вниз - возможно пересечение сверху вниз (SELL)
		// Проверяем, что мы в зоне перекупленности или движемся к ней
		if currentK < stochConfig.SellThreshold-10 {
			log.Printf("⚠️ %%K слишком низко для SELL сигнала")
			return nil
		}
		targetSignal = internal.SELL

		// Вычисляем количество свечей до пересечения
		candlesToCrossing := int(math.Abs(distanceBetweenLines / relativeVelocity))
		candlesToSignal = candlesToCrossing

	} else {
		log.Printf("⚠️ Линии не движутся навстречу друг другу")
		return nil
	}

	// Ограничиваем горизонт предсказания
	maxHorizon := stochConfig.KPeriod * 2
	if candlesToSignal > maxHorizon {
		candlesToSignal = maxHorizon
	}
	if candlesToSignal < 1 {
		candlesToSignal = 1
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
	futurePrice = currentPrice + priceVelocity*float64(candlesToSignal)

	// Вычисляем уверенность в предсказании
	confidence := sg.calculateConfidence(
		kVelocity,
		dVelocity,
		relativeVelocity,
		distanceBetweenLines,
		currentK,
		currentD,
		targetSignal,
		stochConfig,
		priceVelocity,
		candlesToSignal,
		maxHorizon,
		kValues,
		dValues,
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
	futureTimestamp := lastTimestamp + timeInterval*int64(candlesToSignal)

	return &internal.FutureSignal{
		SignalType: targetSignal,
		Date:       futureTimestamp,
		Price:      futurePrice,
		Confidence: confidence,
	}
}

// calculateConfidence вычисляет уверенность в предсказании
func (sg *StochasticSignalGenerator) calculateConfidence(
	kVelocity float64,
	dVelocity float64,
	relativeVelocity float64,
	distanceBetweenLines float64,
	currentK float64,
	currentD float64,
	targetSignal internal.SignalType,
	config *StochasticConfig,
	priceVelocity float64,
	candlesToSignal int,
	maxHorizon int,
	kValues []float64,
	dValues []float64,
	currentIdx int,
	lookback int,
) float64 {
	confidence := 0.5 // Базовая уверенность

	// Фактор 1: Сила относительного движения (чем сильнее, тем лучше)
	relativeStrength := math.Abs(relativeVelocity)
	if relativeStrength > 5.0 {
		confidence += 0.20
	} else if relativeStrength > 2.0 {
		confidence += 0.15
	} else if relativeStrength > 1.0 {
		confidence += 0.10
	}

	// Фактор 2: Близость линий (чем ближе к пересечению, тем выше уверенность)
	distance := math.Abs(distanceBetweenLines)
	if distance < 3 {
		confidence += 0.20
	} else if distance < 5 {
		confidence += 0.15
	} else if distance < 10 {
		confidence += 0.10
	} else if distance > 20 {
		confidence -= 0.10
	}

	// Фактор 3: Положение в правильной зоне
	if targetSignal == internal.BUY {
		// Для BUY сигнала лучше быть в зоне перепроданности
		if currentK < config.BuyThreshold {
			confidence += 0.20
		} else if currentK < config.BuyThreshold+10 {
			confidence += 0.10
		}
	} else if targetSignal == internal.SELL {
		// Для SELL сигнала лучше быть в зоне перекупленности
		if currentK > config.SellThreshold {
			confidence += 0.20
		} else if currentK > config.SellThreshold-10 {
			confidence += 0.10
		}
	}

	// Фактор 4: Стабильность скорости изменения
	if currentIdx >= lookback*2 {
		prevKVelocity := 0.0
		prevDVelocity := 0.0
		for i := lookback; i < lookback*2-1; i++ {
			idx := currentIdx - i
			prevIdx := idx - 1
			kChange := kValues[idx] - kValues[prevIdx]
			dChange := dValues[idx] - dValues[prevIdx]
			prevKVelocity += kChange
			prevDVelocity += dChange
		}
		prevKVelocity /= float64(lookback - 1)
		prevDVelocity /= float64(lookback - 1)

		prevRelativeVelocity := prevKVelocity - prevDVelocity

		// Если относительные скорости одного знака и близки по величине - стабильно
		if relativeVelocity*prevRelativeVelocity > 0 {
			ratio := math.Abs(relativeVelocity / prevRelativeVelocity)
			if ratio > 0.5 && ratio < 2.0 {
				confidence += 0.15
			} else if ratio > 0.3 && ratio < 3.0 {
				confidence += 0.10
			}
		}
	}

	// Фактор 5: Горизонт предсказания (чем дальше, тем менее уверены)
	horizonRatio := float64(candlesToSignal) / float64(maxHorizon)
	if horizonRatio < 0.25 {
		confidence += 0.10
	} else if horizonRatio > 0.75 {
		confidence -= 0.20
	}

	// Фактор 6: Согласованность движения %K и цены
	// Обычно %K и цена движутся в одном направлении
	if (kVelocity > 0 && priceVelocity > 0) || (kVelocity < 0 && priceVelocity < 0) {
		confidence += 0.10
	} else {
		// Дивергенция - может быть сигналом разворота
		confidence -= 0.05
	}

	// Фактор 7: Экстремальные значения %K
	if currentK < 10 || currentK > 90 {
		confidence += 0.10
	} else if currentK < 5 || currentK > 95 {
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

type StochasticConfigGenerator struct{}

func NewStochasticConfigGenerator() *StochasticConfigGenerator {
	return &StochasticConfigGenerator{}
}

func (g *StochasticConfigGenerator) Generate() []internal.StrategyConfigV2 {
	configs := lo.CrossJoinBy4(
		lo.RangeWithSteps[int](10, 20, 2),
		lo.RangeWithSteps[int](3, 8, 1),
		lo.RangeWithSteps[float64](15, 30, 5),
		lo.RangeWithSteps[float64](70, 86, 5),
		func(kPeriod int, dPeriod int, buyThresh float64, sellThresh float64) internal.StrategyConfigV2 {
			return &StochasticConfig{
				KPeriod:       kPeriod,
				DPeriod:       dPeriod,
				BuyThreshold:  buyThresh,
				SellThreshold: sellThresh,
			}
		})

	return configs
}

func NewStochasticStrategyV2(slippage float64) internal.TradingStrategy {
	slippageProvider := internal.NewSlippageProvider(slippage)
	signalGenerator := NewStochasticSignalGenerator()

	configManager := internal.NewConfigManager(
		&StochasticConfig{
			KPeriod:       14,
			DPeriod:       3,
			BuyThreshold:  20.0,
			SellThreshold: 80.0,
		},
		func() internal.StrategyConfigV2 {
			return &StochasticConfig{}
		},
	)

	configGenerator := NewStochasticConfigGenerator()
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	return internal.NewStrategyBase(
		"stochastic_oscillator_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewStochasticStrategyV2(0.01)
	internal.RegisterStrategyV2(strategy)
}
