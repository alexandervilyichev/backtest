// strategies/ma_channel.go

// Moving Average Channel Strategy
//
// Описание стратегии:
// Стратегия использует коридор, построенный на основе двух скользящих средних с разными периодами.
// Создается канал, где верхняя граница находится выше медленной SMA, а нижняя - ниже.
// Стратегия торгует пробои этих границ как сигналы сильного momentum.
//
// Как работает:
// - Рассчитывается быстрая SMA (fastPeriod) и медленная SMA (slowPeriod)
// - Разница между SMA умножается на multiplier для создания ширины канала
// - Верхний канал = медленная SMA + (быстрая SMA - медленная SMA) × multiplier
// - Нижний канал = медленная SMA - (быстрая SMA - медленная SMA) × multiplier
// - Покупка: при пробое цены выше верхнего канала (бычий breakout)
// - Продажа: при пробое цены ниже нижнего канала (медвежий breakout)
//
// Параметры:
// - MAChannelFastPeriod: период быстрой SMA (обычно 10)
// - MAChannelSlowPeriod: период медленной SMA (обычно 20)
// - MAChannelMultiplier: множитель ширины канала (обычно 0.5-2.0)
//
// Сильные стороны:
// - Ловит сильные движения и пробои
// - Хорошо работает в трендовых рынках
// - Учитывает momentum через пробои канала
// - Может быть адаптирована под разные рыночные условия
//
// Слабые стороны:
// - Может давать много ложных сигналов в боковых рынках
// - Чувствительна к выбору ширины канала (multiplier)
// - В волатильных условиях может генерировать слишком много сигналов
// - Требует хорошего money management из-за потенциальных убытков
//
// Лучшие условия для применения:
// - Трендовые рынки с сильными движениями
// - Волатильные активы с четкими пробоями
// - Средне- и долгосрочная торговля
// - В сочетании с фильтрами объема или волатильности

package momentum

import (
	"bt/internal"
	"errors"
	"fmt"
)

type MAChannelConfig struct {
	FastPeriod int     `json:"fast_period"`
	SlowPeriod int     `json:"slow_period"`
	Multiplier float64 `json:"multiplier"`
}

func (c *MAChannelConfig) Validate() error {
	if c.FastPeriod <= 0 {
		return errors.New("fast period must be positive")
	}
	if c.SlowPeriod <= 0 {
		return errors.New("slow period must be positive")
	}
	if c.Multiplier <= 0 {
		return errors.New("multiplier must be positive")
	}
	if c.FastPeriod >= c.SlowPeriod {
		return errors.New("fast period must be less than slow period")
	}
	return nil
}

func (c *MAChannelConfig) DefaultConfigString() string {
	return fmt.Sprintf("MAChannel(fast=%d, slow=%d, mult=%.2f)",
		c.FastPeriod, c.SlowPeriod, c.Multiplier)
}

type MAChannelStrategy struct{}

func (s *MAChannelStrategy) Name() string {
	return "ma_channel"
}

func (s *MAChannelStrategy) DefaultConfig() internal.StrategyConfig {
	return &MAChannelConfig{
		FastPeriod: 10,
		SlowPeriod: 20,
		Multiplier: 1.0,
	}
}

func (s *MAChannelStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	maConfig, ok := config.(*MAChannelConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := maConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	upperChannel, lowerChannel := internal.CalculateMAChannel(candles, maConfig.FastPeriod, maConfig.SlowPeriod, maConfig.Multiplier)
	if upperChannel == nil || lowerChannel == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := maConfig.SlowPeriod; i < len(candles); i++ {
		closePrice := candles[i].Close.ToFloat64()
		upper := upperChannel[i]
		lower := lowerChannel[i]

		if upper == 0 || lower == 0 {
			signals[i] = internal.HOLD
			continue
		}

		if !inPosition {
			// Buy when price breaks above upper channel
			if closePrice > upper {
				signals[i] = internal.BUY
				inPosition = true
				continue
			}
		} else {
			// Sell when price breaks below lower channel
			if closePrice < lower {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *MAChannelStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := &MAChannelConfig{
		FastPeriod: 10,
		SlowPeriod: 20,
		Multiplier: 1.0,
	}
	bestProfit := -1.0

	// Grid search по параметрам
	for fast := 5; fast <= 15; fast += 2 {
		for slow := 15; slow <= 30; slow += 5 {
			for mult := 0.5; mult <= 2.0; mult += 0.25 {
				if fast >= slow {
					continue // fast period must be less than slow period
				}

				config := &MAChannelConfig{
					FastPeriod: fast,
					SlowPeriod: slow,
					Multiplier: mult,
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

	fmt.Printf("Лучшие параметры SOLID MA Channel: fast=%d, slow=%d, multiplier=%.2f, профит=%.4f\n",
		bestConfig.FastPeriod, bestConfig.SlowPeriod, bestConfig.Multiplier, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("ma_channel", &MAChannelStrategy{})
}
