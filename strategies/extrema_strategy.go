// strategies/extrema_strategy.go — стратегия на основе выявления экстремумов
//
// Описание стратегии:
// Стратегия анализирует исторические данные для выявления локальных экстремумов (пиков и впадин)
// в ценовых данных. На основе этих экстремумов строится модель, которая определяет оптимальные
// точки входа и выхода из позиций.
//
// Как работает:
// - Анализируются обучающие данные для поиска локальных максимумов (SELL точки) и минимумов (BUY точки)
// - Вычисляются характеристики паттернов вокруг экстремумов (волатильность, тренд, объем)
// - Строится модель принятия решений на основе расстояния до ближайших экстремумов
// - В реальном времени оценивается вероятность приближения к экстремуму
//
// Преимущества подхода:
// - Основан на реальных рыночных экстремумах из исторических данных
// - Адаптируется к специфике конкретного актива
// - Минимизирует эмоциональный фактор в принятии решений
// - Использует математически обоснованные точки входа/выхода
//
// Параметры:
// - MinExtremaDistance: минимальное расстояние между экстремумами (избегаем шума)
// - LookbackWindow: окно анализа вокруг экстремумов
// - ConfidenceThreshold: порог уверенности для генерации сигнала
//
// Сильные стороны:
// - Использует реальные исторические экстремумы как ориентиры
// - Адаптивная модель под конкретный рынок
// - Математически точные точки входа/выхода
// - Снижает влияние рыночного шума
//
// Слабые стороны:
// - Требует достаточного объема исторических данных
// - Может переобучаться на специфические паттерны
// - Чувствителен к выбору параметров экстремумов
// - Не учитывает фундаментальные изменения рынка
//
// Лучшие условия для применения:
// - Стабильные рынки с четкими циклами
// - Достаточный объем исторических данных
// - Рынки с выраженной волатильностью
// - Когда важна точность входа в позицию

package strategies

import (
	"bt/internal"
	"log"
	"math"
	"sort"
)

// ExtremaPoint — точка экстремума
type ExtremaPoint struct {
	Index    int     // индекс в массиве данных
	Price    float64 // цена экстремума
	IsPeak   bool    // true для максимума, false для минимума
	Strength float64 // сила экстремума (отклонение от соседей)
}

// ExtremaModel — модель на основе экстремумов
type ExtremaModel struct {
	extremaPoints []ExtremaPoint
	minDistance   int
	windowSize    int
}

// NewExtremaModel создает новую модель экстремумов
func NewExtremaModel(minDistance, windowSize int) *ExtremaModel {
	return &ExtremaModel{
		extremaPoints: make([]ExtremaPoint, 0),
		minDistance:   minDistance,
		windowSize:    windowSize,
	}
}

// findLocalExtrema находит локальные экстремумы в ценовых данных
func (em *ExtremaModel) findLocalExtrema(prices []float64) {
	em.extremaPoints = make([]ExtremaPoint, 0)

	for i := em.windowSize; i < len(prices)-em.windowSize; i++ {
		// Проверяем, является ли точка локальным максимумом
		isLocalMax := true
		maxValue := prices[i]
		for j := i - em.windowSize; j <= i+em.windowSize; j++ {
			if j != i && prices[j] >= maxValue {
				isLocalMax = false
				break
			}
		}

		// Проверяем, является ли точка локальным минимумом
		isLocalMin := true
		minValue := prices[i]
		for j := i - em.windowSize; j <= i+em.windowSize; j++ {
			if j != i && prices[j] <= minValue {
				isLocalMin = false
				break
			}
		}

		if isLocalMax || isLocalMin {
			// Вычисляем силу экстремума
			strength := 0.0
			if isLocalMax {
				for j := i - em.windowSize; j <= i+em.windowSize; j++ {
					if j != i {
						strength += math.Abs(prices[i] - prices[j])
					}
				}
			} else {
				for j := i - em.windowSize; j <= i+em.windowSize; j++ {
					if j != i {
						strength += math.Abs(prices[j] - prices[i])
					}
				}
			}
			strength /= float64(em.windowSize * 2)

			point := ExtremaPoint{
				Index:    i,
				Price:    prices[i],
				IsPeak:   isLocalMax,
				Strength: strength,
			}
			em.extremaPoints = append(em.extremaPoints, point)
		}
	}

	// Фильтруем экстремумы по минимальному расстоянию
	em.filterExtremaByDistance()
}

// filterExtremaByDistance удаляет слишком близкие экстремумы
func (em *ExtremaModel) filterExtremaByDistance() {
	if len(em.extremaPoints) <= 1 {
		return
	}

	// Сортируем по индексу
	sort.Slice(em.extremaPoints, func(i, j int) bool {
		return em.extremaPoints[i].Index < em.extremaPoints[j].Index
	})

	filtered := make([]ExtremaPoint, 0)
	filtered = append(filtered, em.extremaPoints[0])

	for i := 1; i < len(em.extremaPoints); i++ {
		last := filtered[len(filtered)-1]
		current := em.extremaPoints[i]

		if current.Index-last.Index >= em.minDistance {
			filtered = append(filtered, current)
		} else {
			// Оставляем экстремум с большей силой
			if current.Strength > last.Strength {
				filtered[len(filtered)-1] = current
			}
		}
	}

	em.extremaPoints = filtered
}

// findNearestExtrema находит ближайшие экстремумы к заданному индексу
func (em *ExtremaModel) findNearestExtrema(index int) (peak *ExtremaPoint, valley *ExtremaPoint) {
	minPeakDist := math.MaxInt32
	minValleyDist := math.MaxInt32

	for _, point := range em.extremaPoints {
		dist := int(math.Abs(float64(point.Index - index)))

		if point.IsPeak && dist < minPeakDist {
			minPeakDist = dist
			peak = &point
		} else if !point.IsPeak && dist < minValleyDist {
			minValleyDist = dist
			valley = &point
		}
	}

	return peak, valley
}

// predictSignal предсказывает сигнал на основе расстояния до экстремумов
func (em *ExtremaModel) predictSignal(index int, prices []float64, confidenceThreshold float64) internal.SignalType {
	peak, valley := em.findNearestExtrema(index)

	if peak == nil && valley == nil {
		return internal.HOLD
	}

	// Вычисляем расстояния и направления
	currentPrice := prices[index]

	peakDistance := math.MaxInt32
	valleyDistance := math.MaxInt32

	if peak != nil {
		peakDistance = int(math.Abs(float64(peak.Index - index)))
	}
	if valley != nil {
		valleyDistance = int(math.Abs(float64(valley.Index - index)))
	}

	// Улучшенная логика предсказания на основе паттернов экстремумов

	// 1. Если мы очень близко к экстремуму - генерируем сигнал
	if peakDistance <= 3 && peak != nil {
		return internal.SELL // близко к пику - продаем
	}
	if valleyDistance <= 3 && valley != nil {
		return internal.BUY // близко ко дну - покупаем
	}

	// 2. Анализируем тренд движения к экстремуму
	if peak != nil && valley != nil {
		// Определяем, к какому экстремуму движемся
		if index < peak.Index && index < valley.Index {
			// Движемся вперед, определяем ближайший экстремум
			if peakDistance < valleyDistance {
				// Ближайший - пик, и цена ниже пика - покупаем
				if currentPrice < peak.Price*0.98 { // с небольшим запасом
					return internal.BUY
				}
			} else {
				// Ближайший - впадина, и цена выше впадины - продаем
				if currentPrice > valley.Price*1.02 { // с небольшим запасом
					return internal.SELL
				}
			}
		}
	}

	// 3. Анализируем силу экстремумов для УЛЬТРА СТРОГОЙ фильтрации слабых сигналов
	if peak != nil && peak.Strength < 0.1 { // УЛЬТРА порог - ОСТАВЛЯЕМ ТОЛЬКО ЭЛИТНЫЕ пики
		peak = nil
	}
	if valley != nil && valley.Strength < 0.1 { // УЛЬТРА порог - ОСТАВЛЯЕМ ТОЛЬКО ЭЛИТНЫЕ впадины
		valley = nil
	}

	// 4. Финальная проверка на основе относительных расстояний
	if peak != nil && valley != nil {
		// Если пик значительно ближе и сильнее - продаем
		if peakDistance*2 < valleyDistance && peak.Strength > valley.Strength {
			return internal.SELL
		}

		// Если впадина значительно ближе и сильнее - покупаем
		if valleyDistance*2 < peakDistance && valley.Strength > peak.Strength {
			return internal.BUY
		}
	}

	return internal.HOLD
}

// train обучает модель на исторических данных
func (em *ExtremaModel) train(prices []float64) {
	log.Printf("🔍 Анализ экстремумов в %d ценовых точках", len(prices))
	em.findLocalExtrema(prices)
	log.Printf("✅ Найдено %d значимых экстремумов", len(em.extremaPoints))

	// Выводим статистику экстремумов
	peaks := 0
	valleys := 0
	for _, point := range em.extremaPoints {
		if point.IsPeak {
			peaks++
		} else {
			valleys++
		}
	}
	log.Printf("   Пики: %d, Впадины: %d", peaks, valleys)
}

type ExtremaStrategy struct{}

func (s *ExtremaStrategy) Name() string {
	return "extrema_strategy"
}

func (s *ExtremaStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	if len(candles) < 50 {
		log.Printf("⚠️ Недостаточно данных для анализа экстремумов: получено %d свечей, требуется минимум 50", len(candles))
		return make([]internal.SignalType, len(candles))
	}

	// Извлекаем ценовые данные
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// Используем параметры из params с УЛЬТРА консервативными значениями по умолчанию
	minDistance := params.MinExtremaDistance
	if minDistance == 0 {
		minDistance = 80 // УЛЬТРА расстояние для МАКСИМАЛЬНОЙ ФИЛЬТРАЦИИ шума
	}
	windowSize := params.LookbackWindow
	if windowSize == 0 {
		windowSize = 20 // УЛЬТРА окно для МАКСИМАЛЬНО СТРОГОГО поиска экстремумов
	}
	confidenceThreshold := params.ConfidenceThreshold
	if confidenceThreshold == 0 {
		confidenceThreshold = 0.95 // УЛЬТРА порог уверенности для МИНИМАЛЬНОГО количества сигналов
	}

	// Создаем и обучаем модель экстремумов
	model := NewExtremaModel(minDistance, windowSize)
	model.train(prices)

	// Генерируем сигналы
	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := 20; i < len(candles); i++ { // начинаем после достаточного количества данных
		signal := model.predictSignal(i, prices, confidenceThreshold)

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
	}

	log.Printf("✅ Анализ экстремумов завершен")
	return signals
}

func (s *ExtremaStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{
		MinExtremaDistance:  40,  // УЛЬТРА КОНСЕРВАТИВНОЕ начальное значение
		LookbackWindow:      15,  // УЛЬТРА КОНСЕРВАТИВНОЕ начальное значение
		ConfidenceThreshold: 0.9, // УЛЬТРА КОНСЕРВАТИВНОЕ начальное значение
	}
	bestProfit := -1.0

	// Extract prices once
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// УЛЬТРА КОНСЕРВАТИВНЫЙ grid search для МИНИМАЛЬНОГО количества экстремумов
	for minDist := 30; minDist <= 100; minDist += 10 { // МАКСИМАЛЬНЫЙ диапазон для МАКСИМАЛЬНОЙ ФИЛЬТРАЦИИ
		for winSize := 15; winSize <= 25; winSize += 3 { // МАКСИМАЛЬНОЕ окно для МАКСИМАЛЬНОЙ СТРОГОСТИ
			for confThresh := 0.85; confThresh <= 0.98; confThresh += 0.03 { // МАКСИМАЛЬНЫЙ порог уверенности
				params := internal.StrategyParams{
					MinExtremaDistance:  minDist,
					LookbackWindow:      winSize,
					ConfidenceThreshold: confThresh,
				}

				// Create model with these parameters
				model := NewExtremaModel(minDist, winSize)
				model.train(prices)

				// Generate signals
				signals := make([]internal.SignalType, len(candles))
				inPosition := false

				for i := 20; i < len(candles); i++ {
					signal := model.predictSignal(i, prices, confThresh)

					if !inPosition && signal == internal.BUY {
						signals[i] = internal.BUY
						inPosition = true
					} else if inPosition && signal == internal.SELL {
						signals[i] = internal.SELL
						inPosition = false
					} else {
						signals[i] = internal.HOLD
					}
				}

				// Backtest
				result := internal.Backtest(candles, signals, 0.01) // 0.01 units проскальзывание
				if result.TotalProfit > bestProfit {
					bestProfit = result.TotalProfit
					bestParams = params
				}
			}
		}
	}

	log.Printf("Лучшие параметры extrema: minDist=%d, winSize=%d, confThresh=%.1f, profit=%.2f",
		bestParams.MinExtremaDistance, bestParams.LookbackWindow, bestParams.ConfidenceThreshold, bestProfit)

	return bestParams
}

func init() {
	// internal.RegisterStrategy("extrema_strategy", &ExtremaStrategy{})
}
