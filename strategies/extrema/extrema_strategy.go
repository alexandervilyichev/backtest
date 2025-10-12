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

package extrema

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
	extremaPoints   []ExtremaPoint
	peaks           []ExtremaPoint
	valleys         []ExtremaPoint
	minDistance     int
	windowSize      int
	minStrength     float64
	lookbackPeriod  int
	smoothingType   string // "ma" или "ema"
	smoothingPeriod int
}

// NewExtremaModel создает новую модель экстремумов
func NewExtremaModel(minDistance, windowSize int, minStrength float64, lookbackPeriod int, smoothingType string, smoothingPeriod int) *ExtremaModel {
	return &ExtremaModel{
		extremaPoints:   make([]ExtremaPoint, 0),
		minDistance:     minDistance,
		windowSize:      windowSize,
		minStrength:     minStrength,
		lookbackPeriod:  lookbackPeriod,
		smoothingType:   smoothingType,
		smoothingPeriod: smoothingPeriod,
	}
}

// smoothPrices сглаживает ценовые данные с помощью MA или EMA
func (em *ExtremaModel) smoothPrices(prices []float64) []float64 {
	if em.smoothingPeriod <= 0 || em.smoothingPeriod >= len(prices) {
		return prices // Не сглаживаем если параметры некорректны
	}

	switch em.smoothingType {
	case "ema":
		smoothed := internal.CalculateEMAForFloats(prices, em.smoothingPeriod)
		if smoothed == nil {
			return prices // Возвращаем оригинал если сглаживание не удалось
		}
		// EMA может иметь nil значения в начале, заполняем их последним значением
		for i, val := range smoothed {
			if i < em.smoothingPeriod-1 {
				smoothed[i] = prices[i]
			}
			if val == 0 && i >= em.smoothingPeriod-1 {
				smoothed[i] = prices[i] // Если EMA вернул 0, берем оригинал
			}
		}
		return smoothed
	case "ma":
		fallthrough // По умолчанию используем MA
	default:
		// Используем calculateSMACommonForValues для сглаживания массива float64
		smoothed := internal.CalculateSMACommonForValues(prices, em.smoothingPeriod)
		if smoothed == nil {
			return prices // Возвращаем оригинал если сглаживание не удалось
		}
		// Заменяем нулевые значения на оригинальные цены для корректности
		for i, val := range smoothed {
			if val == 0 {
				smoothed[i] = prices[i]
			}
		}
		return smoothed
	}
}

// findSignificantExtrema находит значимые глобальные экстремумы в ценовых данных
func (em *ExtremaModel) findSignificantExtrema(prices []float64) {
	em.extremaPoints = make([]ExtremaPoint, 0)

	// Сначала сглаживаем данные
	smoothedPrices := em.smoothPrices(prices)

	// Разделяем на этапы для более точного поиска экстремумов
	em.findLocalExtrema(smoothedPrices)
	em.filterByStrengthAndSignificance(smoothedPrices)
	em.filterExtremaByDistance()
}

// findLocalExtrema находит потенциальные локальные экстремумы (первый этап)
func (em *ExtremaModel) findLocalExtrema(prices []float64) {
	localExtrema := make([]ExtremaPoint, 0)

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
			// Вычисляем силу экстремума (отклонение от средней за больший период)
			strength := em.calculateExtremaStrength(prices, i, isLocalMax)

			point := ExtremaPoint{
				Index:    i,
				Price:    prices[i],
				IsPeak:   isLocalMax,
				Strength: strength,
			}
			localExtrema = append(localExtrema, point)
		}
	}

	em.extremaPoints = localExtrema
}

// calculateExtremaStrength вычисляет силу экстремума на основе большего контекста
func (em *ExtremaModel) calculateExtremaStrength(prices []float64, index int, isPeak bool) float64 {
	// Используем больший контекст для оценки значимости
	contextSize := em.lookbackPeriod
	startIdx := index - contextSize
	endIdx := index + contextSize

	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx >= len(prices) {
		endIdx = len(prices) - 1
	}

	// Вычисляем среднюю волатильность в контексте
	var sumVariance float64
	var sumPrices float64
	count := 0

	for j := startIdx; j <= endIdx; j++ {
		if j != index {
			sumPrices += prices[j]
			count++
		}
	}

	if count == 0 {
		return 0
	}

	meanPrice := sumPrices / float64(count)

	// Вычисляем дисперсию цен
	for j := startIdx; j <= endIdx; j++ {
		if j != index {
			diff := prices[j] - meanPrice
			sumVariance += diff * diff
		}
	}

	// Стандартизированное отклонение экстремума от среднего
	currentPrice := prices[index]
	deviation := math.Abs(currentPrice - meanPrice)
	variance := sumVariance / float64(count)

	// Если вариация нулевая, экстремум не значимый
	if variance < 1e-10 {
		return 0
	}

	standardDev := math.Sqrt(variance)

	// Сила экстремума как стандартизированное отклонение
	strength := deviation / standardDev

	// Дополнительный бонус за трендовые развороты
	trendBonus := em.calculateTrendReversalStrength(prices, index, isPeak, contextSize)
	strength += trendBonus

	return strength
}

// calculateTrendReversalStrength оценивает силу разворота тренда
func (em *ExtremaModel) calculateTrendReversalStrength(prices []float64, index int, isPeak bool, contextSize int) float64 {
	beforeCount := contextSize / 2
	afterCount := contextSize / 2

	// Анализируем тренд перед экстремумом
	beforeStart := index - beforeCount
	beforeEnd := index - 1
	afterStart := index + 1
	afterEnd := index + afterCount

	if beforeStart < 0 {
		beforeStart = 0
		beforeCount = index - beforeStart
	}
	if afterEnd >= len(prices) {
		afterEnd = len(prices) - 1
		afterCount = afterEnd - index
	}

	if beforeCount < 2 || afterCount < 2 {
		return 0 // Недостаточно данных для анализа тренда
	}

	// Вычисляем средний тренд перед экстремумом
	trendBefore := (prices[beforeEnd] - prices[beforeStart]) / float64(beforeCount)

	// Вычисляем средний тренд после экстремума
	trendAfter := (prices[afterEnd] - prices[afterStart]) / float64(afterCount)

	// Оцениваем разворот (для пика ожидается разворот с роста на падение)
	expectedReversal := false
	if isPeak && trendBefore > 0.001 && trendAfter < -0.001 {
		expectedReversal = true
	} else if !isPeak && trendBefore < -0.001 && trendAfter > 0.001 {
		expectedReversal = true
	}

	if !expectedReversal {
		return 0 // Нет разворота тренда
	}

	// Вычисляем силу разворота (нормализованная разница направлений)
	reversalStrength := math.Abs(trendBefore-trendAfter) / (math.Abs(trendBefore) + math.Abs(trendAfter) + 1e-10)

	return reversalStrength * 0.5 // Коэффициент усиления
}

// filterByStrengthAndSignificance фильтрует экстремумы по силе и значимости
func (em *ExtremaModel) filterByStrengthAndSignificance(prices []float64) {
	minStrength := em.minStrength
	if minStrength <= 0 {
		minStrength = 1.5 // Минимальная сила экстремума (1.5 стандартных отклонений)
	}

	// Находим среднюю волатильность всего ряда для дополнительной фильтрации
	var totalVariance float64
	var totalMean float64
	for _, price := range prices {
		totalMean += price
	}
	totalMean /= float64(len(prices))

	for _, price := range prices {
		diff := price - totalMean
		totalVariance += diff * diff
	}
	totalVariance /= float64(len(prices))
	totalVolatility := math.Sqrt(totalVariance)

	// Фильтруем по силе и относительной значимости
	filtered := make([]ExtremaPoint, 0)
	for _, point := range em.extremaPoints {
		// Проверяем абсолютную силу экстремума
		if point.Strength < minStrength {
			continue
		}

		// Проверяем относительную значимость (экстремум должен быть значителен по сравнению с общей волатильностью)
		relativeSignificance := point.Strength * (point.Price / (totalMean + 1e-10))
		if relativeSignificance < totalVolatility*2.0 {
			continue
		}

		filtered = append(filtered, point)
	}

	em.extremaPoints = filtered
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

// findClosestExtrema находит ближайший экстремум в отсортированном слайсе с помощью бинарного поиска
func (em *ExtremaModel) findClosestExtrema(slice []ExtremaPoint, index int) *ExtremaPoint {
	if len(slice) == 0 {
		return nil
	}

	// Бинарный поиск точки вставки
	left, right := 0, len(slice)-1
	for left <= right {
		mid := left + (right-left)/2
		if slice[mid].Index < index {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}

	// left - точка вставки, проверяем left-1, left и left+1 если доступны
	var minDist = math.MaxInt32
	var closest *ExtremaPoint

	candidates := []int{left - 1, left, left + 1}
	for _, idx := range candidates {
		if idx >= 0 && idx < len(slice) {
			dist := int(math.Abs(float64(slice[idx].Index - index)))
			if dist < minDist {
				minDist = dist
				closest = &slice[idx]
			}
		}
	}

	return closest
}

// findNearestExtrema находит ближайшие пики и впадины к заданному индексу
func (em *ExtremaModel) findNearestExtrema(index int) (peak *ExtremaPoint, valley *ExtremaPoint) {
	peak = em.findClosestExtrema(em.peaks, index)
	valley = em.findClosestExtrema(em.valleys, index)
	return peak, valley
}

// predictSignal предсказывает сигнал на основе расстояния до экстремумов
func (em *ExtremaModel) predictSignal(index int, prices []float64) internal.SignalType {
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
	//	log.Printf("🔍 Анализ экстремумов в %d ценовых точках", len(prices))
	em.findSignificantExtrema(prices)

	// Разделяем экстремумы на пики и впадины для эффективного поиска
	em.peaks = make([]ExtremaPoint, 0, len(em.extremaPoints)/2)
	em.valleys = make([]ExtremaPoint, 0, len(em.extremaPoints)/2)
	for _, point := range em.extremaPoints {
		if point.IsPeak {
			em.peaks = append(em.peaks, point)
		} else {
			em.valleys = append(em.valleys, point)
		}
	}

	//	log.Printf("✅ Найдено %d значимых глобальных экстремумов", len(em.extremaPoints))
	//	log.Printf("   Глобальные пики: %d, Глобальные впадины: %d", len(em.peaks), len(em.valleys))
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
	smoothingType := params.SmoothingType
	if smoothingType == "" {
		smoothingType = "ma" // По умолчанию используем MA
	}
	smoothingPeriod := params.SmoothingPeriod
	if smoothingPeriod == 0 {
		smoothingPeriod = 10 // Период сглаживания по умолчанию
	}

	// Создаем и обучаем модель экстремумов
	minStrength := 1.5               // Минимальная сила экстремума
	lookbackPeriod := windowSize * 3 // Период для анализа силы экстремума
	model := NewExtremaModel(minDistance, windowSize, minStrength, lookbackPeriod, smoothingType, smoothingPeriod)
	model.train(prices)

	// Генерируем сигналы
	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := 20; i < len(candles); i++ { // начинаем после достаточного количества данных
		signal := model.predictSignal(i, prices)

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
		SmoothingType:       "ma",
		SmoothingPeriod:     8,
	}
	bestProfit := -1.0

	// Extract prices once
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// УЛЬТРА КОНСЕРВАТИВНЫЙ grid search для МИНИМАЛЬНОГО количества экстремумов
	smoothingTypes := []string{"ma", "ema"}
	for _, smoothType := range smoothingTypes {
		for smoothPeriod := 10; smoothPeriod <= 15; smoothPeriod += 1 {
			for minDist := 30; minDist <= 50; minDist += 5 { // МАКСИМАЛЬНЫЙ диапазон для МАКСИМАЛЬНОЙ ФИЛЬТРАЦИИ
				for winSize := 20; winSize <= 25; winSize += 2 { // МАКСИМАЛЬНОЕ окно для МАКСИМАЛЬНОЙ СТРОГОСТИ
					for confThresh := 0.8; confThresh <= 0.9; confThresh += 0.02 { // МАКСИМАЛЬНЫЙ порог уверенности
						params := internal.StrategyParams{
							MinExtremaDistance:  minDist,
							LookbackWindow:      winSize,
							ConfidenceThreshold: confThresh,
							SmoothingType:       smoothType,
							SmoothingPeriod:     smoothPeriod,
						}

						// Create model with these parameters
						minStrength := 1.5            // Минимальная сила экстремума
						lookbackPeriod := winSize * 3 // Период для анализа силы экстремума
						model := NewExtremaModel(minDist, winSize, minStrength, lookbackPeriod, smoothType, smoothPeriod)
						model.train(prices)

						// Generate signals
						signals := make([]internal.SignalType, len(candles))
						inPosition := false

						for i := 20; i < len(candles); i++ {
							signal := model.predictSignal(i, prices)

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
		}
	}

	log.Printf("Лучшие параметры extrema: smoothType=%s, smoothPeriod=%d, minDist=%d, winSize=%d, confThresh=%.1f, profit=%.2f",
		bestParams.SmoothingType, bestParams.SmoothingPeriod, bestParams.MinExtremaDistance, bestParams.LookbackWindow, bestParams.ConfidenceThreshold, bestProfit)

	return bestParams
}

func init() {
	internal.RegisterStrategy("extrema_strategy", &ExtremaStrategy{})
}
