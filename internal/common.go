// strategies/common.go
// Общие функции для всех стратегий

package internal

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"math"
	"sync"
)

var Cache sync.Map

func keyFor(typeAlgo string, typeInput string, period int) string {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, typeAlgo); err != nil {
		panic(err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, typeInput); err != nil {
		panic(err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, int64(period)); err != nil {
		panic(err)
	}

	hash := md5.Sum(buf.Bytes())
	return hex.EncodeToString(hash[:])

}

// calculateSMACommon вычисляет простую скользящую среднюю
func CalculateSMACommon(candles []Candle, period int) []float64 {
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

// calculateRSICommon вычисляет RSI
func CalculateRSICommon(candles []Candle, period int) []float64 {
	key := keyFor("RSI", "candles", period)
	if cached, ok := Cache.Load(key); ok {
		return cached.([]float64)
	}

	if len(candles) < period+1 {
		return nil
	}

	rsi := make([]float64, len(candles))
	for i := 0; i < period; i++ {
		rsi[i] = 0
	}

	// Рассчитываем изменения цен
	var gains, losses []float64
	for i := 1; i <= period; i++ {
		change := candles[i].Close.ToFloat64() - candles[i-1].Close.ToFloat64()
		if change > 0 {
			gains = append(gains, change)
			losses = append(losses, 0)
		} else {
			gains = append(gains, 0)
			losses = append(losses, -change)
		}
	}

	avgGain := avgCommon(gains)
	avgLoss := avgCommon(losses)

	if avgLoss == 0 {
		rsi[period] = 100
	} else {
		rs := avgGain / avgLoss
		rsi[period] = 100 - (100 / (1 + rs))
	}

	// Рассчитываем RSI для остальных свечей
	for i := period + 1; i < len(candles); i++ {
		change := candles[i].Close.ToFloat64() - candles[i-1].Close.ToFloat64()
		gain := 0.0
		loss := 0.0
		if change > 0 {
			gain = change
		} else {
			loss = -change
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

	Cache.Store(key, rsi)
	return rsi
}

// calculateOBV вычисляет On-Balance Volume
func CalculateOBV(candles []Candle) []float64 {

	if len(candles) < 2 {
		return nil
	}

	obv := make([]float64, len(candles))
	obv[0] = 0

	for i := 1; i < len(candles); i++ {
		currentVol := candles[i].VolumeFloat // используем предвычисленное значение

		currentClose := candles[i].Close.ToFloat64()
		previousClose := candles[i-1].Close.ToFloat64()

		if currentClose > previousClose {
			// Цена выросла - добавляем объем
			obv[i] = obv[i-1] + currentVol
		} else if currentClose < previousClose {
			// Цена упала - вычитаем объем
			obv[i] = obv[i-1] - currentVol
		} else {
			// Цена не изменилась - OBV не меняется
			obv[i] = obv[i-1]
		}
	}

	return obv
}

// avgCommon вычисляет среднее значение
func avgCommon(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// calculateRollingMin вычисляет скользящий минимум
func CalculateRollingMin(candles []Candle, period int) []float64 {
	// key := keyFor("RMin", "candles", period)
	// if cached, ok := Cache.Load(key); ok {
	// 	return cached.([]float64)
	// }

	if len(candles) < period {
		return nil
	}

	minValues := make([]float64, len(candles))
	for i := 0; i < period-1; i++ {
		minValues[i] = 0
	}

	for i := period - 1; i < len(candles); i++ {
		min := candles[i].Low.ToFloat64()
		for j := i - period + 1; j <= i; j++ {
			if candles[j].Low.ToFloat64() < min {
				min = candles[j].Low.ToFloat64()
			}
		}
		minValues[i] = min
	}

	// Cache.Store(key, minValues)
	return minValues
}

// calculateRollingMax вычисляет скользящий максимум
func CalculateRollingMax(candles []Candle, period int) []float64 {
	if len(candles) < period {
		return nil
	}

	maxValues := make([]float64, len(candles))
	for i := 0; i < period-1; i++ {
		maxValues[i] = 0
	}

	for i := period - 1; i < len(candles); i++ {
		max := candles[i].High.ToFloat64()
		for j := i - period + 1; j <= i; j++ {
			if candles[j].High.ToFloat64() > max {
				max = candles[j].High.ToFloat64()
			}
		}
		maxValues[i] = max
	}

	return maxValues
}

// calculateStochastic вычисляет стохастический осциллятор (%K и %D)
func CalculateStochastic(candles []Candle, kPeriod, dPeriod int) ([]float64, []float64) {
	if len(candles) < kPeriod {
		return nil, nil
	}

	kValues := make([]float64, len(candles))
	for i := 0; i < kPeriod-1; i++ {
		kValues[i] = 0
	}

	// Вычисляем %K
	for i := kPeriod - 1; i < len(candles); i++ {
		lowestLow := candles[i].Low.ToFloat64()
		highestHigh := candles[i].High.ToFloat64()

		for j := i - kPeriod + 1; j <= i; j++ {
			if candles[j].Low.ToFloat64() < lowestLow {
				lowestLow = candles[j].Low.ToFloat64()
			}
			if candles[j].High.ToFloat64() > highestHigh {
				highestHigh = candles[j].High.ToFloat64()
			}
		}

		if highestHigh-lowestLow == 0 {
			kValues[i] = 50 // neutral value when range is 0
		} else {
			kValues[i] = 100 * (candles[i].Close.ToFloat64() - lowestLow) / (highestHigh - lowestLow)
		}
	}

	// Вычисляем %D как SMA от %K
	dValues := CalculateSMACommonForValues(kValues, dPeriod)

	return kValues, dValues
}

// calculateSMACommonForValues вычисляет SMA для массива значений
func CalculateSMACommonForValues(values []float64, period int) []float64 {
	key := keyFor("SMA", "values", period)
	if cached, ok := Cache.Load(key); ok {
		return cached.([]float64)
	}

	if len(values) < period {
		return nil
	}

	sma := make([]float64, len(values))
	for i := 0; i < period-1; i++ {
		sma[i] = 0
	}

	for i := period - 1; i < len(values); i++ {
		sum := 0.0
		for j := i - period + 1; j <= i; j++ {
			sum += values[j]
		}
		sma[i] = sum / float64(period)
	}

	Cache.Store(key, sma)
	return sma
}

// calculateVolatilityQstick рассчитывает волатильность цены за период
func CalculateVolatilityQstick(candles []Candle, period int) []float64 {
	key := keyFor("VolatilityQStick", "candles", period)
	if cached, ok := Cache.Load(key); ok {
		return cached.([]float64)
	}

	if len(candles) < period {
		return nil
	}

	volatility := make([]float64, len(candles))

	// Первые period-1 значений — не определены
	for i := 0; i < period-1; i++ {
		volatility[i] = 0
	}

	for i := period - 1; i < len(candles); i++ {
		prices := make([]float64, period)

		// Собираем цены закрытия за период
		for j := i - period + 1; j <= i; j++ {
			prices[j-(i-period+1)] = candles[j].Close.ToFloat64()
		}

		// Рассчитываем среднюю цену
		var mean float64
		for _, price := range prices {
			mean += price
		}
		mean /= float64(period)

		// Рассчитываем дисперсию
		var variance float64
		for _, price := range prices {
			variance += (price - mean) * (price - mean)
		}
		variance /= float64(period)

		volatility[i] = variance
	}

	Cache.Store(key, volatility)
	return volatility
}

// calculateEMA вычисляет экспоненциальную скользящую среднюю
func CalculateEMAForValues(values []float64, period int) []float64 {
	if len(values) < period {
		return nil
	}

	ema := make([]float64, len(values))
	multiplier := 2.0 / (float64(period) + 1.0)

	// Для первых period значений используем SMA как начальное значение
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += values[i]
	}
	ema[period-1] = sum / float64(period)

	// Вычисляем EMA для остальных значений
	for i := period; i < len(values); i++ {
		ema[i] = (values[i] * multiplier) + (ema[i-1] * (1 - multiplier))
	}

	return ema
}

// calculateMACD вычисляет MACD (MACD линия, сигнальная линия, гистограмма)
func CalculateMACDWithSignal(candles []Candle, fastPeriod, slowPeriod, signalPeriod int) ([]float64, []float64, []float64) {
	if len(candles) < slowPeriod {
		return nil, nil, nil
	}

	// Получаем цены закрытия
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// Вычисляем быструю и медленную EMA
	fastEMA := CalculateEMAForValues(prices, fastPeriod)
	slowEMA := CalculateEMAForValues(prices, slowPeriod)
	if fastEMA == nil || slowEMA == nil {
		return nil, nil, nil
	}

	// Вычисляем MACD линию
	macdLine := make([]float64, len(candles))
	for i := 0; i < len(candles); i++ {
		if fastEMA[i] == 0 || slowEMA[i] == 0 {
			macdLine[i] = 0
		} else {
			macdLine[i] = fastEMA[i] - slowEMA[i]
		}
	}

	// Вычисляем сигнальную линию (EMA от MACD)
	signalLine := CalculateEMAForValues(macdLine, signalPeriod)
	if signalLine == nil {
		return nil, nil, nil
	}

	// Вычисляем гистограмму
	histogram := make([]float64, len(candles))
	for i := 0; i < len(candles); i++ {
		histogram[i] = macdLine[i] - signalLine[i]
	}

	return macdLine, signalLine, histogram
}

// calculateMAChannel вычисляет коридор скользящих средних (верхний и нижний каналы)
func CalculateMAChannel(candles []Candle, fastPeriod, slowPeriod int, multiplier float64) ([]float64, []float64) {
	if len(candles) < slowPeriod {
		return nil, nil
	}

	fastMA := CalculateSMACommon(candles, fastPeriod)
	slowMA := CalculateSMACommon(candles, slowPeriod)
	if fastMA == nil || slowMA == nil {
		return nil, nil
	}

	upperChannel := make([]float64, len(candles))
	lowerChannel := make([]float64, len(candles))

	for i := 0; i < len(candles); i++ {
		if fastMA[i] == 0 || slowMA[i] == 0 {
			upperChannel[i] = 0
			lowerChannel[i] = 0
		} else {
			diff := fastMA[i] - slowMA[i]
			upperChannel[i] = slowMA[i] + diff*multiplier
			lowerChannel[i] = slowMA[i] - diff*multiplier
		}
	}

	return upperChannel, lowerChannel
}

// calculateCorrelation вычисляет коэффициент корреляции Пирсона между двумя временными рядами
func calculateCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0
	}

	n := float64(len(x))
	sumX, sumY, sumXY, sumX2, sumY2 := 0.0, 0.0, 0.0, 0.0, 0.0

	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	numerator := n*sumXY - sumX*sumY
	denominatorX := n*sumX2 - sumX*sumX
	denominatorY := n*sumY2 - sumY*sumY

	if denominatorX <= 0 || denominatorY <= 0 {
		return 0
	}

	return numerator / (math.Sqrt(denominatorX) * math.Sqrt(denominatorY))
}

// calculateRollingCorrelation вычисляет скользящую корреляцию между двумя временными рядами
func CalculateRollingCorrelation(x, y []float64, period int) []float64 {
	if len(x) != len(y) || len(x) < period {
		return nil
	}

	correlations := make([]float64, len(x))
	for i := 0; i < period-1; i++ {
		correlations[i] = 0
	}

	for i := period - 1; i < len(x); i++ {
		xSlice := x[i-period+1 : i+1]
		ySlice := y[i-period+1 : i+1]
		correlations[i] = calculateCorrelation(xSlice, ySlice)
	}

	return correlations
}

// calculateMeanStd вычисляет среднее значение и стандартное отклонение массива
func calculateMeanStd(data []float64) (float64, float64) {
	if len(data) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	mean := sum / float64(len(data))
	varSum := 0.0
	for _, v := range data {
		diff := v - mean
		varSum += diff * diff
	}
	variance := varSum / float64(len(data))
	return mean, math.Sqrt(variance)
}

// CalculateRollingStdDevOfReturns вычисляет скользящую волатильность как стандартное отклонение доходностей
func CalculateRollingStdDevOfReturns(prices []float64, period int) []float64 {
	key := keyFor("Rstd", "values", period)
	if cached, ok := Cache.Load(key); ok {
		return cached.([]float64)
	}

	if len(prices) < period+1 {
		return nil
	}
	volatility := make([]float64, len(prices))
	for i := period; i < len(prices); i++ {
		windowStart := i - period
		windowPrices := prices[windowStart:i]
		if len(windowPrices) >= 3 { // минимум 2 доходности
			returns := make([]float64, len(windowPrices)-1)
			for j := 1; j < len(windowPrices); j++ {
				returns[j-1] = (windowPrices[j] - windowPrices[j-1]) / windowPrices[j-1]
			}
			_, stdDev := calculateMeanStd(returns)
			volatility[i] = stdDev
		}
	}

	Cache.Store(key, volatility)
	return volatility
}

// CalculateStdDevOfReturns вычисляет волатильность как стандартное отклонение доходностей для всего массива
func CalculateStdDevOfReturns(prices []float64) float64 {
	if len(prices) < 2 {
		return 0
	}
	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		returns[i-1] = (prices[i] - prices[i-1]) / prices[i-1]
	}
	_, stdDev := calculateMeanStd(returns)
	return stdDev
}
