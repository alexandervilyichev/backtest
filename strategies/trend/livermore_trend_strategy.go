package trend

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
)

type LivermoreConfig struct {
	EMAPeriod        int     `json:"ema_period"`
	VolumeMultiplier float64 `json:"volume_multiplier"`
	AvgVolumePeriod  int     `json:"avg_volume_period"`
}

func (c *LivermoreConfig) Validate() error {
	if c.EMAPeriod <= 0 {
		return errors.New("EMA period must be positive")
	}
	if c.VolumeMultiplier <= 0 {
		return errors.New("volume multiplier must be positive")
	}
	if c.AvgVolumePeriod <= 0 {
		return errors.New("average volume period must be positive")
	}
	return nil
}

func (c *LivermoreConfig) DefaultConfigString() string {
	return fmt.Sprintf("Livermore(EMA=%d, VolMult=%.2f, VolAvg=%d)",
		c.EMAPeriod, c.VolumeMultiplier, c.AvgVolumePeriod)
}

// LivermoreTrendStrategy implements Jesse Livermore's trend following strategy.
type LivermoreTrendStrategy struct {
	internal.BaseConfig
}

// Name returns the strategy name.
func (s *LivermoreTrendStrategy) Name() string {
	return "livermore_trend"
}

// calculateEMA calculates Exponential Moving Average for given period.
func calculateEMA(prices []float64, period int) []float64 {
	if len(prices) < period {
		return nil
	}
	ema := make([]float64, len(prices))
	multiplier := 2.0 / (float64(period) + 1.0)

	// First EMA value is SMA
	var sum float64
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	ema[period-1] = sum / float64(period)

	// Calculate subsequent EMAs
	for i := period; i < len(prices); i++ {
		ema[i] = (prices[i]-ema[i-1])*multiplier + ema[i-1]
	}

	return ema
}

// calculateAvgVolume calculates average volume for given period.
func calculateAvgVolume(volumes []float64, period int) []float64 {
	if len(volumes) < period {
		return nil
	}
	avgVol := make([]float64, len(volumes))

	for i := period - 1; i < len(volumes); i++ {
		var sum float64
		for j := i - period + 1; j <= i; j++ {
			sum += volumes[j]
		}
		avgVol[i] = sum / float64(period)
	}

	return avgVol
}

func (s *LivermoreTrendStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	liveConfig, ok := config.(*LivermoreConfig)
	if !ok {
		log.Println("Invalid Livermore config type")
		return make([]internal.SignalType, len(candles))
	}

	if err := liveConfig.Validate(); err != nil {
		log.Printf("Livermore config validation error: %v", err)
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < liveConfig.EMAPeriod || len(candles) < liveConfig.AvgVolumePeriod {
		log.Println("Not enough candles for Livermore strategy")
		return make([]internal.SignalType, len(candles))
	}

	// Prepare close prices and volumes
	closes := make([]float64, len(candles))
	volumes := make([]float64, len(candles))
	highs := make([]float64, len(candles))
	lows := make([]float64, len(candles))

	for i, c := range candles {
		closes[i] = c.Close.ToFloat64()
		volumes[i] = c.VolumeFloat64()
		highs[i] = c.High.ToFloat64()
		lows[i] = c.Low.ToFloat64()
	}

	// Calculate EMA
	ema := calculateEMA(closes, liveConfig.EMAPeriod)
	if ema == nil {
		return make([]internal.SignalType, len(candles))
	}

	// Calculate average volume
	avgVol := calculateAvgVolume(volumes, liveConfig.AvgVolumePeriod)
	if avgVol == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false
	trend := "none" // "bullish", "bearish", "none"

	// Start from max period
	startIndex := liveConfig.EMAPeriod - 1
	if liveConfig.AvgVolumePeriod > startIndex+1 {
		startIndex = liveConfig.AvgVolumePeriod - 1
	}

	for i := startIndex + 1; i < len(candles); i++ {
		currentClose := closes[i]
		currentVolume := volumes[i]

		// Determine trend based on close vs EMA
		if currentClose > ema[i] {
			trend = "bullish"
		} else if currentClose < ema[i] {
			trend = "bearish"
		}

		// Check for breakout with volume
		if trend == "bullish" && !inPosition {
			// Check if closing above previous high with sufficient volume
			if currentClose > highs[i-1] && currentVolume > avgVol[i]*liveConfig.VolumeMultiplier {
				signals[i] = internal.BUY
				inPosition = true
			}
		} else if trend == "bearish" && inPosition {
			// Check if closing below previous low with sufficient volume
			if currentClose < lows[i-1] && currentVolume > avgVol[i]*liveConfig.VolumeMultiplier {
				signals[i] = internal.SELL
				inPosition = false
			}
		}

		if signals[i] == 0 {
			signals[i] = internal.HOLD
		}
	}

	return signals
}

func (s *LivermoreTrendStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := s.DefaultConfig().(*LivermoreConfig)
	bestProfit := -1.0

	// Test different parameters
	emaOptions := []int{10, 20, 50}
	volMultOptions := []float64{1.0, 1.5, 2.0}
	avgVolOptions := []int{10, 20}

	for _, emaPeriod := range emaOptions {
		for _, volMult := range volMultOptions {
			for _, avgVolPeriod := range avgVolOptions {
				config := &LivermoreConfig{
					EMAPeriod:        emaPeriod,
					VolumeMultiplier: volMult,
					AvgVolumePeriod:  avgVolPeriod,
				}
				if config.Validate() != nil {
					continue
				}

				signals := s.GenerateSignalsWithConfig(candles, config)
				result := internal.Backtest(candles, signals, s.GetSlippage())

				if result.TotalProfit >= bestProfit {
					bestProfit = result.TotalProfit
					bestConfig = config
				}
			}
		}
	}

	fmt.Printf("Best Livermore params: EMA=%d, VolMult=%.2f, AvgVol=%d → profit=%.4f\n",
		bestConfig.EMAPeriod, bestConfig.VolumeMultiplier, bestConfig.AvgVolumePeriod, bestProfit)

	return bestConfig
}

// init registers the strategy.
func init() {
	internal.RegisterStrategy("livermore_trend", &LivermoreTrendStrategy{
		BaseConfig: internal.BaseConfig{
			Config: &LivermoreConfig{
				EMAPeriod:        20,
				VolumeMultiplier: 1.5,
				AvgVolumePeriod:  20,
			},
		},
	})
}
