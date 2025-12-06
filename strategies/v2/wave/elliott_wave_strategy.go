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
	MinSignalDistance  int     `json:"min_signal_distance"` // минимальное расстояние между сигналами
	AllowShort         bool    `json:"allow_short"`         // разрешить короткие позиции
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
	if c.MinSignalDistance < 0 {
		return errors.New("min signal distance must be non-negative")
	}
	return nil
}

func (c *ElliottWaveConfig) String() string {
	return fmt.Sprintf("ElliottWave(min_len=%d, max_len=%d, fib_thresh=%.3f, trend_str=%.1f, min_sig_dist=%d, short=%v)",
		c.MinWaveLength, c.MaxWaveLength, c.FibonacciThreshold, c.TrendStrength, c.MinSignalDistance, c.AllowShort)
}

type ElliottWaveSignalGenerator struct{}

func NewElliottWaveSignalGenerator() *ElliottWaveSignalGenerator {
	return &ElliottWaveSignalGenerator{}
}

// PredictNextSignal предсказывает следующий сигнал на основе волнового анализа
func (sg *ElliottWaveSignalGenerator) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	ewConfig, ok := config.(*ElliottWaveConfig)
	if !ok {
		return nil
	}

	if err := ewConfig.Validate(); err != nil {
		return nil
	}

	if len(candles) < 20 {
		return nil
	}

	// Извлекаем ценовые данные
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// Создаем анализатор волн
	analyzer := NewElliottWaveAnalyzer(ewConfig.MinWaveLength, ewConfig.MaxWaveLength, ewConfig.FibonacciThreshold, ewConfig.TrendStrength)
	analyzer.findSignificantExtrema(prices)
	analyzer.identifyWavePattern()

	if len(analyzer.wavePoints) < 2 {
		return nil
	}

	// Находим последние две волновые точки
	lastPoint := analyzer.wavePoints[len(analyzer.wavePoints)-1]
	var prevPoint WavePoint
	if len(analyzer.wavePoints) >= 2 {
		prevPoint = analyzer.wavePoints[len(analyzer.wavePoints)-2]
	}

	currentIdx := len(candles) - 1
	currentPrice := prices[currentIdx]

	// Вычисляем среднюю длину волны
	avgWaveLength := 0
	if len(analyzer.wavePoints) >= 2 {
		for i := 1; i < len(analyzer.wavePoints); i++ {
			avgWaveLength += analyzer.wavePoints[i].Index - analyzer.wavePoints[i-1].Index
		}
		avgWaveLength /= (len(analyzer.wavePoints) - 1)
	} else {
		avgWaveLength = (ewConfig.MinWaveLength + ewConfig.MaxWaveLength) / 2
	}

	// Расстояние от последней волновой точки
	distanceFromLastWave := currentIdx - lastPoint.Index

	// Предсказываем следующую волновую точку
	var predictedIndex int
	var predictedPrice float64
	var signalType internal.SignalType
	var confidence float64

	// Если мы близко к последней волновой точке, ждем формирования следующей
	if distanceFromLastWave < avgWaveLength/2 {
		// Предсказываем следующую волновую точку
		predictedIndex = lastPoint.Index + avgWaveLength

		// Экстраполируем цену на основе предыдущего движения
		if len(analyzer.wavePoints) >= 2 {
			priceMove := lastPoint.Price - prevPoint.Price
			predictedPrice = lastPoint.Price + priceMove

			// Определяем тип сигнала
			if lastPoint.IsPeak {
				// После пика ожидаем минимум, затем сигнал BUY
				signalType = internal.BUY
				predictedPrice = lastPoint.Price - internal.Abs(priceMove)*0.618 // коррекция Фибоначчи
			} else {
				// После минимума ожидаем максимум, затем сигнал SELL
				signalType = internal.SELL
				predictedPrice = lastPoint.Price + internal.Abs(priceMove)*1.618 // расширение Фибоначчи
			}

			// Уверенность зависит от регулярности волн
			waveRegularity := 1.0 - internal.Abs(float64(distanceFromLastWave-avgWaveLength))/float64(avgWaveLength)
			if waveRegularity < 0 {
				waveRegularity = 0
			}
			confidence = 0.3 + waveRegularity*0.4 // базовая уверенность 30-70%
		} else {
			return nil
		}
	} else {
		// Мы уже далеко от последней волновой точки, ожидаем разворот скоро
		remainingDistance := avgWaveLength - distanceFromLastWave
		if remainingDistance < 0 {
			remainingDistance = avgWaveLength / 4 // если просрочили, ожидаем в ближайшее время
		}

		predictedIndex = currentIdx + remainingDistance

		// Определяем направление на основе текущей позиции относительно последней волны
		priceChangeFromWave := (currentPrice - lastPoint.Price) / lastPoint.Price

		if lastPoint.IsPeak {
			// После пика, если цена упала, ожидаем BUY на минимуме
			if priceChangeFromWave < -0.01 {
				signalType = internal.BUY
				// Предсказываем минимум чуть ниже текущей цены
				predictedPrice = currentPrice * 0.98
				confidence = 0.5 + internal.Min(internal.Abs(priceChangeFromWave)*10, 0.3)
			} else {
				// Цена еще не упала достаточно, предсказываем падение
				if len(analyzer.wavePoints) >= 2 {
					priceMove := internal.Abs(lastPoint.Price - prevPoint.Price)
					signalType = internal.SELL
					predictedPrice = lastPoint.Price - priceMove*0.5
					confidence = 0.4
				} else {
					return nil
				}
			}
		} else {
			// После минимума, ожидаем рост
			// Если цена уже выросла значительно, ожидаем SELL на максимуме
			if priceChangeFromWave > 0.01 {
				signalType = internal.SELL
				predictedPrice = currentPrice * 1.02
				confidence = 0.5 + internal.Min(priceChangeFromWave*10, 0.3)
			} else {
				// Цена еще не выросла, предсказываем BUY
				if len(analyzer.wavePoints) >= 2 {
					priceMove := internal.Abs(lastPoint.Price - prevPoint.Price)
					signalType = internal.BUY
					predictedPrice = lastPoint.Price + priceMove*0.5
					confidence = 0.4
				} else {
					return nil
				}
			}
		}
	}

	// Ограничиваем уверенность
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.1 {
		confidence = 0.1
	}

	// Вычисляем дату сигнала
	if len(candles) < 2 {
		return nil
	}

	timeInterval := (candles[len(candles)-1].ToTime().Unix() - candles[0].ToTime().Unix()) / int64(len(candles)-1)
	lastTimestamp := candles[len(candles)-1].ToTime().Unix()
	futureTimestamp := lastTimestamp + timeInterval*int64(predictedIndex-currentIdx)

	return &internal.FutureSignal{
		SignalType: signalType,
		Date:       futureTimestamp,
		Price:      predictedPrice,
		Confidence: confidence,
	}
}

// WavePoint представляет точку волны Эллиотта
type WavePoint struct {
	Index    int     // индекс в массиве свечей
	Price    float64 // цена точки
	WaveType int     // тип волны (1, 2, 3, 4, 5, A=6, B=7, C=8)
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
// Оптимизированная версия с O(n) сложностью вместо O(n^2)
func (ewa *ElliottWaveAnalyzer) findSignificantExtrema(prices []float64) {
	ewa.wavePoints = make([]WavePoint, 0)

	if len(prices) < ewa.minWaveLength*2+1 {
		return
	}

	lookback := ewa.minWaveLength

	// Используем скользящее окно для эффективного поиска экстремумов O(n)
	for i := lookback; i < len(prices)-lookback; i++ {
		currentPrice := prices[i]
		isLocalMax := true
		isLocalMin := true
		minInWindow := currentPrice
		maxInWindow := currentPrice

		// Проверяем окно вокруг текущей точки
		for j := i - lookback; j <= i+lookback; j++ {
			if j == i {
				continue
			}

			price := prices[j]

			// Обновляем min/max для расчета силы
			if price < minInWindow {
				minInWindow = price
			}
			if price > maxInWindow {
				maxInWindow = price
			}

			// Проверяем условия экстремума
			if price > currentPrice {
				isLocalMax = false
			}
			if price < currentPrice {
				isLocalMin = false
			}

			// Ранний выход если не экстремум
			if !isLocalMax && !isLocalMin {
				break
			}
		}

		if isLocalMax || isLocalMin {
			strength := maxInWindow - minInWindow

			point := WavePoint{
				Index:    i,
				Price:    currentPrice,
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

// identifyWavePattern идентифицирует паттерн волн Эллиотта с валидацией по правилам теории
func (ewa *ElliottWaveAnalyzer) identifyWavePattern() []WavePoint {
	if len(ewa.wavePoints) < 5 {
		return ewa.wavePoints
	}

	// Определяем направление тренда по первым точкам
	trendDirection := 0.0
	if len(ewa.wavePoints) >= 2 {
		trendDirection = ewa.wavePoints[len(ewa.wavePoints)-1].Price - ewa.wavePoints[0].Price
	}
	ewa.trendDirection = trendDirection

	// Присваиваем типы волн с учетом правил Эллиотта
	waveIdx := 0
	impulseWaves := []int{1, 2, 3, 4, 5}
	correctionWaves := []int{6, 7, 8} // A=6, B=7, C=8

	inImpulse := true
	impulseCount := 0
	correctionCount := 0

	for i := 0; i < len(ewa.wavePoints); i++ {
		point := &ewa.wavePoints[i]

		if inImpulse {
			if impulseCount < len(impulseWaves) {
				point.WaveType = impulseWaves[impulseCount]
				impulseCount++

				// Валидация волн по правилам Эллиотта
				if impulseCount >= 3 && !ewa.validateImpulseWaves(i) {
					// Если валидация не прошла, сбрасываем счетчик
					impulseCount = 0
					point.WaveType = 1 // Wave1
				}

				if impulseCount >= 5 {
					inImpulse = false
					impulseCount = 0
				}
			}
		} else {
			if correctionCount < len(correctionWaves) {
				point.WaveType = correctionWaves[correctionCount]
				correctionCount++

				if correctionCount >= 3 {
					inImpulse = true
					correctionCount = 0
				}
			}
		}

		waveIdx++
	}

	return ewa.wavePoints
}

// validateImpulseWaves проверяет правила Эллиотта для импульсных волн
func (ewa *ElliottWaveAnalyzer) validateImpulseWaves(currentIdx int) bool {
	if currentIdx < 4 {
		return true // недостаточно волн для валидации
	}

	// Находим последние 5 точек для проверки
	startIdx := currentIdx - 4
	if startIdx < 0 {
		return true
	}

	wave1Idx := startIdx
	wave2Idx := startIdx + 1
	wave3Idx := startIdx + 2
	wave4Idx := startIdx + 3
	wave5Idx := startIdx + 4

	if wave5Idx >= len(ewa.wavePoints) {
		return true
	}

	wave1 := ewa.wavePoints[wave1Idx]
	wave2 := ewa.wavePoints[wave2Idx]
	wave3 := ewa.wavePoints[wave3Idx]
	wave4 := ewa.wavePoints[wave4Idx]

	// Правило 1: Волна 2 не должна откатываться более чем на 100% от волны 1
	if wave1.IsPeak != wave2.IsPeak {
		if ewa.trendDirection > 0 {
			// Восходящий тренд: wave2 не должна быть ниже начала wave1
			if wave2.Price < wave1.Price {
				return false
			}
		} else {
			// Нисходящий тренд: wave2 не должна быть выше начала wave1
			if wave2.Price > wave1.Price {
				return false
			}
		}
	}

	// Правило 2: Волна 3 не должна быть самой короткой импульсной волной
	if wave3Idx < len(ewa.wavePoints) {
		wave1Length := internal.Abs(wave2.Price - wave1.Price)
		wave3Length := internal.Abs(wave3.Price - wave2.Price)

		if wave5Idx < len(ewa.wavePoints) {
			wave5 := ewa.wavePoints[wave5Idx]
			wave5Length := internal.Abs(wave5.Price - wave4.Price)

			// Волна 3 не должна быть короче волн 1 и 5
			if wave3Length < wave1Length && wave3Length < wave5Length {
				return false
			}
		}
	}

	// Правило 3: Волна 4 не должна входить в территорию волны 1
	if wave4Idx < len(ewa.wavePoints) {
		if ewa.trendDirection > 0 {
			// Восходящий тренд: wave4 не должна быть ниже вершины wave1
			if wave4.Price < wave1.Price {
				return false
			}
		} else {
			// Нисходящий тренд: wave4 не должна быть выше дна wave1
			if wave4.Price > wave1.Price {
				return false
			}
		}
	}

	return true
}

// validateFibonacciRetracement проверяет уровни коррекции Фибоначчи между волнами
func (ewa *ElliottWaveAnalyzer) validateFibonacciRetracement(wave1, wave2 WavePoint) bool {
	if wave1.Index >= wave2.Index {
		return false
	}

	// Вычисляем откат в процентах
	waveMove := internal.Abs(wave2.Price - wave1.Price)
	if waveMove == 0 {
		return false
	}

	// Для волны 2: откат должен быть между 38.2% и 61.8% от волны 1
	retracementLevels := []float64{0.236, 0.382, 0.5, 0.618, 0.786}

	currentRetrace := internal.Abs(wave2.Price-wave1.Price) / waveMove

	// Проверяем, попадает ли откат в допустимые уровни Фибоначчи
	for _, level := range retracementLevels {
		if internal.Abs(currentRetrace-level) < ewa.fibThreshold {
			return true
		}
	}

	return false
}

// predictSignal генерирует торговый сигнал на основе волнового анализа и типов волн
func (ewa *ElliottWaveAnalyzer) predictSignal(currentIndex int, prices []float64, allowShort bool) internal.SignalType {
	if len(ewa.wavePoints) < 2 {
		return internal.HOLD
	}

	// Находим ближайшую волновую точку и предыдущие
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
	distanceFromWave := currentIndex - lastWavePoint.Index

	// Не генерируем сигналы слишком близко к волновой точке
	if distanceFromWave < 3 {
		return internal.HOLD
	}

	// Используем тип волны для генерации сигналов
	waveType := lastWavePoint.WaveType
	if waveType < 0 {
		waveType = -waveType
	}

	// BUY сигналы: начало волн 1, 3, 5 (после коррекции)
	if !lastWavePoint.IsPeak {
		// После минимума - потенциальное начало импульсной волны
		priceChange := (currentPrice - lastWavePoint.Price) / lastWavePoint.Price

		// Проверяем, что цена начала расти
		if priceChange > 0.005 {
			// Волна 2 или 4 завершилась - начинается волна 3 или 5
			if waveType == 2 || waveType == 4 || waveType == 8 { // Wave2, Wave4, WaveC
				// Валидация Фибоначчи для коррекционных волн
				if prevWavePoint != nil && ewa.validateFibonacciRetracement(*prevWavePoint, *lastWavePoint) {
					return internal.BUY
				}
				// Даже без идеальной коррекции Фибоначчи, если тренд сильный
				if priceChange > 0.015 {
					return internal.BUY
				}
			}

			// Начало нового импульса после коррекции ABC
			if waveType == 8 && priceChange > 0.01 { // WaveC
				return internal.BUY
			}
		}
	}

	// SELL сигналы: конец волны 5 или во время коррекционных волн
	if lastWavePoint.IsPeak {
		priceChange := (currentPrice - lastWavePoint.Price) / lastWavePoint.Price

		// Проверяем, что цена начала падать
		if priceChange < -0.005 {
			// Волна 5 завершилась - начинается коррекция
			if waveType == 5 { // Wave5
				return internal.SELL
			}

			// Волна 1 или 3 завершилась - начинается коррекция 2 или 4
			if waveType == 1 || waveType == 3 { // Wave1, Wave3
				if priceChange < -0.01 {
					return internal.SELL
				}
			}

			// Волна B завершилась - начинается волна C
			if waveType == 7 && priceChange < -0.015 { // WaveB
				return internal.SELL
			}
		}
	}

	// Дополнительная логика для коротких позиций (если разрешено)
	if allowShort {
		// SHORT сигналы в нисходящем тренде
		if ewa.trendDirection < 0 {
			if !lastWavePoint.IsPeak {
				priceChange := (currentPrice - lastWavePoint.Price) / lastWavePoint.Price
				if priceChange < -0.01 && (waveType == 3 || waveType == 5) { // Wave3, Wave5
					return internal.SELL
				}
			}
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

func (s *ElliottWaveSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	ewConfig, ok := config.(*ElliottWaveConfig)
	if !ok {
		log.Printf("❌ Ошибка: неверный тип конфигурации для Elliott Wave стратегии")
		return make([]internal.SignalType, len(candles))
	}

	if err := ewConfig.Validate(); err != nil {
		log.Printf("❌ Ошибка валидации конфигурации Elliott Wave: %v", err)
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

	// Создаем и обучаем анализатор волн
	analyzer := NewElliottWaveAnalyzer(ewConfig.MinWaveLength, ewConfig.MaxWaveLength, ewConfig.FibonacciThreshold, ewConfig.TrendStrength)
	analyzer.findSignificantExtrema(prices)
	wavePoints := analyzer.identifyWavePattern()

	log.Printf("✅ Найдено %d волновых точек для Elliott Wave анализа", len(wavePoints))

	// Генерируем сигналы на основе волновых паттернов
	signals := make([]internal.SignalType, len(candles))
	inLongPosition := false
	inShortPosition := false
	lastSignalIndex := -1

	// Используем параметр из конфигурации
	minSignalDistance := ewConfig.MinSignalDistance
	if minSignalDistance == 0 {
		minSignalDistance = 10 // значение по умолчанию
	}

	for i := 20; i < len(candles); i++ {
		signal := analyzer.predictSignal(i, prices, ewConfig.AllowShort)

		// Проверяем минимальное расстояние между сигналами
		if lastSignalIndex >= 0 && i-lastSignalIndex < minSignalDistance {
			signals[i] = internal.HOLD
			continue
		}

		// Логика для длинных и коротких позиций
		if ewConfig.AllowShort {
			// Разрешены и длинные, и короткие позиции
			if signal == internal.BUY {
				if inShortPosition {
					// Закрываем короткую позицию
					signals[i] = internal.BUY
					inShortPosition = false
					lastSignalIndex = i
				} else if !inLongPosition {
					// Открываем длинную позицию
					signals[i] = internal.BUY
					inLongPosition = true
					lastSignalIndex = i
				}
			} else if signal == internal.SELL {
				if inLongPosition {
					// Закрываем длинную позицию
					signals[i] = internal.SELL
					inLongPosition = false
					lastSignalIndex = i
				} else if !inShortPosition {
					// Открываем короткую позицию
					signals[i] = internal.SELL
					inShortPosition = true
					lastSignalIndex = i
				}
			} else {
				signals[i] = internal.HOLD
			}
		} else {
			// Только длинные позиции
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
	}

	return signals
}

type ElliottWaveConfigGenerator struct{}

func NewElliottWaveConfigGenerator() *ElliottWaveConfigGenerator {
	return &ElliottWaveConfigGenerator{}
}

func (s *ElliottWaveConfigGenerator) Generate() []internal.StrategyConfigV2 {
	// Генерируем конфигурации с новыми параметрами
	configs := []internal.StrategyConfigV2{}

	for _, minLen := range lo.RangeWithSteps(3, 10, 2) {
		for _, maxLen := range lo.RangeWithSteps(30, 80, 20) {
			for _, fibThresh := range lo.RangeWithSteps(0.5, 0.8, 0.15) {
				for _, trendStr := range lo.RangeWithSteps(0.2, 0.5, 0.15) {
					for _, minSigDist := range []int{5, 10, 15} {
						for _, allowShort := range []bool{false, true} {
							configs = append(configs, &ElliottWaveConfig{
								MinWaveLength:      minLen,
								MaxWaveLength:      maxLen,
								FibonacciThreshold: fibThresh,
								TrendStrength:      trendStr,
								MinSignalDistance:  minSigDist,
								AllowShort:         allowShort,
							})
						}
					}
				}
			}
		}
	}

	return configs
}

func NewElliottWaveStrategyV2(slippage float64) internal.TradingStrategy {
	// 1. Создаем провайдер проскальзывания
	slippageProvider := internal.NewSlippageProvider(slippage)

	// 2. Создаем генератор сигналов
	signalGenerator := NewElliottWaveSignalGenerator()

	// 3. Создаем менеджер конфигурации
	configManager := internal.NewConfigManager(
		&ElliottWaveConfig{},
		func() internal.StrategyConfigV2 {
			return &ElliottWaveConfig{}
		},
	)

	// 4. Создаем генератор конфигураций для оптимизации
	configGenerator := NewElliottWaveConfigGenerator()

	// 5. Создаем оптимизатор (переиспользуем универсальный GridSearchOptimizer!)
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	// 6. Собираем всё вместе через композицию
	return internal.NewStrategyBase(
		"elliott_wave_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewElliottWaveStrategyV2(0.01) // default slippage 0.01
	internal.RegisterStrategyV2(strategy)
}
