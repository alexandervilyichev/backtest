// strategies/ma_ema_correlation.go

// MA-EMA Correlation Strategy
//
// Описание стратегии:
// Стратегия основана на анализе корреляции между простой скользящей средней (MA/SMA)
// и экспоненциальной скользящей средней (EMA). Корреляция рассчитывается в скользящем окне,
// и сигналы генерируются на основе пороговых значений корреляции.
//
// Как работает:
// - Рассчитывается MA и EMA на ценах закрытия
// - Вычисляется скользящая корреляция между MA и EMA за определенный период
// - BUY: когда корреляция превышает верхний порог (высокая положительная корреляция)
// - SELL: когда корреляция опускается ниже нижнего порога (отрицательная корреляция)
// - HOLD: в остальных случаях
//
// Параметры:
// - MA период: период для простой скользящей средней
// - EMA период: период для экспоненциальной скользящей средней
// - Lookback период: окно для расчета скользящей корреляции
// - Threshold: порог корреляции для генерации сигналов
//
// Сильные стороны:
// - Учитывает взаимосвязь между разными типами скользящих средних
// - Может выявлять периоды сильного тренда или неопределенности
// - Гибкая настройка параметров
//
// Слабые стороны:
// - Требует тщательной оптимизации параметров
// - Может быть чувствительна к выбору периодов MA и EMA
// - Корреляция не всегда является надежным индикатором направления
//
// Лучшие условия для применения:
// - Рынки с четкими трендами
// - В комбинации с другими индикаторами
// - Для фильтрации сигналов других стратегий

package moving_averages

import (
	"bt/internal"
	"errors"
	"fmt"
)

type MAEmaCorrelationConfig struct {
	MAPeriod  int     `json:"ma_period"`
	EMAPeriod int     `json:"ema_period"`
	Lookback  int     `json:"lookback"`
	Threshold float64 `json:"threshold"`
}

func (c *MAEmaCorrelationConfig) Validate() error {
	if c.MAPeriod <= 0 {
		return errors.New("ma period must be positive")
	}
	if c.EMAPeriod <= 0 {
		return errors.New("ema period must be positive")
	}
	if c.Lookback <= 0 {
		return errors.New("lookback must be positive")
	}
	if c.Threshold <= 0 || c.Threshold >= 1.0 {
		return errors.New("threshold must be between 0 and 1.0")
	}
	return nil
}

func (c *MAEmaCorrelationConfig) DefaultConfigString() string {
	return fmt.Sprintf("MAEmaCorr(ma=%d, ema=%d, lookbk=%d, thresh=%.2f)",
		c.MAPeriod, c.EMAPeriod, c.Lookback, c.Threshold)
}

type MaEmaCorrelationStrategy struct{}

func (s *MaEmaCorrelationStrategy) Name() string {
	return "ma_ema_correlation"
}

func (s *MaEmaCorrelationStrategy) DefaultConfig() internal.StrategyConfig {
	return &MAEmaCorrelationConfig{
		MAPeriod:  20,
		EMAPeriod: 20,
		Lookback:  10,
		Threshold: 0.7,
	}
}

func (s *MaEmaCorrelationStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	maEmaConfig, ok := config.(*MAEmaCorrelationConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := maEmaConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	// Получаем цены закрытия
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// Рассчитываем MA и EMA
	ma := internal.CalculateSMACommonForValues(prices, maEmaConfig.MAPeriod)
	ema := internal.CalculateEMAForValues(prices, maEmaConfig.EMAPeriod)

	if ma == nil || ema == nil {
		return make([]internal.SignalType, len(candles))
	}

	// Рассчитываем скользящую корреляцию между MA и EMA
	correlations := internal.CalculateRollingCorrelation(ma, ema, maEmaConfig.Lookback)
	if correlations == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	// Начинаем анализ после достаточного количества данных
	startIndex := maEmaConfig.MAPeriod + maEmaConfig.EMAPeriod + maEmaConfig.Lookback - 3 // приблизительно

	for i := startIndex; i < len(candles); i++ {
		corr := correlations[i]

		// BUY: высокая положительная корреляция
		if !inPosition && corr > maEmaConfig.Threshold {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		// SELL: отрицательная корреляция
		if inPosition && corr < -maEmaConfig.Threshold {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *MaEmaCorrelationStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := &MAEmaCorrelationConfig{
		MAPeriod:  20,
		EMAPeriod: 20,
		Lookback:  10,
		Threshold: 0.7,
	}
	bestProfit := -1.0

	// Оптимизируем параметры
	for maPeriod := 10; maPeriod <= 30; maPeriod += 5 {
		for emaPeriod := 10; emaPeriod <= 30; emaPeriod += 5 {
			for lookback := 5; lookback <= 15; lookback += 5 {
				for threshold := 0.5; threshold <= 0.9; threshold += 0.1 {
					config := &MAEmaCorrelationConfig{
						MAPeriod:  maPeriod,
						EMAPeriod: emaPeriod,
						Lookback:  lookback,
						Threshold: threshold,
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
	}

	fmt.Printf("Лучшие параметры MA-EMA: ma=%d, ema=%d, lookback=%d, threshold=%.2f, профит=%.4f\n",
		bestConfig.MAPeriod, bestConfig.EMAPeriod, bestConfig.Lookback, bestConfig.Threshold, bestProfit)

	return bestConfig
}

func init() {
	internal.RegisterStrategy("ma_ema_correlation", &MaEmaCorrelationStrategy{})
}
