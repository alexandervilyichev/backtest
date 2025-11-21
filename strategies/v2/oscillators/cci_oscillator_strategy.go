// CCI Oscillator Strategy V2
//
// Описание стратегии:
// CCI - осциллятор, измеряющий отклонение цены от ее статистической средней.
// Индикатор показывает, насколько текущая цена отклоняется от средней цены за определенный период.
// CCI считается перекупленным выше +100 и перепроданным ниже -100.
//
// Параметры:
// - Period: период расчета CCI (обычно 14-20)
// - BuyLevel: уровень перепроданности для покупки (обычно -100)
// - SellLevel: уровень перекупленности для продажи (обычно +100)

package oscillators

import (
	"bt/internal"
	"errors"
	"fmt"
	"math"

	"github.com/samber/lo"
)

type CCIConfigV2 struct {
	Period    int     `json:"period"`
	BuyLevel  float64 `json:"buy_level"`
	SellLevel float64 `json:"sell_level"`
}

func (c *CCIConfigV2) Validate() error {
	if c.Period <= 0 {
		return errors.New("period must be positive")
	}
	if c.BuyLevel >= 0 {
		return errors.New("buy level must be negative")
	}
	if c.SellLevel <= 0 {
		return errors.New("sell level must be positive")
	}
	if c.BuyLevel >= c.SellLevel {
		return errors.New("buy level must be less than sell level")
	}
	return nil
}

func (c *CCIConfigV2) String() string {
	return fmt.Sprintf("CCI(period=%d, buy=%.1f, sell=%.1f)",
		c.Period, c.BuyLevel, c.SellLevel)
}

type CCISignalGeneratorV2 struct{}

func NewCCISignalGeneratorV2() *CCISignalGeneratorV2 {
	return &CCISignalGeneratorV2{}
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

	for i := 0; i < period-1; i++ {
		cci[i] = 0
	}

	for i := period - 1; i < len(candles); i++ {
		var tpSum float64
		typicalPrices := make([]float64, 0, period)

		for j := i - period + 1; j <= i; j++ {
			tp := calculateTypicalPrice(candles[j])
			typicalPrices = append(typicalPrices, tp)
			tpSum += tp
		}

		ma := tpSum / float64(period)

		var mdSum float64
		for _, tp := range typicalPrices {
			mdSum += math.Abs(tp - ma)
		}
		meanDeviation := mdSum / float64(period)

		currentTp := calculateTypicalPrice(candles[i])
		if meanDeviation == 0 {
			cci[i] = 0
		} else {
			cci[i] = (currentTp - ma) / (0.015 * meanDeviation)
		}
	}

	return cci
}

func (sg *CCISignalGeneratorV2) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	cciConfig, ok := config.(*CCIConfigV2)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := cciConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	cciValues := calculateCCI(candles, cciConfig.Period)
	if cciValues == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := cciConfig.Period; i < len(candles); i++ {
		cci := cciValues[i]

		if !inPosition && cci <= cciConfig.BuyLevel {
			if i > 0 && candles[i].Close.ToFloat64() >= candles[i-1].Close.ToFloat64() {
				signals[i] = internal.BUY
				inPosition = true
				continue
			}
		}

		if inPosition && cci >= cciConfig.SellLevel {
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

// PredictNextSignal предсказывает следующий сигнал на основе CCI
func (sg *CCISignalGeneratorV2) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	cciConfig, ok := config.(*CCIConfigV2)
	if !ok {
		return nil
	}

	if err := cciConfig.Validate(); err != nil {
		return nil
	}

	if len(candles) < cciConfig.Period*2 {
		return nil
	}

	cciValues := calculateCCI(candles, cciConfig.Period)
	if cciValues == nil {
		return nil
	}

	currentIdx := len(candles) - 1
	currentCCI := cciValues[currentIdx]
	currentPrice := candles[currentIdx].Close.ToFloat64()

	// Вычисляем скорость изменения CCI
	lookback := 3
	if currentIdx < cciConfig.Period+lookback {
		lookback = 1
	}

	cciVelocity := (currentCCI - cciValues[currentIdx-lookback]) / float64(lookback)

	// Определяем текущее состояние и предсказываем
	var signalType internal.SignalType
	var predictedCandles int
	var predictedPrice float64
	var confidence float64

	// Если CCI движется к уровню перепроданности
	if currentCCI > cciConfig.BuyLevel && cciVelocity < 0 {
		// Предсказываем, когда CCI достигнет уровня покупки
		distanceToLevel := currentCCI - cciConfig.BuyLevel
		if internal.Abs(cciVelocity) > 0.1 {
			predictedCandles = int(distanceToLevel / internal.Abs(cciVelocity))
			if predictedCandles > cciConfig.Period*3 {
				return nil // Слишком далеко
			}
			if predictedCandles < 1 {
				predictedCandles = 1
			}

			signalType = internal.BUY

			// Экстраполируем цену
			priceVelocity := 0.0
			if currentIdx >= lookback {
				priceVelocity = (currentPrice - candles[currentIdx-lookback].Close.ToFloat64()) / float64(lookback)
			}
			predictedPrice = currentPrice + priceVelocity*float64(predictedCandles)

			// Уверенность зависит от скорости движения CCI и близости к уровню
			velocityFactor := internal.Min(internal.Abs(cciVelocity)/10.0, 0.4)
			distanceFactor := 0.0
			if distanceToLevel < 50 {
				distanceFactor = 0.3
			} else if distanceToLevel < 100 {
				distanceFactor = 0.2
			} else {
				distanceFactor = 0.1
			}
			confidence = 0.3 + velocityFactor + distanceFactor
		} else {
			return nil
		}
	} else if currentCCI < cciConfig.SellLevel && cciVelocity > 0 {
		// Предсказываем, когда CCI достигнет уровня продажи
		distanceToLevel := cciConfig.SellLevel - currentCCI
		if cciVelocity > 0.1 {
			predictedCandles = int(distanceToLevel / cciVelocity)
			if predictedCandles > cciConfig.Period*3 {
				return nil
			}
			if predictedCandles < 1 {
				predictedCandles = 1
			}

			signalType = internal.SELL

			priceVelocity := 0.0
			if currentIdx >= lookback {
				priceVelocity = (currentPrice - candles[currentIdx-lookback].Close.ToFloat64()) / float64(lookback)
			}
			predictedPrice = currentPrice + priceVelocity*float64(predictedCandles)

			velocityFactor := internal.Min(cciVelocity/10.0, 0.4)
			distanceFactor := 0.0
			if distanceToLevel < 50 {
				distanceFactor = 0.3
			} else if distanceToLevel < 100 {
				distanceFactor = 0.2
			} else {
				distanceFactor = 0.1
			}
			confidence = 0.3 + velocityFactor + distanceFactor
		} else {
			return nil
		}
	} else if currentCCI <= cciConfig.BuyLevel {
		// Уже в зоне перепроданности, ожидаем разворот вверх
		signalType = internal.BUY
		predictedCandles = cciConfig.Period / 4
		if predictedCandles < 1 {
			predictedCandles = 1
		}

		// Предсказываем небольшой рост
		predictedPrice = currentPrice * 1.01
		confidence = 0.5 + internal.Min(internal.Abs(currentCCI-cciConfig.BuyLevel)/100.0, 0.3)
	} else if currentCCI >= cciConfig.SellLevel {
		// Уже в зоне перекупленности, ожидаем разворот вниз
		signalType = internal.SELL
		predictedCandles = cciConfig.Period / 4
		if predictedCandles < 1 {
			predictedCandles = 1
		}

		predictedPrice = currentPrice * 0.99
		confidence = 0.5 + internal.Min((currentCCI-cciConfig.SellLevel)/100.0, 0.3)
	} else {
		// В нейтральной зоне, предсказание ненадежно
		return nil
	}

	// Ограничиваем уверенность
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.1 {
		confidence = 0.1
	}

	// Вычисляем дату сигнала
	if len(candles) < 2 {
		return nil
	}

	timeInterval := (candles[len(candles)-1].ToTime().Unix() - candles[0].ToTime().Unix()) / int64(len(candles)-1)
	lastTimestamp := candles[len(candles)-1].ToTime().Unix()
	futureTimestamp := lastTimestamp + timeInterval*int64(predictedCandles)

	return &internal.FutureSignal{
		SignalType: signalType,
		Date:       futureTimestamp,
		Price:      predictedPrice,
		Confidence: confidence,
	}
}

type CCIConfigGeneratorV2 struct{}

func NewCCIConfigGeneratorV2() *CCIConfigGeneratorV2 {
	return &CCIConfigGeneratorV2{}
}

func (g *CCIConfigGeneratorV2) Generate() []internal.StrategyConfigV2 {
	configs := lo.CrossJoinBy3(
		lo.RangeWithSteps[int](5, 10, 1),
		lo.RangeWithSteps[float64](-200, -150, 5),
		lo.RangeWithSteps[float64](150, 220, 5),
		func(period int, buy float64, sell float64) internal.StrategyConfigV2 {
			return &CCIConfigV2{
				Period:    period,
				BuyLevel:  buy,
				SellLevel: sell,
			}
		})

	return configs
}

func NewCCIOscillatorStrategyV2(slippage float64) internal.TradingStrategy {
	slippageProvider := internal.NewSlippageProvider(slippage)
	signalGenerator := NewCCISignalGeneratorV2()

	configManager := internal.NewConfigManager(
		&CCIConfigV2{
			Period:    20,
			BuyLevel:  -100.0,
			SellLevel: 100.0,
		},
		func() internal.StrategyConfigV2 {
			return &CCIConfigV2{}
		},
	)

	configGenerator := NewCCIConfigGeneratorV2()
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	return internal.NewStrategyBase(
		"cci_oscillator_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewCCIOscillatorStrategyV2(0.01)
	internal.RegisterStrategyV2(strategy)
}
