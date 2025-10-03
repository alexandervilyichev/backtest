// strategies/ml_strategy.go — вейвлет анализ для прогнозирования сигналов
//
// Описание стратегии:
// Стратегия использует вейвлет анализ для декомпозиции ценовых данных на различные частотные компоненты.
// Вейвлет преобразование позволяет анализировать рынок на разных временных масштабах,
// выявлять тренды и фильтровать рыночный шум.
//
// Как работает:
// - Применяется дискретное вейвлет преобразование (DWT) с вейвлетом Хаара
// - Анализируются аппроксимационные коэффициенты (трендовая компонента)
// - Анализируются детализирующие коэффициенты (шумовая компонента)
// - Сигналы генерируются на основе соотношения трендовых и шумовых компонент
// - BUY: когда трендовая компонента доминирует над шумовой
// - SELL: когда шумовая компонента превышает трендовую
//
// Преимущества вейвлет анализа:
// - Многоуровневый анализ (разные масштабы времени)
// - Эффективное разделение тренда и шума
// - Локализация во времени и частоте
// - Отсутствие необходимости в обучении (математический подход)
//
// Параметры:
// - WaveletLevels: количество уровней декомпозиции (обычно 3-5)
// - TrendThreshold: порог доминирования тренда (обычно 0.1-0.3)
// - NoiseThreshold: порог превышения шума (обычно 0.2-0.4)
//
// Сильные стороны:
// - Математически обоснованный подход
// - Не требует обучения на данных
// - Хорошо фильтрует рыночный шум
// - Работает быстро (без обучения)
// - Адаптируется к разным временным масштабам
//
// Слабые стороны:
// - Чувствителен к выбору параметров декомпозиции
// - Может быть сложным для понимания
// - Требует достаточной длины временного ряда
// - Не учитывает фундаментальные факторы
//
// Лучшие условия для применения:
// - Волатильные рынки с выраженными трендами
// - Средне- и долгосрочная торговля
// - Рынки с различными временными масштабами
// - Когда важно фильтровать рыночный шум

package strategies

import (
	"bt/internal"
	"log"
	"math"
)

// WaveletAnalysis — структура для вейвлет анализа
type WaveletAnalysis struct {
	levels int // количество уровней декомпозиции
}

// NewWaveletAnalysis создает новый анализатор вейвлетов
func NewWaveletAnalysis(levels int) *WaveletAnalysis {
	return &WaveletAnalysis{levels: levels}
}

// dwtHaar — дискретное вейвлет преобразование с вейвлетом Хаара
func (wa *WaveletAnalysis) dwtHaar(data []float64) ([]float64, []float64) {
	if len(data) == 0 {
		return nil, nil
	}

	n := len(data)
	// Создаем массивы для хранения всех коэффициентов
	approx := make([]float64, n)
	detail := make([]float64, n)

	// Копируем исходные данные
	copy(approx, data)

	// Выполняем многоуровневую декомпозицию
	currentLength := n
	for level := 0; level < wa.levels && currentLength >= 2; level++ {
		half := currentLength / 2

		// Применяем вейвлет Хаара к первой половине массива
		for i := 0; i < half; i++ {
			a := approx[i]
			b := approx[i+half]

			// Approximation coefficients
			approx[i] = (a + b) / math.Sqrt(2)

			// Detail coefficients сохраняем во второй половине
			detail[i+half] = (a - b) / math.Sqrt(2)
		}

		// Обновляем рабочую длину для следующего уровня
		currentLength = half
	}

	return approx, detail
}

// analyzeWaveletSignal — анализирует вейвлет коэффициенты для генерации сигналов
func (wa *WaveletAnalysis) analyzeWaveletSignal(approx, detail []float64, trendThreshold, noiseThreshold float64) (float64, float64) {
	if len(approx) == 0 || len(detail) == 0 {
		return 0, 0
	}

	// Вычисляем энергию трендовой компоненты (аппроксимационные коэффициенты)
	// Используем последние коэффициенты для анализа текущего состояния
	trendEnergy := 0.0
	approxCount := 0
	startIdx := len(approx) - int(math.Min(float64(len(approx)), 16)) // последние 16 коэффициентов
	for i := startIdx; i < len(approx); i++ {
		if i >= 0 {
			trendEnergy += math.Abs(approx[i]) // используем абсолютное значение
			approxCount++
		}
	}
	if approxCount > 0 {
		trendEnergy /= float64(approxCount)
	}

	// Вычисляем энергию шумовой компоненты (детализирующие коэффициенты)
	noiseEnergy := 0.0
	detailCount := 0
	detailStartIdx := len(detail) - int(math.Min(float64(len(detail)), 32)) // последние 32 коэффициента
	for i := detailStartIdx; i < len(detail); i++ {
		if i >= 0 {
			noiseEnergy += math.Abs(detail[i]) // используем абсолютное значение
			detailCount++
		}
	}
	if detailCount > 0 {
		noiseEnergy /= float64(detailCount)
	}

	// Нормализуем энергии относительно общего уровня
	totalEnergy := trendEnergy + noiseEnergy
	if totalEnergy > 0 {
		trendEnergy /= totalEnergy
		noiseEnergy /= totalEnergy
	}

	return trendEnergy, noiseEnergy
}

// generateSignal — генерирует торговый сигнал на основе вейвлет анализа
func (wa *WaveletAnalysis) generateSignal(trendEnergy, noiseEnergy, trendThreshold, noiseThreshold float64) internal.SignalType {
	// BUY: тренд доминирует над шумом
	if trendEnergy > trendThreshold && noiseEnergy < noiseThreshold {
		return internal.BUY
	}

	// SELL: шум превышает порог (даже если тренд еще силен - выход из позиции)
	if noiseEnergy > noiseThreshold {
		return internal.SELL
	}

	// HOLD: нейтральное состояние
	return internal.HOLD
}

type MLStrategy struct{}

func (s *MLStrategy) Name() string {
	return "ml_strategy"
}

func (s *MLStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	if len(candles) < 32 { // минимум для 5 уровней декомпозиции (2^5 = 32)
		log.Printf("⚠️ Недостаточно данных для вейвлет анализа: получено %d свечей, требуется минимум 32", len(candles))
		return make([]internal.SignalType, len(candles))
	}

	// Параметры вейвлет анализа
	levels := 4            // количество уровней декомпозиции
	trendThreshold := 0.95 // порог доминирования тренда (адаптирован под данные)
	noiseThreshold := 0.03 // порог превышения шума (адаптирован под данные)

	// Создаем анализатор вейвлетов
	wa := NewWaveletAnalysis(levels)

	// Извлекаем ценовые данные для анализа
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	log.Printf("🔍 Выполняем вейвлет анализ %d свечей с %d уровнями декомпозиции", len(candles), levels)

	// Применяем вейвлет преобразование
	approx, detail := wa.dwtHaar(prices)
	if approx == nil || detail == nil {
		log.Println("❌ Ошибка вейвлет преобразования")
		return make([]internal.SignalType, len(candles))
	}

	// Генерируем сигналы на основе вейвлет анализа
	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	// Начинаем анализ после достаточного количества данных для декомпозиции
	windowSize := 1 << levels // 2^levels
	startIdx := windowSize - 1

	for i := startIdx; i < len(candles); i++ {
		// Анализируем вейвлет коэффициенты в окне
		windowStart := i - windowSize + 1
		if windowStart < 0 {
			windowStart = 0
		}

		// Берем последние коэффициенты для анализа
		windowApprox := approx[windowStart : i+1]
		windowDetail := detail[windowStart : i+1]

		// Анализируем энергии тренда и шума
		trendEnergy, noiseEnergy := wa.analyzeWaveletSignal(windowApprox, windowDetail, trendThreshold, noiseThreshold)

		// Генерируем сигнал
		signal := wa.generateSignal(trendEnergy, noiseEnergy, trendThreshold, noiseThreshold)

		// Применяем логику позиционирования
		if !inPosition && signal == internal.BUY {
			signals[i] = internal.BUY
			inPosition = true
		} else if inPosition && signal == internal.SELL {
			signals[i] = internal.SELL
			inPosition = false
		} else {
			signals[i] = internal.HOLD
		}

		// Отладочный вывод (можно убрать для продакшена)
		if i%100 == 0 {
			log.Printf("   Свеча %d: тренд=%.3f, шум=%.3f, сигнал=%v", i, trendEnergy, noiseEnergy, signal)
		}
	}

	// Все сигналы до startIdx — HOLD
	for i := 0; i < startIdx; i++ {
		signals[i] = internal.HOLD
	}

	log.Printf("✅ Вейвлет анализ завершен, сгенерировано сигналов")
	return signals
}

func (s *MLStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	return internal.StrategyParams{}
}

func featureToSlice(fs internal.FeatureSet) []float64 {
	return []float64{
		fs.RSI,
		fs.SMA5,
		fs.SMA10,
		fs.SMA20,
		fs.EMA12,
		fs.EMA26,
		fs.MACD,
		fs.MACDSignal,
		fs.BollingerUpper,
		fs.BollingerLower,
		fs.VolumeRatio,
		fs.Momentum1,
		fs.Momentum3,
		fs.Momentum5,
		fs.Volatility20,
	}
}

func init() {
	internal.RegisterStrategy("ml_strategy", &MLStrategy{})
}
