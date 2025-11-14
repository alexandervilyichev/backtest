package lines

import (
	"bt/internal"
	"errors"
	"fmt"
)

type WaveletDenoiseConfig struct {
	LookbackPeriod int     `json:"lookback_period"`
	BuyThreshold   float64 `json:"buy_threshold"`
	SellThreshold  float64 `json:"sell_threshold"`
}

func (c *WaveletDenoiseConfig) Validate() error {
	if c.LookbackPeriod <= 0 {
		return errors.New("lookback period must be positive")
	}
	if c.BuyThreshold <= 0 || c.BuyThreshold >= 1.0 {
		return errors.New("buy threshold must be between 0 and 1.0")
	}
	if c.SellThreshold <= 0 || c.SellThreshold >= 1.0 {
		return errors.New("sell threshold must be between 0 and 1.0")
	}
	if c.SellThreshold <= c.BuyThreshold {
		return errors.New("sell threshold must be greater than buy threshold")
	}
	return nil
}

func (c *WaveletDenoiseConfig) DefaultConfigString() string {
	return fmt.Sprintf("WaveletDenoise(lookback=%d, buy_thresh=%.3f, sell_thresh=%.3f)",
		c.LookbackPeriod, c.BuyThreshold, c.SellThreshold)
}

type WaveletDenoiseStrategy struct {
	internal.BaseConfig
}

func (s *WaveletDenoiseStrategy) Name() string {
	return "wavelet_denoise"
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

func (s *WaveletDenoiseStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	waveletConfig, ok := config.(*WaveletDenoiseConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := waveletConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

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

	// Calculate rolling min on denoised prices
	supportLevels := calculateRollingMinForValues(denoisedPrices, waveletConfig.LookbackPeriod)
	if supportLevels == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false
	var entryPrice float64

	for i := waveletConfig.LookbackPeriod; i < len(candles); i++ {
		support := supportLevels[i]
		closePrice := denoisedPrices[i]

		if !inPosition && closePrice <= support*(1+waveletConfig.BuyThreshold) {
			signals[i] = internal.BUY
			inPosition = true
			entryPrice = closePrice
			continue
		}

		if inPosition {
			// Sell if price breaks below support
			if closePrice <= support*(1-waveletConfig.SellThreshold) {
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

func (s *WaveletDenoiseStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := s.DefaultConfig().(*WaveletDenoiseConfig)
	bestProfit := -1.0

	// Grid search по параметрам (same as support line)
	for lookback := 10; lookback <= 50; lookback += 5 {
		for buyThresh := 0.001; buyThresh <= 0.02; buyThresh += 0.002 {
			for sellThresh := 0.005; sellThresh <= 0.05; sellThresh += 0.005 {
				config := &WaveletDenoiseConfig{
					LookbackPeriod: lookback,
					BuyThreshold:   buyThresh,
					SellThreshold:  sellThresh,
				}
				if config.Validate() != nil {
					continue
				}

				signals := s.GenerateSignalsWithConfig(candles, config)
				result := internal.Backtest(candles, signals, 0.01)
				if result.TotalProfit >= bestProfit {
					bestProfit = result.TotalProfit
					bestConfig = config
				}
			}
		}
	}

	fmt.Printf("Лучшие параметры Wavelet: lookback=%d, buy_thresh=%.4f, sell_thresh=%.4f, профит=%.4f\n",
		bestConfig.LookbackPeriod, bestConfig.BuyThreshold, bestConfig.SellThreshold, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("wavelet_denoise", &WaveletDenoiseStrategy{
		BaseConfig: internal.BaseConfig{
			Config: &WaveletDenoiseConfig{
				LookbackPeriod: 20,
				BuyThreshold:   0.005,
				SellThreshold:  0.01,
			},
		}})
}
