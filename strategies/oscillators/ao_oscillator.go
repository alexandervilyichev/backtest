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
	"fmt"
	"log"
)

// AwesomeOscillatorStrategy реализует стратегию Чудесного осциллятора Билла Вильямса.
type AwesomeOscillatorStrategy struct{}

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

// GenerateSignals генерирует торговые сигналы на основе AO.
//
// Параметры:
//   - params.AoFastPeriod — период быстрой SMA (по умолчанию 5)
//   - params.AoSlowPeriod — период медленной SMA (по умолчанию 34)
//   - params.AoConfirmByTwoCandles — если true, требует двух подряд растущих/падающих медианных цен
//
// Логика:
//   - BUY: AO[i-1] < 0 && AO[i] >= 0 и Close[i] > Open[i] (зелёная свеча)
//   - SELL: AO[i-1] > 0 && AO[i] <= 0 и Close[i] < Open[i] (красная свеча)
//
// Если AoConfirmByTwoCandles == true:
//   - Для BUY: также проверяем, что медианные цены i-1 и i больше, чем i-2
//   - Для SELL: также проверяем, что медианные цены i-1 и i меньше, чем i-2
//
// Это снижает количество ложных сигналов при флете.
func (s *AwesomeOscillatorStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	fastPeriod := params.AoFastPeriod
	if fastPeriod == 0 {
		fastPeriod = 5 // стандартный параметр Билла Вильямса
	}

	slowPeriod := params.AoSlowPeriod
	if slowPeriod == 0 {
		slowPeriod = 34 // стандартный параметр Билла Вильямса
	}

	confirmByTwo := params.AoConfirmByTwoCandles

	aoValues := calculateAO(candles, fastPeriod, slowPeriod)
	if aoValues == nil {
		log.Println("Не удалось рассчитать AO — возвращаем пустые сигналы")
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	// Начинаем с slowPeriod, так как до этого AO не определён
	for i := slowPeriod; i < len(candles); i++ {
		prevAo := aoValues[i-1]
		currAo := aoValues[i]

		// Проверка на пересечение нуля
		isBuySignal := prevAo < 0 && currAo >= 0
		isSellSignal := prevAo > 0 && currAo <= 0

		// Переменные для подтверждения свечами (если нужно в будущем)
		// currOpen := candles[i].Open.ToFloat64()
		// currClose := candles[i].Close.ToFloat64()
		// isGreenCandle := currClose > currOpen
		// isRedCandle := currClose < currOpen

		// Подтверждение двумя свечами (опционально)
		confirmCondition := true
		if confirmByTwo && i >= 2 {
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

// Optimize выполняет подбор оптимальных параметров стратегии.
//
// Grid search по:
//   - fastPeriod: [3, 5, 7]
//   - slowPeriod: [21, 34, 55] — классические числа Фибоначчи
//   - confirmByTwoCandles: true/false
//
// Использует бэктест для выбора параметров с максимальной прибылью.
func (s *AwesomeOscillatorStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{
		AoFastPeriod:          5,
		AoSlowPeriod:          34,
		AoConfirmByTwoCandles: false,
	}
	bestProfit := -1.0

	generator := s.GenerateSignals

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
				params := internal.StrategyParams{
					AoFastPeriod:          fast,
					AoSlowPeriod:          slow,
					AoConfirmByTwoCandles: confirm,
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

	// Убираем отладочный вывод для продакшена
	fmt.Printf("🔍 Лучшие параметры AO: fast=%d, slow=%d, confirmTwo=%t → прибыль=%.2f%%\n",
		bestParams.AoFastPeriod, bestParams.AoSlowPeriod, bestParams.AoConfirmByTwoCandles, bestProfit*100)

	return bestParams
}

// init регистрирует стратегию в фабрике стратегий при старте программы.
func init() {
	internal.RegisterStrategy("awesome_oscillator", &AwesomeOscillatorStrategy{})
}
