// strategies/awesome_oscillator.go
// Реализация стратегии "Чудесный осциллятор" (Awesome Oscillator, AO) Билла Вильямса.
//
// Описание:
//   Awesome Oscillator (AO) — индикатор, измеряющий изменение рыночной энергии.
//   Он рассчитывается как разница между двумя простыми скользящими средними (SMA)
//   медианной цены (High + Low) / 2:
//
//     AO(t) = SMA(MedianPrice, 5) - SMA(MedianPrice, 34)
//
//   Где:
//     - MedianPrice = (High + Low) / 2 — лучшая оценка "истинной" цены за свечу.
//     - 5 — короткий период (реагирует на краткосрочные импульсы).
//     - 34 — длинный период (отражает долгосрочный тренд).
//
//   Значение AO:
//     - AO > 0: Краткосрочная энергия выше долгосрочной → восходящий импульс.
//     - AO < 0: Краткосрочная энергия ниже долгосрочной → нисходящий импульс.
//
//   Торговые правила Билла Вильямса:
//     - Покупка (BUY): AO пересекает ноль снизу вверх (AO[i-1] < 0 && AO[i] >= 0)
//       и последняя свеча — зелёная (Close > Open).
//     - Продажа (SELL): AO пересекает ноль сверху вниз (AO[i-1] > 0 && AO[i] <= 0)
//       и последняя свеча — красная (Close < Open).
//
//   Дополнительно: можно включить подтверждение "двумя свечами":
//     - Для BUY: последние две медианные цены должны расти (показывает устойчивость).
//     - Для SELL: последние две медианные цены должны падать.
//
//   Преимущество AO: он чувствителен к изменениям объема и силы движения,
//   не реагирует на шум, как RSI или MACD, и идеален для торговли на таймфреймах M15-H1.

package oscillators

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
)

type AOConfig struct {
	FastPeriod          int  `json:"fast_period"`
	SlowPeriod          int  `json:"slow_period"`
	ConfirmByTwoCandles bool `json:"confirm_by_two_candles"`
}

func (c *AOConfig) Validate() error {
	if c.FastPeriod <= 0 {
		return errors.New("fast period must be positive")
	}
	if c.SlowPeriod <= 0 {
		return errors.New("slow period must be positive")
	}
	if c.FastPeriod >= c.SlowPeriod {
		return errors.New("fast period must be less than slow period")
	}
	return nil
}

func (c *AOConfig) DefaultConfigString() string {
	return fmt.Sprintf("AO(fast=%d, slow=%d, confirm_two=%t)",
		c.FastPeriod, c.SlowPeriod, c.ConfirmByTwoCandles)
}

// AwesomeOscillatorStrategy реализует стратегию Чудесного осциллятора Билла Вильямса.
type AwesomeOscillatorStrategy struct {
	internal.BaseConfig
}

// Name возвращает имя стратегии.
func (s *AwesomeOscillatorStrategy) Name() string {
	return "awesome_oscillator"
}

// calculateMedianPrice возвращает медианную цену для одной свечи: (High + Low) / 2
func calculateMedianPrice(c internal.Candle) float64 {
	h := c.High.ToFloat64()
	l := c.Low.ToFloat64()
	return (h + l) / 2.0
}

// calculateAO вычисляет значения Awesome Oscillator для массива свечей.
// Возвращает срез значений AO, где индекс соответствует индексу свечи.
// Первые slowPeriod значений будут 0 (недостаточно данных).
func calculateAO(candles []internal.Candle, fastPeriod, slowPeriod int) []float64 {
	if len(candles) < slowPeriod {
		log.Printf("Недостаточно данных для расчета AO (нужно минимум %d свечей)", slowPeriod)
		return nil
	}

	ao := make([]float64, len(candles))

	// Вычисляем медианные цены
	medians := make([]float64, len(candles))
	for i := range candles {
		medians[i] = calculateMedianPrice(candles[i])
	}

	// Вычисляем SMA для быстрого и медленного периода
	smaFast := make([]float64, len(candles))
	smaSlow := make([]float64, len(candles))

	// Расчет SMA (простое скользящее среднее)
	for i := 0; i < len(candles); i++ {
		if i < fastPeriod-1 {
			smaFast[i] = 0
		} else {
			var sum float64
			for j := i - fastPeriod + 1; j <= i; j++ {
				sum += medians[j]
			}
			smaFast[i] = sum / float64(fastPeriod)
		}

		if i < slowPeriod-1 {
			smaSlow[i] = 0
		} else {
			var sum float64
			for j := i - slowPeriod + 1; j <= i; j++ {
				sum += medians[j]
			}
			smaSlow[i] = sum / float64(slowPeriod)
		}
	}

	// AO = SMA_fast - SMA_slow
	for i := 0; i < len(candles); i++ {
		if smaFast[i] == 0 || smaSlow[i] == 0 {
			ao[i] = 0
		} else {
			ao[i] = smaFast[i] - smaSlow[i]
		}
	}

	return ao
}

func (s *AwesomeOscillatorStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	aoConfig, ok := config.(*AOConfig)
	if !ok {
		log.Println("Invalid AO config type")
		return make([]internal.SignalType, len(candles))
	}

	if err := aoConfig.Validate(); err != nil {
		log.Printf("AO config validation error: %v", err)
		return make([]internal.SignalType, len(candles))
	}

	aoValues := calculateAO(candles, aoConfig.FastPeriod, aoConfig.SlowPeriod)
	if aoValues == nil {
		log.Println("Не удалось рассчитать AO — возвращаем пустые сигналы")
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	// Начинаем с slowPeriod, так как до этого AO не определён
	for i := aoConfig.SlowPeriod; i < len(candles); i++ {
		prevAo := aoValues[i-1]
		currAo := aoValues[i]

		// Проверка на пересечение нуля
		isBuySignal := prevAo < 0 && currAo >= 0
		isSellSignal := prevAo > 0 && currAo <= 0

		// Подтверждение двумя свечами (опционально)
		confirmCondition := true
		if aoConfig.ConfirmByTwoCandles && i >= 2 {
			medPrev2 := calculateMedianPrice(candles[i-2])
			medPrev1 := calculateMedianPrice(candles[i-1])
			medCurr := calculateMedianPrice(candles[i])

			if isBuySignal {
				// Две подряд растущие медианные цены
				confirmCondition = medPrev1 > medPrev2 && medCurr > medPrev1
			} else if isSellSignal {
				// Две подряд падающие медианные цены
				confirmCondition = medPrev1 < medPrev2 && medCurr < medPrev1
			}
		}

		// Генерация сигнала (упрощенная версия - только пересечение нуля)
		if isBuySignal && confirmCondition && !inPosition {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		if isSellSignal && confirmCondition && inPosition {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *AwesomeOscillatorStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := s.DefaultConfig().(*AOConfig)
	bestProfit := -1.0

	// Перебираем параметры
	fastOptions := []int{3, 5, 7}
	slowOptions := []int{21, 34, 55}
	confirmOptions := []bool{false, true}

	for _, fast := range fastOptions {
		for _, slow := range slowOptions {
			// Исключаем некорректные пары
			if fast >= slow {
				continue
			}
			for _, confirm := range confirmOptions {
				config := &AOConfig{
					FastPeriod:          fast,
					SlowPeriod:          slow,
					ConfirmByTwoCandles: confirm,
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

	// Убираем отладочный вывод для продакшена
	fmt.Printf("🔍 Лучшие параметры AO: fast=%d, slow=%d, confirmTwo=%t → прибыль=%.4f\n",
		bestConfig.FastPeriod, bestConfig.SlowPeriod, bestConfig.ConfirmByTwoCandles, bestProfit)

	return bestConfig
}

// init регистрирует стратегию в фабрике стратегий при старте программы.
func init() {
	internal.RegisterStrategy("awesome_oscillator", &AwesomeOscillatorStrategy{
		BaseConfig: internal.BaseConfig{
			Config: &AOConfig{
				FastPeriod:          5,
				SlowPeriod:          34,
				ConfirmByTwoCandles: false,
			},
		},
	})
}
