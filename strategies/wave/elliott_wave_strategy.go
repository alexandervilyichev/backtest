// Elliott Wave Strategy
//
// Описание стратегии:
// Стратегия основана на теории волн Эллиотта, которая предполагает, что рыночные цены
// движутся в предсказуемых паттернах, называемых волнами. Полный цикл состоит из
// 5 импульсных волн (в направлении основного тренда) и 3 коррекционных волн.
//
// Как работает:
// - Идентифицирует локальные максимумы и минимумы для определения волновой структуры
// - Определяет фазу волнового цикла (импульсные волны 1, 3, 5 или коррекционные 2, 4)
// - Покупка: в начале импульсных волн (1, 3, 5) при восходящем тренде
// - Продажа: в конце импульсных волн или во время коррекционных волн
// - Использует отношения Фибоначчи для подтверждения волновой структуры
//
// Параметры:
// - MinWaveLength: минимальная длина волны в свечах (по умолчанию 5)
// - MaxWaveLength: максимальная длина волны в свечах (по умолчанию 50)
// - FibonacciThreshold: порог отношения Фибоначчи для подтверждения (по умолчанию 0.618)
// - TrendStrength: минимальная сила тренда для генерации сигналов (по умолчанию 0.3)
//
// Сильные стороны:
// - Основана на фундаментальной теории рыночной психологии
// - Учитывает естественные циклы рынка
// - Хорошо работает на всех таймфреймах
// - Может предсказывать развороты заранее
//
// Слабые стороны:
// - Субъективность в определении волн
// - Требует опыта для правильной интерпретации
// - Может давать ложные сигналы в боковых рынках
// - Сложность автоматизации всех правил Эллиотта
//
// Лучшие условия для применения:
// - Трендовые рынки с четкими импульсами
// - Средне- и долгосрочная торговля
// - В сочетании с другими индикаторами подтверждения
// - На активах с хорошей волатильностью и ликвидностью

package strategies

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
	"math"
)

type ElliottWaveConfig struct {
	MinWaveLength      int     `json:"min_wave_length"`
	MaxWaveLength      int     `json:"max_wave_length"`
	FibonacciThreshold float64 `json:"fibonacci_threshold"`
	TrendStrength      float64 `json:"trend_strength"`
}

func (c *ElliottWaveConfig) Validate() error {
	if c.MinWaveLength <= 0 {
		return errors.New("min wave length must be positive")
	}
	if c.MaxWaveLength <= c.MinWaveLength {
		return errors.New("max wave length must be greater than min")
	}
	if c.FibonacciThreshold <= 0 || c.FibonacciThreshold >= 2.0 {
		return errors.New("fibonacci threshold must be between 0 and 2.0")
	}
	if c.TrendStrength < 0 {
		return errors.New("trend strength must be non-negative")
	}
	return nil
}

func (c *ElliottWaveConfig) DefaultConfigString() string {
	return fmt.Sprintf("ElliottWave(min_len=%d, max_len=%d, fib_thresh=%.3f, trend_str=%.1f)",
		c.MinWaveLength, c.MaxWaveLength, c.FibonacciThreshold, c.TrendStrength)
}

// WavePoint представляет точку волны Эллиотта
type WavePoint struct {
	Index    int     // индекс в массиве свечей
	Price    float64 // цена точки
	WaveType int     // тип волны (1, 2, 3, 4, 5, A, B, C)
	IsPeak   bool    // true для максимума, false для минимума
	Strength float64 // сила волны (амплитуда движения)
}

// ElliottWaveAnalyzer анализирует волновую структуру Эллиотта
type ElliottWaveAnalyzer struct {
	wavePoints     []WavePoint
	minWaveLength  int
	maxWaveLength  int
	fibThreshold   float64
	trendStrength  float64
	trendDirection float64 // new
}

// NewElliottWaveAnalyzer создает новый анализатор волн Эллиотта
func NewElliottWaveAnalyzer(minLen, maxLen int, fibThresh, trendStr float64) *ElliottWaveAnalyzer {
	return &ElliottWaveAnalyzer{
		wavePoints:     make([]WavePoint, 0),
		minWaveLength:  minLen,
		maxWaveLength:  maxLen,
		fibThreshold:   fibThresh,
		trendStrength:  trendStr,
		trendDirection: 0, // init
	}
}

// findSignificantExtrema находит значимые экстремумы для волнового анализа
func (ewa *ElliottWaveAnalyzer) findSignificantExtrema(prices []float64) {
	ewa.wavePoints = make([]WavePoint, 0)

	for i := ewa.minWaveLength; i < len(prices)-ewa.minWaveLength; i++ {
		// Проверяем локальный максимум
		isLocalMax := true
		maxValue := prices[i]
		for j := i - ewa.minWaveLength; j <= i+ewa.minWaveLength; j++ {
			if j != i && prices[j] >= maxValue {
				isLocalMax = false
				break
			}
		}

		// Проверяем локальный минимум
		isLocalMin := true
		minValue := prices[i]
		for j := i - ewa.minWaveLength; j <= i+ewa.minWaveLength; j++ {
			if j != i && prices[j] <= minValue {
				isLocalMin = false
				break
			}
		}

		if isLocalMax || isLocalMin {
			// Вычисляем силу экстремума
			strength := 0.0
			count := 0
			for j := i - ewa.minWaveLength; j <= i+ewa.minWaveLength; j++ {
				if j != i {
					strength += math.Abs(prices[i] - prices[j])
					count++
				}
			}
			if count > 0 {
				strength /= float64(count)
			}

			point := WavePoint{
				Index:    i,
				Price:    prices[i],
				IsPeak:   isLocalMax,
				Strength: strength,
			}
			ewa.wavePoints = append(ewa.wavePoints, point)
		}
	}

	// Фильтруем по максимальной длине волны
	ewa.filterByWaveLength()
}

// filterByWaveLength фильтрует экстремумы по длине волны
func (ewa *ElliottWaveAnalyzer) filterByWaveLength() {
	if len(ewa.wavePoints) <= 2 {
		return
	}

	filtered := make([]WavePoint, 0)
	filtered = append(filtered, ewa.wavePoints[0])

	for i := 1; i < len(ewa.wavePoints); i++ {
		last := filtered[len(filtered)-1]
		current := ewa.wavePoints[i]

		distance := current.Index - last.Index

		// Пропускаем если расстояние слишком мало
		if distance < ewa.minWaveLength {
			// Оставляем точку с большей силой
			if current.Strength > last.Strength {
				filtered[len(filtered)-1] = current
			}
			continue
		}

		// Пропускаем если расстояние слишком велико (разрыв в данных)
		if distance > ewa.maxWaveLength {
			continue
		}

		filtered = append(filtered, current)
	}

	ewa.wavePoints = filtered
}

// identifyWavePattern идентифицирует паттерн волн Эллиотта
func (ewa *ElliottWaveAnalyzer) identifyWavePattern() []WavePoint {
	if len(ewa.wavePoints) < 3 {
		return ewa.wavePoints
	}

	// Определяем направление тренда
	trendDirection := 0.0
	if len(ewa.wavePoints) >= 2 {
		trendDirection = ewa.wavePoints[len(ewa.wavePoints)-1].Price - ewa.wavePoints[0].Price
	}

	waveNumber := 1
	inImpulse := true // начинаем с импульсной волны

	for i := 0; i < len(ewa.wavePoints); i++ {
		point := &ewa.wavePoints[i]

		if inImpulse {
			// Импульсные волны (1, 3, 5)
			if trendDirection > 0 {
				point.WaveType = waveNumber
			} else {
				point.WaveType = -waveNumber // отрицательные для нисходящего тренда
			}

			waveNumber++
			if waveNumber > 5 {
				inImpulse = false
				waveNumber = 1
			}
		} else {
			// Коррекционные волны (2, 4) или (A, B, C)
			if trendDirection > 0 {
				point.WaveType = 10 + waveNumber // A=11, B=12, C=13
			} else {
				point.WaveType = -(10 + waveNumber)
			}

			waveNumber++
			if waveNumber > 3 {
				inImpulse = true
				waveNumber = 1
			}
		}
	}

	// Сохраняем направление тренда для использования в сигналах
	ewa.trendDirection = trendDirection

	return ewa.wavePoints
}

// checkFibonacciRatio проверяет отношения Фибоначчи между волнами
func (ewa *ElliottWaveAnalyzer) checkFibonacciRatio() bool {
	if len(ewa.wavePoints) < 5 {
		return false
	}

	// Проверяем отношение волны 2 к волне 1 (должно быть около 0.618)
	if len(ewa.wavePoints) >= 2 {
		wave1 := math.Abs(ewa.wavePoints[1].Price - ewa.wavePoints[0].Price)
		wave2 := math.Abs(ewa.wavePoints[2].Price - ewa.wavePoints[1].Price)

		if wave1 > 0 {
			ratio := wave2 / wave1
			if math.Abs(ratio-0.618) < ewa.fibThreshold {
				return true
			}
		}
	}

	// Проверяем отношение волны 4 к волне 3
	if len(ewa.wavePoints) >= 4 {
		wave3 := math.Abs(ewa.wavePoints[3].Price - ewa.wavePoints[2].Price)
		wave4 := math.Abs(ewa.wavePoints[4].Price - ewa.wavePoints[3].Price)

		if wave3 > 0 {
			ratio := wave4 / wave3
			if math.Abs(ratio-0.382) < ewa.fibThreshold || math.Abs(ratio-0.618) < ewa.fibThreshold {
				return true
			}
		}
	}

	return false
}

// predictSignal генерирует торговый сигнал на основе волнового анализа
func (ewa *ElliottWaveAnalyzer) predictSignal(currentIndex int, prices []float64) internal.SignalType {
	if len(ewa.wavePoints) < 1 {
		return internal.HOLD
	}

	// Находим ближайшую волновую точку
	var lastWavePoint *WavePoint
	for i := len(ewa.wavePoints) - 1; i >= 0; i-- {
		if ewa.wavePoints[i].Index <= currentIndex {
			lastWavePoint = &ewa.wavePoints[i]
			break
		}
	}

	if lastWavePoint == nil {
		return internal.HOLD
	}

	currentPrice := prices[currentIndex]
	priceChange := currentPrice - lastWavePoint.Price

	// Улучшенная логика: генерируем сигналы на основе волн и тренда

	// Основной сигнал: breakout after extrema
	// После локального минимума - BUY если цена выше минимума
	if !lastWavePoint.IsPeak && currentPrice > lastWavePoint.Price {
		return internal.BUY
	}

	// После локального максимума - SELL если цена ниже максимума
	if lastWavePoint.IsPeak && currentPrice < lastWavePoint.Price {
		return internal.SELL
	}

	// Торговля на откатах в зависимости от тренда
	// BUY при мелком откате от максимума в восходящем тренде
	if lastWavePoint.IsPeak && math.Abs(priceChange)/lastWavePoint.Price < 0.02 && ewa.trendDirection > 0 {
		return internal.BUY
	}

	// SELL при мелком откате от минимума в нисходящем тренде
	if !lastWavePoint.IsPeak && math.Abs(priceChange)/lastWavePoint.Price < 0.02 && ewa.trendDirection < 0 {
		return internal.SELL
	}

	// Дополнительные сигналы на основе типов волн
	// В импульсных волнах генерируем сигналы в направлении тренда
	if lastWavePoint.WaveType > 0 && lastWavePoint.WaveType <= 5 {
		if ewa.trendDirection > 0 && !lastWavePoint.IsPeak {
			return internal.BUY
		}
		if ewa.trendDirection < 0 && lastWavePoint.IsPeak {
			return internal.SELL
		}
	}

	// Проверяем Фибоначчи для усиления сигналов
	if ewa.checkFibonacciRatio() {
		// В условиях Фибоначчи генерируем сигналы модуляции тренда
		if ewa.trendDirection > 0 {
			return internal.BUY
		}
		if ewa.trendDirection < 0 {
			return internal.SELL
		}
	}

	return internal.HOLD
}

// abs возвращает абсолютное значение
// func abs(x int) int {
// 	if x < 0 {
// 		return -x
// 	}
// 	return x
// }

type ElliottWaveStrategy struct{}

func (s *ElliottWaveStrategy) Name() string {
	return "elliott_wave"
}

// func (s *ElliottWaveStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
// 	if len(candles) < 20 {
// 		log.Printf("⚠️ Недостаточно данных для волнового анализа Эллиотта: получено %d свечей, требуется минимум 20", len(candles))
// 		return make([]internal.SignalType, len(candles))
// 	}

// 	// Извлекаем параметры с более мягкими значениями по умолчанию
// 	minWaveLength := params.MinWaveLength
// 	if minWaveLength == 0 {
// 		minWaveLength = 3 // уменьшили с 5 до 3
// 	}

// 	maxWaveLength := params.MaxWaveLength
// 	if maxWaveLength == 0 {
// 		maxWaveLength = 30 // уменьшили с 50 до 30
// 	}

// 	fibThreshold := params.FibonacciThreshold
// 	if fibThreshold == 0 {
// 		fibThreshold = 0.8 // увеличили с 0.618 до 0.8 для большей гибкости
// 	}

// 	trendStrength := params.TrendStrength
// 	if trendStrength == 0 {
// 		trendStrength = 0.1 // уменьшили с 0.3 до 0.1 для меньшей строгости
// 	}

// 	// Извлекаем ценовые данные
// 	prices := make([]float64, len(candles))
// 	for i, candle := range candles {
// 		prices[i] = candle.Close.ToFloat64()
// 	}

// 	log.Printf("🔍 Анализ волн Эллиотта: мин.длина=%d, макс.длина=%d, фиб=%f, тренд=%f",
// 		minWaveLength, maxWaveLength, fibThreshold, trendStrength)

// 	// Создаем и обучаем анализатор волн
// 	analyzer := NewElliottWaveAnalyzer(minWaveLength, maxWaveLength, fibThreshold, trendStrength)
// 	analyzer.findSignificantExtrema(prices)
// 	wavePoints := analyzer.identifyWavePattern()

// 	log.Printf("✅ Найдено %d волновых точек", len(wavePoints))

// 	// Генерируем сигналы
// 	signals := make([]internal.SignalType, len(candles))
// 	inPosition := false
// 	positionEntryPrice := 0.0

// 	for i := 20; i < len(candles); i++ {
// 		signal := analyzer.predictSignal(i, prices)

// 		currentPrice := prices[i]

// 		// Логика входа в позицию
// 		if !inPosition {
// 			switch signal {
// 			case internal.BUY:
// 				signals[i] = internal.BUY
// 				inPosition = true
// 				positionEntryPrice = currentPrice
// 				// log.Printf("   BUY сигнал на свече %d: цена=%.2f", i, currentPrice)
// 			case internal.SELL:
// 				signals[i] = internal.SELL
// 				inPosition = true
// 				positionEntryPrice = currentPrice
// 				// log.Printf("   SELL сигнал на свече %d: цена=%.2f", i, currentPrice)
// 			default:
// 				signals[i] = internal.HOLD
// 			}
// 		} else {
// 			// Логика выхода из позиции
// 			priceChangePercent := (currentPrice - positionEntryPrice) / positionEntryPrice

// 			// Выходим при достижении цели прибыли (3% для BUY, -3% для SELL)
// 			if (inPosition && signal == internal.BUY && priceChangePercent > 0.03) ||
// 				(inPosition && signal == internal.SELL && priceChangePercent < -0.03) {
// 				signals[i] = internal.SELL
// 				inPosition = false
// 				// log.Printf("   SELL (цель) на свече %d: цена=%.2f, изменение=%.2f%%",
// 				// 	i, currentPrice, priceChangePercent*100)
// 			} else if signal == internal.SELL && inPosition {
// 				// Выходим если получаем прямой сигнал на выход
// 				signals[i] = internal.SELL
// 				inPosition = false
// 				// log.Printf("   SELL сигнал на свече %d: цена=%.2f", i, currentPrice)
// 			} else if signal == internal.BUY && inPosition {
// 				// Выходим из короткой позиции если получаем сигнал на покупку
// 				signals[i] = internal.BUY
// 				inPosition = false
// 				// log.Printf("   BUY (выход из SELL) на свече %d: цена=%.2f", i, currentPrice)
// 			} else {
// 				// Удерживаем позицию или выходим при стоп-лоссе (3% убыток)
// 				if (inPosition && signal == internal.BUY && priceChangePercent < -0.03) ||
// 					(inPosition && signal == internal.SELL && priceChangePercent > 0.03) {
// 					signals[i] = internal.SELL
// 					inPosition = false
// 					// log.Printf("   SELL (стоп-лосс) на свече %d: цена=%.2f, изменение=%.2f%%",
// 					// 	i, currentPrice, priceChangePercent*100)
// 				} else {
// 					signals[i] = internal.HOLD
// 				}
// 			}
// 		}
// 	}

// 	log.Printf("✅ Волновой анализ Эллиотта завершен")
// 	return signals
// }

func (s *ElliottWaveStrategy) DefaultConfig() internal.StrategyConfig {
	return &ElliottWaveConfig{
		MinWaveLength:      5,
		MaxWaveLength:      50,
		FibonacciThreshold: 0.618,
		TrendStrength:      0.3,
	}
}

func (s *ElliottWaveStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	ewConfig, ok := config.(*ElliottWaveConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := ewConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < 20 {
		log.Printf("⚠️ Недостаточно данных для волнового анализа Эллиотта: получено %d свечей, требуется минимум 20", len(candles))
		return make([]internal.SignalType, len(candles))
	}

	// Извлекаем ценовые данные
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	log.Printf("🔍 Анализ волн Эллиотта: мин.длина=%d, макс.длина=%d, фиб=%f, тренд=%f",
		ewConfig.MinWaveLength, ewConfig.MaxWaveLength, ewConfig.FibonacciThreshold, ewConfig.TrendStrength)

	// Создаем и обучаем анализатор волн
	analyzer := NewElliottWaveAnalyzer(ewConfig.MinWaveLength, ewConfig.MaxWaveLength, ewConfig.FibonacciThreshold, ewConfig.TrendStrength)
	analyzer.findSignificantExtrema(prices)
	wavePoints := analyzer.identifyWavePattern()

	log.Printf("✅ Найдено %d волновых точек", len(wavePoints))

	// Генерируем сигналы
	signals := make([]internal.SignalType, len(candles))
	inPosition := false
	positionEntryPrice := 0.0

	for i := 20; i < len(candles); i++ {
		signal := analyzer.predictSignal(i, prices)

		currentPrice := prices[i]

		// Логика входа в позицию
		if !inPosition {
			switch signal {
			case internal.BUY:
				signals[i] = internal.BUY
				inPosition = true
				positionEntryPrice = currentPrice
			case internal.SELL:
				signals[i] = internal.SELL
				inPosition = true
				positionEntryPrice = currentPrice
			default:
				signals[i] = internal.HOLD
			}
		} else {
			// Логика выхода из позиции
			priceChangePercent := (currentPrice - positionEntryPrice) / positionEntryPrice

			// Выходим при достижении цели прибыли (3% для BUY, -3% для SELL)
			if (inPosition && signal == internal.BUY && priceChangePercent > 0.03) ||
				(inPosition && signal == internal.SELL && priceChangePercent < -0.03) {
				signals[i] = internal.SELL
				inPosition = false
			} else if signal == internal.SELL && inPosition {
				// Выходим если получаем прямой сигнал на выход
				signals[i] = internal.SELL
				inPosition = false
			} else if signal == internal.BUY && inPosition {
				// Выходим из короткой позиции если получаем сигнал на покупку
				signals[i] = internal.BUY
				inPosition = false
			} else {
				// Удерживаем позицию или выходим при стоп-лоссе (3% убыток)
				if (inPosition && signal == internal.BUY && priceChangePercent < -0.03) ||
					(inPosition && signal == internal.SELL && priceChangePercent > 0.03) {
					signals[i] = internal.SELL
					inPosition = false
				} else {
					signals[i] = internal.HOLD
				}
			}
		}
	}

	log.Printf("✅ Волновой анализ Эллиотта завершен")
	return signals
}

func (s *ElliottWaveStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := &ElliottWaveConfig{
		MinWaveLength:      5,
		MaxWaveLength:      50,
		FibonacciThreshold: 0.618,
		TrendStrength:      0.3,
	}
	bestProfit := -1.0

	// Grid search по параметрам
	for minLen := 3; minLen <= 10; minLen += 2 {
		for maxLen := 30; maxLen <= 80; maxLen += 10 {
			for fibThresh := 0.5; fibThresh <= 0.8; fibThresh += 0.1 {
				for trendStr := 0.2; trendStr <= 0.5; trendStr += 0.1 {
					config := &ElliottWaveConfig{
						MinWaveLength:      minLen,
						MaxWaveLength:      maxLen,
						FibonacciThreshold: fibThresh,
						TrendStrength:      trendStr,
					}
					if config.Validate() != nil {
						continue
					}

					signals := s.GenerateSignalsWithConfig(candles, config)
					result := internal.Backtest(candles, signals, 0.01)

					if result.TotalProfit > bestProfit {
						bestProfit = result.TotalProfit
						bestConfig = config
					}
				}
			}
		}
	}

	fmt.Printf("Лучшие параметры SOLID волн Эллиотта: min_len=%d, max_len=%d, fib_thresh=%.3f, trend_str=%.1f, профит=%.4f\n",
		bestConfig.MinWaveLength, bestConfig.MaxWaveLength, bestConfig.FibonacciThreshold,
		bestConfig.TrendStrength, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("elliott_wave", &ElliottWaveStrategy{})
}
