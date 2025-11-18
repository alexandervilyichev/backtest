// strategies/rsi_oscillator.go

// RSI Oscillator Strategy
//
// Описание стратегии:
// Стратегия использует индекс относительной силы (RSI) для определения перекупленных и перепроданных уровней рынка.
// RSI измеряет скорость и изменение ценовых движений на шкале от 0 до 100.
//
// Как работает:
// - Рассчитывается RSI с заданным периодом (по умолчанию 14)
// - Покупка: когда RSI опускается ниже уровня перепроданности (по умолчанию 30)
// - Продажа: когда RSI поднимается выше уровня перекупленности (по умолчанию 70)
// - Стратегия удерживает позицию до противоположного сигнала
//
// Параметры:
// - RsiPeriod: период расчета RSI (обычно 14)
// - RsiBuyThreshold: уровень перепроданности для покупки (обычно 30)
// - RsiSellThreshold: уровень перекупленности для продажи (обычно 70)
//
// Сильные стороны:
// - Простота реализации и понимания
// - Хорошо работает в oscillating рынках
// - RSI является стандартным и проверенным индикатором
// - Хорошо фильтрует шум на рынке
//
// Слабые стороны:
// - Может давать ложные сигналы в сильных трендах
// - Не учитывает направление тренда
// - В боковых движениях может генерировать много сделок
// - Задержка сигнала из-за smoothing
//
// Лучшие условия для применения:
// - Боковые/осциллирующие рынки
// - Среднесрочная торговля
// - В сочетании с трендовыми фильтрами
// - На волатильных активах

package oscillators

import (
	"bt/internal"
	"errors"
	"fmt"

	"github.com/samber/lo"
)

type RSIConfig struct {
	Period        int     `json:"period"`
	BuyThreshold  float64 `json:"buy_threshold"`
	SellThreshold float64 `json:"sell_threshold"`
}

func (c *RSIConfig) Validate() error {
	if c.Period <= 0 {
		return errors.New("period must be positive")
	}
	if c.BuyThreshold >= c.SellThreshold {
		return errors.New("buy threshold must be less than sell threshold")
	}
	return nil
}

func (c *RSIConfig) DefaultConfigString() string {
	return fmt.Sprintf("RSI(period=%d, buy_thresh=%.1f, sell_thresh=%.1f)",
		c.Period, c.BuyThreshold, c.SellThreshold)
}

type RSIOscillatorStrategy struct {
	internal.BaseConfig
	internal.BaseStrategy
}

func (s *RSIOscillatorStrategy) Name() string {
	return "rsi_oscillator"
}

func (s *RSIOscillatorStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	rsiConfig, ok := config.(*RSIConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := rsiConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	rsiValues := internal.CalculateRSICommon(candles, rsiConfig.Period)
	if rsiValues == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := rsiConfig.Period; i < len(candles); i++ {
		rsi := rsiValues[i]

		if !inPosition && rsi < rsiConfig.BuyThreshold {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		if inPosition && rsi > rsiConfig.SellThreshold {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *RSIOscillatorStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {

	configs := lo.CrossJoinBy3(
		lo.RangeWithSteps[int](10, 20, 1),
		lo.RangeWithSteps[float64](10, 35, 1),
		lo.RangeWithSteps[float64](65, 86, 1),
		func(period int, buyThresh float64, sellThresh float64) internal.StrategyConfig {
			return &RSIConfig{
				Period:        period,
				BuyThreshold:  buyThresh,
				SellThreshold: sellThresh,
			}
		})

	max := s.ProcessConfigs(s, candles, configs)

	bestConfig := max.A.(*RSIConfig)
	bestProfit := max.B

	fmt.Printf("Лучшие параметры RSI: период=%d, покупка=%.1f, продажа=%.1f, профит=%.4f\n",
		bestConfig.Period, bestConfig.BuyThreshold, bestConfig.SellThreshold, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("rsi_oscillator", &RSIOscillatorStrategy{
		BaseConfig: internal.BaseConfig{
			Config: &RSIConfig{
				Period:        14,
				BuyThreshold:  30.0,
				SellThreshold: 70.0,
			},
		},
	})
}
