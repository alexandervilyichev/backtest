// strategies/envelopes_strategy.go

// Envelopes Strategy
//
// Envelopes Strategy
//
// Описание стратегии:
// Envelopes - канал вокруг скользящей средней с фиксированным процентом отклонения.
//
// Как работает:
// - Рассчитывается простая скользящая средняя (SMA) за заданный период
// - Верхняя полоса = SMA * (1 + процент)
// - Нижняя полоса = SMA * (1 - процент)
// - Покупка: когда цена закрывается выше верхней полосы (breakout above)
// - Продажа: когда цена закрывается ниже нижней полосы (breakout below)
//
// Параметры:
// - Period: период расчета SMA (обычно 20)
// - Percentage: процент отклонения (обычно 0.02 - 0.05)
//
// Сильные стороны:
// - Простота реализации и интерпретации
// - Хорошо определяет breakout движения
// - Учитывает абсолютные уровни цен
// - Стабильная ширина канала
//
// Слабые стороны:
// - В боковых рынках генерирует ложные сигналы
// - Фиксированная ширина не адаптируется к волатильности
// - Запаздывает в сильных трендах
// - Чувствителен к выбору параметров
//
// Лучшие условия для применения:
// - Трендовые рынки для подтверждения breakout
// - Комбинация с другими индикаторами объема/волатильности
// - Акции с достаточной ликвидностью
// - Для поиска сильных направленных движений

package volatility

import (
	"bt/internal"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type EnvelopesConfig struct {
	Period     int     `json:"period"`
	Percentage float64 `json:"percentage"`
}

func (c *EnvelopesConfig) Validate() error {
	if c.Period <= 0 {
		return errors.New("period must be positive")
	}
	if c.Percentage <= 0 || c.Percentage >= 0.5 {
		return errors.New("percentage must be between 0 and 0.5")
	}
	return nil
}

func (c *EnvelopesConfig) DefaultConfigString() string {
	return fmt.Sprintf("Envelopes(period=%d, pct=%.3f)",
		c.Period, c.Percentage)
}

type EnvelopesStrategy struct{ internal.BaseConfig }

func (s *EnvelopesStrategy) Name() string {
	return "envelopes"
}

// calculateEnvelopes вычисляет верхнюю, среднюю и нижнюю полосы Envelopes
func calculateEnvelopes(candles []internal.Candle, period int, percentage float64) (upper []float64, middle []float64, lower []float64) {
	middle = internal.CalculateSMACommon(candles, period)
	if middle == nil {
		return nil, nil, nil
	}

	length := len(candles)
	upper = make([]float64, length)
	lower = make([]float64, length)

	for i := 0; i < length; i++ {
		if middle[i] == 0 {
			upper[i] = 0
			lower[i] = 0
		} else {
			upper[i] = middle[i] * (1 + percentage)
			lower[i] = middle[i] * (1 - percentage)
		}
	}

	return upper, middle, lower
}

func (s *EnvelopesStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	envConfig, ok := config.(*EnvelopesConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := envConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	upper, _, lower := calculateEnvelopes(candles, envConfig.Period, envConfig.Percentage)
	if upper == nil || lower == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := envConfig.Period; i < len(candles); i++ {
		currentPrice := candles[i].Close.ToFloat64()
		currentLower := lower[i]
		currentUpper := upper[i]

		// BUY: breakout above upper envelope
		if !inPosition && currentPrice > currentUpper {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		// SELL: breakout below lower envelope
		if inPosition && currentPrice < currentLower {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *EnvelopesStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := s.DefaultConfig().(*EnvelopesConfig)
	bestProfit := -1.0

	var results []internal.GridSearchResult
	// Grid search по параметрам
	for period := 5; period <= 90; period += 5 {
		for percentage := 0.01; percentage <= 0.04; percentage += 0.0005 {
			config := &EnvelopesConfig{
				Period:     period,
				Percentage: percentage,
			}
			if config.Validate() != nil {
				continue
			}

			signals := s.GenerateSignalsWithConfig(candles, config)
			result := internal.Backtest(candles, signals, s.GetSlippage())

			// Collect results for mesh format
			results = append(results, internal.GridSearchResult{
				X:      period,
				Y:      int(10000 * percentage),
				Profit: result.TotalProfit,
			})

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

	fmt.Printf("Лучшие параметры Envelopes: period=%d, percentage=%.3f, профит=%.4f\n",
		bestConfig.Period, bestConfig.Percentage, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("envelopes", &EnvelopesStrategy{
		BaseConfig: internal.BaseConfig{
			Config: &EnvelopesConfig{
				Period:     20,
				Percentage: 0.02,
			},
		},
	})
}
