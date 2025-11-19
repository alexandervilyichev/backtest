package spline

import (
	"bt/internal"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type QuadraticVariableTrendSplineConfig struct {
	MinSegmentLength int `json:"min_segment_length"`
	MaxSegmentLength int `json:"max_segment_length"`
}

func (c *QuadraticVariableTrendSplineConfig) Validate() error {
	if c.MinSegmentLength < 3 {
		c.MinSegmentLength = 3
	}
	if c.MaxSegmentLength < c.MinSegmentLength {
		c.MaxSegmentLength = c.MinSegmentLength * 2
	}

	return nil
}

func (c *QuadraticVariableTrendSplineConfig) DefaultConfigString() string {
	return fmt.Sprintf("QuadraticVariableTrendSpline(min_segment_length=%d, max_segment_length=%d)",
		c.MinSegmentLength, c.MaxSegmentLength)
}

type QuadraticSplineSegment struct {
	StartIdx    int
	EndIdx      int
	A           float64 // coefficient for x^2
	B           float64 // coefficient for x
	C           float64 // constant term
	IsAscending bool    // overall trend direction
	InflectionX float64 // x-coordinate of inflection point (where derivative = 0)
}

type QuadraticVariableTrendSplineStrategy struct{ internal.BaseConfig }

func (s *QuadraticVariableTrendSplineStrategy) Name() string {
	return "quadratic_variable_trend_spline"
}

func (s *QuadraticVariableTrendSplineStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	splineConfig, ok := config.(*QuadraticVariableTrendSplineConfig)
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

	// Fit alternating quadratic splines
	segments := s.fitAlternatingQuadraticSplines(prices, splineConfig)

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
			// Check if both segments have sufficient slope magnitude at their respective points
			prevSegment := segments[i-1]
			currSegment := segments[i]

			// If previous segment was ascending and current is descending -> SELL signal
			if prevSegment.IsAscending && !currSegment.IsAscending {
				signals[changePoint] = internal.SELL
			} else if !prevSegment.IsAscending && currSegment.IsAscending {
				// If previous segment was descending and current is ascending -> BUY signal
				signals[changePoint] = internal.BUY
			}

		}
	}

	// Print spline parameters
	// fmt.Println("Quadratic Spline Parameters:")
	// for i, segment := range segments {
	// 	fmt.Printf("Segment %d: StartIdx=%d, EndIdx=%d, A=%.8f, B=%.6f, C=%.6f, IsAscending=%t, InflectionX=%.2f\n",
	// 		i, segment.StartIdx, segment.EndIdx, segment.A, segment.B, segment.C, segment.IsAscending, segment.InflectionX)
	// }

	return signals
}

func (s *QuadraticVariableTrendSplineStrategy) fitAlternatingQuadraticSplines(prices []float64, config *QuadraticVariableTrendSplineConfig) []QuadraticSplineSegment {
	var segments []QuadraticSplineSegment
	n := len(prices)

	if n < config.MinSegmentLength {
		return segments
	}

	currentIdx := 0
	isAscending := true // Start with ascending segment

	for currentIdx < n-config.MinSegmentLength {
		segment := s.fitQuadraticSegment(prices, currentIdx, n, isAscending, config)
		if segment.EndIdx <= currentIdx {
			break
		}

		segments = append(segments, segment)
		currentIdx = segment.EndIdx
		isAscending = !isAscending // Alternate direction
	}

	return segments
}

func (s *QuadraticVariableTrendSplineStrategy) fitQuadraticSegment(prices []float64, startIdx, maxEndIdx int, isAscending bool, config *QuadraticVariableTrendSplineConfig) QuadraticSplineSegment {
	minLength := config.MinSegmentLength
	maxLength := int(math.Min(float64(config.MaxSegmentLength), float64(maxEndIdx-startIdx)))

	if maxLength < minLength {
		maxLength = minLength
	}

	if maxLength < minLength {
		return QuadraticSplineSegment{StartIdx: startIdx, EndIdx: startIdx}
	}

	bestSegment := QuadraticSplineSegment{StartIdx: startIdx, EndIdx: startIdx + minLength}
	bestR2 := -1.0

	// Try different segment lengths
	for length := minLength; length <= maxLength; length++ {
		endIdx := startIdx + length
		if endIdx > len(prices) {
			break
		}

		a, b, c := s.quadraticRegression(prices[startIdx:endIdx])
		r2 := s.calculateQuadraticR2(prices[startIdx:endIdx], a, b, c)

		// Determine overall trend direction based on the slope at the midpoint
		midX := float64(length-1) / 2.0
		slopeAtMid := 2*a*midX + b
		slopeMatches := (isAscending && slopeAtMid > 0) || (!isAscending && slopeAtMid < 0)

		// Calculate inflection point (where derivative = 0)
		var inflectionX float64
		if math.Abs(a) > 1e-10 {
			inflectionX = -b / (2 * a) // x where 2a*x + b = 0
		} else {
			inflectionX = math.Inf(1) // No inflection point for linear case
		}

		if slopeMatches && r2 > bestR2 {
			bestR2 = r2
			bestSegment = QuadraticSplineSegment{
				StartIdx:    startIdx,
				EndIdx:      endIdx,
				A:           a,
				B:           b,
				C:           c,
				IsAscending: isAscending,
				InflectionX: inflectionX,
			}
		}
	}

	return bestSegment
}

func (s *QuadraticVariableTrendSplineStrategy) quadraticRegression(y []float64) (a, b, c float64) {
	n := float64(len(y))
	if n < 3 {
		return 0, 0, y[0]
	}

	sumX, sumX2, sumX3, sumX4 := 0.0, 0.0, 0.0, 0.0
	sumY, sumXY, sumX2Y := 0.0, 0.0, 0.0

	for i, yi := range y {
		xi := float64(i)
		x2 := xi * xi
		x3 := x2 * xi
		x4 := x3 * xi

		sumX += xi
		sumX2 += x2
		sumX3 += x3
		sumX4 += x4
		sumY += yi
		sumXY += xi * yi
		sumX2Y += x2 * yi
	}

	// Solve the system of equations using matrix inversion for quadratic regression
	// Matrix form: A * coeffs = B
	// A = [sumX4, sumX3, sumX2; sumX3, sumX2, sumX; sumX2, sumX, n]
	// coeffs = [a, b, c]
	// B = [sumX2Y, sumXY, sumY]

	// Calculate the determinant of A
	detA := sumX4*(sumX2*n-sumX*sumX) - sumX3*(sumX3*n-sumX*sumX2) + sumX2*(sumX3*sumX-sumX2*sumX2)

	if math.Abs(detA) < 1e-10 {
		// Degenerate case, fall back to linear regression
		slope, intercept := s.linearRegression(y)
		return 0, slope, intercept
	}

	// Calculate adjugate matrix elements for Cramer's rule
	// a = det(A_a) / det(A) where A_a replaces first column with B
	detAa := sumX2Y*(sumX2*n-sumX*sumX) - sumXY*(sumX3*n-sumX*sumX2) + sumY*(sumX3*sumX-sumX2*sumX2)
	a = detAa / detA

	// b = det(A_b) / det(A) where A_b replaces second column with B
	detAb := sumX4*(sumXY*n-sumX*sumY) - sumX2Y*(sumX3*n-sumX*sumX2) + sumX2*(sumX3*sumY-sumX2*sumXY)
	b = detAb / detA

	// c = det(A_c) / det(A) where A_c replaces third column with B
	detAc := sumX4*(sumX2*sumY-sumX*sumXY) - sumX3*(sumX3*sumY-sumX*sumX2Y) + sumX2*(sumX3*sumXY-sumX2*sumX2Y)
	c = detAc / detA

	return a, b, c
}

func (s *QuadraticVariableTrendSplineStrategy) linearRegression(y []float64) (slope, intercept float64) {
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

func (s *QuadraticVariableTrendSplineStrategy) calculateQuadraticR2(y []float64, a, b, c float64) float64 {
	if len(y) < 3 {
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
		predicted := a*float64(i*i) + b*float64(i) + c
		ssRes += (yi - predicted) * (yi - predicted)
		ssTot += (yi - mean) * (yi - mean)
	}

	if ssTot == 0 {
		return 0
	}

	return 1 - ssRes/ssTot
}

func (s *QuadraticVariableTrendSplineStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := s.DefaultConfig().(*QuadraticVariableTrendSplineConfig)
	bestProfit := -1.0

	var results []internal.GridSearchResult

	for minLen := 5; minLen < 80; minLen += 5 {
		for maxLen := 40; maxLen < 420; maxLen += 5 {
			if maxLen < minLen {
				continue
			}

			config := &QuadraticVariableTrendSplineConfig{
				MinSegmentLength: minLen,
				MaxSegmentLength: maxLen,
			}

			if config.Validate() != nil {
				continue
			}

			signals := s.GenerateSignalsWithConfig(candles, config)
			result := internal.Backtest(candles, signals, s.GetSlippage())

			// Collect results for mesh format
			results = append(results, internal.GridSearchResult{
				X:      minLen,
				Y:      maxLen,
				Profit: result.TotalProfit,
			})

			// Select configuration with highest profit
			if result.TotalProfit >= bestProfit {
				bestProfit = result.TotalProfit
				bestConfig = config
			}
		}
	}

	// Save results in mesh format
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling results: %v\n", err)
		return bestConfig
	}

	err = os.WriteFile("grid_search_results.json", data, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
	}

	fmt.Printf("Лучшие параметры Quadratic Variable Trend Spline: min_length=%d, max_length=%d, профит=%.4f\n",
		bestConfig.MinSegmentLength, bestConfig.MaxSegmentLength, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("quadratic_variable_trend_spline", &QuadraticVariableTrendSplineStrategy{
		BaseConfig: internal.BaseConfig{
			Config: &QuadraticVariableTrendSplineConfig{
				MinSegmentLength: 5,
				MaxSegmentLength: 50,
			},
		},
	})
}
