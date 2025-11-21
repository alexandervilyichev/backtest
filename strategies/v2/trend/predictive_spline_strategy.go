// Predictive Quadratic Spline Strategy
//
// Описание стратегии:
// Стратегия использует квадратичные сплайны для моделирования трендов и предсказания
// будущих точек разворота. В отличие от v1 версии, которая определяет развороты
// постфактум, эта версия экстраполирует текущий тренд и предсказывает, где он
// может развернуться, выставляя сигналы заранее.
//
// Как работает:
// - Анализирует исторические данные и строит квадратичные сплайны для текущего тренда
// - Вычисляет точку перегиба (inflection point) квадратичной функции
// - Экстраполирует тренд на несколько свечей вперед
// - Определяет ожидаемую точку разворота на основе:
//   * Точки перегиба квадратичной функции (где производная = 0)
//   * Исторической длины предыдущих трендов
//   * Силы текущего тренда (коэффициент R²)
// - Выставляет сигнал BUY/SELL за несколько свечей до ожидаемого разворота
//
// Параметры:
// - MinSegmentLength: минимальная длина сегмента для анализа (по умолчанию 5)
// - MaxSegmentLength: максимальная длина сегмента для анализа (по умолчанию 50)
// - PredictionHorizon: горизонт предсказания в свечах (по умолчанию 5)
// - MinR2Threshold: минимальный R² для доверия к модели (по умолчанию 0.7)
// - SignalAdvance: за сколько свечей до разворота выставлять сигнал (по умолчанию 3)
//
// Сильные стороны:
// - Предсказывает развороты заранее, а не постфактум
// - Использует математическую модель для экстраполяции
// - Учитывает силу тренда через R²
// - Может работать на разных таймфреймах
//
// Слабые стороны:
// - Экстраполяция может быть неточной при резких изменениях рынка
// - Требует достаточно данных для построения надежной модели
// - Может давать ранние сигналы в сильных трендах
// - Чувствительна к выбросам и шуму в данных
//
// Лучшие условия для применения:
// - Рынки с плавными трендами и предсказуемыми разворотами
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

type PredictiveSplineConfig struct {
	MinSegmentLength  int     `json:"min_segment_length"`
	MaxSegmentLength  int     `json:"max_segment_length"`
	PredictionHorizon int     `json:"prediction_horizon"`
	MinR2Threshold    float64 `json:"min_r2_threshold"`
	SignalAdvance     int     `json:"signal_advance"`
	MinPriceChange    float64 `json:"min_price_change"`     // Минимальное изменение цены для сигнала (%)
	MinTrendStrength  float64 `json:"min_trend_strength"`   // Минимальная сила тренда
}

func (c *PredictiveSplineConfig) Validate() error {
	if c.MinSegmentLength < 3 {
		return errors.New("min segment length must be at least 3")
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
	if c.MinPriceChange < 0 {
		c.MinPriceChange = 0.01 // 1% по умолчанию
	}
	if c.MinTrendStrength < 0 {
		c.MinTrendStrength = 0.5 // по умолчанию
	}
	return nil
}

func (c *PredictiveSplineConfig) String() string {
	return fmt.Sprintf("PredictiveSpline(min_len=%d, max_len=%d, horizon=%d, r2=%.2f, advance=%d, price_chg=%.2f%%, trend_str=%.2f)",
		c.MinSegmentLength, c.MaxSegmentLength, c.PredictionHorizon, c.MinR2Threshold, c.SignalAdvance, 
		c.MinPriceChange*100, c.MinTrendStrength)
}

// SplineSegment представляет квадратичный сегмент сплайна
type SplineSegment struct {
	StartIdx    int
	EndIdx      int
	A           float64 // коэффициент x²
	B           float64 // коэффициент x
	C           float64 // константа
	R2          float64 // коэффициент детерминации
	IsAscending bool    // направление тренда
	InflectionX float64 // точка перегиба (где производная = 0)
}

// PredictedReversal представляет предсказанную точку разворота
type PredictedReversal struct {
	PredictedIndex int     // предсказанный индекс разворота
	PredictedPrice float64 // предсказанная цена разворота
	Confidence     float64 // уверенность в предсказании (0-1)
	SignalType     internal.SignalType
}

type PredictiveSplineSignalGenerator struct{}

func NewPredictiveSplineSignalGenerator() *PredictiveSplineSignalGenerator {
	return &PredictiveSplineSignalGenerator{}
}

// SplineAnalyzer анализирует ценовые данные с помощью квадратичных сплайнов
type SplineAnalyzer struct {
	minSegmentLength  int
	maxSegmentLength  int
	predictionHorizon int
	minR2Threshold    float64
	signalAdvance     int
	minPriceChange    float64
	minTrendStrength  float64
	// Кэш для оптимизации
	lastAnalyzedIdx int
	lastSegment     *SplineSegment
}

func NewSplineAnalyzer(config *PredictiveSplineConfig) *SplineAnalyzer {
	return &SplineAnalyzer{
		minSegmentLength:  config.MinSegmentLength,
		maxSegmentLength:  config.MaxSegmentLength,
		predictionHorizon: config.PredictionHorizon,
		minR2Threshold:    config.MinR2Threshold,
		signalAdvance:     config.SignalAdvance,
		minPriceChange:    config.MinPriceChange,
		minTrendStrength:  config.MinTrendStrength,
	}
}

// fitQuadraticRegression выполняет квадратичную регрессию для данных
func (sa *SplineAnalyzer) fitQuadraticRegression(y []float64) (a, b, c, r2 float64) {
	n := float64(len(y))
	if n < 3 {
		return 0, 0, y[0], 0
	}

	sumX, sumX2, sumX3, sumX4 := 0.0, 0.0, 0.0, 0.0
	sumY, sumXY, sumX2Y := 0.0, 0.0, 0.0

	for i, yi := range y {
		xi := float64(i)
		x2 := xi * xi
		x3 := x2 * xi
		x4 := x3 * xi

		sumX += xi
		sumX2 += x2
		sumX3 += x3
		sumX4 += x4
		sumY += yi
		sumXY += xi * yi
		sumX2Y += x2 * yi
	}

	// Решаем систему уравнений для квадратичной регрессии
	detA := sumX4*(sumX2*n-sumX*sumX) - sumX3*(sumX3*n-sumX*sumX2) + sumX2*(sumX3*sumX-sumX2*sumX2)

	if math.Abs(detA) < 1e-10 {
		// Вырожденный случай, используем линейную регрессию
		slope, intercept := sa.linearRegression(y)
		r2 = sa.calculateLinearR2(y, slope, intercept)
		return 0, slope, intercept, r2
	}

	detAa := sumX2Y*(sumX2*n-sumX*sumX) - sumXY*(sumX3*n-sumX*sumX2) + sumY*(sumX3*sumX-sumX2*sumX2)
	a = detAa / detA

	detAb := sumX4*(sumXY*n-sumX*sumY) - sumX2Y*(sumX3*n-sumX*sumX2) + sumX2*(sumX3*sumY-sumX2*sumXY)
	b = detAb / detA

	detAc := sumX4*(sumX2*sumY-sumX*sumXY) - sumX3*(sumX3*sumY-sumX*sumX2Y) + sumX2*(sumX3*sumXY-sumX2*sumX2Y)
	c = detAc / detA

	// Вычисляем R²
	r2 = sa.calculateQuadraticR2(y, a, b, c)

	return a, b, c, r2
}

func (sa *SplineAnalyzer) linearRegression(y []float64) (slope, intercept float64) {
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

	slope = (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)
	intercept = (sumY - slope*sumX) / n

	return slope, intercept
}

func (sa *SplineAnalyzer) calculateQuadraticR2(y []float64, a, b, c float64) float64 {
	if len(y) < 3 {
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
		predicted := a*float64(i*i) + b*float64(i) + c
		ssRes += (yi - predicted) * (yi - predicted)
		ssTot += (yi - mean) * (yi - mean)
	}

	if ssTot == 0 {
		return 0
	}

	return 1 - ssRes/ssTot
}

func (sa *SplineAnalyzer) calculateLinearR2(y []float64, slope, intercept float64) float64 {
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
func (sa *SplineAnalyzer) analyzeCurrentTrend(prices []float64, currentIdx int) *SplineSegment {
	// Кэш: если анализировали недавно, используем кэшированный результат
	if sa.lastSegment != nil && currentIdx-sa.lastAnalyzedIdx < sa.minSegmentLength/2 {
		return sa.lastSegment
	}

	// Определяем окно для анализа
	startIdx := currentIdx - sa.maxSegmentLength
	if startIdx < 0 {
		startIdx = 0
	}

	if currentIdx-startIdx < sa.minSegmentLength {
		return nil
	}

	// Извлекаем данные для анализа
	window := prices[startIdx : currentIdx+1]

	// Пробуем разные длины сегмента (оптимизация: меньше итераций)
	var bestSegment *SplineSegment
	bestR2 := -1.0

	// Оптимизация: пробуем не все длины, а с шагом
	step := sa.minSegmentLength / 2
	if step < 1 {
		step = 1
	}

	for length := sa.minSegmentLength; length <= len(window) && length <= sa.maxSegmentLength; length += step {
		segmentStart := len(window) - length
		segment := window[segmentStart:]

		a, b, c, r2 := sa.fitQuadraticRegression(segment)

		if r2 > bestR2 {
			bestR2 = r2

			// Определяем направление тренда по производной в конце сегмента
			lastX := float64(len(segment) - 1)
			slope := 2*a*lastX + b
			isAscending := slope > 0

			// Вычисляем точку перегиба
			var inflectionX float64
			if math.Abs(a) > 1e-10 {
				inflectionX = -b / (2 * a)
			} else {
				inflectionX = math.Inf(1)
			}

			bestSegment = &SplineSegment{
				StartIdx:    startIdx + segmentStart,
				EndIdx:      currentIdx,
				A:           a,
				B:           b,
				C:           c,
				R2:          r2,
				IsAscending: isAscending,
				InflectionX: inflectionX,
			}
		}
	}

	// Кэшируем результат
	sa.lastAnalyzedIdx = currentIdx
	sa.lastSegment = bestSegment

	return bestSegment
}

// predictReversal предсказывает точку разворота на основе текущего тренда
func (sa *SplineAnalyzer) predictReversal(segment *SplineSegment, currentIdx int, prices []float64) *PredictedReversal {
	if segment == nil || segment.R2 < sa.minR2Threshold {
		return nil
	}

	// Длина текущего сегмента
	segmentLength := segment.EndIdx - segment.StartIdx + 1

	// Проверяем силу тренда (изменение цены в сегменте)
	startPrice := prices[segment.StartIdx]
	currentPrice := prices[currentIdx]
	priceChangePercent := math.Abs(currentPrice-startPrice) / startPrice
	
	// Фильтр 1: Минимальное изменение цены
	if priceChangePercent < sa.minPriceChange {
		return nil // Тренд слишком слабый
	}

	// Фильтр 2: Проверяем силу тренда через наклон
	localX := float64(segmentLength - 1)
	slope := 2*segment.A*localX + segment.B
	trendStrength := math.Abs(slope) / currentPrice * 100 // Нормализованная сила тренда в %
	
	if trendStrength < sa.minTrendStrength {
		return nil // Тренд недостаточно сильный
	}

	// Предсказываем точку разворота на основе нескольких факторов:
	
	// 1. Если есть точка перегиба в будущем, используем её
	inflectionDistance := segment.InflectionX - localX

	var predictedDistance int
	
	if !math.IsInf(segment.InflectionX, 0) && inflectionDistance > 0 && inflectionDistance < float64(sa.predictionHorizon*3) {
		// Точка перегиба близко - используем её
		predictedDistance = int(math.Ceil(inflectionDistance))
	} else {
		// Используем среднюю длину сегмента или горизонт предсказания
		avgSegmentLength := (sa.minSegmentLength + sa.maxSegmentLength) / 2
		if segmentLength >= avgSegmentLength {
			// Текущий сегмент уже достаточно длинный - ожидаем разворот скоро
			predictedDistance = sa.predictionHorizon
		} else {
			// Сегмент еще короткий - разворот дальше
			predictedDistance = avgSegmentLength - segmentLength
			if predictedDistance > sa.predictionHorizon*3 {
				predictedDistance = sa.predictionHorizon * 3
			}
		}
	}

	// Ограничиваем предсказание горизонтом
	if predictedDistance > sa.predictionHorizon*3 {
		predictedDistance = sa.predictionHorizon * 3
	}
	if predictedDistance < sa.predictionHorizon {
		predictedDistance = sa.predictionHorizon
	}

	predictedIdx := currentIdx + predictedDistance

	// Экстраполируем цену в точке разворота
	futureX := localX + float64(predictedDistance)
	predictedPrice := segment.A*futureX*futureX + segment.B*futureX + segment.C

	// Определяем тип сигнала (противоположный текущему тренду)
	var signalType internal.SignalType
	if segment.IsAscending {
		// Восходящий тренд -> ожидаем разворот вниз -> выставляем SELL
		signalType = internal.SELL
	} else {
		// Нисходящий тренд -> ожидаем разворот вверх -> выставляем BUY
		signalType = internal.BUY
	}

	// Уверенность зависит от R², расстояния до предсказания и силы тренда
	distanceFactor := 1.0 - float64(predictedDistance)/float64(sa.predictionHorizon*4)
	if distanceFactor < 0 {
		distanceFactor = 0
	}
	
	confidence := segment.R2 * distanceFactor * (priceChangePercent / 0.1) // Нормализуем к 10%
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0 {
		confidence = 0
	}

	return &PredictedReversal{
		PredictedIndex: predictedIdx,
		PredictedPrice: predictedPrice,
		Confidence:     confidence,
		SignalType:     signalType,
	}
}

func (sg *PredictiveSplineSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	psConfig, ok := config.(*PredictiveSplineConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := psConfig.Validate(); err != nil {
		log.Printf("⚠️ Ошибка валидации конфигурации: %v", err)
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < psConfig.MinSegmentLength*2 {
		log.Printf("⚠️ Недостаточно данных: получено %d свечей, требуется минимум %d", len(candles), psConfig.MinSegmentLength*2)
		return make([]internal.SignalType, len(candles))
	}

	// Извлекаем цены закрытия
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	analyzer := NewSplineAnalyzer(psConfig)
	signals := make([]internal.SignalType, len(candles))

	// Храним активные предсказания
	var activePrediction *PredictedReversal
	lastSignalIdx := -1
	lastSignalType := internal.HOLD // Отслеживаем тип последнего сигнала
	
	// Адаптивное минимальное расстояние между сигналами
	minSignalDistance := psConfig.MinSegmentLength
	if len(candles) > 10000 {
		minSignalDistance = int(float64(psConfig.MinSegmentLength) * 1.5)
	}
	
	// Адаптивный порог уверенности в зависимости от длины истории
	confidenceThreshold := 0.25 // Более низкий базовый порог
	if len(candles) > 10000 {
		confidenceThreshold = 0.30 // Для очень больших данных немного повышаем
	} else if len(candles) > 5000 {
		confidenceThreshold = 0.28
	}

	// Начинаем анализ после накопления достаточных данных
	startIdx := psConfig.MaxSegmentLength
	if startIdx < psConfig.MinSegmentLength*2 {
		startIdx = psConfig.MinSegmentLength * 2
	}

	for i := startIdx; i < len(candles); i++ {
		// Проверяем, не пора ли выставить сигнал по активному предсказанию
		if activePrediction != nil {
			signalIdx := activePrediction.PredictedIndex - psConfig.SignalAdvance
			
			if i == signalIdx && activePrediction.Confidence >= confidenceThreshold {
				// Проверяем минимальное расстояние от последнего сигнала
				if lastSignalIdx < 0 || i-lastSignalIdx >= minSignalDistance {
					// КРИТИЧНО: Проверяем чередование типов сигналов
					if lastSignalType == internal.HOLD || activePrediction.SignalType != lastSignalType {
						signals[i] = activePrediction.SignalType
						lastSignalIdx = i
						lastSignalType = activePrediction.SignalType
						activePrediction = nil // Сбрасываем предсказание после выставления сигнала
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
		// Адаптивный интервал анализа
		analysisInterval := psConfig.MinSegmentLength / 2
		if analysisInterval < 3 {
			analysisInterval = 3
		}
		if len(candles) > 10000 {
			analysisInterval = psConfig.MinSegmentLength
		}
		
		if activePrediction == nil && (lastSignalIdx < 0 || i-lastSignalIdx >= analysisInterval) {
			segment := analyzer.analyzeCurrentTrend(prices, i)
			if segment != nil && segment.R2 >= psConfig.MinR2Threshold {
				prediction := analyzer.predictReversal(segment, i, prices)
				if prediction != nil && prediction.Confidence >= confidenceThreshold {
					activePrediction = prediction
				}
			}
		}
	}

	return signals
}

type PredictiveSplineConfigGenerator struct{}

func NewPredictiveSplineConfigGenerator() *PredictiveSplineConfigGenerator {
	return &PredictiveSplineConfigGenerator{}
}

func (g *PredictiveSplineConfigGenerator) Generate() []internal.StrategyConfigV2 {
	configs := []internal.StrategyConfigV2{}
	
	// Оптимизированный набор с акцентом на разнообразие количества сделок
	minLengths := []int{8, 12, 16}
	maxLengths := []int{50, 70, 90}
	horizons := []int{5, 7, 10}
	r2Thresholds := []float64{0.65, 0.70, 0.75}
	advances := []int{3, 5}
	
	// Специально подобранные комбинации фильтров для разного количества сделок
	filterCombos := []struct {
		priceChange  float64
		trendStrength float64
	}{
		{0.003, 0.12}, // Очень мягкие - много сделок
		{0.005, 0.15}, // Мягкие - средне-много сделок
		{0.007, 0.20}, // Умеренные - среднее количество
		{0.010, 0.25}, // Средние - средне-мало сделок
		{0.012, 0.30}, // Строгие - мало сделок
		{0.015, 0.35}, // Очень строгие - очень мало сделок
	}
	
	// Генерируем комбинации
	for _, minLen := range minLengths {
		for _, maxLen := range maxLengths {
			if maxLen <= minLen*2 {
				continue
			}
			for _, horizon := range horizons {
				for _, r2 := range r2Thresholds {
					for _, advance := range advances {
						for _, combo := range filterCombos {
							configs = append(configs, &PredictiveSplineConfig{
								MinSegmentLength:  minLen,
								MaxSegmentLength:  maxLen,
								PredictionHorizon: horizon,
								MinR2Threshold:    r2,
								SignalAdvance:     advance,
								MinPriceChange:    combo.priceChange,
								MinTrendStrength:  combo.trendStrength,
							})
						}
					}
				}
			}
		}
	}

	return configs
}

type PredictiveSplineStrategy struct {
	internal.BaseConfig
	internal.BaseStrategy
}

func (s *PredictiveSplineStrategy) Name() string {
	return "predictive_spline"
}

func NewPredictiveSplineStrategyV2(slippage float64) internal.TradingStrategy {
	// 1. Создаем провайдер проскальзывания
	slippageProvider := internal.NewSlippageProvider(slippage)

	// 2. Создаем генератор сигналов
	signalGenerator := NewPredictiveSplineSignalGenerator()

	// 3. Создаем менеджер конфигурации
	configManager := internal.NewConfigManager(
		&PredictiveSplineConfig{
			MinSegmentLength:  10,
			MaxSegmentLength:  80,
			PredictionHorizon: 7,
			MinR2Threshold:    0.70,
			SignalAdvance:     3,
			MinPriceChange:    0.008,  // 0.8%
			MinTrendStrength:  0.25,
		},
		func() internal.StrategyConfigV2 {
			return &PredictiveSplineConfig{}
		},
	)

	// 4. Создаем генератор конфигураций для оптимизации
	configGenerator := NewPredictiveSplineConfigGenerator()

	// 5. Создаем оптимизатор
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	// 6. Собираем всё вместе через композицию
	return internal.NewStrategyBase(
		"predictive_spline_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewPredictiveSplineStrategyV2(0.01) // default slippage 0.01
	internal.RegisterStrategyV2(strategy)
}
