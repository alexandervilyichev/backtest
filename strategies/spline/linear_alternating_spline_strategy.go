package spline

import (
	"bt/internal"
	"encoding/json"
	"fmt"
	"math"
)

type LinearAlternatingSplineConfig struct {
	MinSegmentLength int     `json:"min_segment_length"`
	MinSlope         float64 `json:"min_slope"`
}

func (c *LinearAlternatingSplineConfig) Validate() error {
	if c.MinSegmentLength < 2 {
		c.MinSegmentLength = 2
	}
	if c.MinSlope < 0 {
		c.MinSlope = 0.001
	}
	return nil
}

func (c *LinearAlternatingSplineConfig) DefaultConfigString() string {
	return fmt.Sprintf("LinearAlternatingSpline(min_segment_length=%d, min_slope=%.4f)",
		c.MinSegmentLength, c.MinSlope)
}

type SplineSegment struct {
	StartIdx    int
	EndIdx      int
	Slope       float64
	Intercept   float64
	IsAscending bool
}

type LinearAlternatingSplineStrategy struct{}

func (s *LinearAlternatingSplineStrategy) Name() string {
	return "linear_alternating_spline"
}

func (s *LinearAlternatingSplineStrategy) DefaultConfig() internal.StrategyConfig {
	return &LinearAlternatingSplineConfig{
		MinSegmentLength: 5,
		MinSlope:         0.001,
	}
}

func (s *LinearAlternatingSplineStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	splineConfig, ok := config.(*LinearAlternatingSplineConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := splineConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < splineConfig.MinSegmentLength*2 {
		return make([]internal.SignalType, len(candles))
	}

	// Extract closing prices
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// Fit alternating linear splines
	segments := s.fitAlternatingSplines(prices, splineConfig)

	// Generate signals at trend change points
	signals := make([]internal.SignalType, len(candles))

	// If first segment is ascending and has sufficient slope, generate BUY signal at the beginning
	if len(segments) > 0 && segments[0].IsAscending {
		signals[0] = internal.BUY
	}

	// Find trend change points
	for i := 1; i < len(segments); i++ {
		changePoint := segments[i].StartIdx
		if changePoint < len(signals) {
			// Check if both segments have sufficient slope magnitude
			prevSlopeMagnitude := math.Abs(segments[i-1].Slope)
			currSlopeMagnitude := math.Abs(segments[i].Slope)

			if prevSlopeMagnitude >= splineConfig.MinSlope && currSlopeMagnitude >= splineConfig.MinSlope {
				// If previous segment was ascending and current is descending -> SELL signal
				if segments[i-1].IsAscending && !segments[i].IsAscending {
					signals[changePoint] = internal.SELL
				} else if !segments[i-1].IsAscending && segments[i].IsAscending {
					// If previous segment was descending and current is ascending -> BUY signal
					signals[changePoint] = internal.BUY
				}
			}
		}
	}

	// If last segment is ascending and has sufficient slope, generate SELL signal at the end
	// if len(segments) > 0 && segments[len(segments)-1].IsAscending {
	// 	lastIdx := len(signals) - 1
	// 	signals[lastIdx] = internal.SELL
	// }

	// Print spline parameters
	// fmt.Println("Spline Parameters:")
	// for i, segment := range segments {
	// 	fmt.Printf("Segment %d: StartIdx=%d, EndIdx=%d, Slope=%.6f, Intercept=%.6f, IsAscending=%t\n",
	// 		i, segment.StartIdx, segment.EndIdx, segment.Slope, segment.Intercept, segment.IsAscending)
	// }

	return signals
}

func (s *LinearAlternatingSplineStrategy) fitAlternatingSplines(prices []float64, config *LinearAlternatingSplineConfig) []SplineSegment {
	var segments []SplineSegment
	n := len(prices)

	if n < config.MinSegmentLength {
		return segments
	}

	currentIdx := 0
	isAscending := true // Start with ascending segment

	for currentIdx < n-config.MinSegmentLength {
		segment := s.fitSegment(prices, currentIdx, n, isAscending, config)
		if segment.EndIdx <= currentIdx {
			break
		}

		segments = append(segments, segment)
		currentIdx = segment.EndIdx
		isAscending = !isAscending // Alternate direction
	}

	return segments
}

func (s *LinearAlternatingSplineStrategy) fitSegment(prices []float64, startIdx, maxEndIdx int, isAscending bool, config *LinearAlternatingSplineConfig) SplineSegment {
	minLength := config.MinSegmentLength
	maxLength := int(math.Min(float64(maxEndIdx-startIdx), float64(len(prices)-startIdx)))

	if maxLength < minLength {
		return SplineSegment{StartIdx: startIdx, EndIdx: startIdx}
	}

	bestSegment := SplineSegment{StartIdx: startIdx, EndIdx: startIdx + minLength}
	bestR2 := -1.0

	// Try different segment lengths
	for length := minLength; length <= maxLength; length++ {
		endIdx := startIdx + length
		if endIdx > len(prices) {
			break
		}

		slope, intercept := s.linearRegression(prices[startIdx:endIdx])
		r2 := s.calculateR2(prices[startIdx:endIdx], slope, intercept)

		// Check if slope direction matches required direction
		slopeMatches := (isAscending && slope > 0) || (!isAscending && slope < 0)

		if slopeMatches && r2 > bestR2 {
			bestR2 = r2
			bestSegment = SplineSegment{
				StartIdx:    startIdx,
				EndIdx:      endIdx,
				Slope:       slope,
				Intercept:   intercept,
				IsAscending: isAscending,
			}
		}
	}

	return bestSegment
}

func (s *LinearAlternatingSplineStrategy) linearRegression(y []float64) (slope, intercept float64) {
	n := float64(len(y))
	if n < 2 {
		return 0, y[0]
	}

	sumX, sumY, sumXY, sumXX := 0.0, 0.0, 0.0, 0.0

	for i, yi := range y {
		xi := float64(i)
		sumX += xi
		sumY += yi
		sumXY += xi * yi
		sumXX += xi * xi
	}

	slope = (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)
	intercept = (sumY - slope*sumX) / n

	return slope, intercept
}

func (s *LinearAlternatingSplineStrategy) calculateR2(y []float64, slope, intercept float64) float64 {
	if len(y) < 2 {
		return 0
	}

	mean := 0.0
	for _, yi := range y {
		mean += yi
	}
	mean /= float64(len(y))

	ssRes := 0.0
	ssTot := 0.0

	for i, yi := range y {
		predicted := slope*float64(i) + intercept
		ssRes += (yi - predicted) * (yi - predicted)
		ssTot += (yi - mean) * (yi - mean)
	}

	if ssTot == 0 {
		return 0
	}

	return 1 - ssRes/ssTot
}

func (s *LinearAlternatingSplineStrategy) LoadConfigFromMap(raw json.RawMessage) internal.StrategyConfig {
	config := s.DefaultConfig()
	if err := json.Unmarshal(raw, config); err != nil {
		return nil
	}
	return config
}

func (s *LinearAlternatingSplineStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := &LinearAlternatingSplineConfig{
		MinSegmentLength: 5,
		MinSlope:         0.001,
	}
	bestProfit := -1.0

	// Grid search over parameter combinations
	minLengths := []int{5, 10, 15, 20, 30}
	minSlopes := []float64{0.0005, 0.001, 0.002, 0.005, 0.01}

	for _, minLen := range minLengths {
		for _, minSlope := range minSlopes {
			config := &LinearAlternatingSplineConfig{
				MinSegmentLength: minLen,
				MinSlope:         minSlope,
			}

			if config.Validate() != nil {
				continue
			}

			signals := s.GenerateSignalsWithConfig(candles, config)
			result := internal.Backtest(candles, signals, 0.01)

			// Select configuration with highest profit
			if result.TotalProfit >= bestProfit {
				bestProfit = result.TotalProfit
				bestConfig = config
			}
		}
	}

	fmt.Printf("Лучшие параметры Linear Alternating Spline: min_length=%d, min_slope=%.4f, профит=%.4f\n",
		bestConfig.MinSegmentLength, bestConfig.MinSlope, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("linear_alternating_spline", &LinearAlternatingSplineStrategy{})
}
