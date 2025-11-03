// strategies/volume_breakout.go

// Volume Breakout Strategy
//
// Описание стратегии:
// Стратегия основана на анализе объема торгов как подтверждения силы ценового движения.
// Высокий объем при ценовом движении указывает на сильный интерес участников рынка,
// что может сигнализировать о продолжении движения. Стратегия ищет breakout моменты
// с повышенным объемом как сигналы для входа в позицию.
//
// Как работает:
// - Рассчитывается средний объем за последние 3 свечи
// - Покупка: зеленая свеча (close > open) с объемом выше среднего в volumeMultiplier раз
// - Продажа: красная свеча (close < open) или объем в 2*volumeMultiplier раз выше среднего
// - Объем подтверждает силу движения: высокий объем = сильный интерес = продолжение движения
//
// Параметры:
// - VolumeMultiplier: множитель для определения высокого объема (обычно 1.2-2.0)
//   Чем выше множитель, тем более значимый объем требуется для сигнала
//
// Сильные стороны:
// - Учитывает рыночную активность через объем
// - Хорошо фильтрует слабые движения
// - Логичная идея: объем подтверждает силу тренда
// - Может быть эффективна в различных рыночных условиях
//
// Слабые стороны:
// - Зависит от качества данных объема
// - Может пропускать движения без высокого объема
// - В волатильных условиях может генерировать ложные сигналы
// - Требует достаточной истории для расчета среднего объема
//
// Лучшие условия для применения:
// - Рынки с хорошей ликвидностью и объемом данных
// - В сочетании с ценовым анализом
// - На активах с понятной динамикой объема
// - Среднесрочная торговля с подтверждением тренда

package volume

import (
	"bt/internal"
	"errors"
	"fmt"
)

type VolumeBreakoutConfig struct {
	Multiplier float64 `json:"multiplier"`
}

func (c *VolumeBreakoutConfig) Validate() error {
	if c.Multiplier <= 1.0 {
		return errors.New("multiplier must be greater than 1.0")
	}
	return nil
}

func (c *VolumeBreakoutConfig) DefaultConfigString() string {
	return fmt.Sprintf("VolumeBreakout(mult=%.2f)",
		c.Multiplier)
}

type VolumeBreakoutStrategy struct{}

func (s *VolumeBreakoutStrategy) Name() string {
	return "volume_breakout"
}

func (s *VolumeBreakoutStrategy) DefaultConfig() internal.StrategyConfig {
	return &VolumeBreakoutConfig{
		Multiplier: 1.5,
	}
}

func (s *VolumeBreakoutStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	vbConfig, ok := config.(*VolumeBreakoutConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := vbConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	for i := range candles {
		if i < 3 {
			signals[i] = internal.HOLD
			continue
		}

		var totalVolume float64
		for j := i - 3; j < i; j++ {
			vol := candles[j].VolumeFloat // используем предвычисленное значение
			totalVolume += vol
		}
		avgVolume := totalVolume / 3.0

		currentVol := candles[i].VolumeFloat // используем предвычисленное значение

		openPrice := candles[i].Open.ToFloat64()
		closePrice := candles[i].Close.ToFloat64()

		// BUY: зеленая свеча с высоким объемом
		if !inPosition && closePrice > openPrice && currentVol > avgVolume*vbConfig.Multiplier {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		// SELL: красная свеча или достижение цели
		if inPosition && (closePrice < openPrice || currentVol > avgVolume*vbConfig.Multiplier*2) {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *VolumeBreakoutStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := &VolumeBreakoutConfig{
		Multiplier: 1.5,
	}
	bestProfit := -1.0

	for mult := 1.0; mult <= 3.0; mult += 0.1 {
		config := &VolumeBreakoutConfig{
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

	fmt.Printf("Лучшие параметры Volume Breakout: multiplier=%.2f, профит=%.4f\n",
		bestConfig.Multiplier, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("volume_breakout", &VolumeBreakoutStrategy{})
}
