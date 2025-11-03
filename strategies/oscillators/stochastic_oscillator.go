// strategies/stochastic_oscillator.go

// Stochastic Oscillator Strategy
//
// Описание стратегии:
// Стратегия использует стохастический осциллятор - momentum индикатор, который сравнивает closing price
// с диапазоном цен за определенный период. Состоит из двух линий: %K (быстрая) и %D (сигнальная).
//
// Как работает:
// - Рассчитывается %K: 100 * (close - lowest_low) / (highest_high - lowest_low)
// - Рассчитывается %D: SMA от %K за сигнальный период
// - Покупка: когда %K пересекает %D снизу вверх, и обе линии ниже уровня перепроданности
// - Продажа: когда %K пересекает %D сверху вниз, и обе линии выше уровня перекупленности
//
// Параметры:
// - StochasticKPeriod: период для расчета %K (обычно 14)
// - StochasticDPeriod: период smoothing для %D (обычно 3)
// - StochasticBuyLevel: уровень перепроданности для покупки (обычно 20)
// - StochasticSellLevel: уровень перекупленности для продажи (обычно 80)
//
// Сильные стороны:
// - Хорошо определяет перекупленность/перепроданность
// - Учитывает momentum и скорость движения цены
// - Реагирует быстрее RSI на изменения
// - Эффективен в ranging рынках
//
// Слабые стороны:
// - Может давать много ложных сигналов в трендовых рынках
// - Чувствителен к выбору периодов
// - В волатильных условиях может генерировать whipsaws
// - Не учитывает общий тренд рынка
//
// Лучшие условия для применения:
// - Боковые/осциллирующие рынки
// - Кратко- и среднесрочная торговля
// - В сочетании с трендовыми индикаторами
// - На активах с четкими циклами

package oscillators

import (
	"bt/internal"
	"errors"
	"fmt"
)

type StochasticConfig struct {
	KPeriod   int     `json:"k_period"`
	DPeriod   int     `json:"d_period"`
	BuyLevel  float64 `json:"buy_level"`
	SellLevel float64 `json:"sell_level"`
}

func (c *StochasticConfig) Validate() error {
	if c.KPeriod <= 0 {
		return errors.New("k period must be positive")
	}
	if c.DPeriod <= 0 {
		return errors.New("d period must be positive")
	}
	if c.BuyLevel >= c.SellLevel {
		return errors.New("buy level must be less than sell level")
	}
	return nil
}

func (c *StochasticConfig) DefaultConfigString() string {
	return fmt.Sprintf("Stochastic(k=%d, d=%d, buy=%.1f, sell=%.1f)",
		c.KPeriod, c.DPeriod, c.BuyLevel, c.SellLevel)
}

type StochasticOscillatorStrategy struct{}

func (s *StochasticOscillatorStrategy) Name() string {
	return "stochastic_oscillator"
}

func (s *StochasticOscillatorStrategy) DefaultConfig() internal.StrategyConfig {
	return &StochasticConfig{
		KPeriod:   14,
		DPeriod:   3,
		BuyLevel:  20.0,
		SellLevel: 80.0,
	}
}

func (s *StochasticOscillatorStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	stochConfig, ok := config.(*StochasticConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := stochConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	kValues, dValues := internal.CalculateStochastic(candles, stochConfig.KPeriod, stochConfig.DPeriod)
	if kValues == nil || dValues == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := stochConfig.KPeriod + stochConfig.DPeriod - 1; i < len(candles); i++ {
		k := kValues[i]
		d := dValues[i]
		kPrev := kValues[i-1]
		dPrev := dValues[i-1]

		if !inPosition {
			// Buy when %K crosses above %D and both are below buy level
			if kPrev <= dPrev && k > d && k < stochConfig.BuyLevel && d < stochConfig.BuyLevel {
				signals[i] = internal.BUY
				inPosition = true
				continue
			}
		} else {
			// Sell when %K crosses below %D and both are above sell level
			if kPrev >= dPrev && k < d && k > stochConfig.SellLevel && d > stochConfig.SellLevel {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *StochasticOscillatorStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := &StochasticConfig{
		KPeriod:   14,
		DPeriod:   3,
		BuyLevel:  20.0,
		SellLevel: 80.0,
	}
	bestProfit := -1.0

	// Grid search по параметрам
	for kPeriod := 10; kPeriod <= 20; kPeriod += 2 {
		for dPeriod := 2; dPeriod <= 5; dPeriod++ {
			for buyLevel := 15.0; buyLevel <= 30.0; buyLevel += 5 {
				for sellLevel := 70.0; sellLevel <= 85.0; sellLevel += 5 {
					config := &StochasticConfig{
						KPeriod:   kPeriod,
						DPeriod:   dPeriod,
						BuyLevel:  buyLevel,
						SellLevel: sellLevel,
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
	}

	fmt.Printf("Лучшие параметры Stochastic: k=%d, d=%d, buy=%.1f, sell=%.1f, профит=%.4f\n",
		bestConfig.KPeriod, bestConfig.DPeriod, bestConfig.BuyLevel, bestConfig.SellLevel, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("stochastic_oscillator", &StochasticOscillatorStrategy{})
}
