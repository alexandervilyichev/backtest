// strategies/macd.go

// MACD (Moving Average Convergence Divergence) Strategy
//
// Описание стратегии:
// MACD - это трендовый momentum индикатор, который показывает связь между двумя скользящими средними цены.
// Состоит из MACD линии (разность быстрой и медленной EMA), сигнальной линии (EMA от MACD) и гистограммы.
//
// Как работает:
// - Рассчитывается быстрая EMA (обычно 12 периодов) и медленная EMA (обычно 26 периодов)
// - MACD линия = быстрая EMA - медленная EMA
// - Сигнальная линия = EMA от MACD линии (обычно 9 периодов)
// - Покупка: когда MACD пересекает сигнальную линию снизу вверх (бычий crossover)
// - Продажа: когда MACD пересекает сигнальную линию сверху вниз (медвежий crossover)
//
// Параметры:
// - MACDFastPeriod: период быстрой EMA (обычно 12)
// - MACDSlowPeriod: период медленной EMA (обычно 26)
// - MACDSignalPeriod: период сигнальной линии (обычно 9)
//
// Сильные стороны:
// - Хорошо определяет изменения тренда и momentum
// - Классический и проверенный индикатор
// - Учитывает как направление, так и скорость движения цены
// - Гистограмма помогает визуально оценить силу сигнала
//
// Слабые стороны:
// - Может давать ложные сигналы в боковых рынках (whipsaws)
// - Запаздывает по сравнению с более быстрыми индикаторами
// - Чувствителен к выбору периодов EMA
// - В очень волатильных условиях может генерировать много шума
//
// Лучшие условия для применения:
// - Трендовые рынки с четкими направлениями
// - Средне- и долгосрочная торговля
// - В сочетании с другими индикаторами подтверждения
// - На активах с хорошей трендовой характеристикой

package momentum

import (
	"bt/internal"
	"errors"
	"fmt"
)

type MACDConfig struct {
	FastPeriod   int `json:"fast_period"`
	SlowPeriod   int `json:"slow_period"`
	SignalPeriod int `json:"signal_period"`
}

func (c *MACDConfig) Validate() error {
	if c.FastPeriod <= 0 {
		return errors.New("fast period must be positive")
	}
	if c.SlowPeriod <= 0 {
		return errors.New("slow period must be positive")
	}
	if c.SignalPeriod <= 0 {
		return errors.New("signal period must be positive")
	}
	if c.FastPeriod >= c.SlowPeriod {
		return errors.New("fast period must be less than slow period")
	}
	return nil
}

func (c *MACDConfig) DefaultConfigString() string {
	return fmt.Sprintf("MACD(fast=%d, slow=%d, signal=%d)",
		c.FastPeriod, c.SlowPeriod, c.SignalPeriod)
}

type MACDStrategy struct{}

func (s *MACDStrategy) Name() string {
	return "macd"
}

func (s *MACDStrategy) DefaultConfig() internal.StrategyConfig {
	return &MACDConfig{
		FastPeriod:   12,
		SlowPeriod:   26,
		SignalPeriod: 9,
	}
}

func (s *MACDStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	macdConfig, ok := config.(*MACDConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := macdConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	macdLine, signalLine, _ := internal.CalculateMACDWithSignal(candles, macdConfig.FastPeriod, macdConfig.SlowPeriod, macdConfig.SignalPeriod)
	if macdLine == nil || signalLine == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := macdConfig.SlowPeriod + macdConfig.SignalPeriod - 1; i < len(candles); i++ {
		macd := macdLine[i]
		signal := signalLine[i]
		macdPrev := macdLine[i-1]
		signalPrev := signalLine[i-1]

		if !inPosition {
			// Buy when MACD crosses above signal line
			if macdPrev <= signalPrev && macd > signal {
				signals[i] = internal.BUY
				inPosition = true
				continue
			}
		} else {
			// Sell when MACD crosses below signal line
			if macdPrev >= signalPrev && macd < signal {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *MACDStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := &MACDConfig{
		FastPeriod:   12,
		SlowPeriod:   26,
		SignalPeriod: 9,
	}
	bestProfit := -1.0

	// Grid search по периодам
	for fast := 8; fast <= 16; fast += 2 {
		for slow := 20; slow <= 32; slow += 4 {
			for signal := 6; signal <= 12; signal += 2 {
				if fast >= slow {
					continue // fast period must be less than slow period
				}

				config := &MACDConfig{
					FastPeriod:   fast,
					SlowPeriod:   slow,
					SignalPeriod: signal,
				}
				if config.Validate() != nil {
					continue
				}

				signals := s.GenerateSignalsWithConfig(candles, config)
				result := internal.Backtest(candles, signals, 0.01) // 0.01 units проскальзывание
				if result.TotalProfit > bestProfit {
					bestProfit = result.TotalProfit
					bestConfig = config
				}
			}
		}
	}

	fmt.Printf("Лучшие параметры SOLID MACD: fast=%d, slow=%d, signal=%d, профит=%.4f\n",
		bestConfig.FastPeriod, bestConfig.SlowPeriod, bestConfig.SignalPeriod, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("macd", &MACDStrategy{})
}
