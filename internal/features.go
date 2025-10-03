// features.go — корректная реализация с жёсткими стартовыми индексами
package internal

import (
	"fmt"
	"log"
	"math"
	"sort"
)

// FeatureSet — набор признаков для ML-модели
type FeatureSet struct {
	RSI            float64 // RSI(14)
	SMA5           float64 // SMA(5)
	SMA10          float64 // SMA(10)
	SMA20          float64 // SMA(20)
	EMA12          float64 // EMA(12)
	EMA26          float64 // EMA(26)
	MACD           float64 // MACD(12,26,9)
	MACDSignal     float64 // Signal line (9)
	BollingerUpper float64 // BB upper (20,2)
	BollingerLower float64 // BB lower (20,2)
	VolumeRatio    float64 // current_volume / avg_volume_10
	Momentum1      float64 // price_change_1
	Momentum3      float64 // price_change_3
	Momentum5      float64 // price_change_5
	Volatility20   float64 // std_dev_20
}

// ErrNotEnoughData — ошибка недостатка данных
type ErrNotEnoughData struct {
	got, need int
}

func (e *ErrNotEnoughData) Error() string {
	return fmt.Sprintf("недостаточно данных: получено %d, требуется минимум %d", e.got, e.need)
}

// ExtractFeatures извлекает признаки из массива свечей.
// Возвращает пару: [признаки, целевые значения] для всех свечей,
// начиная с первой, где все индикаторы уже полностью рассчитаны,
// и заканчивая предпоследней свечой (т.к. нужна следующая для цели).
func ExtractFeatures(candles []Candle) ([]FeatureSet, []float64, error) {
	if len(candles) < 50 { // Минимум для надёжного расчёта всех индикаторов
		return nil, nil, &ErrNotEnoughData{len(candles), 50}
	}

	// Вычисляем все индикаторы — они будут той же длины, что и candles
	rsi := calculateRSI(candles, 14)
	sma5 := calculateSMA(candles, 5)
	sma10 := calculateSMA(candles, 10)
	sma20 := calculateSMA(candles, 20)
	ema12 := calculateEMA(candles, 12)
	ema26 := calculateEMA(candles, 26)
	macd, macdSignal := calculateMACD(candles)
	bbUpper, bbLower := calculateBollingerBands(candles, 20, 2)
	volumeAvg10 := calculateVolumeAvg(candles, 10)
	momentums1 := calculateMomentum(candles, 1)
	momentums3 := calculateMomentum(candles, 3)
	momentums5 := calculateMomentum(candles, 5)
	volatility20 := calculateVolatility(candles, 20)

	// ✅ ЖЁСТКО ЗАДАННЫЕ МИНИМАЛЬНЫЕ ИНДЕКСЫ, С КОТОРЫХ ИНДИКАТОРЫ ДАЮТ ОСМЫСЛЕННЫЕ ЗНАЧЕНИЯ
	startIdx := maxN(
		14, // RSI(14) — первый валидный на индексе 14
		4,  // SMA(5) — первый валидный на индексе 4
		9,  // SMA(10) — первый валидный на индексе 9
		19, // SMA(20) — первый валидный на индексе 19
		11, // EMA(12) — первый валидный на индексе 11
		25, // EMA(26) — первый валидный на индексе 25
		33, // MACD(12,26,9) — первый валидный Signal на индексе 33
		19, // Bollinger(20) — первый валидный на индексе 19
		9,  // VolumeAvg(10) — первый валидный на индексе 9
		1,  // Momentum(1) — первый валидный на индексе 1
		3,  // Momentum(3) — первый валидный на индексе 3
		5,  // Momentum(5) — первый валидный на индексе 5
		19, // Volatility(20) — первый валидный на индексе 19
	)

	// 💥 Защита от старых версий кода — если startIdx > 100 — значит, используется старый код
	if startIdx > 100 {
		log.Fatalf("💥 КРИТИЧЕСКАЯ ОШИБКА: startIdx=%d — вероятно, используется старая версия features.go. Проверьте код.", startIdx)
	}

	// Максимальный индекс, для которого существует следующая свеча (i+1)
	maxValidIndex := len(candles) - 2

	if startIdx > maxValidIndex {
		log.Printf("⚠️ Недостаточно данных: startIdx=%d, maxValidIndex=%d, len(candles)=%d",
			startIdx, maxValidIndex, len(candles))
		return nil, nil, &ErrNotEnoughData{len(candles), maxValidIndex + 1}
	}

	features := make([]FeatureSet, 0, maxValidIndex-startIdx+1)
	targets := make([]float64, 0, maxValidIndex-startIdx+1)

	for i := startIdx; i <= maxValidIndex; i++ {
		currentClose := candles[i].Close.ToFloat64()
		nextClose := candles[i+1].Close.ToFloat64()
		target := 1.0
		if nextClose <= currentClose {
			target = 0.0
		}

		fs := FeatureSet{
			RSI:            rsi[i],
			SMA5:           sma5[i],
			SMA10:          sma10[i],
			SMA20:          sma20[i],
			EMA12:          ema12[i],
			EMA26:          ema26[i],
			MACD:           macd[i],
			MACDSignal:     macdSignal[i],
			BollingerUpper: bbUpper[i],
			BollingerLower: bbLower[i],
			VolumeRatio:    candles[i].VolumeFloat64() / volumeAvg10[i],
			Momentum1:      momentums1[i],
			Momentum3:      momentums3[i],
			Momentum5:      momentums5[i],
			Volatility20:   volatility20[i],
		}

		features = append(features, fs)
		targets = append(targets, target)
	}

	if len(features) == 0 {
		return nil, nil, &ErrNotEnoughData{len(candles), maxValidIndex + 1}
	}

	return features, targets, nil
}

// maxN — максимум из нескольких чисел
func maxN(vals ...int) int {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// === Ниже — прежние функции без изменений ===

func calculateRSI(candles []Candle, period int) []float64 {
	if len(candles) < period+1 {
		return nil
	}
	rsi := make([]float64, len(candles))
	for i := 0; i < period; i++ {
		rsi[i] = 0
	}

	var gains, losses []float64
	for i := 1; i <= period; i++ {
		change := candles[i].Close.ToFloat64() - candles[i-1].Close.ToFloat64()
		if change > 0 {
			gains = append(gains, change)
			losses = append(losses, 0)
		} else {
			gains = append(gains, 0)
			losses = append(losses, math.Abs(change))
		}
	}

	avgGain := avg(gains)
	avgLoss := avg(losses)

	if avgLoss == 0 {
		rsi[period] = 100
	} else {
		rs := avgGain / avgLoss
		rsi[period] = 100 - (100 / (1 + rs))
	}

	for i := period + 1; i < len(candles); i++ {
		change := candles[i].Close.ToFloat64() - candles[i-1].Close.ToFloat64()
		gain, loss := 0.0, 0.0
		if change > 0 {
			gain = change
		} else {
			loss = math.Abs(change)
		}
		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
		if avgLoss == 0 {
			rsi[i] = 100
		} else {
			rs := avgGain / avgLoss
			rsi[i] = 100 - (100 / (1 + rs))
		}
	}
	return rsi
}

func calculateSMA(candles []Candle, period int) []float64 {
	if len(candles) < period {
		return nil
	}
	sma := make([]float64, len(candles))
	for i := 0; i < period-1; i++ {
		sma[i] = 0
	}
	for i := period - 1; i < len(candles); i++ {
		sum := 0.0
		for j := i - period + 1; j <= i; j++ {
			sum += candles[j].Close.ToFloat64()
		}
		sma[i] = sum / float64(period)
	}
	return sma
}

func calculateEMA(candles []Candle, period int) []float64 {
	if len(candles) < period {
		return nil
	}
	ema := make([]float64, len(candles))
	alpha := 2.0 / float64(period+1)

	// Первое значение — SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += candles[i].Close.ToFloat64()
	}
	ema[period-1] = sum / float64(period)

	for i := period; i < len(candles); i++ {
		closePrice := candles[i].Close.ToFloat64()
		ema[i] = closePrice*alpha + ema[i-1]*(1-alpha)
	}
	return ema
}

func calculateMACD(candles []Candle) ([]float64, []float64) {
	ema12 := calculateEMA(candles, 12)
	ema26 := calculateEMA(candles, 26)
	if len(ema12) != len(ema26) || len(ema12) == 0 {
		return nil, nil
	}

	macd := make([]float64, len(ema12))
	signal := make([]float64, len(ema12))

	for i := range ema12 {
		macd[i] = ema12[i] - ema26[i]
	}

	// Сглаживаем MACD по 9 периодам → Signal Line
	for i := 0; i < 8; i++ {
		signal[i] = 0
	}
	sum := 0.0
	for i := 0; i < 9; i++ {
		sum += macd[i]
	}
	signal[8] = sum / 9.0

	for i := 9; i < len(macd); i++ {
		signal[i] = macd[i]*0.2 + signal[i-1]*0.8
	}

	return macd, signal
}

func calculateBollingerBands(candles []Candle, period, multiplier int) ([]float64, []float64) {
	if len(candles) < period {
		return nil, nil
	}
	sma := calculateSMA(candles, period)
	stdDev := calculateStdDev(candles, period)

	upper := make([]float64, len(sma))
	lower := make([]float64, len(sma))

	for i := 0; i < period-1; i++ {
		upper[i] = 0
		lower[i] = 0
	}

	for i := period - 1; i < len(sma); i++ {
		upper[i] = sma[i] + float64(multiplier)*stdDev[i]
		lower[i] = sma[i] - float64(multiplier)*stdDev[i]
	}

	return upper, lower
}

func calculateStdDev(candles []Candle, period int) []float64 {
	if len(candles) < period {
		return nil
	}
	stdDev := make([]float64, len(candles))
	for i := 0; i < period-1; i++ {
		stdDev[i] = 0
	}

	for i := period - 1; i < len(candles); i++ {
		sum := 0.0
		for j := i - period + 1; j <= i; j++ {
			sum += candles[j].Close.ToFloat64()
		}
		mean := sum / float64(period)
		var sqDiffSum float64
		for j := i - period + 1; j <= i; j++ {
			diff := candles[j].Close.ToFloat64() - mean
			sqDiffSum += diff * diff
		}
		stdDev[i] = math.Sqrt(sqDiffSum / float64(period))
	}
	return stdDev
}

func calculateVolumeAvg(candles []Candle, period int) []float64 {
	if len(candles) < period {
		return nil
	}
	avg := make([]float64, len(candles))
	for i := 0; i < period-1; i++ {
		avg[i] = 0
	}
	for i := period - 1; i < len(candles); i++ {
		sum := 0.0
		for j := i - period + 1; j <= i; j++ {
			sum += candles[j].VolumeFloat64()
		}
		avg[i] = sum / float64(period)
	}
	return avg
}

func calculateMomentum(candles []Candle, period int) []float64 {
	if len(candles) < period {
		return nil
	}
	mom := make([]float64, len(candles))
	for i := 0; i < period; i++ {
		mom[i] = 0
	}
	for i := period; i < len(candles); i++ {
		prev := candles[i-period].Close.ToFloat64()
		curr := candles[i].Close.ToFloat64()
		mom[i] = curr - prev
	}
	return mom
}

func calculateVolatility(candles []Candle, period int) []float64 {
	return calculateStdDev(candles, period)
}

func avg(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// normalizeFeatures нормализует матрицу X по каждому признаку (min-max)
func NormalizeFeatures(X [][]float64) [][]float64 {
	if len(X) == 0 || len(X[0]) == 0 {
		return X
	}
	nFeatures := len(X[0])
	mins := make([]float64, nFeatures)
	maxs := make([]float64, nFeatures)

	// Найти мин/макс по каждому признаку
	for j := 0; j < nFeatures; j++ {
		mins[j] = X[0][j]
		maxs[j] = X[0][j]
		for i := 1; i < len(X); i++ {
			if X[i][j] < mins[j] {
				mins[j] = X[i][j]
			}
			if X[i][j] > maxs[j] {
				maxs[j] = X[i][j]
			}
		}
	}

	// Нормализовать
	Xnorm := make([][]float64, len(X))
	for i := range X {
		Xnorm[i] = make([]float64, nFeatures)
		for j := 0; j < nFeatures; j++ {
			rangeVal := maxs[j] - mins[j]
			if rangeVal == 0 {
				Xnorm[i][j] = 0
			} else {
				Xnorm[i][j] = (X[i][j] - mins[j]) / rangeVal
			}
		}
	}
	return Xnorm
}

// median вычисляет медиану среза float64
func Median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := make([]float64, len(xs))
	copy(sorted, xs)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2.0
	}
	return sorted[n/2]
}
