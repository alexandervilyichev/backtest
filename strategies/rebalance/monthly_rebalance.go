package rebalance

import (
	"bt/internal"
	"log"
	"time"
)

type MonthlyRebalanceConfig struct{}

func (c *MonthlyRebalanceConfig) Validate() error {
	return nil
}

func (c *MonthlyRebalanceConfig) DefaultConfigString() string {
	return "MonthlyRebalance()"
}

type MonthlyRebalanceStrategy struct{}

func (s *MonthlyRebalanceStrategy) Name() string {
	return "monthly_rebalance"
}

func (s *MonthlyRebalanceStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	if len(candles) < 2 {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))

	// Group candles by month
	monthlyCandles := make(map[string][]int) // "YYYY-MM" -> []indices

	for i, candle := range candles {
		t := candle.ToTime()
		monthKey := t.Format("2006-01")
		monthlyCandles[monthKey] = append(monthlyCandles[monthKey], i)
	}

	// Get all months in chronological order
	var months []string
	for month := range monthlyCandles {
		months = append(months, month)
	}

	// Sort months chronologically
	for i := 0; i < len(months)-1; i++ {
		for j := i + 1; j < len(months); j++ {
			if months[i] > months[j] {
				months[i], months[j] = months[j], months[i]
			}
		}
	}

	// Process each month to find sell day (second-to-last working day of current month)
	// and buy day (first working day of next month)
	for i, month := range months {
		indices := monthlyCandles[month]

		// Sort all candles in the month by time
		for k := 0; k < len(indices)-1; k++ {
			for j := k + 1; j < len(indices); j++ {
				if candles[indices[k]].ToTime().After(candles[indices[j]].ToTime()) {
					indices[k], indices[j] = indices[j], indices[k]
				}
			}
		}

		// Find the first candle of the second-to-last working day
		var sellIdx *int
		workingDaysMap := make(map[string]int) // "YYYY-MM-DD" -> first index of the day

		// Collect first candle index for each working day
		for _, idx := range indices {
			t := candles[idx].ToTime()
			weekday := t.Weekday()
			dayKey := t.Format("2006-01-02")
			if weekday != time.Saturday && weekday != time.Sunday {
				if _, exists := workingDaysMap[dayKey]; !exists {
					workingDaysMap[dayKey] = idx
				}
			}
		}

		// Get working days in chronological order
		var workingDays []int
		var days []string
		for day := range workingDaysMap {
			days = append(days, day)
		}
		// Sort days
		for i := 0; i < len(days)-1; i++ {
			for j := i + 1; j < len(days); j++ {
				if days[i] > days[j] {
					days[i], days[j] = days[j], days[i]
				}
			}
		}
		for _, day := range days {
			workingDays = append(workingDays, workingDaysMap[day])
		}

		// For the first month, buy on the first working day
		if i == 0 && len(workingDays) >= 1 {
			buyIdx := workingDays[0] // First working day
			signals[buyIdx] = internal.BUY
			buyCandle := candles[buyIdx]
			log.Printf("📉 BUY: %s at price %.4f (first working day of first month)", buyCandle.Time, buyCandle.Close.ToFloat64())
		}

		// Need at least 2 working days for sell signal
		if len(workingDays) >= 2 {
			sellIdx = &workingDays[len(workingDays)-2] // Second-to-last working day
			signals[*sellIdx] = internal.SELL
			sellCandle := candles[*sellIdx]
			log.Printf("📈 SELL: %s at price %.4f (first candle of second-to-last working day)", sellCandle.Time, sellCandle.Close.ToFloat64())
		}

		// Check if there's a next month for buy signal
		if i < len(months)-1 {
			nextMonth := months[i+1]
			nextIndices := monthlyCandles[nextMonth]

			// Find first working day of next month
			var firstWorkingDay *int
			for _, idx := range nextIndices {
				t := candles[idx].ToTime()
				weekday := t.Weekday()
				if weekday != time.Saturday && weekday != time.Sunday {
					firstWorkingDay = &idx
					break
				}
			}

			if firstWorkingDay != nil {
				signals[*firstWorkingDay] = internal.BUY
				buyCandle := candles[*firstWorkingDay]
				log.Printf("📉 BUY: %s at price %.4f", buyCandle.Time, buyCandle.Close.ToFloat64())
			}
		}

		_ = month // avoid unused variable warning
	}

	return signals
}

func (s *MonthlyRebalanceStrategy) DefaultConfig() internal.StrategyConfig {
	return &MonthlyRebalanceConfig{}
}

func (s *MonthlyRebalanceStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	mrConfig, ok := config.(*MonthlyRebalanceConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := mrConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < 2 {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))

	// Group candles by month
	monthlyCandles := make(map[string][]int) // "YYYY-MM" -> []indices

	for i, candle := range candles {
		t := candle.ToTime()
		monthKey := t.Format("2006-01")
		monthlyCandles[monthKey] = append(monthlyCandles[monthKey], i)
	}

	// Get all months in chronological order
	var months []string
	for month := range monthlyCandles {
		months = append(months, month)
	}

	// Sort months chronologically
	for i := 0; i < len(months)-1; i++ {
		for j := i + 1; j < len(months); j++ {
			if months[i] > months[j] {
				months[i], months[j] = months[j], months[i]
			}
		}
	}

	// Process each month to find sell day (second-to-last working day of current month)
	// and buy day (first working day of next month)
	for i, month := range months {
		indices := monthlyCandles[month]

		// Sort all candles in the month by time
		for k := 0; k < len(indices)-1; k++ {
			for j := k + 1; j < len(indices); j++ {
				if candles[indices[k]].ToTime().After(candles[indices[j]].ToTime()) {
					indices[k], indices[j] = indices[j], indices[k]
				}
			}
		}

		// Find the first candle of the second-to-last working day
		var sellIdx *int
		workingDaysMap := make(map[string]int) // "YYYY-MM-DD" -> first index of the day

		// Collect first candle index for each working day
		for _, idx := range indices {
			t := candles[idx].ToTime()
			weekday := t.Weekday()
			dayKey := t.Format("2006-01-02")
			if weekday != time.Saturday && weekday != time.Sunday {
				if _, exists := workingDaysMap[dayKey]; !exists {
					workingDaysMap[dayKey] = idx
				}
			}
		}

		// Get working days in chronological order
		var workingDays []int
		var days []string
		for day := range workingDaysMap {
			days = append(days, day)
		}
		// Sort days
		for i := 0; i < len(days)-1; i++ {
			for j := i + 1; j < len(days); j++ {
				if days[i] > days[j] {
					days[i], days[j] = days[j], days[i]
				}
			}
		}
		for _, day := range days {
			workingDays = append(workingDays, workingDaysMap[day])
		}

		// For the first month, buy on the first working day
		if i == 0 && len(workingDays) >= 1 {
			buyIdx := workingDays[0] // First working day
			signals[buyIdx] = internal.BUY
			buyCandle := candles[buyIdx]
			log.Printf("📉 BUY: %s at price %.4f (first working day of first month)", buyCandle.Time, buyCandle.Close.ToFloat64())
		}

		// Need at least 2 working days for sell signal
		if len(workingDays) >= 2 {
			sellIdx = &workingDays[len(workingDays)-2] // Second-to-last working day
			signals[*sellIdx] = internal.SELL
			sellCandle := candles[*sellIdx]
			log.Printf("📈 SELL: %s at price %.4f (first candle of second-to-last working day)", sellCandle.Time, sellCandle.Close.ToFloat64())
		}

		// Check if there's a next month for buy signal
		if i < len(months)-1 {
			nextMonth := months[i+1]
			nextIndices := monthlyCandles[nextMonth]

			// Find first working day of next month
			var firstWorkingDay *int
			for _, idx := range nextIndices {
				t := candles[idx].ToTime()
				weekday := t.Weekday()
				if weekday != time.Saturday && weekday != time.Sunday {
					firstWorkingDay = &idx
					break
				}
			}

			if firstWorkingDay != nil {
				signals[*firstWorkingDay] = internal.BUY
				buyCandle := candles[*firstWorkingDay]
				log.Printf("📉 BUY: %s at price %.4f", buyCandle.Time, buyCandle.Close.ToFloat64())
			}
		}

		_ = month // avoid unused variable warning
	}

	return signals
}

func (s *MonthlyRebalanceStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	// This strategy doesn't have parameters to optimize, but return best config
	return &MonthlyRebalanceConfig{}
}

func init() {
	internal.RegisterStrategy("monthly_rebalance", &MonthlyRebalanceStrategy{})
}
