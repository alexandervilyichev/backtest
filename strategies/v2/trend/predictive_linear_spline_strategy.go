// Predictive Linear Spline Strategy V2
//
// Описание стратегии:
// Стратегия использует линейные сплайны для моделирования трендов и ПРОГНОЗИРОВАНИЯ
// будущих точек разворота. В отличие от linear_spline_strategy, которая определяет
// развороты постфактум, эта версия экстраполирует текущий линейный тренд и предсказывает,
// где он может развернуться, выставляя сигналы заранее.
//
// Как работает:
// - Анализирует исторические данные и строит линейные сплайны для текущего тренда
// - Экстраполирует линейный тренд на несколько свечей вперед
// - Определяет ожидаемую точку разворота на основе:
//   * Исторической длины предыдущих трендов
//   * Силы текущего тренда (коэффициент R² и наклон)
//   * Расстояния от начала тренда
//   * Ускорения/замедления тренда
// - Выставляет сигнал BUY/SELL за несколько свечей до ожидаемого разворота
//
// Параметры:
// - MinSegmentLength: минимальная длина сегмента для анализа (по умолчанию 5)
// - MaxSegmentLength: максимальная длина сегмента для анализа (по умолчанию 50)
// - PredictionHorizon: горизонт предсказания в свечах (по умолчанию 5)
// - MinR2Threshold: минимальный R² для доверия к модели (по умолчанию 0.6)
// - SignalAdvance: за сколько свечей до разворота выставлять сигнал (по умолчанию 3)
// - MinSlopeThreshold: минимальный наклон для определения тренда (по умолчанию 0.0005)
// - TrendExhaustionFactor: коэффициент истощения тренда (по умолчанию 0.8)
//
// Сильные стороны:
// - Предсказывает развороты заранее, а не постфактум
// - Использует простую линейную экстраполяцию
// - Учитывает историческую длину трендов
// - Анализирует ускорение/замедление тренда
// - Может работать на разных таймфреймах
//
// Слабые стороны:
// - Линейная экстраполяция может быть неточной при нелинейных движениях
// - Требует достаточно данных для анализа исторических трендов
// - Может давать ранние сигналы в сильных трендах
// - Чувствительна к выбросам и шуму в данных
//
// Лучшие условия для применения:
// - Рынки с предсказуемыми трендами и разворотами
// - Средне- и долгосрочная торговля
// - В сочетании с индикаторами подтверждения тренда
// - На активах с хорошей ликвидностью и низким шумом

package trend

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
	"math"
)

type PredictiveLinearSplineConfig struct {
	MinSegmentLength      int     `json:"min_segment_length"`
	MaxSegmentLength      int     `json:"max_segment_length"`
	PredictionHorizon     int     `json:"prediction_horizon"`
	MinR2Threshold        float64 `json:"min_r2_threshold"`
	SignalAdvance         int     `json:"signal_advance"`
	MinSlopeThreshold     float64 `json:"min_slope_threshold"`
	TrendExhaustionFactor float64 `json:"trend_exhaustion_factor"`
	MinPriceChange        float64 `json:"min_price_change"`
}

func (c *PredictiveLinearSplineConfig) Validate() error {
	if c.MinSegmentLength < 2 {
		return errors.New("min segment length must be at least 2")
	}
	if c.MaxSegmentLength <= c.MinSegmentLength {
		return errors.New("max segment length must be greater than min")
	}
	if c.PredictionHorizon < 1 {
		return errors.New("prediction horizon must be positive")
	}
	if c.MinR2Threshold < 0 || c.MinR2Threshold > 1 {
		return errors.New("min R2 threshold must be between 0 and 1")
	}
	if c.SignalAdvance < 1 {
		return errors.New("signal advance must be positive")
	}
	if c.MinSlopeThreshold < 0 {
		return errors.New("min slope threshold must be non-negative")
	}
	if c.TrendExhaustionFactor <= 0 || c.TrendExhaustionFactor > 1 {
		c.TrendExhaustionFactor = 0.8 // по умолчанию
	}
	if c.MinPriceChange <= 0 {
		c.MinPriceChange = 0.003 // 0.3% по умолчанию (более мягкий фильтр)
	}
	return nil
}

func (c *PredictiveLinearSplineConfig) String() string {
	return fmt.Sprintf("PredictiveLinearSpline(min_len=%d, max_len=%d, horizon=%d, r2=%.2f, advance=%d, slope=%.5f, exhaust=%.2f, price_chg=%.2f%%)",
		c.MinSegmentLength, c.MaxSegmentLength, c.PredictionHorizon, c.MinR2Threshold,
		c.SignalAdvance, c.MinSlopeThreshold, c.TrendExhaustionFactor, c.MinPriceChange*100)
}

// PredictiveLinearSegment представляет линейный сегмент с предсказанием
type PredictiveLinearSegment struct {
	StartIdx    int
	EndIdx      int
	Slope       float64
	Intercept   float64
	R2          float64
	IsAscending bool
	StartPrice  float64
	EndPrice    float64
}

// PredictedLinearReversal представляет предсказанную точку разворота
type PredictedLinearReversal struct {
	PredictedIndex int
	PredictedPrice float64
	Confidence     float64
	SignalType     internal.SignalType
	TrendLength    int
}

type PredictiveLinearSplineSignalGenerator struct{}

func NewPredictiveLinearSplineSignalGenerator() *PredictiveLinearSplineSignalGenerator {
	return &PredictiveLinearSplineSignalGenerator{}
}

// PredictNextSignal предсказывает ближайший сигнал в будущем
func (sg *PredictiveLinearSplineSignalGenerator) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	plsConfig, ok := config.(*PredictiveLinearSplineConfig)
	if !ok {
		return nil
	}

	if err := plsConfig.Validate(); err != nil {
		log.Printf("⚠️ Ошибка валидации конфигурации: %v", err)
		return nil
	}

	if len(candles) < plsConfig.MinSegmentLength*2 {
		log.Printf("⚠️ Недостаточно данных для предсказания: получено %d свечей, требуется минимум %d", len(candles), plsConfig.MinSegmentLength*2)
		return nil
	}

	// Извлекаем цены закрытия
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	analyzer := NewPredictiveLinearAnalyzer(plsConfig)
	currentIdx := len(candles) - 1

	// Анализируем текущий тренд
	segment := analyzer.analyzeCurrentTrend(prices, currentIdx)
	if segment == nil {
		log.Printf("⚠️ Не удалось определить текущий тренд")
		return nil
	}

	if segment.R2 < plsConfig.MinR2Threshold {
		log.Printf("⚠️ Недостаточная уверенность в тренде (R²=%.3f < %.3f)", segment.R2, plsConfig.MinR2Threshold)
		return nil
	}

	// Предсказываем разворот
	prediction := analyzer.predictReversal(segment, currentIdx, prices)
	if prediction == nil {
		log.Printf("⚠️ Не удалось предсказать разворот")
		return nil
	}

	// Адаптивный порог уверенности
	confidenceThreshold := 0.30
	if len(candles) > 10000 {
		confidenceThreshold = 0.40
	} else if len(candles) > 5000 {
		confidenceThreshold = 0.35
	}

	if prediction.Confidence < confidenceThreshold {
		log.Printf("⚠️ Недостаточная уверенность в предсказании (%.3f < %.3f)", prediction.Confidence, confidenceThreshold)
		return nil
	}

	// Вычисляем индекс сигнала (за SignalAdvance свечей до разворота)
	signalIdx := prediction.PredictedIndex - plsConfig.SignalAdvance
	if signalIdx <= currentIdx {
		// Сигнал должен быть в будущем
		signalIdx = currentIdx + 1
	}

	// Вычисляем дату сигнала
	// Предполагаем, что свечи идут с постоянным интервалом
	if len(candles) < 2 {
		return nil
	}

	// Вычисляем средний интервал между свечами
	timeInterval := (candles[len(candles)-1].ToTime().Unix() - candles[0].ToTime().Unix()) / int64(len(candles)-1)
	lastTimestamp := candles[len(candles)-1].ToTime().Unix()
	futureTimestamp := lastTimestamp + timeInterval*int64(signalIdx-currentIdx)

	// Экстраполируем цену в точке сигнала
	localX := float64(segment.EndIdx - segment.StartIdx)
	futureX := localX + float64(signalIdx-currentIdx)
	predictedPrice := segment.Slope*futureX + segment.Intercept

	return &internal.FutureSignal{
		SignalType: prediction.SignalType,
		Date:       futureTimestamp,
		Price:      predictedPrice,
		Confidence: prediction.Confidence,
	}
}

// PredictiveLinearAnalyzer анализирует ценовые данные с помощью линейных сплайнов
type PredictiveLinearAnalyzer struct {
	minSegmentLength      int
	maxSegmentLength      int
	predictionHorizon     int
	minR2Threshold        float64
	signalAdvance         int
	minSlopeThreshold     float64
	trendExhaustionFactor float64
	minPriceChange        float64
	// История трендов для анализа
	historicalTrends []*PredictiveLinearSegment
	lastAnalyzedIdx  int
	currentSegment   *PredictiveLinearSegment
}

func NewPredictiveLinearAnalyzer(config *PredictiveLinearSplineConfig) *PredictiveLinearAnalyzer {
	return &PredictiveLinearAnalyzer{
		minSegmentLength:      config.MinSegmentLength,
		maxSegmentLength:      config.MaxSegmentLength,
		predictionHorizon:     config.PredictionHorizon,
		minR2Threshold:        config.MinR2Threshold,
		signalAdvance:         config.SignalAdvance,
		minSlopeThreshold:     config.MinSlopeThreshold,
		trendExhaustionFactor: config.TrendExhaustionFactor,
		minPriceChange:        config.MinPriceChange,
		historicalTrends:      make([]*PredictiveLinearSegment, 0),
	}
}

// linearRegression выполняет линейную регрессию для данных
func (pla *PredictiveLinearAnalyzer) linearRegression(y []float64) (slope, intercept float64) {
	n := float64(len(y))
	if n < 2 {
		return 0, y[0]
	}

	sumX, sumY, sumXY, sumXX := 0.0, 0.0, 0.0, 0.0

	for i, yi := range y {
		xi := float64(i)
		sumX += xi
		sumY += yi
		sumXY += xi * yi
		sumXX += xi * xi
	}

	denominator := n*sumXX - sumX*sumX
	if math.Abs(denominator) < 1e-10 {
		return 0, sumY / n
	}

	slope = (n*sumXY - sumX*sumY) / denominator
	intercept = (sumY - slope*sumX) / n

	return slope, intercept
}

// calculateR2 вычисляет коэффициент детерминации
func (pla *PredictiveLinearAnalyzer) calculateR2(y []float64, slope, intercept float64) float64 {
	if len(y) < 2 {
		return 0
	}

	mean := 0.0
	for _, yi := range y {
		mean += yi
	}
	mean /= float64(len(y))

	ssRes := 0.0
	ssTot := 0.0

	for i, yi := range y {
		predicted := slope*float64(i) + intercept
		ssRes += (yi - predicted) * (yi - predicted)
		ssTot += (yi - mean) * (yi - mean)
	}

	if ssTot == 0 {
		return 0
	}

	return 1 - ssRes/ssTot
}

// analyzeCurrentTrend анализирует текущий тренд и строит модель
func (pla *PredictiveLinearAnalyzer) analyzeCurrentTrend(prices []float64, currentIdx int) *PredictiveLinearSegment {
	// Кэш: если анализировали недавно, используем кэшированный результат
	if pla.currentSegment != nil && currentIdx-pla.lastAnalyzedIdx < pla.minSegmentLength/2 {
		return pla.currentSegment
	}

	// Определяем окно для анализа
	startIdx := currentIdx - pla.maxSegmentLength
	if startIdx < 0 {
		startIdx = 0
	}

	if currentIdx-startIdx < pla.minSegmentLength {
		return nil
	}

	// Извлекаем данные для анализа
	window := prices[startIdx : currentIdx+1]

	// Пробуем разные длины сегмента
	var bestSegment *PredictiveLinearSegment
	bestR2 := -1.0

	step := pla.minSegmentLength / 2
	if step < 1 {
		step = 1
	}

	for length := pla.minSegmentLength; length <= len(window) && length <= pla.maxSegmentLength; length += step {
		segmentStart := len(window) - length
		segment := window[segmentStart:]

		slope, intercept := pla.linearRegression(segment)
		r2 := pla.calculateR2(segment, slope, intercept)

		// Проверяем минимальный наклон
		if math.Abs(slope) < pla.minSlopeThreshold {
			continue
		}

		if r2 > bestR2 {
			bestR2 = r2

			isAscending := slope > 0

			bestSegment = &PredictiveLinearSegment{
				StartIdx:    startIdx + segmentStart,
				EndIdx:      currentIdx,
				Slope:       slope,
				Intercept:   intercept,
				R2:          r2,
				IsAscending: isAscending,
				StartPrice:  segment[0],
				EndPrice:    segment[len(segment)-1],
			}
		}
	}

	// Кэшируем результат
	pla.lastAnalyzedIdx = currentIdx
	pla.currentSegment = bestSegment

	return bestSegment
}

// calculateAverageTrendLength вычисляет среднюю длину исторических трендов
func (pla *PredictiveLinearAnalyzer) calculateAverageTrendLength() float64 {
	if len(pla.historicalTrends) == 0 {
		return float64(pla.minSegmentLength+pla.maxSegmentLength) / 2
	}

	totalLength := 0
	for _, trend := range pla.historicalTrends {
		totalLength += trend.EndIdx - trend.StartIdx + 1
	}

	return float64(totalLength) / float64(len(pla.historicalTrends))
}

// calculateTrendMomentum вычисляет "импульс" тренда (ускорение/замедление)
func (pla *PredictiveLinearAnalyzer) calculateTrendMomentum(prices []float64, segment *PredictiveLinearSegment) float64 {
	if segment == nil || segment.EndIdx-segment.StartIdx < pla.minSegmentLength {
		return 1.0
	}

	segmentLength := segment.EndIdx - segment.StartIdx + 1
	if segmentLength < 4 {
		return 1.0
	}

	// Сравниваем наклон первой и второй половины тренда
	midPoint := segment.StartIdx + segmentLength/2

	firstHalf := prices[segment.StartIdx:midPoint]
	secondHalf := prices[midPoint : segment.EndIdx+1]

	slope1, _ := pla.linearRegression(firstHalf)
	slope2, _ := pla.linearRegression(secondHalf)

	// Если наклоны одного знака, вычисляем отношение
	if (slope1 > 0 && slope2 > 0) || (slope1 < 0 && slope2 < 0) {
		if math.Abs(slope1) < 1e-10 {
			return 1.0
		}
		momentum := math.Abs(slope2) / math.Abs(slope1)
		// Ограничиваем значение
		if momentum > 2.0 {
			momentum = 2.0
		}
		if momentum < 0.5 {
			momentum = 0.5
		}
		return momentum
	}

	// Если знаки разные, тренд уже разворачивается
	return 0.5
}

// predictReversal предсказывает точку разворота на основе текущего тренда
func (pla *PredictiveLinearAnalyzer) predictReversal(segment *PredictiveLinearSegment, currentIdx int, prices []float64) *PredictedLinearReversal {
	if segment == nil || segment.R2 < pla.minR2Threshold {
		return nil
	}

	// Длина текущего сегмента
	currentTrendLength := segment.EndIdx - segment.StartIdx + 1

	// Проверяем минимальное изменение цены
	priceChange := math.Abs(segment.EndPrice-segment.StartPrice) / segment.StartPrice
	if priceChange < pla.minPriceChange {
		return nil
	}

	// Вычисляем среднюю длину исторических трендов
	avgTrendLength := pla.calculateAverageTrendLength()

	// Вычисляем импульс тренда (ускорение/замедление)
	momentum := pla.calculateTrendMomentum(prices, segment)

	// Упрощенное предсказание расстояния до разворота
	var predictedDistance int

	// Базовое расстояние зависит от текущей длины тренда
	exhaustionRatio := float64(currentTrendLength) / avgTrendLength

	if exhaustionRatio >= pla.trendExhaustionFactor {
		// Тренд близок к истощению - разворот скоро
		predictedDistance = pla.predictionHorizon
	} else if exhaustionRatio >= 0.5 {
		// Тренд в середине - умеренное расстояние
		predictedDistance = pla.predictionHorizon * 2
	} else {
		// Тренд только начался - разворот далеко
		predictedDistance = pla.predictionHorizon * 3
	}

	// Корректируем на основе импульса
	if momentum < 0.8 {
		// Тренд сильно замедляется - разворот ближе
		predictedDistance = int(float64(predictedDistance) * 0.7)
	} else if momentum > 1.2 {
		// Тренд ускоряется - разворот дальше
		predictedDistance = int(float64(predictedDistance) * 1.3)
	}

	// Минимальное расстояние
	if predictedDistance < pla.predictionHorizon {
		predictedDistance = pla.predictionHorizon
	}

	predictedIdx := currentIdx + predictedDistance

	// Экстраполируем цену в точке разворота
	localX := float64(currentTrendLength - 1)
	futureX := localX + float64(predictedDistance)
	predictedPrice := segment.Slope*futureX + segment.Intercept

	// Определяем тип сигнала (противоположный текущему тренду)
	var signalType internal.SignalType
	if segment.IsAscending {
		signalType = internal.SELL
	} else {
		signalType = internal.BUY
	}

	// Улучшенная формула уверенности с акцентом на качество
	// Базовая уверенность от R² (основной фактор)
	confidence := segment.R2 * 0.6 // Снижаем вес R² для баланса

	// Бонус за истощение тренда (важный фактор)
	exhaustionFactor := float64(currentTrendLength) / avgTrendLength
	if exhaustionFactor > pla.trendExhaustionFactor {
		exhaustionBonus := (exhaustionFactor - pla.trendExhaustionFactor) * 0.4
		confidence += exhaustionBonus
	} else {
		// Штраф за слишком ранний сигнал
		earlyPenalty := (pla.trendExhaustionFactor - exhaustionFactor) * 0.2
		confidence -= earlyPenalty
	}

	// Бонус за замедление тренда (признак разворота)
	if momentum < 0.9 {
		momentumBonus := (0.9 - momentum) * 0.3
		confidence += momentumBonus
	} else if momentum > 1.1 {
		// Штраф за ускорение (тренд продолжается)
		momentumPenalty := (momentum - 1.1) * 0.2
		confidence -= momentumPenalty
	}

	// Штраф за слишком далекое предсказание
	if predictedDistance > pla.predictionHorizon*2 {
		distancePenalty := float64(predictedDistance-pla.predictionHorizon*2) / float64(pla.predictionHorizon*2)
		confidence -= distancePenalty * 0.4
	}

	// Бонус за сильное изменение цены (подтверждение тренда)
	priceChangeBonus := 0.0
	if priceChange > 0.02 {
		priceChangeBonus = 0.15
	} else if priceChange > 0.01 {
		priceChangeBonus = 0.10
	} else if priceChange > 0.005 {
		priceChangeBonus = 0.05
	}
	confidence += priceChangeBonus

	// Штраф за слишком короткий тренд
	if currentTrendLength < pla.minSegmentLength*2 {
		shortTrendPenalty := 0.15
		confidence -= shortTrendPenalty
	}

	// Ограничиваем диапазон
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0 {
		confidence = 0
	}

	return &PredictedLinearReversal{
		PredictedIndex: predictedIdx,
		PredictedPrice: predictedPrice,
		Confidence:     confidence,
		SignalType:     signalType,
		TrendLength:    currentTrendLength,
	}
}

// addToHistory добавляет завершенный тренд в историю
func (pla *PredictiveLinearAnalyzer) addToHistory(segment *PredictiveLinearSegment) {
	if segment == nil {
		return
	}
	pla.historicalTrends = append(pla.historicalTrends, segment)

	// Ограничиваем размер истории (храним последние 20 трендов)
	if len(pla.historicalTrends) > 20 {
		pla.historicalTrends = pla.historicalTrends[1:]
	}
}

func (sg *PredictiveLinearSplineSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	plsConfig, ok := config.(*PredictiveLinearSplineConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := plsConfig.Validate(); err != nil {
		log.Printf("⚠️ Ошибка валидации конфигурации: %v", err)
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < plsConfig.MinSegmentLength*2 {
		log.Printf("⚠️ Недостаточно данных: получено %d свечей, требуется минимум %d", len(candles), plsConfig.MinSegmentLength*2)
		return make([]internal.SignalType, len(candles))
	}

	// Извлекаем цены закрытия
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	analyzer := NewPredictiveLinearAnalyzer(plsConfig)
	signals := make([]internal.SignalType, len(candles))

	// Храним активные предсказания
	var activePrediction *PredictedLinearReversal
	lastSignalIdx := -1
	lastSignalType := internal.HOLD

	// Адаптивное минимальное расстояние между сигналами
	minSignalDistance := plsConfig.MinSegmentLength * 2
	if len(candles) > 10000 {
		minSignalDistance = plsConfig.MinSegmentLength * 3 // Больше расстояние для больших данных
	}

	// Адаптивный порог уверенности в зависимости от размера данных
	confidenceThreshold := 0.30
	if len(candles) > 10000 {
		confidenceThreshold = 0.40 // Более строгий порог для больших данных
	} else if len(candles) > 5000 {
		confidenceThreshold = 0.35
	}

	// Начинаем анализ после накопления достаточных данных
	startIdx := plsConfig.MaxSegmentLength
	if startIdx < plsConfig.MinSegmentLength*2 {
		startIdx = plsConfig.MinSegmentLength * 2
	}

	for i := startIdx; i < len(candles); i++ {
		// Проверяем, не пора ли выставить сигнал по активному предсказанию
		if activePrediction != nil {
			signalIdx := activePrediction.PredictedIndex - plsConfig.SignalAdvance

			if i == signalIdx && activePrediction.Confidence >= confidenceThreshold {
				if lastSignalIdx < 0 || i-lastSignalIdx >= minSignalDistance {
					if lastSignalType == internal.HOLD || activePrediction.SignalType != lastSignalType {
						signals[i] = activePrediction.SignalType
						lastSignalIdx = i
						lastSignalType = activePrediction.SignalType

						// Добавляем текущий тренд в историю
						if analyzer.currentSegment != nil {
							analyzer.addToHistory(analyzer.currentSegment)
						}

						activePrediction = nil
						continue
					}
				}
			}

			// Если прошли точку предсказания, сбрасываем его
			if i > activePrediction.PredictedIndex {
				activePrediction = nil
			}
		}

		// Анализируем текущий тренд и делаем новое предсказание
		analysisInterval := plsConfig.MinSegmentLength / 2
		if analysisInterval < 3 {
			analysisInterval = 3
		}

		if activePrediction == nil && (lastSignalIdx < 0 || i-lastSignalIdx >= analysisInterval) {
			segment := analyzer.analyzeCurrentTrend(prices, i)
			if segment != nil {
				if segment.R2 >= plsConfig.MinR2Threshold {
					prediction := analyzer.predictReversal(segment, i, prices)
					if prediction != nil {
						if prediction.Confidence >= confidenceThreshold {
							activePrediction = prediction
						}
					}
				}
			}
		}
	}

	return signals
}

type PredictiveLinearSplineConfigGenerator struct{}

func NewPredictiveLinearSplineConfigGenerator() *PredictiveLinearSplineConfigGenerator {
	return &PredictiveLinearSplineConfigGenerator{}
}

func (g *PredictiveLinearSplineConfigGenerator) Generate() []internal.StrategyConfigV2 {
	configs := []internal.StrategyConfigV2{}

	// Более консервативные параметры для лучших результатов на больших данных
	minLengths := []int{ /*5, 8, 12, 15, 20,*/ 20, 50, 125}
	maxLengths := []int{ /*30, 40, 60, 80, 100,*/ 100, 150, 445}
	horizons := []int{5,7 /*, 7, 10, 20, 40, 100*/}
	r2Thresholds := []float64{0.645, 0.65, 0.655}
	advances := []int{ 3, 4 ,5}
	slopeThresholds := []float64{0.00045, 0.00055, 0.00065, 0.00075}
	exhaustionFactors := []float64{0.40, 0.50, 0.60, 0.70}
	priceChanges := []float64{0.008 /*, 0.0085, 0.015*/}

	// Генерируем комбинации с фокусом на качество, а не количество
	for _, minLen := range minLengths {
		for _, maxLen := range maxLengths {
			if maxLen <= minLen*2 {
				continue
			}
			// Ограничиваем количество комбинаций для ускорения
			for _, horizon := range horizons {
				for _, r2 := range r2Thresholds {
					// Пропускаем некоторые комбинации для оптимизации

					for _, advance := range advances {
						for _, slope := range slopeThresholds {
							for _, exhaust := range exhaustionFactors {

								for _, priceChg := range priceChanges {
									// Добавляем только логичные комбинации:
									// Высокий R² + высокое изменение цены
									// Низкий R² + низкое изменение цены
									if (r2 >= 0.75 && priceChg >= 0.008) || (r2 <= 0.70 && priceChg <= 0.010) {
										configs = append(configs, &PredictiveLinearSplineConfig{
											MinSegmentLength:      minLen,
											MaxSegmentLength:      maxLen,
											PredictionHorizon:     horizon,
											MinR2Threshold:        r2,
											SignalAdvance:         advance,
											MinSlopeThreshold:     slope,
											TrendExhaustionFactor: exhaust,
											MinPriceChange:        priceChg,
										})
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return configs
}

type PredictiveLinearSplineStrategy struct{}

func (s *PredictiveLinearSplineStrategy) Name() string {
	return "predictive_linear_spline"
}

func NewPredictiveLinearSplineStrategyV2(slippage float64) internal.TradingStrategy {
	slippageProvider := internal.NewSlippageProvider(slippage)
	signalGenerator := NewPredictiveLinearSplineSignalGenerator()

	configManager := internal.NewConfigManager(
		&PredictiveLinearSplineConfig{
			MinSegmentLength:      125,
			MaxSegmentLength:      445,
			PredictionHorizon:     5,
			MinR2Threshold:        0.65,
			SignalAdvance:         5,
			MinSlopeThreshold:     0.00055,
			TrendExhaustionFactor: 0.60,
			MinPriceChange:        0.008,
		},



		func() internal.StrategyConfigV2 {
			return &PredictiveLinearSplineConfig{}
		},
	)

	configGenerator := NewPredictiveLinearSplineConfigGenerator()
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	return internal.NewStrategyBase(
		"predictive_linear_spline_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewPredictiveLinearSplineStrategyV2(0.01)
	internal.RegisterStrategyV2(strategy)
}
