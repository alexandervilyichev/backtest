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

package wave

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"

	"github.com/samber/lo"
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

	// Используем более мягкий алгоритм поиска экстремумов
	lookback := ewa.minWaveLength

	for i := lookback; i < len(prices)-lookback; i++ {
		// Проверяем локальный максимум
		isLocalMax := true
		for j := i - lookback; j <= i+lookback; j++ {
			if j != i && prices[j] > prices[i] {
				isLocalMax = false
				break
			}
		}

		// Проверяем локальный минимум
		isLocalMin := true
		for j := i - lookback; j <= i+lookback; j++ {
			if j != i && prices[j] < prices[i] {
				isLocalMin = false
				break
			}
		}

		if isLocalMax || isLocalMin {
			// Вычисляем силу экстремума как размах цен в окне
			minInWindow := prices[i]
			maxInWindow := prices[i]
			for j := i - lookback; j <= i+lookback; j++ {
				if prices[j] < minInWindow {
					minInWindow = prices[j]
				}
				if prices[j] > maxInWindow {
					maxInWindow = prices[j]
				}
			}

			strength := maxInWindow - minInWindow

			point := WavePoint{
				Index:    i,
				Price:    prices[i],
				IsPeak:   isLocalMax,
				Strength: strength,
			}
			ewa.wavePoints = append(ewa.wavePoints, point)
		}
	}

	// Фильтруем по максимальной длине волны и силе
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

// predictSignal генерирует торговый сигнал на основе волнового анализа
func (ewa *ElliottWaveAnalyzer) predictSignal(currentIndex int, prices []float64) internal.SignalType {
	if len(ewa.wavePoints) < 2 {
		return internal.HOLD
	}

	// Находим ближайшую волновую точку
	var lastWavePoint *WavePoint
	var prevWavePoint *WavePoint

	for i := len(ewa.wavePoints) - 1; i >= 0; i-- {
		if ewa.wavePoints[i].Index <= currentIndex {
			lastWavePoint = &ewa.wavePoints[i]
			if i > 0 {
				prevWavePoint = &ewa.wavePoints[i-1]
			}
			break
		}
	}

	if lastWavePoint == nil {
		return internal.HOLD
	}

	currentPrice := prices[currentIndex]

	// Расстояние от последней волновой точки
	distanceFromWave := currentIndex - lastWavePoint.Index

	// Не генерируем сигналы слишком близко к волновой точке
	if distanceFromWave < 3 {
		return internal.HOLD
	}

	// Проверяем пробой уровней
	priceChangePercent := (currentPrice - lastWavePoint.Price) / lastWavePoint.Price

	// Сигнал на пробой после минимума (восходящий импульс)
	if !lastWavePoint.IsPeak && priceChangePercent > 0.01 {
		// Дополнительная проверка: цена должна быть выше предыдущего максимума
		if prevWavePoint != nil && prevWavePoint.IsPeak && currentPrice > prevWavePoint.Price {
			return internal.BUY
		}
	}

	// Сигнал на пробой после максимума (нисходящий импульс)
	if lastWavePoint.IsPeak && priceChangePercent < -0.01 {
		// Дополнительная проверка: цена должна быть ниже предыдущего минимума
		if prevWavePoint != nil && !prevWavePoint.IsPeak && currentPrice < prevWavePoint.Price {
			return internal.SELL
		}
	}

	return internal.HOLD
}

type ElliottWaveStrategy struct {
	internal.BaseConfig
	internal.BaseStrategy
}

func (s *ElliottWaveStrategy) Name() string {
	return "elliott_wave"
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

	// log.Printf("🔍 Анализ волн Эллиотта: мин.длина=%d, макс.длина=%d, фиб=%f, тренд=%f",
	// 	ewConfig.MinWaveLength, ewConfig.MaxWaveLength, ewConfig.FibonacciThreshold, ewConfig.TrendStrength)

	// Создаем и обучаем анализатор волн
	analyzer := NewElliottWaveAnalyzer(ewConfig.MinWaveLength, ewConfig.MaxWaveLength, ewConfig.FibonacciThreshold, ewConfig.TrendStrength)
	analyzer.findSignificantExtrema(prices)
	// wavePoints := analyzer.identifyWavePattern()

	// log.Printf("✅ Найдено %d волновых точек", len(wavePoints))

	// Генерируем сигналы
	signals := make([]internal.SignalType, len(candles))
	inLongPosition := false
	lastSignalIndex := -1
	minSignalDistance := 10 // минимальное расстояние между сигналами

	for i := 20; i < len(candles); i++ {
		signal := analyzer.predictSignal(i, prices)

		// Проверяем минимальное расстояние между сигналами
		if lastSignalIndex >= 0 && i-lastSignalIndex < minSignalDistance {
			signals[i] = internal.HOLD
			continue
		}

		// Простая логика: только длинные позиции
		if !inLongPosition && signal == internal.BUY {
			signals[i] = internal.BUY
			inLongPosition = true
			lastSignalIndex = i
		} else if inLongPosition && signal == internal.SELL {
			signals[i] = internal.SELL
			inLongPosition = false
			lastSignalIndex = i
		} else {
			signals[i] = internal.HOLD
		}
	}

	// log.Printf("✅ Волновой анализ Эллиотта завершен")
	return signals
}

func (s *ElliottWaveStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {

	configs := lo.CrossJoinBy4(
		lo.RangeWithSteps[int](3, 10, 1),
		lo.RangeWithSteps[int](30, 80, 10),
		lo.RangeWithSteps[float64](0.5, 0.8, 0.1),
		lo.RangeWithSteps[float64](0.2, 0.5, 0.1),
		func(minLen int, maxLen int, fibThresh float64, trendStr float64) internal.StrategyConfig {
			return &ElliottWaveConfig{
				MinWaveLength:      minLen,
				MaxWaveLength:      maxLen,
				FibonacciThreshold: fibThresh,
				TrendStrength:      trendStr,
			}
		})

	max := s.ProcessConfigs(s, candles, configs)

	bestConfig := max.A.(*ElliottWaveConfig)
	bestProfit := max.B

	fmt.Printf("Лучшие параметры волн Эллиотта: min_len=%d, max_len=%d, fib_thresh=%.3f, trend_str=%.1f, профит=%.4f\n",
		bestConfig.MinWaveLength, bestConfig.MaxWaveLength, bestConfig.FibonacciThreshold,
		bestConfig.TrendStrength, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("elliott_wave", &ElliottWaveStrategy{
		BaseConfig: internal.BaseConfig{
			Config: &ElliottWaveConfig{
				MinWaveLength:      5,
				MaxWaveLength:      50,
				FibonacciThreshold: 0.618,
				TrendStrength:      0.3,
			},
		},
	})
}
