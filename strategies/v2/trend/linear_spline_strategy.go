// Linear Spline Strategy V2
//
// Описание стратегии:
// Стратегия использует линейные сплайны для моделирования трендов и определения
// точек разворота. В отличие от predictive_spline_strategy, эта версия НЕ прогнозирует
// будущее, а определяет развороты постфактум на основе исторических данных.
//
// Как работает:
// - Анализирует исторические данные и строит чередующиеся линейные сплайны
// - Каждый сплайн представляет собой линейный тренд (восходящий или нисходящий)
// - Определяет точки разворота тренда по изменению направления
// - Выставляет сигналы BUY/SELL в точках смены тренда
// - Использует R² для оценки качества аппроксимации тренда
//
// Параметры:
// - MinSegmentLength: минимальная длина сегмента тренда (по умолчанию 5)
// - MaxSegmentLength: максимальная длина сегмента тренда (по умолчанию 50)
// - MinR2Threshold: минимальный R² для доверия к тренду (по умолчанию 0.6)
// - MinSlopeThreshold: минимальный наклон для определения тренда (по умолчанию 0.0001)
//
// Сильные стороны:
// - Простая и понятная логика
// - Не использует прогнозирование, работает только с фактами
// - Хорошо работает на трендовых рынках
// - Низкая вычислительная сложность
//
// Слабые стороны:
// - Определяет развороты с запаздыванием
// - Может давать ложные сигналы на боковых рынках
// - Чувствительна к шуму в данных
//
// Лучшие условия для применения:
// - Трендовые рынки с четкими разворотами
// - Средне- и долгосрочная торговля
// - Активы с низким уровнем шума

package trend

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
	"math"
)

type LinearSplineConfig struct {
	MinSegmentLength  int     `json:"min_segment_length"`
	MaxSegmentLength  int     `json:"max_segment_length"`
	MinR2Threshold    float64 `json:"min_r2_threshold"`
	MinSlopeThreshold float64 `json:"min_slope_threshold"`
}

func (c *LinearSplineConfig) Validate() error {
	if c.MinSegmentLength < 2 {
		return errors.New("min segment length must be at least 2")
	}
	if c.MaxSegmentLength <= c.MinSegmentLength {
		return errors.New("max segment length must be greater than min")
	}
	if c.MinR2Threshold < 0 || c.MinR2Threshold > 1 {
		return errors.New("min R2 threshold must be between 0 and 1")
	}
	if c.MinSlopeThreshold < 0 {
		return errors.New("min slope threshold must be non-negative")
	}
	return nil
}

func (c *LinearSplineConfig) String() string {
	return fmt.Sprintf("LinearSpline(min_len=%d, max_len=%d, r2=%.2f, slope=%.4f)",
		c.MinSegmentLength, c.MaxSegmentLength, c.MinR2Threshold, c.MinSlopeThreshold)
}

// LinearSegment представляет линейный сегмент сплайна
type LinearSegment struct {
	StartIdx    int
	EndIdx      int
	Slope       float64
	Intercept   float64
	R2          float64
	IsAscending bool
}

type LinearSplineSignalGenerator struct{}

func NewLinearSplineSignalGenerator() *LinearSplineSignalGenerator {
	return &LinearSplineSignalGenerator{}
}

// LinearSplineAnalyzer анализирует ценовые данные с помощью линейных сплайнов
type LinearSplineAnalyzer struct {
	minSegmentLength  int
	maxSegmentLength  int
	minR2Threshold    float64
	minSlopeThreshold float64
}

func NewLinearSplineAnalyzer(config *LinearSplineConfig) *LinearSplineAnalyzer {
	return &LinearSplineAnalyzer{
		minSegmentLength:  config.MinSegmentLength,
		maxSegmentLength:  config.MaxSegmentLength,
		minR2Threshold:    config.MinR2Threshold,
		minSlopeThreshold: config.MinSlopeThreshold,
	}
}

// linearRegression выполняет линейную регрессию для данных
func (la *LinearSplineAnalyzer) linearRegression(y []float64) (slope, intercept float64) {
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
func (la *LinearSplineAnalyzer) calculateR2(y []float64, slope, intercept float64) float64 {
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

// fitSegment подбирает оптимальный линейный сегмент заданного направления
func (la *LinearSplineAnalyzer) fitSegment(prices []float64, startIdx int, isAscending bool) *LinearSegment {
	if startIdx >= len(prices)-la.minSegmentLength {
		return nil
	}

	var bestSegment *LinearSegment
	bestR2 := -1.0

	// Пробуем разные длины сегмента
	maxEnd := startIdx + la.maxSegmentLength
	if maxEnd > len(prices) {
		maxEnd = len(prices)
	}

	for endIdx := startIdx + la.minSegmentLength; endIdx <= maxEnd; endIdx++ {
		segment := prices[startIdx:endIdx]
		slope, intercept := la.linearRegression(segment)
		r2 := la.calculateR2(segment, slope, intercept)

		// Проверяем направление тренда
		slopeMatches := (isAscending && slope > la.minSlopeThreshold) ||
			(!isAscending && slope < -la.minSlopeThreshold)

		if slopeMatches && r2 > bestR2 && r2 >= la.minR2Threshold {
			bestR2 = r2
			bestSegment = &LinearSegment{
				StartIdx:    startIdx,
				EndIdx:      endIdx,
				Slope:       slope,
				Intercept:   intercept,
				R2:          r2,
				IsAscending: isAscending,
			}
		}
	}

	return bestSegment
}

// fitAlternatingSplines строит чередующиеся линейные сплайны
func (la *LinearSplineAnalyzer) fitAlternatingSplines(prices []float64) []*LinearSegment {
	var segments []*LinearSegment

	if len(prices) < la.minSegmentLength*2 {
		return segments
	}

	// Определяем начальное направление по первым свечам
	firstSegmentPrices := prices[:la.minSegmentLength]
	slope, _ := la.linearRegression(firstSegmentPrices)
	isAscending := slope > 0

	currentIdx := 0

	for currentIdx < len(prices)-la.minSegmentLength {
		segment := la.fitSegment(prices, currentIdx, isAscending)
		
		if segment == nil {
			// Не удалось найти сегмент, пробуем противоположное направление
			isAscending = !isAscending
			segment = la.fitSegment(prices, currentIdx, isAscending)
			
			if segment == nil {
				// Не удалось найти сегмент в обоих направлениях, сдвигаемся
				currentIdx++
				continue
			}
		}

		segments = append(segments, segment)
		currentIdx = segment.EndIdx
		isAscending = !isAscending // Чередуем направление
	}

	return segments
}

func (sg *LinearSplineSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	lsConfig, ok := config.(*LinearSplineConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := lsConfig.Validate(); err != nil {
		log.Printf("⚠️ Ошибка валидации конфигурации: %v", err)
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < lsConfig.MinSegmentLength*2 {
		log.Printf("⚠️ Недостаточно данных: получено %d свечей, требуется минимум %d", 
			len(candles), lsConfig.MinSegmentLength*2)
		return make([]internal.SignalType, len(candles))
	}

	// Извлекаем цены закрытия
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	analyzer := NewLinearSplineAnalyzer(lsConfig)
	segments := analyzer.fitAlternatingSplines(prices)

	// Генерируем сигналы в точках смены тренда
	signals := make([]internal.SignalType, len(candles))

	// Если первый сегмент восходящий, выставляем BUY в начале
	if len(segments) > 0 && segments[0].IsAscending {
		signals[segments[0].StartIdx] = internal.BUY
	}

	// Выставляем сигналы в точках смены тренда
	for i := 1; i < len(segments); i++ {
		changePoint := segments[i].StartIdx
		
		if changePoint >= len(signals) {
			continue
		}

		// Переход от восходящего к нисходящему -> SELL
		if segments[i-1].IsAscending && !segments[i].IsAscending {
			signals[changePoint] = internal.SELL
		} else if !segments[i-1].IsAscending && segments[i].IsAscending {
			// Переход от нисходящего к восходящему -> BUY
			signals[changePoint] = internal.BUY
		}
	}

	return signals
}

type LinearSplineConfigGenerator struct{}

func NewLinearSplineConfigGenerator() *LinearSplineConfigGenerator {
	return &LinearSplineConfigGenerator{}
}

func (g *LinearSplineConfigGenerator) Generate() []internal.StrategyConfigV2 {
	configs := []internal.StrategyConfigV2{}

	minLengths := []int{5, 8, 12, 15, 20}
	maxLengths := []int{30, 50, 70, 100, 150}
	r2Thresholds := []float64{0.5, 0.6, 0.7, 0.8}
	slopeThresholds := []float64{0.0001, 0.0005, 0.001, 0.002}

	for _, minLen := range minLengths {
		for _, maxLen := range maxLengths {
			if maxLen <= minLen*2 {
				continue
			}
			for _, r2 := range r2Thresholds {
				for _, slope := range slopeThresholds {
					configs = append(configs, &LinearSplineConfig{
						MinSegmentLength:  minLen,
						MaxSegmentLength:  maxLen,
						MinR2Threshold:    r2,
						MinSlopeThreshold: slope,
					})
				}
			}
		}
	}

	return configs
}

type LinearSplineStrategy struct {
	internal.BaseConfig
	internal.BaseStrategy
}

func (s *LinearSplineStrategy) Name() string {
	return "linear_spline"
}

func NewLinearSplineStrategyV2(slippage float64) internal.TradingStrategy {
	// 1. Создаем провайдер проскальзывания
	slippageProvider := internal.NewSlippageProvider(slippage)

	// 2. Создаем генератор сигналов
	signalGenerator := NewLinearSplineSignalGenerator()

	// 3. Создаем менеджер конфигурации
	configManager := internal.NewConfigManager(
		&LinearSplineConfig{
			MinSegmentLength:  10,
			MaxSegmentLength:  50,
			MinR2Threshold:    0.6,
			MinSlopeThreshold: 0.0005,
		},
		func() internal.StrategyConfigV2 {
			return &LinearSplineConfig{}
		},
	)

	// 4. Создаем генератор конфигураций для оптимизации
	configGenerator := NewLinearSplineConfigGenerator()

	// 5. Создаем оптимизатор
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	// 6. Собираем всё вместе через композицию
	return internal.NewStrategyBase(
		"linear_spline_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewLinearSplineStrategyV2(0.01) // default slippage 0.01
	internal.RegisterStrategyV2(strategy)
}
