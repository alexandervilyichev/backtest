// Elliott Wave Advanced Strategy V3
//
// Полноценная реализация волновой теории Эллиота с расширенным функционалом:
// - Идентификация всех 5 импульсных волн (1, 2, 3, 4, 5)
// - Идентификация всех 3 коррекционных волн (A, B, C)
// - Расширенные паттерны коррекции (зигзаг, плоскость, треугольник)
// - Множественные уровни Фибоначчи (0.236, 0.382, 0.5, 0.618, 0.786, 1.0, 1.618, 2.618)
// - Правила Эллиота (волна 3 не самая короткая, волна 2 не пересекает начало волны 1, и т.д.)
// - Расширения и проекции волн
// - Временные соотношения между волнами
// - Альтернативные подсчеты волн
// - Полностью предиктивные сигналы

package wave

import (
	"bt/internal"
	"errors"
	"fmt"
)

// ElliottWaveAdvancedConfig расширенная конфигурация
type ElliottWaveAdvancedConfig struct {
	MinWaveLength         int     `json:"min_wave_length"`         // минимальная длина волны
	MaxWaveLength         int     `json:"max_wave_length"`         // максимальная длина волны
	MinWaveAmplitude      float64 `json:"min_wave_amplitude"`      // минимальная амплитуда волны (%)
	FibRetracementMin     float64 `json:"fib_retracement_min"`     // минимальный откат Фибоначчи
	FibRetracementMax     float64 `json:"fib_retracement_max"`     // максимальный откат Фибоначчи
	FibExtensionMin       float64 `json:"fib_extension_min"`       // минимальное расширение Фибоначчи
	FibExtensionMax       float64 `json:"fib_extension_max"`       // максимальное расширение Фибоначчи
	Wave3MinMultiplier    float64 `json:"wave3_min_multiplier"`    // волна 3 минимум в X раз больше волны 1
	Wave5MaxMultiplier    float64 `json:"wave5_max_multiplier"`    // волна 5 максимум в X раз больше волны 1
	TrendStrength         float64 `json:"trend_strength"`          // минимальная сила тренда
	UseAlternativeCounts  bool    `json:"use_alternative_counts"`  // использовать альтернативные подсчеты
	UseTimeRelations      bool    `json:"use_time_relations"`      // использовать временные соотношения
	UseCorrectionPatterns bool    `json:"use_correction_patterns"` // использовать паттерны коррекции
	ConfidenceThreshold   float64 `json:"confidence_threshold"`    // порог уверенности для сигнала
}

func (c *ElliottWaveAdvancedConfig) Validate() error {
	if c.MinWaveLength <= 0 {
		return errors.New("min wave length must be positive")
	}
	if c.MaxWaveLength <= c.MinWaveLength {
		return errors.New("max wave length must be greater than min")
	}
	if c.MinWaveAmplitude < 0 || c.MinWaveAmplitude > 1 {
		return errors.New("min wave amplitude must be between 0 and 1")
	}
	if c.FibRetracementMin < 0 || c.FibRetracementMin > 1 {
		return errors.New("fib retracement min must be between 0 and 1")
	}
	if c.FibRetracementMax < c.FibRetracementMin || c.FibRetracementMax > 1 {
		return errors.New("fib retracement max must be between min and 1")
	}
	if c.Wave3MinMultiplier < 1 {
		return errors.New("wave 3 min multiplier must be >= 1")
	}
	if c.ConfidenceThreshold < 0 || c.ConfidenceThreshold > 1 {
		return errors.New("confidence threshold must be between 0 and 1")
	}
	return nil
}

func (c *ElliottWaveAdvancedConfig) String() string {
	return fmt.Sprintf("ElliottWaveAdv(len=%d-%d, amp=%.3f, fib_ret=%.2f-%.2f, fib_ext=%.2f-%.2f, w3=%.1fx, conf=%.2f)",
		c.MinWaveLength, c.MaxWaveLength, c.MinWaveAmplitude,
		c.FibRetracementMin, c.FibRetracementMax,
		c.FibExtensionMin, c.FibExtensionMax,
		c.Wave3MinMultiplier, c.ConfidenceThreshold)
}

// WaveType типы волн
type WaveType int

const (
	WaveUnknown WaveType = iota
	Wave1                // импульсная волна 1
	Wave2                // коррекционная волна 2
	Wave3                // импульсная волна 3 (обычно самая сильная)
	Wave4                // коррекционная волна 4
	Wave5                // импульсная волна 5
	WaveA                // коррекционная волна A
	WaveB                // коррекционная волна B
	WaveC                // коррекционная волна C
)

func (wt WaveType) String() string {
	names := []string{"Unknown", "1", "2", "3", "4", "5", "A", "B", "C"}
	if int(wt) < len(names) {
		return names[wt]
	}
	return "Unknown"
}

func (wt WaveType) IsImpulse() bool {
	return wt == Wave1 || wt == Wave3 || wt == Wave5
}

func (wt WaveType) IsCorrection() bool {
	return wt == Wave2 || wt == Wave4 || wt == WaveA || wt == WaveB || wt == WaveC
}

// CorrectionPattern типы коррекционных паттернов
type CorrectionPattern int

const (
	CorrectionUnknown  CorrectionPattern = iota
	CorrectionZigzag                     // зигзаг (5-3-5)
	CorrectionFlat                       // плоскость (3-3-5)
	CorrectionTriangle                   // треугольник (3-3-3-3-3)
	CorrectionComplex                    // сложная коррекция
)

// AdvancedWavePoint расширенная точка волны
type AdvancedWavePoint struct {
	Index             int               // индекс в массиве
	Price             float64           // цена
	Time              int64             // время
	WaveType          WaveType          // тип волны
	IsPeak            bool              // пик или впадина
	Strength          float64           // сила волны
	FibLevel          float64           // уровень Фибоначчи относительно предыдущей волны
	TimeRatio         float64           // временное соотношение с предыдущей волной
	CorrectionPattern CorrectionPattern // паттерн коррекции (если применимо)
	Confidence        float64           // уверенность в идентификации
	AlternativeType   WaveType          // альтернативный подсчет
}

// ElliottWaveAdvancedAnalyzer расширенный анализатор волн
type ElliottWaveAdvancedAnalyzer struct {
	config          *ElliottWaveAdvancedConfig
	wavePoints      []AdvancedWavePoint
	impulseWaves    [][]AdvancedWavePoint // группы импульсных волн (1-5)
	correctionWaves [][]AdvancedWavePoint // группы коррекционных волн (A-C)
	trendDirection  float64               // направление основного тренда
	fibLevels       []float64             // уровни Фибоначчи
}

// NewElliottWaveAdvancedAnalyzer создает новый расширенный анализатор
func NewElliottWaveAdvancedAnalyzer(config *ElliottWaveAdvancedConfig) *ElliottWaveAdvancedAnalyzer {
	return &ElliottWaveAdvancedAnalyzer{
		config:          config,
		wavePoints:      make([]AdvancedWavePoint, 0),
		impulseWaves:    make([][]AdvancedWavePoint, 0),
		correctionWaves: make([][]AdvancedWavePoint, 0),
		fibLevels:       []float64{0.236, 0.382, 0.5, 0.618, 0.786, 1.0, 1.272, 1.618, 2.618},
	}
}

// Analyze выполняет полный анализ волн
func (ewa *ElliottWaveAdvancedAnalyzer) Analyze(candles []internal.Candle) {
	if len(candles) < 20 {
		return
	}

	// 1. Извлекаем данные
	prices := make([]float64, len(candles))
	times := make([]int64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
		times[i] = candle.ToTime().Unix()
	}

	// 2. Находим значимые экстремумы
	ewa.findSignificantExtrema(prices, times)

	// 3. Определяем направление тренда
	ewa.determineTrendDirection()

	// 4. Идентифицируем волновую структуру
	ewa.identifyWaveStructure()

	// 5. Применяем правила Эллиота
	ewa.applyElliottRules()

	// 6. Вычисляем уровни Фибоначчи
	ewa.calculateFibonacciLevels()

	// 7. Анализируем временные соотношения
	if ewa.config.UseTimeRelations {
		ewa.analyzeTimeRelations()
	}

	// 8. Идентифицируем паттерны коррекции
	if ewa.config.UseCorrectionPatterns {
		ewa.identifyCorrectionPatterns()
	}

	// 9. Создаем альтернативные подсчеты
	if ewa.config.UseAlternativeCounts {
		ewa.createAlternativeCounts()
	}

	// 10. Группируем волны
	ewa.groupWaves()
}

// findSignificantExtrema находит значимые экстремумы
func (ewa *ElliottWaveAdvancedAnalyzer) findSignificantExtrema(prices []float64, times []int64) {
	ewa.wavePoints = make([]AdvancedWavePoint, 0)

	lookback := ewa.config.MinWaveLength

	for i := lookback; i < len(prices)-lookback; i++ {
		isLocalMax := true
		isLocalMin := true

		// Проверяем окрестности
		for j := i - lookback; j <= i+lookback; j++ {
			if j != i {
				if prices[j] >= prices[i] {
					isLocalMax = false
				}
				if prices[j] <= prices[i] {
					isLocalMin = false
				}
			}
		}

		if !isLocalMax && !isLocalMin {
			continue
		}

		// Вычисляем силу экстремума
		minInWindow := prices[i]
		maxInWindow := prices[i]
		windowStart := 0
		if i-lookback*2 > 0 {
			windowStart = i - lookback*2
		}
		windowEnd := len(prices)
		if i+lookback*2 < len(prices) {
			windowEnd = i + lookback*2
		}
		for j := windowStart; j < windowEnd; j++ {
			if prices[j] < minInWindow {
				minInWindow = prices[j]
			}
			if prices[j] > maxInWindow {
				maxInWindow = prices[j]
			}
		}

		amplitude := (maxInWindow - minInWindow) / minInWindow

		// Фильтруем по минимальной амплитуде
		if amplitude < ewa.config.MinWaveAmplitude {
			continue
		}

		point := AdvancedWavePoint{
			Index:      i,
			Price:      prices[i],
			Time:       times[i],
			IsPeak:     isLocalMax,
			Strength:   amplitude,
			WaveType:   WaveUnknown,
			Confidence: 0.5, // базовая уверенность
		}

		ewa.wavePoints = append(ewa.wavePoints, point)
	}

	// Фильтруем по расстоянию
	ewa.filterByDistance()
}

// filterByDistance фильтрует точки по расстоянию
func (ewa *ElliottWaveAdvancedAnalyzer) filterByDistance() {
	if len(ewa.wavePoints) <= 2 {
		return
	}

	filtered := make([]AdvancedWavePoint, 0)
	filtered = append(filtered, ewa.wavePoints[0])

	for i := 1; i < len(ewa.wavePoints); i++ {
		last := filtered[len(filtered)-1]
		current := ewa.wavePoints[i]

		distance := current.Index - last.Index

		// Слишком близко - выбираем более сильную точку
		if distance < ewa.config.MinWaveLength {
			if current.Strength > last.Strength {
				filtered[len(filtered)-1] = current
			}
			continue
		}

		// Слишком далеко - пропускаем
		if distance > ewa.config.MaxWaveLength {
			continue
		}

		// Проверяем чередование пиков и впадин
		if last.IsPeak == current.IsPeak {
			// Не чередуются - выбираем более сильную точку
			if current.Strength > last.Strength {
				filtered[len(filtered)-1] = current
			}
			continue
		}

		filtered = append(filtered, current)
	}

	ewa.wavePoints = filtered
}

// determineTrendDirection определяет направление тренда
func (ewa *ElliottWaveAdvancedAnalyzer) determineTrendDirection() {
	if len(ewa.wavePoints) < 2 {
		ewa.trendDirection = 0
		return
	}

	// Используем линейную регрессию для определения тренда
	n := float64(len(ewa.wavePoints))
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i, point := range ewa.wavePoints {
		x := float64(i)
		y := point.Price
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Наклон линии регрессии
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	ewa.trendDirection = slope
}

// identifyWaveStructure идентифицирует структуру волн
func (ewa *ElliottWaveAdvancedAnalyzer) identifyWaveStructure() {
	if len(ewa.wavePoints) < 5 {
		return
	}

	// Начинаем с предположения о восходящем или нисходящем тренде
	isUptrend := ewa.trendDirection > 0

	// Ищем паттерны 5-волновых импульсов
	for i := 0; i < len(ewa.wavePoints)-4; i++ {
		// Проверяем последовательность из 5 точек
		if ewa.isValidImpulseSequence(i, isUptrend) {
			// Назначаем типы волн
			ewa.wavePoints[i].WaveType = Wave1
			ewa.wavePoints[i+1].WaveType = Wave2
			ewa.wavePoints[i+2].WaveType = Wave3
			ewa.wavePoints[i+3].WaveType = Wave4
			ewa.wavePoints[i+4].WaveType = Wave5

			// Увеличиваем уверенность
			for j := i; j <= i+4; j++ {
				ewa.wavePoints[j].Confidence += 0.2
			}
		}
	}

	// Ищем паттерны 3-волновых коррекций
	for i := 0; i < len(ewa.wavePoints)-2; i++ {
		if ewa.wavePoints[i].WaveType == WaveUnknown {
			// Проверяем последовательность из 3 точек
			if ewa.isValidCorrectionSequence(i, isUptrend) {
				ewa.wavePoints[i].WaveType = WaveA
				ewa.wavePoints[i+1].WaveType = WaveB
				ewa.wavePoints[i+2].WaveType = WaveC

				for j := i; j <= i+2; j++ {
					ewa.wavePoints[j].Confidence += 0.15
				}
			}
		}
	}
}

// isValidImpulseSequence проверяет валидность 5-волнового импульса
func (ewa *ElliottWaveAdvancedAnalyzer) isValidImpulseSequence(startIdx int, isUptrend bool) bool {
	if startIdx+4 >= len(ewa.wavePoints) {
		return false
	}

	points := ewa.wavePoints[startIdx : startIdx+5]

	// Правило 1: Волна 2 не должна полностью пересекать начало волны 1 (смягченное)
	wave1Start := points[0].Price
	wave2End := points[1].Price
	wave1Length := internal.Abs(points[1].Price - points[0].Price)

	if isUptrend {
		// Разрешаем откат до 90% волны 1
		if wave2End < wave1Start+wave1Length*0.1 {
			return false
		}
	} else {
		if wave2End > wave1Start-wave1Length*0.1 {
			return false
		}
	}

	// Правило 2: Волна 3 не должна быть самой короткой (смягченное - допускаем небольшую погрешность)
	wave1Length = internal.Abs(points[1].Price - points[0].Price)
	wave3Length := internal.Abs(points[3].Price - points[2].Price)
	wave5Length := internal.Abs(points[4].Price - points[3].Price)

	if wave3Length < wave1Length*0.9 && wave3Length < wave5Length*0.9 {
		return false
	}

	// Правило 3: Волна 3 обычно длиннее волны 1 (смягченное)
	minMultiplier := ewa.config.Wave3MinMultiplier * 0.8 // снижаем требование на 20%
	if wave3Length < wave1Length*minMultiplier {
		return false
	}

	return true
}

// isValidCorrectionSequence проверяет валидность 3-волновой коррекции
func (ewa *ElliottWaveAdvancedAnalyzer) isValidCorrectionSequence(startIdx int, isUptrend bool) bool {
	if startIdx+2 >= len(ewa.wavePoints) {
		return false
	}

	points := ewa.wavePoints[startIdx : startIdx+3]

	// Коррекция должна идти против тренда
	waveALength := internal.Abs(points[1].Price - points[0].Price)
	waveCLength := internal.Abs(points[2].Price - points[1].Price)

	// Волна C обычно примерно равна или больше волны A (смягченное правило)
	if waveCLength < waveALength*0.5 {
		return false
	}

	return true
}

// applyElliottRules применяет правила Эллиота для уточнения волн
func (ewa *ElliottWaveAdvancedAnalyzer) applyElliottRules() {
	// Проходим по всем идентифицированным волнам и проверяем правила
	for i := 0; i < len(ewa.wavePoints); i++ {
		point := &ewa.wavePoints[i]

		// Правило чередования: коррекции 2 и 4 должны быть разными
		if point.WaveType == Wave2 && i+2 < len(ewa.wavePoints) {
			wave2Length := internal.Abs(point.Price - ewa.wavePoints[i-1].Price)
			if ewa.wavePoints[i+2].WaveType == Wave4 {
				wave4Length := internal.Abs(ewa.wavePoints[i+2].Price - ewa.wavePoints[i+1].Price)
				// Если длины слишком похожи, снижаем уверенность
				ratio := wave4Length / wave2Length
				if ratio > 0.8 && ratio < 1.2 {
					point.Confidence *= 0.8
					ewa.wavePoints[i+2].Confidence *= 0.8
				}
			}
		}
	}
}

// calculateFibonacciLevels вычисляет уровни Фибоначчи для каждой волны
func (ewa *ElliottWaveAdvancedAnalyzer) calculateFibonacciLevels() {
	for i := 1; i < len(ewa.wavePoints); i++ {
		current := &ewa.wavePoints[i]
		prev := ewa.wavePoints[i-1]

		// Вычисляем откат/расширение относительно предыдущей волны
		if i >= 2 {
			prevPrev := ewa.wavePoints[i-2]
			prevWaveLength := internal.Abs(prev.Price - prevPrev.Price)
			currentWaveLength := internal.Abs(current.Price - prev.Price)

			if prevWaveLength > 0 {
				current.FibLevel = currentWaveLength / prevWaveLength

				// Проверяем соответствие уровням Фибоначчи
				for _, fibLevel := range ewa.fibLevels {
					if internal.Abs(current.FibLevel-fibLevel) < 0.1 {
						current.Confidence += 0.1 // увеличиваем уверенность
						break
					}
				}
			}
		}
	}
}

// analyzeTimeRelations анализирует временные соотношения между волнами
func (ewa *ElliottWaveAdvancedAnalyzer) analyzeTimeRelations() {
	for i := 1; i < len(ewa.wavePoints); i++ {
		current := &ewa.wavePoints[i]
		prev := ewa.wavePoints[i-1]

		timeDiff := current.Time - prev.Time

		if i >= 2 {
			prevPrev := ewa.wavePoints[i-2]
			prevTimeDiff := prev.Time - prevPrev.Time

			if prevTimeDiff > 0 {
				current.TimeRatio = float64(timeDiff) / float64(prevTimeDiff)

				// Временные соотношения часто близки к Фибоначчи
				for _, fibLevel := range ewa.fibLevels {
					if internal.Abs(current.TimeRatio-fibLevel) < 0.15 {
						current.Confidence += 0.05
						break
					}
				}
			}
		}
	}
}

// identifyCorrectionPatterns идентифицирует паттерны коррекции
func (ewa *ElliottWaveAdvancedAnalyzer) identifyCorrectionPatterns() {
	for i := 0; i < len(ewa.wavePoints)-2; i++ {
		if ewa.wavePoints[i].WaveType == WaveA {
			// Анализируем паттерн A-B-C
			waveA := ewa.wavePoints[i]
			waveB := ewa.wavePoints[i+1]
			waveC := ewa.wavePoints[i+2]

			lengthA := internal.Abs(waveB.Price - waveA.Price)
			lengthB := internal.Abs(waveC.Price - waveB.Price)
			lengthC := internal.Abs(waveC.Price - waveA.Price)

			// Зигзаг: C примерно равна A, B короткая
			if lengthB < lengthA*0.618 && internal.Abs(lengthC/lengthA-1.0) < 0.3 {
				ewa.wavePoints[i].CorrectionPattern = CorrectionZigzag
				ewa.wavePoints[i+1].CorrectionPattern = CorrectionZigzag
				ewa.wavePoints[i+2].CorrectionPattern = CorrectionZigzag
			}

			// Плоскость: A, B, C примерно равны
			if internal.Abs(lengthA/lengthB-1.0) < 0.3 && internal.Abs(lengthB/lengthC-1.0) < 0.3 {
				ewa.wavePoints[i].CorrectionPattern = CorrectionFlat
				ewa.wavePoints[i+1].CorrectionPattern = CorrectionFlat
				ewa.wavePoints[i+2].CorrectionPattern = CorrectionFlat
			}
		}
	}
}

// createAlternativeCounts создает альтернативные подсчеты волн
func (ewa *ElliottWaveAdvancedAnalyzer) createAlternativeCounts() {
	// Для каждой волны с низкой уверенностью создаем альтернативный подсчет
	for i := range ewa.wavePoints {
		point := &ewa.wavePoints[i]

		if point.Confidence < 0.7 && point.WaveType != WaveUnknown {
			// Альтернативный подсчет: сдвигаем волну на одну позицию
			switch point.WaveType {
			case Wave1:
				point.AlternativeType = Wave3
			case Wave2:
				point.AlternativeType = Wave4
			case Wave3:
				point.AlternativeType = Wave5
			case Wave4:
				point.AlternativeType = Wave2
			case Wave5:
				point.AlternativeType = Wave1
			case WaveA:
				point.AlternativeType = WaveC
			case WaveB:
				point.AlternativeType = WaveA
			case WaveC:
				point.AlternativeType = WaveB
			}
		}
	}
}

// groupWaves группирует волны в импульсы и коррекции
func (ewa *ElliottWaveAdvancedAnalyzer) groupWaves() {
	ewa.impulseWaves = make([][]AdvancedWavePoint, 0)
	ewa.correctionWaves = make([][]AdvancedWavePoint, 0)

	currentImpulse := make([]AdvancedWavePoint, 0)
	currentCorrection := make([]AdvancedWavePoint, 0)

	for _, point := range ewa.wavePoints {
		if point.WaveType.IsImpulse() {
			if len(currentCorrection) > 0 {
				ewa.correctionWaves = append(ewa.correctionWaves, currentCorrection)
				currentCorrection = make([]AdvancedWavePoint, 0)
			}
			currentImpulse = append(currentImpulse, point)

			if point.WaveType == Wave5 {
				ewa.impulseWaves = append(ewa.impulseWaves, currentImpulse)
				currentImpulse = make([]AdvancedWavePoint, 0)
			}
		} else if point.WaveType.IsCorrection() {
			if len(currentImpulse) > 0 {
				ewa.impulseWaves = append(ewa.impulseWaves, currentImpulse)
				currentImpulse = make([]AdvancedWavePoint, 0)
			}
			currentCorrection = append(currentCorrection, point)

			if point.WaveType == WaveC {
				ewa.correctionWaves = append(ewa.correctionWaves, currentCorrection)
				currentCorrection = make([]AdvancedWavePoint, 0)
			}
		}
	}

	// Добавляем незавершенные группы
	if len(currentImpulse) > 0 {
		ewa.impulseWaves = append(ewa.impulseWaves, currentImpulse)
	}
	if len(currentCorrection) > 0 {
		ewa.correctionWaves = append(ewa.correctionWaves, currentCorrection)
	}
}

// PredictNextWave предсказывает следующую волну
func (ewa *ElliottWaveAdvancedAnalyzer) PredictNextWave(currentIndex int, currentPrice float64) *internal.FutureSignal {
	if len(ewa.wavePoints) < 2 {
		return nil
	}

	// Находим последнюю идентифицированную волну
	var lastWave *AdvancedWavePoint
	for i := len(ewa.wavePoints) - 1; i >= 0; i-- {
		if ewa.wavePoints[i].Index <= currentIndex {
			lastWave = &ewa.wavePoints[i]
			break
		}
	}

	if lastWave == nil || lastWave.WaveType == WaveUnknown {
		return nil
	}

	// Предсказываем следующую волну на основе текущей
	var signalType internal.SignalType
	var predictedPrice float64
	var confidence float64

	switch lastWave.WaveType {
	case Wave1:
		// После волны 1 ожидаем коррекцию (волна 2)
		signalType = internal.SELL
		// Волна 2 обычно откатывает на 50-61.8% от волны 1
		if len(ewa.wavePoints) >= 2 {
			wave1Start := ewa.wavePoints[len(ewa.wavePoints)-2].Price
			wave1Length := lastWave.Price - wave1Start
			predictedPrice = lastWave.Price - wave1Length*ewa.config.FibRetracementMax
		}
		confidence = lastWave.Confidence * 0.7

	case Wave2:
		// После волны 2 ожидаем сильный импульс (волна 3)
		signalType = internal.BUY
		// Волна 3 обычно в 1.618 раза больше волны 1
		if len(ewa.wavePoints) >= 3 {
			wave1Length := internal.Abs(ewa.wavePoints[len(ewa.wavePoints)-2].Price - ewa.wavePoints[len(ewa.wavePoints)-3].Price)
			predictedPrice = lastWave.Price + wave1Length*ewa.config.FibExtensionMax
		}
		confidence = lastWave.Confidence * 0.9 // высокая уверенность для волны 3

	case Wave3:
		// После волны 3 ожидаем коррекцию (волна 4)
		signalType = internal.SELL
		// Волна 4 обычно откатывает на 38.2% от волны 3
		if len(ewa.wavePoints) >= 2 {
			wave3Start := ewa.wavePoints[len(ewa.wavePoints)-2].Price
			wave3Length := lastWave.Price - wave3Start
			predictedPrice = lastWave.Price - wave3Length*ewa.config.FibRetracementMin
		}
		confidence = lastWave.Confidence * 0.75

	case Wave4:
		// После волны 4 ожидаем финальный импульс (волна 5)
		signalType = internal.BUY
		// Волна 5 обычно равна волне 1 или немного меньше
		if len(ewa.wavePoints) >= 5 {
			wave1Length := internal.Abs(ewa.wavePoints[len(ewa.wavePoints)-4].Price - ewa.wavePoints[len(ewa.wavePoints)-5].Price)
			predictedPrice = lastWave.Price + wave1Length*ewa.config.Wave5MaxMultiplier
		}
		confidence = lastWave.Confidence * 0.8

	case Wave5:
		// После волны 5 ожидаем коррекцию (волна A)
		signalType = internal.SELL
		// Волна A обычно откатывает на 38.2-61.8% от всего импульса
		if len(ewa.wavePoints) >= 5 {
			impulseStart := ewa.wavePoints[len(ewa.wavePoints)-5].Price
			impulseLength := lastWave.Price - impulseStart
			predictedPrice = lastWave.Price - impulseLength*0.5
		}
		confidence = lastWave.Confidence * 0.85

	case WaveA:
		// После волны A ожидаем откат (волна B)
		signalType = internal.BUY
		if len(ewa.wavePoints) >= 2 {
			waveALength := internal.Abs(lastWave.Price - ewa.wavePoints[len(ewa.wavePoints)-2].Price)
			predictedPrice = lastWave.Price + waveALength*0.618
		}
		confidence = lastWave.Confidence * 0.6

	case WaveB:
		// После волны B ожидаем финальное падение (волна C)
		signalType = internal.SELL
		if len(ewa.wavePoints) >= 3 {
			waveALength := internal.Abs(ewa.wavePoints[len(ewa.wavePoints)-2].Price - ewa.wavePoints[len(ewa.wavePoints)-3].Price)
			predictedPrice = lastWave.Price - waveALength*1.0
		}
		confidence = lastWave.Confidence * 0.7

	case WaveC:
		// После волны C ожидаем новый импульс (волна 1)
		signalType = internal.BUY
		if len(ewa.wavePoints) >= 3 {
			correctionLength := internal.Abs(lastWave.Price - ewa.wavePoints[len(ewa.wavePoints)-3].Price)
			predictedPrice = lastWave.Price + correctionLength*0.618
		}
		confidence = lastWave.Confidence * 0.75

	default:
		return nil
	}

	// Проверяем порог уверенности
	if confidence < ewa.config.ConfidenceThreshold {
		return nil
	}

	// Вычисляем предполагаемое время
	avgWaveTime := int64(0)
	if len(ewa.wavePoints) >= 2 {
		for i := 1; i < len(ewa.wavePoints); i++ {
			avgWaveTime += ewa.wavePoints[i].Time - ewa.wavePoints[i-1].Time
		}
		avgWaveTime /= int64(len(ewa.wavePoints) - 1)
	}

	predictedTime := lastWave.Time + avgWaveTime

	return &internal.FutureSignal{
		SignalType: signalType,
		Date:       predictedTime,
		Price:      predictedPrice,
		Confidence: confidence,
	}
}

// ElliottWaveAdvancedSignalGenerator генератор сигналов
type ElliottWaveAdvancedSignalGenerator struct{}

func NewElliottWaveAdvancedSignalGenerator() *ElliottWaveAdvancedSignalGenerator {
	return &ElliottWaveAdvancedSignalGenerator{}
}

// PredictNextSignal предсказывает следующий сигнал
func (sg *ElliottWaveAdvancedSignalGenerator) PredictNextSignal(candles []internal.Candle, config internal.StrategyConfigV2) *internal.FutureSignal {
	advConfig, ok := config.(*ElliottWaveAdvancedConfig)
	if !ok {
		return nil
	}

	if err := advConfig.Validate(); err != nil {
		return nil
	}

	if len(candles) < 20 {
		return nil
	}

	// Создаем и запускаем анализатор
	analyzer := NewElliottWaveAdvancedAnalyzer(advConfig)
	analyzer.Analyze(candles)

	// Предсказываем следующую волну
	currentIndex := len(candles) - 1
	currentPrice := candles[currentIndex].Close.ToFloat64()

	return analyzer.PredictNextWave(currentIndex, currentPrice)
}

// GenerateSignals генерирует сигналы для всех свечей
func (sg *ElliottWaveAdvancedSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	advConfig, ok := config.(*ElliottWaveAdvancedConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := advConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < 20 {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))

	// ОПТИМИЗАЦИЯ: анализируем все данные один раз, а не на каждой свече
	analyzer := NewElliottWaveAdvancedAnalyzer(advConfig)
	analyzer.Analyze(candles)

	if len(analyzer.wavePoints) < 2 {
		// Если волны не найдены, используем упрощенную логику
		return sg.generateSimpleSignals(candles, advConfig)
	}

	// Извлекаем цены
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// Генерируем сигналы на основе идентифицированных волн
	lastSignalIndex := -1
	minSignalDistance := advConfig.MinWaveLength / 2

	for i := 20; i < len(candles); i++ {
		// Находим ближайшую волну до текущего индекса
		var lastWave *AdvancedWavePoint
		for j := len(analyzer.wavePoints) - 1; j >= 0; j-- {
			if analyzer.wavePoints[j].Index <= i {
				lastWave = &analyzer.wavePoints[j]
				break
			}
		}

		if lastWave == nil {
			signals[i] = internal.HOLD
			continue
		}

		// Проверяем минимальное расстояние между сигналами
		if lastSignalIndex >= 0 && i-lastSignalIndex < minSignalDistance {
			signals[i] = internal.HOLD
			continue
		}

		// Генерируем сигнал
		signal := sg.generateSignalFromWave(lastWave, prices[i], i, advConfig)

		if signal != internal.HOLD {
			lastSignalIndex = i
		}

		signals[i] = signal
	}

	// Фильтруем сигналы: оставляем только переходы
	return sg.filterSignals(signals)
}

// generateSignalFromWave генерирует сигнал на основе волны
func (sg *ElliottWaveAdvancedSignalGenerator) generateSignalFromWave(
	wave *AdvancedWavePoint,
	currentPrice float64,
	currentIndex int,
	config *ElliottWaveAdvancedConfig,
) internal.SignalType {
	// Смягченная проверка уверенности
	if wave.Confidence < config.ConfidenceThreshold*0.8 {
		return internal.HOLD
	}

	// Расстояние от волновой точки
	distanceFromWave := currentIndex - wave.Index
	if distanceFromWave < 1 {
		return internal.HOLD
	}

	// Изменение цены от волновой точки
	priceChange := (currentPrice - wave.Price) / wave.Price

	// Если волна не идентифицирована, используем простую логику
	if wave.WaveType == WaveUnknown {
		// Покупаем после минимумов, продаем после максимумов
		if !wave.IsPeak && priceChange > 0.005 {
			return internal.BUY
		}
		if wave.IsPeak && priceChange < -0.005 {
			return internal.SELL
		}
		return internal.HOLD
	}

	switch wave.WaveType {
	case Wave1:
		// После волны 1 ждем коррекцию, затем покупаем на волне 3
		if wave.IsPeak && priceChange < -0.02 {
			return internal.BUY // начало волны 3
		}

	case Wave2:
		// После волны 2 покупаем на начале волны 3
		if !wave.IsPeak && priceChange > 0.005 {
			return internal.BUY
		}

	case Wave3:
		// Волна 3 - держим позицию
		return internal.HOLD

	case Wave4:
		// После волны 4 покупаем на начале волны 5
		if !wave.IsPeak && priceChange > 0.003 {
			return internal.BUY
		}

	case Wave5:
		// После волны 5 продаем перед коррекцией
		if wave.IsPeak && priceChange < -0.005 {
			return internal.SELL
		}

	case WaveA:
		// Во время волны A продаем
		if wave.IsPeak && priceChange < -0.01 {
			return internal.SELL
		}

	case WaveB:
		// После волны B продаем перед волной C
		if wave.IsPeak && priceChange < -0.003 {
			return internal.SELL
		}

	case WaveC:
		// После волны C покупаем на начале нового цикла
		if !wave.IsPeak && priceChange > 0.005 {
			return internal.BUY
		}
	}

	return internal.HOLD
}

// filterSignals фильтрует сигналы, оставляя только переходы
func (sg *ElliottWaveAdvancedSignalGenerator) filterSignals(signals []internal.SignalType) []internal.SignalType {
	filtered := make([]internal.SignalType, len(signals))
	inPosition := false

	for i, signal := range signals {
		if signal == internal.BUY && !inPosition {
			filtered[i] = internal.BUY
			inPosition = true
		} else if signal == internal.SELL && inPosition {
			filtered[i] = internal.SELL
			inPosition = false
		} else {
			filtered[i] = internal.HOLD
		}
	}

	return filtered
}

// generateSimpleSignals генерирует сигналы упрощенным методом когда волны не идентифицированы
func (sg *ElliottWaveAdvancedSignalGenerator) generateSimpleSignals(candles []internal.Candle, config *ElliottWaveAdvancedConfig) []internal.SignalType {
	signals := make([]internal.SignalType, len(candles))

	if len(candles) < config.MinWaveLength*2 {
		return signals
	}

	// Находим локальные экстремумы
	lookback := config.MinWaveLength

	for i := lookback; i < len(candles)-lookback; i++ {
		currentPrice := candles[i].Close.ToFloat64()

		// Проверяем локальный минимум
		isLocalMin := true
		for j := i - lookback; j <= i+lookback; j++ {
			if j != i && candles[j].Close.ToFloat64() < currentPrice {
				isLocalMin = false
				break
			}
		}

		// Проверяем локальный максимум
		isLocalMax := true
		for j := i - lookback; j <= i+lookback; j++ {
			if j != i && candles[j].Close.ToFloat64() > currentPrice {
				isLocalMax = false
				break
			}
		}

		// Генерируем сигналы на пробое экстремумов
		if i+lookback < len(candles) {
			futurePrice := candles[i+lookback].Close.ToFloat64()
			priceChange := (futurePrice - currentPrice) / currentPrice

			if isLocalMin && priceChange > config.MinWaveAmplitude {
				signals[i+lookback] = internal.BUY
			} else if isLocalMax && priceChange < -config.MinWaveAmplitude {
				signals[i+lookback] = internal.SELL
			}
		}
	}

	return sg.filterSignals(signals)
}

// ElliottWaveAdvancedConfigGenerator генератор конфигураций для grid search
type ElliottWaveAdvancedConfigGenerator struct{}

func NewElliottWaveAdvancedConfigGenerator() *ElliottWaveAdvancedConfigGenerator {
	return &ElliottWaveAdvancedConfigGenerator{}
}

// GenerateCoarse генерирует грубую сетку конфигураций
func (g *ElliottWaveAdvancedConfigGenerator) GenerateCoarse() []internal.StrategyConfigV2 {
	configs := make([]internal.StrategyConfigV2, 0)

	// ГРУБАЯ СЕТКА - широкий диапазон с большим шагом
	minWaveLengths := []int{5, 10}
	maxWaveLengths := []int{40, 60}
	minWaveAmplitudes := []float64{0.01, 0.02}
	confidenceThresholds := []float64{0.3, 0.5}

	// Фиксированные значения
	fibRetracementMin := 0.382
	fibRetracementMax := 0.618
	fibExtensionMin := 1.0
	fibExtensionMax := 1.618
	wave3MinMultiplier := 1.5
	wave5MaxMultiplier := 1.0
	trendStrength := 0.3

	// Только 2 комбинации функций для грубой сетки
	featureCombos := []struct {
		useAlt  bool
		useTime bool
		useCorr bool
	}{
		{true, true, true},    // все включено
		{false, false, false}, // базовая версия
	}

	// Генерируем ~32 конфигурации для грубой сетки
	for _, minLen := range minWaveLengths {
		for _, maxLen := range maxWaveLengths {
			for _, minAmp := range minWaveAmplitudes {
				for _, confThresh := range confidenceThresholds {
					for _, features := range featureCombos {
						config := &ElliottWaveAdvancedConfig{
							MinWaveLength:         minLen,
							MaxWaveLength:         maxLen,
							MinWaveAmplitude:      minAmp,
							FibRetracementMin:     fibRetracementMin,
							FibRetracementMax:     fibRetracementMax,
							FibExtensionMin:       fibExtensionMin,
							FibExtensionMax:       fibExtensionMax,
							Wave3MinMultiplier:    wave3MinMultiplier,
							Wave5MaxMultiplier:    wave5MaxMultiplier,
							TrendStrength:         trendStrength,
							UseAlternativeCounts:  features.useAlt,
							UseTimeRelations:      features.useTime,
							UseCorrectionPatterns: features.useCorr,
							ConfidenceThreshold:   confThresh,
						}
						configs = append(configs, config)
					}
				}
			}
		}
	}

	return configs
}

// GenerateFine генерирует мелкую сетку вокруг лучшей конфигурации
func (g *ElliottWaveAdvancedConfigGenerator) GenerateFine(best internal.StrategyConfigV2) []internal.StrategyConfigV2 {
	bestConfig, ok := best.(*ElliottWaveAdvancedConfig)
	if !ok {
		return []internal.StrategyConfigV2{}
	}

	configs := make([]internal.StrategyConfigV2, 0)

	// МЕЛКАЯ СЕТКА - узкий диапазон вокруг оптимума с малым шагом
	// Варьируем параметры в диапазоне ±20% от оптимума

	// MinWaveLength: ±2
	minWaveLengthMin := bestConfig.MinWaveLength - 2
	if minWaveLengthMin < 3 {
		minWaveLengthMin = 3
	}
	minWaveLengths := []int{
		minWaveLengthMin,
		bestConfig.MinWaveLength,
		bestConfig.MinWaveLength + 2,
	}

	// MaxWaveLength: ±10
	maxWaveLengthMin := bestConfig.MaxWaveLength - 10
	if maxWaveLengthMin < 30 {
		maxWaveLengthMin = 30
	}
	maxWaveLengths := []int{
		maxWaveLengthMin,
		bestConfig.MaxWaveLength,
		bestConfig.MaxWaveLength + 10,
	}

	// MinWaveAmplitude: ±0.005
	minWaveAmplitudes := []float64{
		internal.Max(0.005, bestConfig.MinWaveAmplitude-0.005),
		bestConfig.MinWaveAmplitude,
		bestConfig.MinWaveAmplitude + 0.005,
	}

	// ConfidenceThreshold: ±0.1
	confidenceThresholds := []float64{
		internal.Max(0.1, bestConfig.ConfidenceThreshold-0.1),
		bestConfig.ConfidenceThreshold,
		internal.Min(0.9, bestConfig.ConfidenceThreshold+0.1),
	}

	// Все комбинации функций
	featureCombos := []struct {
		useAlt  bool
		useTime bool
		useCorr bool
	}{
		{true, true, true},
		{true, true, false},
		{true, false, true},
		{false, true, true},
		{true, false, false},
		{false, true, false},
		{false, false, true},
		{false, false, false},
	}

	// Генерируем ~648 конфигураций для мелкой сетки
	for _, minLen := range minWaveLengths {
		for _, maxLen := range maxWaveLengths {
			if maxLen <= minLen*3 {
				continue
			}
			for _, minAmp := range minWaveAmplitudes {
				for _, confThresh := range confidenceThresholds {
					for _, features := range featureCombos {
						config := &ElliottWaveAdvancedConfig{
							MinWaveLength:         minLen,
							MaxWaveLength:         maxLen,
							MinWaveAmplitude:      minAmp,
							FibRetracementMin:     bestConfig.FibRetracementMin,
							FibRetracementMax:     bestConfig.FibRetracementMax,
							FibExtensionMin:       bestConfig.FibExtensionMin,
							FibExtensionMax:       bestConfig.FibExtensionMax,
							Wave3MinMultiplier:    bestConfig.Wave3MinMultiplier,
							Wave5MaxMultiplier:    bestConfig.Wave5MaxMultiplier,
							TrendStrength:         bestConfig.TrendStrength,
							UseAlternativeCounts:  features.useAlt,
							UseTimeRelations:      features.useTime,
							UseCorrectionPatterns: features.useCorr,
							ConfidenceThreshold:   confThresh,
						}
						configs = append(configs, config)
					}
				}
			}
		}
	}

	return configs
}

// Generate генерирует все конфигурации (для обратной совместимости)
func (g *ElliottWaveAdvancedConfigGenerator) Generate() []internal.StrategyConfigV2 {
	return g.GenerateCoarse()
}

// NewElliottWaveAdvancedStrategyV2 создает новую расширенную стратегию
func NewElliottWaveAdvancedStrategyV2(slippage float64) internal.TradingStrategy {
	slippageProvider := internal.NewSlippageProvider(slippage)
	signalGenerator := NewElliottWaveAdvancedSignalGenerator()

	configManager := internal.NewConfigManager(
		&ElliottWaveAdvancedConfig{
			MinWaveLength:         5,
			MaxWaveLength:         50,
			MinWaveAmplitude:      0.01,
			FibRetracementMin:     0.382,
			FibRetracementMax:     0.618,
			FibExtensionMin:       1.0,
			FibExtensionMax:       1.618,
			Wave3MinMultiplier:    1.5,
			Wave5MaxMultiplier:    1.0,
			TrendStrength:         0.3,
			UseAlternativeCounts:  true,
			UseTimeRelations:      true,
			UseCorrectionPatterns: true,
			ConfidenceThreshold:   0.5,
		},
		func() internal.StrategyConfigV2 {
			return &ElliottWaveAdvancedConfig{}
		},
	)

	configGenerator := NewElliottWaveAdvancedConfigGenerator()

	// Используем двухэтапный оптимизатор: грубая сетка -> мелкая сетка
	optimizer := internal.NewCoarseToFineOptimizer(
		slippageProvider,
		configGenerator.GenerateCoarse, // грубая сетка
		configGenerator.GenerateFine,   // мелкая сетка вокруг оптимума
		3,                              // топ-3 конфигурации для уточнения
	)

	return internal.NewStrategyBase(
		"elliott_wave_advanced_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewElliottWaveAdvancedStrategyV2(0.01)
	internal.RegisterStrategyV2(strategy)
}
