package lines

import "bt/internal"

type WaveletDenoiseStrategy struct{}

func (s *WaveletDenoiseStrategy) Name() string {
	return "wavelet_denoise"
}

func (s *WaveletDenoiseStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	if len(candles) < 8 {
		return make([]internal.SignalType, len(candles))
	}

	// Extract close prices
	prices := make([]float64, len(candles))
	for i, c := range candles {
		prices[i] = c.Close.ToFloat64()
	}

	// Ensure even length by trimming if necessary
	n := len(prices)
	if n%2 != 0 {
		n--
		prices = prices[:n]
	}

	// Apply DWT
	approx, detail := internal.DWT(prices[:n])

	// Denoise by zeroing detail coefficients
	zeroDetail := make([]float64, len(detail))

	// Reconstruct denoised signal
	denoised := internal.IDWT(approx, zeroDetail)

	// Pad back to original length if trimmed
	denoisedPrices := make([]float64, len(candles))
	copy(denoisedPrices, denoised)
	if len(denoised) < len(candles) {
		for i := len(denoised); i < len(candles); i++ {
			denoisedPrices[i] = denoised[len(denoised)-1] // repeat last value
		}
	}

	// Use support line logic on denoised prices
	lookback := params.SupportLookbackPeriod
	if lookback == 0 {
		lookback = 20
	}

	// Calculate rolling min on denoised prices
	supportLevels := calculateRollingMinForValues(denoisedPrices, lookback)
	if supportLevels == nil {
		return make([]internal.SignalType, len(candles))
	}

	buyThreshold := params.SupportBuyThreshold
	sellThreshold := params.SupportSellThreshold
	if buyThreshold == 0 {
		buyThreshold = 0.005 // 0.5%
	}
	if sellThreshold == 0 {
		sellThreshold = 0.01 // 1%
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false
	var entryPrice float64

	for i := lookback; i < len(candles); i++ {
		support := supportLevels[i]
		closePrice := denoisedPrices[i]

		if !inPosition && closePrice <= support*(1+buyThreshold) {
			signals[i] = internal.BUY
			inPosition = true
			entryPrice = closePrice
			continue
		}

		if inPosition {
			// Sell if price breaks below support
			if closePrice <= support*(1-sellThreshold) {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
			// Take profit if price rises 3% above entry
			if closePrice >= entryPrice*1.03 {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func calculateRollingMinForValues(values []float64, period int) []float64 {
	if len(values) < period {
		return nil
	}

	minValues := make([]float64, len(values))
	for i := 0; i < period-1; i++ {
		minValues[i] = 0
	}

	for i := period - 1; i < len(values); i++ {
		min := values[i]
		for j := i - period + 1; j <= i; j++ {
			if values[j] < min {
				min = values[j]
			}
		}
		minValues[i] = min
	}

	return minValues
}

func (s *WaveletDenoiseStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	// Use same optimization as support line
	supportStrategy := &SupportLineStrategy{}
	return supportStrategy.Optimize(candles)
}

func init() {
	// internal.RegisterStrategy("wavelet_denoise", &WaveletDenoiseStrategy{})
}
