// strategies/macd.go

// Улучшенная MACD (Moving Average Convergence Divergence) Strategy
//
// Описание стратегии:
// Улучшенная версия классического MACD с дополнительными фильтрами для снижения ложных сигналов
// и повышения качества входов в позиции.
//
// Основные улучшения:
// 1. Трендовый фильтр на основе EMA200 для подтверждения общего направления
// 2. Фильтр силы сигнала на основе гистограммы MACD
// 3. Фильтр волатильности для избежания торговли в периоды высокой волатильности
// 4. Улучшенная логика выхода с учетом профита и риска
// 5. Адаптивные параметры для разных рыночных условий
//
// Как работает:
// - Рассчитывается быстрая EMA и медленная EMA для определения краткосрочного momentum
// - Сигнальная линия сглаживает MACD для генерации торговых сигналов
// - Гистограмма показывает силу и направление momentum
// - Дополнительные фильтры подтверждают качество сигналов
//
// Параметры:
// - FastPeriod: период быстрой EMA (оптимизируется)
// - SlowPeriod: период медленной EMA (оптимизируется)
// - SignalPeriod: период сигнальной линии (оптимизируется)
// - TrendPeriod: период для трендового фильтра (обычно 200)
// - VolatilityPeriod: период для расчета волатильности (обычно 20)
// - MinSignalStrength: минимальная сила сигнала для входа (оптимизируется)
//
// Сильные стороны:
// - Сниженное количество ложных сигналов благодаря фильтрам
// - Лучшая адаптация к разным рыночным условиям
// - Улучшенное соотношение риск/прибыль
// - Более стабильные результаты на разных инструментах
//
// Слабые стороны:
// - Больше параметров для оптимизации
// - Может пропустить быстрые движения в начале тренда
// - Требует больше вычислительных ресурсов

package momentum

import (
	"bt/internal"
	"errors"
	"fmt"
	"math"
)

type MACDConfig struct {
	FastPeriod              int     `json:"fast_period"`
	SlowPeriod              int     `json:"slow_period"`
	SignalPeriod            int     `json:"signal_period"`
	TrendPeriod             int     `json:"trend_period"`
	VolatilityPeriod        int     `json:"volatility_period"`
	MinSignalStrength       float64 `json:"min_signal_strength"`
	StopLossPercent         float64 `json:"stop_loss_percent"`
	TakeProfitPercent       float64 `json:"take_profit_percent"`
	UseTrendFilter          bool    `json:"use_trend_filter"`
	UseVolatilityFilter     bool    `json:"use_volatility_filter"`
	UseSignalStrengthFilter bool    `json:"use_signal_strength_filter"`
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
	if c.TrendPeriod <= 0 {
		return errors.New("trend period must be positive")
	}
	if c.VolatilityPeriod <= 0 {
		return errors.New("volatility period must be positive")
	}
	if c.FastPeriod >= c.SlowPeriod {
		return errors.New("fast period must be less than slow period")
	}
	if c.MinSignalStrength < 0 {
		return errors.New("min signal strength must be non-negative")
	}
	if c.StopLossPercent <= 0 || c.StopLossPercent >= 1 {
		return errors.New("stop loss percent must be between 0 and 1")
	}
	if c.TakeProfitPercent <= 0 || c.TakeProfitPercent >= 1 {
		return errors.New("take profit percent must be between 0 and 1")
	}
	return nil
}

func (c *MACDConfig) DefaultConfigString() string {
	return fmt.Sprintf("MACD(fast=%d, slow=%d, signal=%d, trend=%d, vol=%d, strength=%.2f)",
		c.FastPeriod, c.SlowPeriod, c.SignalPeriod, c.TrendPeriod, c.VolatilityPeriod, c.MinSignalStrength)
}

type MACDStrategy struct {
	internal.BaseConfig
}

func (s *MACDStrategy) Name() string {
	return "macd"
}

func (s *MACDStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	macdConfig, ok := config.(*MACDConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := macdConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	// Рассчитываем MACD с гистограммой
	macdLine, signalLine, histogram := internal.CalculateMACDWithSignal(candles, macdConfig.FastPeriod, macdConfig.SlowPeriod, macdConfig.SignalPeriod)
	if macdLine == nil || signalLine == nil || histogram == nil {
		return make([]internal.SignalType, len(candles))
	}

	// Рассчитываем трендовый фильтр (EMA)
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}
	trendEMA := internal.CalculateEMAForValues(prices, macdConfig.TrendPeriod)
	if trendEMA == nil {
		return make([]internal.SignalType, len(candles))
	}

	// Рассчитываем волатильность (ATR)
	volatility := s.calculateVolatility(candles, macdConfig.VolatilityPeriod)
	if volatility == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false
	entryPrice := 0.0
	stopLoss := 0.0
	takeProfit := 0.0

	// Находим индекс начала работы стратегии
	startIdx := macdConfig.SlowPeriod + macdConfig.SignalPeriod + macdConfig.TrendPeriod - 1
	if startIdx >= len(candles) {
		return make([]internal.SignalType, len(candles))
	}

	for i := startIdx; i < len(candles); i++ {
		currentPrice := candles[i].Close.ToFloat64()

		// Проверяем условия выхода если в позиции
		if inPosition {
			exitSignal := s.checkExitConditions(currentPrice, stopLoss, takeProfit)
			if exitSignal {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		// Проверяем условия входа если не в позиции
		if !inPosition {
			entrySignal := s.checkEntryConditions(i, macdLine, signalLine, histogram, trendEMA, volatility, candles, macdConfig)
			if entrySignal {
				signals[i] = internal.BUY
				inPosition = true
				entryPrice = currentPrice
				stopLoss = entryPrice * (1 - macdConfig.StopLossPercent)
				takeProfit = entryPrice * (1 + macdConfig.TakeProfitPercent)
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

// calculateVolatility рассчитывает волатильность на основе True Range
func (s *MACDStrategy) calculateVolatility(candles []internal.Candle, period int) []float64 {
	if len(candles) < period {
		return nil
	}

	volatility := make([]float64, len(candles))

	// Рассчитываем True Range для каждой свечи
	trueRanges := make([]float64, len(candles))
	for i := 1; i < len(candles); i++ {
		tr1 := candles[i].High.ToFloat64() - candles[i].Low.ToFloat64()
		tr2 := math.Abs(candles[i].High.ToFloat64() - candles[i-1].Close.ToFloat64())
		tr3 := math.Abs(candles[i].Low.ToFloat64() - candles[i-1].Close.ToFloat64())
		trueRanges[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// Рассчитываем среднюю волатильность
	for i := period - 1; i < len(candles); i++ {
		sum := 0.0
		for j := i - period + 1; j <= i; j++ {
			sum += trueRanges[j]
		}
		volatility[i] = sum / float64(period)
	}

	return volatility
}

// checkEntryConditions проверяет условия для входа в позицию
func (s *MACDStrategy) checkEntryConditions(idx int, macdLine, signalLine, histogram, trendEMA []float64, volatility []float64, candles []internal.Candle, config *MACDConfig) bool {
	macd := macdLine[idx]
	signal := signalLine[idx]
	macdPrev := macdLine[idx-1]
	signalPrev := signalLine[idx-1]
	hist := histogram[idx]
	histPrev := histogram[idx-1]

	currentPrice := candles[idx].Close.ToFloat64()
	trend := trendEMA[idx]
	vol := volatility[idx]

	// 1. Проверка трендового фильтра (если включен)
	if config.UseTrendFilter {
		if currentPrice <= trend {
			return false
		}
	}

	// 2. Проверка волатильности (если включен)
	if config.UseVolatilityFilter {
		avgVolatility := vol
		if avgVolatility > currentPrice*0.15 { // Увеличил порог с 10% до 15%
			return false
		}
	}

	// 3. Проверка силы сигнала MACD (если включен)
	if config.UseSignalStrengthFilter {
		signalStrength := math.Abs(hist - histPrev)
		if signalStrength < config.MinSignalStrength {
			return false
		}
	}

	// 4. Проверка crossover с подтверждением гистограммы
	macdCrossUp := macdPrev <= signalPrev && macd > signal
	histogramConfirm := hist > histPrev && hist > 0

	return macdCrossUp && histogramConfirm
}

// checkExitConditions проверяет условия для выхода из позиции
func (s *MACDStrategy) checkExitConditions(currentPrice, stopLoss, takeProfit float64) bool {
	// Проверка стоп-лосс
	if currentPrice <= stopLoss {
		return true
	}

	// Проверка тейк-профит
	if currentPrice >= takeProfit {
		return true
	}

	return false
}

func (s *MACDStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := s.DefaultConfig().(*MACDConfig)
	bestProfit := -1.0

	// Расширенный grid search по параметрам
	for fast := 8; fast <= 20; fast += 2 {
		for slow := fast + 8; slow <= fast+20; slow += 4 {
			for signal := 6; signal <= 14; signal += 2 {
				for strength := 0.05; strength <= 0.3; strength += 0.05 {
					for stopLoss := 0.03; stopLoss <= 0.08; stopLoss += 0.01 {
						for takeProfit := 0.1; takeProfit <= 0.25; takeProfit += 0.05 {
							config := &MACDConfig{
								FastPeriod:        fast,
								SlowPeriod:        slow,
								SignalPeriod:      signal,
								TrendPeriod:       200,
								VolatilityPeriod:  20,
								MinSignalStrength: strength,
								StopLossPercent:   stopLoss,
								TakeProfitPercent: takeProfit,
							}

							if config.Validate() != nil {
								continue
							}

							signals := s.GenerateSignalsWithConfig(candles, config)
							result := internal.Backtest(candles, signals, s.GetSlippage()) // Уменьшенное проскальзывание

							// Оцениваем только по прибыли
							if result.TotalProfit >= bestProfit {
								bestProfit = result.TotalProfit
								bestConfig = config
							}
						}
					}
				}
			}
		}
	}

	fmt.Printf("Лучшие параметры улучшенного MACD:\n")
	fmt.Printf("  Периоды: fast=%d, slow=%d, signal=%d\n",
		bestConfig.FastPeriod, bestConfig.SlowPeriod, bestConfig.SignalPeriod)
	fmt.Printf("  Фильтры: strength=%.2f, stop=%.2f%%, profit=%.2f%%\n",
		bestConfig.MinSignalStrength, bestConfig.StopLossPercent*100, bestConfig.TakeProfitPercent*100)
	fmt.Printf("  Результат: профит=%.4f\n", bestProfit)

	return bestConfig
}

func init() {
	// internal.RegisterStrategy("macd", &MACDStrategy{
	// 	BaseConfig: internal.BaseConfig{
	// 		Config: &MACDConfig{
	// 			FastPeriod:              12,
	// 			SlowPeriod:              26,
	// 			SignalPeriod:            9,
	// 			TrendPeriod:             50, // Уменьшил с 200 до 50 для менее строгого фильтра
	// 			VolatilityPeriod:        20,
	// 			MinSignalStrength:       0.05, // Уменьшил с 0.1 до 0.05 для менее строгого фильтра
	// 			StopLossPercent:         0.05, // 5%
	// 			TakeProfitPercent:       0.15, // 15%
	// 			UseTrendFilter:          true,
	// 			UseVolatilityFilter:     false, // Отключаем по умолчанию для большего количества сигналов
	// 			UseSignalStrengthFilter: false, // Отключаем по умолчанию для большего количества сигналов
	// 		},
	// 	},
	// })
}
