package trend

import (
	"bt/internal"
	"fmt"
	"log"
	"math"
)

// FOMOConfig defines configuration parameters for the FOMO strategy
type FOMOConfig struct {
	VolumeLookback int     `json:"volume_lookback"`
	PriceLookback  int     `json:"price_lookback"`
	Threshold      float64 `json:"threshold"`
	MinVolumeRatio float64 `json:"min_volume_ratio"`
	MinPriceChange float64 `json:"min_price_change"`
}

// DefaultConfig returns default configuration
func (c *FOMOStrategy) DefaultConfig() internal.StrategyConfig {
	return &FOMOConfig{
		VolumeLookback: 20,
		PriceLookback:  10,
		Threshold:      0.7,
		MinVolumeRatio: 0.8,
		MinPriceChange: 0.01,
	}
}

// Validate validates configuration parameters
func (c *FOMOConfig) Validate() error {
	if c.VolumeLookback <= 0 || c.PriceLookback <= 0 {
		return fmt.Errorf("lookback periods must be positive")
	}
	if c.Threshold <= 0 || c.Threshold > 1 {
		return fmt.Errorf("threshold must be between 0 and 1")
	}
	if c.MinVolumeRatio <= 0 {
		return fmt.Errorf("min volume ratio must be positive")
	}
	return nil
}

// DefaultConfigString implements StrategyConfig interface
func (c *FOMOConfig) DefaultConfigString() string {
	return fmt.Sprintf("FOMO(volume_lookback=%d, price_lookback=%d, threshold=%.2f)",
		c.VolumeLookback, c.PriceLookback, c.Threshold)
}

type FOMOStrategy struct {
	config *FOMOConfig
}

func (s *FOMOStrategy) Name() string {
	return "FOMO"
}

func (s *FOMOStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	fomoConfig, ok := config.(*FOMOConfig)
	if !ok {
		log.Fatal("Invalid configuration type for FOMO strategy")
	}

	signals := make([]internal.SignalType, len(candles))

	// Calculate moving averages for volume and price
	volumeMA := make([]float64, len(candles))
	priceChangeMA := make([]float64, len(candles))

	for i := range candles {
		// Volume moving average
		if i >= fomoConfig.VolumeLookback-1 {
			var sumVolume float64
			for j := i - fomoConfig.VolumeLookback + 1; j <= i; j++ {
				sumVolume += candles[j].VolumeFloat64()
			}
			volumeMA[i] = sumVolume / float64(fomoConfig.VolumeLookback)
		}

		// Price change moving average
		if i >= fomoConfig.PriceLookback-1 {
			var sumPriceChange float64
			for j := i - fomoConfig.PriceLookback + 1; j <= i; j++ {
				if j > 0 {
					change := (candles[j].Close.ToFloat64() - candles[j-1].Close.ToFloat64()) / candles[j-1].Close.ToFloat64()
					sumPriceChange += math.Abs(change)
				}
			}
			priceChangeMA[i] = sumPriceChange / float64(fomoConfig.PriceLookback)
		}
	}

	// Calculate FOMO index and generate signals
	for i := range candles {
		// Skip early periods where calculations aren't possible
		if i < max(fomoConfig.VolumeLookback, fomoConfig.PriceLookback) {
			signals[i] = internal.HOLD
			continue
		}

		// Calculate FOMO components
		volumeFactor := 1.0
		priceFactor := 1.0

		if candles[i].VolumeFloat64() < volumeMA[i]*fomoConfig.MinVolumeRatio {
			volumeFactor = 1 + (1 - (candles[i].VolumeFloat64() / volumeMA[i]))
		}

		if i > 0 {
			currentPriceChange := math.Abs((candles[i].Close.ToFloat64() - candles[i-1].Close.ToFloat64()) / candles[i-1].Close.ToFloat64())
			if currentPriceChange < priceChangeMA[i]*fomoConfig.MinPriceChange {
				priceFactor = 1 + (1 - (currentPriceChange / priceChangeMA[i]))
			}
		}

		// Calculate FOMO index
		fomoIndex := volumeFactor * priceFactor

		// Generate signal based on FOMO index
		if fomoIndex > fomoConfig.Threshold {
			// High FOMO - trend continuation
			if candles[i].Close.ToFloat64() > candles[i-1].Close.ToFloat64() {
				signals[i] = internal.BUY
			} else if candles[i].Close.ToFloat64() < candles[i-1].Close.ToFloat64() {
				signals[i] = internal.SELL
			} else {
				signals[i] = internal.HOLD
			}
		} else {
			// Low FOMO - potential reversal
			signals[i] = internal.HOLD
		}
	}

	return signals
}

func (s *FOMOStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	// Simple optimization by testing different parameters
	bestConfig := s.DefaultConfig()

	// Test different parameter combinations (simplified implementation)
	for _, volLookback := range []int{10, 20, 30} {
		for _, priceLookback := range []int{5, 10, 15} {
			for _, threshold := range []float64{0.5, 0.7, 0.9} {
				config := &FOMOConfig{
					VolumeLookback: volLookback,
					PriceLookback:  priceLookback,
					Threshold:      threshold,
					MinVolumeRatio: 0.8,
					MinPriceChange: 0.01,
				}

				_ = s.GenerateSignalsWithConfig(candles, config)
				// Evaluate performance (simplified)
				// ...
			}
		}
	}

	return bestConfig
}

func init() {
	internal.RegisterStrategy("fomo", &FOMOStrategy{})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
