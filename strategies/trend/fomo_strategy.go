package trend

import (
	"bt/internal"
	"fmt"
	"log"
	"math"
)

// FOMOConfig defines configuration parameters for the FOMO strategy
// This strategy models Fear of Missing Out - psychological pressure to enter trades
// when seeing rapid price movements with high volume and momentum
type FOMOConfig struct {
	// Lookback periods for calculating averages
	VolumeLookback    int     `json:"volume_lookback"`     // Period for volume average
	MomentumLookback  int     `json:"momentum_lookback"`   // Period for momentum calculation
	VolatilityLookback int    `json:"volatility_lookback"` // Period for volatility calculation
	
	// FOMO trigger thresholds
	VolumeSpike       float64 `json:"volume_spike"`        // Volume must be X times average
	MomentumThreshold float64 `json:"momentum_threshold"`  // Minimum momentum for FOMO
	VolatilityBoost   float64 `json:"volatility_boost"`    // Volatility multiplier for FOMO
	
	// Psychological factors
	ConsecutiveBars   int     `json:"consecutive_bars"`    // Consecutive moves to trigger FOMO
	FearDecay         float64 `json:"fear_decay"`          // How quickly FOMO fear decays
	GreedMultiplier   float64 `json:"greed_multiplier"`    // Amplifies signals during strong trends
	
	// Risk management
	MaxFOMOStrength   float64 `json:"max_fomo_strength"`   // Cap on FOMO intensity
	CooldownPeriod    int     `json:"cooldown_period"`     // Bars to wait after FOMO signal
}

// DefaultConfig returns default configuration
func (c *FOMOStrategy) DefaultConfig() internal.StrategyConfig {
	return &FOMOConfig{
		VolumeLookback:    20,
		MomentumLookback:  10,
		VolatilityLookback: 15,
		VolumeSpike:       1.8,  // Volume must be 1.8x average (баланс)
		MomentumThreshold: 0.015, // 1.5% momentum threshold (баланс)
		VolatilityBoost:   1.3,  // 30% volatility boost
		ConsecutiveBars:   3,    // 3 consecutive moves (возврат к более строгому)
		FearDecay:         0.85, // 15% decay per bar
		GreedMultiplier:   1.4,  // 40% greed amplification
		MaxFOMOStrength:   4.0,  // Max 4x signal strength
		CooldownPeriod:    3,    // 3 bar cooldown (средний период)
	}
}

// Validate validates configuration parameters
func (c *FOMOConfig) Validate() error {
	if c.VolumeLookback <= 0 || c.MomentumLookback <= 0 || c.VolatilityLookback <= 0 {
		return fmt.Errorf("lookback periods must be positive")
	}
	if c.VolumeSpike <= 1.0 {
		return fmt.Errorf("volume spike must be greater than 1.0")
	}
	if c.MomentumThreshold <= 0 {
		return fmt.Errorf("momentum threshold must be positive")
	}
	if c.FearDecay <= 0 || c.FearDecay >= 1 {
		return fmt.Errorf("fear decay must be between 0 and 1")
	}
	if c.ConsecutiveBars <= 0 {
		return fmt.Errorf("consecutive bars must be positive")
	}
	return nil
}

// DefaultConfigString implements StrategyConfig interface
func (c *FOMOConfig) DefaultConfigString() string {
	return fmt.Sprintf("FOMO(vol_spike=%.1f, momentum=%.3f, consecutive=%d)",
		c.VolumeSpike, c.MomentumThreshold, c.ConsecutiveBars)
}

type FOMOStrategy struct {
	config *FOMOConfig
}

func (s *FOMOStrategy) Name() string {
	return "FOMO"
}

// FOMOState tracks psychological state for FOMO calculation
type FOMOState struct {
	consecutiveUp   int     // Consecutive up moves
	consecutiveDown int     // Consecutive down moves
	fearLevel       float64 // Current fear level (0-1)
	greedLevel      float64 // Current greed level (0-1)
	lastSignalBar   int     // Last bar where signal was generated
}

func (s *FOMOStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	fomoConfig, ok := config.(*FOMOConfig)
	if !ok {
		log.Fatal("Invalid configuration type for FOMO strategy")
	}

	signals := make([]internal.SignalType, len(candles))
	
	// Pre-calculate indicators
	volumeMA := s.calculateVolumeMA(candles, fomoConfig.VolumeLookback)
	momentum := s.calculateMomentum(candles, fomoConfig.MomentumLookback)
	volatility := s.calculateVolatility(candles, fomoConfig.VolatilityLookback)
	
	// Initialize FOMO state
	state := &FOMOState{}
	
	maxLookback := max(max(fomoConfig.VolumeLookback, fomoConfig.MomentumLookback), fomoConfig.VolatilityLookback)
	
	for i := range candles {
		// Skip early periods where calculations aren't possible
		if i < maxLookback {
			signals[i] = internal.HOLD
			continue
		}
		
		// Update psychological state
		s.updatePsychologicalState(candles, i, state, fomoConfig)
		
		// Check cooldown period
		if i-state.lastSignalBar < fomoConfig.CooldownPeriod {
			signals[i] = internal.HOLD
			continue
		}
		
		// Calculate FOMO strength
		fomoStrength := s.calculateFOMOStrength(candles, i, volumeMA, momentum, volatility, state, fomoConfig)
		
		// Generate signal based on FOMO strength and market conditions
		signal := s.generateFOMOSignal(candles, i, fomoStrength, state, fomoConfig)
		
		if signal != internal.HOLD {
			state.lastSignalBar = i
		}
		
		signals[i] = signal
	}

	return signals
}

// calculateVolumeMA calculates volume moving average
func (s *FOMOStrategy) calculateVolumeMA(candles []internal.Candle, period int) []float64 {
	volumeMA := make([]float64, len(candles))
	
	for i := range candles {
		if i >= period-1 {
			var sum float64
			for j := i - period + 1; j <= i; j++ {
				sum += candles[j].VolumeFloat64()
			}
			volumeMA[i] = sum / float64(period)
		}
	}
	
	return volumeMA
}

// calculateMomentum calculates price momentum
func (s *FOMOStrategy) calculateMomentum(candles []internal.Candle, period int) []float64 {
	momentum := make([]float64, len(candles))
	
	for i := range candles {
		if i >= period {
			current := candles[i].Close.ToFloat64()
			previous := candles[i-period].Close.ToFloat64()
			momentum[i] = (current - previous) / previous
		}
	}
	
	return momentum
}

// calculateVolatility calculates price volatility (standard deviation of returns)
func (s *FOMOStrategy) calculateVolatility(candles []internal.Candle, period int) []float64 {
	volatility := make([]float64, len(candles))
	
	for i := range candles {
		if i >= period {
			// Calculate returns
			returns := make([]float64, period)
			var meanReturn float64
			
			for j := 0; j < period; j++ {
				idx := i - period + 1 + j
				if idx > 0 {
					returns[j] = (candles[idx].Close.ToFloat64() - candles[idx-1].Close.ToFloat64()) / candles[idx-1].Close.ToFloat64()
					meanReturn += returns[j]
				}
			}
			meanReturn /= float64(period)
			
			// Calculate standard deviation
			var variance float64
			for _, ret := range returns {
				variance += math.Pow(ret-meanReturn, 2)
			}
			volatility[i] = math.Sqrt(variance / float64(period))
		}
	}
	
	return volatility
}

// updatePsychologicalState updates the psychological state based on recent price action
func (s *FOMOStrategy) updatePsychologicalState(candles []internal.Candle, i int, state *FOMOState, config *FOMOConfig) {
	if i == 0 {
		return
	}
	
	currentPrice := candles[i].Close.ToFloat64()
	previousPrice := candles[i-1].Close.ToFloat64()
	
	// Update consecutive moves
	if currentPrice > previousPrice {
		state.consecutiveUp++
		state.consecutiveDown = 0
	} else if currentPrice < previousPrice {
		state.consecutiveDown++
		state.consecutiveUp = 0
	} else {
		// No change - reset consecutive counters
		state.consecutiveUp = 0
		state.consecutiveDown = 0
	}
	
	// Update fear level - только при достижении полных последовательностей
	if state.consecutiveUp >= config.ConsecutiveBars {
		state.fearLevel = math.Min(1.0, state.fearLevel+0.3) // Fear of missing upward move
	} else if state.consecutiveDown >= config.ConsecutiveBars {
		state.fearLevel = math.Min(1.0, state.fearLevel+0.3) // Fear of missing downward move
	} else {
		state.fearLevel *= config.FearDecay // Decay fear over time
	}
	
	// Update greed level - только при значительных изменениях цены
	priceChange := math.Abs((currentPrice - previousPrice) / previousPrice)
	if priceChange > config.MomentumThreshold {
		state.greedLevel = math.Min(1.0, state.greedLevel+0.4)
	} else {
		state.greedLevel *= config.FearDecay // Use same decay rate
	}
	
	// Убираем базовые уровни - эмоции должны накапливаться естественно
	state.fearLevel = math.Max(0.0, state.fearLevel)
	state.greedLevel = math.Max(0.0, state.greedLevel)
}

// calculateFOMOStrength calculates the overall FOMO strength
func (s *FOMOStrategy) calculateFOMOStrength(candles []internal.Candle, i int, volumeMA, momentum, volatility []float64, state *FOMOState, config *FOMOConfig) float64 {
	// Volume spike factor - строгий расчет
	volumeSpikeFactor := 0.0
	if volumeMA[i] > 0 {
		volumeRatio := candles[i].VolumeFloat64() / volumeMA[i]
		if volumeRatio >= config.VolumeSpike {
			volumeSpikeFactor = math.Min(2.0, volumeRatio/config.VolumeSpike)
		}
	}
	
	// Momentum factor - строгий расчет
	momentumFactor := 0.0
	if math.Abs(momentum[i]) >= config.MomentumThreshold {
		momentumFactor = math.Min(2.0, math.Abs(momentum[i])/config.MomentumThreshold)
	}
	
	// Volatility boost
	volatilityFactor := 1.0
	if volatility[i] > 0 {
		volatilityFactor = 1.0 + (volatility[i] * config.VolatilityBoost)
	}
	
	// Psychological factors - требуют накопленных эмоций
	psychologicalFactor := (state.fearLevel + state.greedLevel) / 2.0
	
	// Consecutive moves amplification - только для полных последовательностей
	consecutiveFactor := 1.0
	consecutiveCount := max(state.consecutiveUp, state.consecutiveDown)
	if consecutiveCount >= config.ConsecutiveBars {
		consecutiveFactor = 1.0 + float64(consecutiveCount-config.ConsecutiveBars+1)*0.5
	}
	
	// Возвращаемся к умножению для более строгих условий
	// Все основные факторы должны быть > 0 для генерации сигнала
	if volumeSpikeFactor == 0.0 || momentumFactor == 0.0 || psychologicalFactor < 0.3 {
		return 0.0
	}
	
	fomoStrength := volumeSpikeFactor * momentumFactor * volatilityFactor * psychologicalFactor * consecutiveFactor
	
	// Apply greed multiplier during strong trends
	if momentum[i] > config.MomentumThreshold*2.0 {
		fomoStrength *= config.GreedMultiplier
	}
	
	// Cap the maximum FOMO strength
	return math.Min(config.MaxFOMOStrength, fomoStrength)
}

// generateFOMOSignal generates trading signal based on FOMO strength
func (s *FOMOStrategy) generateFOMOSignal(candles []internal.Candle, i int, fomoStrength float64, state *FOMOState, config *FOMOConfig) internal.SignalType {
	if i == 0 {
		return internal.HOLD
	}
	
	currentPrice := candles[i].Close.ToFloat64()
	previousPrice := candles[i-1].Close.ToFloat64()
	
	// Высокий порог для качественных сигналов
	minFOMOThreshold := 1.5
	
	// Require minimum FOMO strength to generate signal
	if fomoStrength < minFOMOThreshold {
		return internal.HOLD
	}
	
	// Основные FOMO сигналы - строгие условия
	if state.consecutiveUp >= config.ConsecutiveBars && currentPrice > previousPrice && fomoStrength >= 2.0 {
		// FOMO buy signal - fear of missing upward trend
		return internal.BUY
	}
	
	if state.consecutiveDown >= config.ConsecutiveBars && currentPrice < previousPrice && fomoStrength >= 2.0 {
		// FOMO sell signal - fear of missing downward trend
		return internal.SELL
	}
	
	// Экстремальные FOMO условия - только при очень высокой силе
	if fomoStrength >= config.MaxFOMOStrength*0.8 {
		if currentPrice > previousPrice && state.consecutiveUp >= config.ConsecutiveBars-1 {
			return internal.BUY
		} else if currentPrice < previousPrice && state.consecutiveDown >= config.ConsecutiveBars-1 {
			return internal.SELL
		}
	}
	
	return internal.HOLD
}

func (s *FOMOStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := s.DefaultConfig().(*FOMOConfig)
	
	// Test different parameter combinations for psychological FOMO factors
	for _, volumeSpike := range []float64{1.5, 2.0, 2.5, 3.0} {
		for _, momentumThreshold := range []float64{0.01, 0.02, 0.03, 0.05} {
			for _, consecutiveBars := range []int{2, 3, 4, 5} {
				for _, fearDecay := range []float64{0.7, 0.8, 0.9} {
					config := &FOMOConfig{
						VolumeLookback:    20,
						MomentumLookback:  10,
						VolatilityLookback: 15,
						VolumeSpike:       volumeSpike,
						MomentumThreshold: momentumThreshold,
						VolatilityBoost:   1.5,
						ConsecutiveBars:   consecutiveBars,
						FearDecay:         fearDecay,
						GreedMultiplier:   1.3,
						MaxFOMOStrength:   3.0,
						CooldownPeriod:    5,
					}

					signals := s.GenerateSignalsWithConfig(candles, config)
					
					// Evaluate performance based on FOMO-specific metrics
					score := s.evaluateFOMOPerformance(candles, signals)
					
					// Update best config if this performs better
					if score > s.evaluateFOMOPerformance(candles, s.GenerateSignalsWithConfig(candles, bestConfig)) {
						bestConfig = config
					}
				}
			}
		}
	}

	return bestConfig
}

// evaluateFOMOPerformance evaluates strategy performance with FOMO-specific metrics
func (s *FOMOStrategy) evaluateFOMOPerformance(candles []internal.Candle, signals []internal.SignalType) float64 {
	if len(candles) != len(signals) || len(candles) < 2 {
		return 0.0
	}
	
	var totalReturn float64 = 1.0
	var signalCount int
	var correctSignals int
	
	for i := 1; i < len(candles); i++ {
		if signals[i-1] == internal.HOLD {
			continue
		}
		
		signalCount++
		currentReturn := (candles[i].Close.ToFloat64() - candles[i-1].Close.ToFloat64()) / candles[i-1].Close.ToFloat64()
		
		// Apply signal direction
		if signals[i-1] == internal.BUY && currentReturn > 0 {
			totalReturn *= (1.0 + currentReturn)
			correctSignals++
		} else if signals[i-1] == internal.SELL && currentReturn < 0 {
			totalReturn *= (1.0 - currentReturn)
			correctSignals++
		} else {
			// Wrong signal - apply loss
			totalReturn *= (1.0 - math.Abs(currentReturn)*0.5)
		}
	}
	
	// Calculate accuracy
	accuracy := 0.0
	if signalCount > 0 {
		accuracy = float64(correctSignals) / float64(signalCount)
	}
	
	// Combine return and accuracy with preference for accuracy (FOMO should be precise)
	return (totalReturn-1.0)*0.3 + accuracy*0.7
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
