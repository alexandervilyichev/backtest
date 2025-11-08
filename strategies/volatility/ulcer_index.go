// strategies/volatility/ulcer_index.go

// Ulcer Index Strategy
//
// Описание стратегии:
// Ulcer Index (UI) измеряет downside risk (риск снижения). Индекс растет по мере того,
// как цена уходит все дальше от недавнего максимума, и падает по мере достижения новых максимумов.
//
// Как рассчитывается:
// 1. Определяем максимальную цену за период (running maximum)
// 2. Для каждой свечи рассчитываем drawdown: (текущая цена - максимум) / максимум * 100%
// 3. Возводим drawdown в квадрат для каждой свечи
// 4. Берем среднее значение квадратов drawdown'ов
// 5. Берем квадратный корень от среднего
//
// Формула:
// UI = √(Σ((Close - MaxHigh) / MaxHigh)² / n)
//
// Параметры:
// - Period: период расчета (обычно 14 дней)
//
// Сигналы стратегии:
// - BUY: когда UI падает ниже определенного порога (риск снижается)
// - SELL: когда UI растет выше определенного порога (риск увеличивается)
// - HOLD: в остальных случаях
//
// Сильные стороны:
// - Хорошо измеряет downside volatility
// - Чувствителен к просадкам от максимумов
// - Помогает определить периоды повышенного риска
// - Полезен для risk-adjusted returns
//
// Слабые стороны:
// - Не является торговым индикатором сам по себе
// - Требует дополнительных фильтров для генерации сигналов
// - Зависит от правильного выбора периода расчета
//
// Лучшие условия для применения:
// - Для оценки риска портфеля
// - В комбинации с другими индикаторами для фильтрации сигналов
// - Для определения оптимальных точек входа/выхода в периоды волатильности

package volatility

import (
	"bt/internal"
	"errors"
	"fmt"
	"math"
)

type UlcerIndexConfig struct {
	Period        int     `json:"period"`
	BuyThreshold  float64 `json:"buyThreshold"`  // порог для BUY сигнала
	SellThreshold float64 `json:"sellThreshold"` // порог для SELL сигнала
}

func (c *UlcerIndexConfig) Validate() error {
	if c.Period <= 0 {
		return errors.New("period must be positive")
	}
	if c.BuyThreshold <= 0 {
		return errors.New("buyThreshold must be positive")
	}
	if c.SellThreshold <= c.BuyThreshold {
		return errors.New("sellThreshold must be greater than buyThreshold")
	}
	return nil
}

func (c *UlcerIndexConfig) DefaultConfigString() string {
	return fmt.Sprintf("UlcerIndex(period=%d, buy=%.4f, sell=%.4f)",
		c.Period, c.BuyThreshold, c.SellThreshold)
}

type UlcerIndexStrategy struct{}

func (s *UlcerIndexStrategy) Name() string {
	return "ulcer_index"
}

// calculateUlcerIndex рассчитывает Ulcer Index для заданного периода
func calculateUlcerIndex(candles []internal.Candle, period int) []float64 {
	if len(candles) < period {
		return nil
	}

	ulcerIndex := make([]float64, len(candles))

	// Первые period-1 значений не определены
	for i := 0; i < period-1; i++ {
		ulcerIndex[i] = 0
	}

	// Для каждого окна рассчитываем Ulcer Index
	for i := period - 1; i < len(candles); i++ {
		// Находим максимум в текущем окне
		maxHigh := candles[i-period+1].High.ToFloat64()
		for j := i - period + 2; j <= i; j++ {
			if candles[j].High.ToFloat64() > maxHigh {
				maxHigh = candles[j].High.ToFloat64()
			}
		}

		// Если максимум равен 0, пропускаем расчет
		if maxHigh == 0 {
			ulcerIndex[i] = 0
			continue
		}

		// Рассчитываем сумму квадратов drawdown'ов
		var sumSquaredDrawdown float64
		for j := i - period + 1; j <= i; j++ {
			currentPrice := candles[j].Close.ToFloat64()
			drawdown := (currentPrice - maxHigh) / maxHigh
			sumSquaredDrawdown += drawdown * drawdown
		}

		// Ulcer Index = квадратный корень от среднего квадрата drawdown'а
		ulcerIndex[i] = math.Sqrt(sumSquaredDrawdown / float64(period))
	}

	return ulcerIndex
}

func (s *UlcerIndexStrategy) DefaultConfig() internal.StrategyConfig {
	return &UlcerIndexConfig{
		Period:        14,
		BuyThreshold:  0.05, // BUY когда UI < 5%
		SellThreshold: 0.15, // SELL когда UI > 15%
	}
}

func (s *UlcerIndexStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	uiConfig, ok := config.(*UlcerIndexConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := uiConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	ulcerIndex := calculateUlcerIndex(candles, uiConfig.Period)
	if ulcerIndex == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := uiConfig.Period; i < len(candles); i++ {
		currentUI := ulcerIndex[i]

		// BUY сигнал: когда Ulcer Index падает ниже порога (риск снижается)
		if !inPosition && currentUI < uiConfig.BuyThreshold {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		// SELL сигнал: когда Ulcer Index растет выше порога (риск увеличивается)
		if inPosition && currentUI > uiConfig.SellThreshold {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *UlcerIndexStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := &UlcerIndexConfig{
		Period:        14,
		BuyThreshold:  0.05,
		SellThreshold: 0.15,
	}
	bestProfit := -1.0

	// Grid search по параметрам
	for period := 300; period <= 400; period += 10 {
		for buyThreshold := 0.02; buyThreshold <= 0.03; buyThreshold += 0.002 {
			for sellThreshold := buyThreshold + 0.01; sellThreshold <= 0.07; sellThreshold += 0.002 {
				config := &UlcerIndexConfig{
					Period:        period,
					BuyThreshold:  buyThreshold,
					SellThreshold: sellThreshold,
				}
				if config.Validate() != nil {
					continue
				}

				signals := s.GenerateSignalsWithConfig(candles, config)
				result := internal.Backtest(candles, signals, 0.01) // 0.01 units проскальзывание
				if result.TotalProfit >= bestProfit {
					bestProfit = result.TotalProfit
					bestConfig = config
				}
			}
		}
	}

	fmt.Printf("Лучшие параметры Ulcer Index: period=%d, buy=%.4f, sell=%.4f, профит=%.4f\n",
		bestConfig.Period, bestConfig.BuyThreshold, bestConfig.SellThreshold, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("ulcer_index", &UlcerIndexStrategy{})
}
