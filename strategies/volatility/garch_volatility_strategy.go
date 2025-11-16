// strategies/volatility/garch_volatility_strategy.go
//
// Стратегия на основе модели GARCH для прогнозирования волатильности
//
// GARCH(1,1) модель:
// r_t = μ + ε_t
// ε_t = σ_t * z_t, где z_t ~ N(0,1)
// σ²_t = ω + α*ε²_{t-1} + β*σ²_{t-1}
//
// Стратегия использует прогнозы волатильности для:
// 1. Определения периодов высокой/низкой волатильности
// 2. Адаптации размера позиций
// 3. Генерации сигналов на основе режимов волатильности

package volatility

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
	"math"
)

type GARCHVolatilityConfig struct {
	WindowSize          int     `json:"window_size"`           // размер окна для калибровки
	ForecastHorizon     int     `json:"forecast_horizon"`      // горизонт прогноза волатильности
	VolatilityThreshold float64 `json:"volatility_threshold"`  // порог волатильности для сигналов
	TrendThreshold      float64 `json:"trend_threshold"`       // порог тренда
	UseVolatilityRegime bool    `json:"use_volatility_regime"` // использовать режимы волатильности
}

func (c *GARCHVolatilityConfig) Validate() error {
	if c.WindowSize < 30 {
		return errors.New("window size must be at least 30")
	}
	if c.ForecastHorizon < 1 {
		return errors.New("forecast horizon must be positive")
	}
	if c.VolatilityThreshold <= 0 {
		return errors.New("volatility threshold must be positive")
	}
	if c.TrendThreshold <= 0 {
		return errors.New("trend threshold must be positive")
	}
	return nil
}

func (c *GARCHVolatilityConfig) DefaultConfigString() string {
	return fmt.Sprintf("GARCH_Vol(window=%d, horizon=%d, vol_thresh=%.3f)",
		c.WindowSize, c.ForecastHorizon, c.VolatilityThreshold)
}

// GARCHVolModel представляет модель GARCH для волатильности
type GARCHVolModel struct {
	Omega   float64   // константа (ω)
	Alpha   float64   // коэффициент ARCH (α)
	Beta    float64   // коэффициент GARCH (β)
	Mu      float64   // средняя доходность (μ)
	Sigma2  []float64 // условная дисперсия
	Returns []float64 // доходности
}

// NewGARCHVolModel создает новую модель GARCH
func NewGARCHVolModel() *GARCHVolModel {
	return &GARCHVolModel{
		Sigma2:  make([]float64, 0),
		Returns: make([]float64, 0),
	}
}

// calibrate калибрует параметры GARCH модели на исторических данных
func (model *GARCHVolModel) calibrate(prices []float64) error {
	if len(prices) < 10 {
		return errors.New("insufficient data for GARCH calibration")
	}

	// Вычисляем логарифмические доходности
	model.Returns = make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		model.Returns[i-1] = math.Log(prices[i] / prices[i-1])
	}

	// Вычисляем среднюю доходность
	model.Mu = calculateMean(model.Returns)

	// Центрируем доходности
	centeredReturns := make([]float64, len(model.Returns))
	for i, ret := range model.Returns {
		centeredReturns[i] = ret - model.Mu
	}

	// Начальные параметры
	model.Omega = 0.00001
	model.Alpha = 0.1
	model.Beta = 0.85

	// Простая калибровка методом моментов
	unconditionalVar := calculateVariance(centeredReturns, 0)

	// Итеративная оптимизация параметров
	for iter := 0; iter < 20; iter++ {
		// Вычисляем условную волатильность
		model.Sigma2 = make([]float64, len(centeredReturns))
		model.Sigma2[0] = unconditionalVar

		for i := 1; i < len(centeredReturns); i++ {
			model.Sigma2[i] = model.Omega +
				model.Alpha*centeredReturns[i-1]*centeredReturns[i-1] +
				model.Beta*model.Sigma2[i-1]
		}

		// Обновляем параметры (упрощенный метод)
		sumAlpha := 0.0
		sumBeta := 0.0
		sumOmega := 0.0

		for i := 1; i < len(centeredReturns); i++ {
			if model.Sigma2[i] > 0 {
				weight := 1.0 / model.Sigma2[i]
				sumAlpha += weight * centeredReturns[i-1] * centeredReturns[i-1]
				sumBeta += weight * model.Sigma2[i-1]
				sumOmega += weight
			}
		}

		if sumOmega > 0 {
			newAlpha := sumAlpha / sumOmega * 0.1
			newBeta := sumBeta / sumOmega * 0.85

			// Ограничения для стабильности
			if newAlpha > 0 && newAlpha < 0.3 && newBeta > 0.5 && newBeta < 0.95 {
				if newAlpha+newBeta < 0.99 {
					model.Alpha = newAlpha
					model.Beta = newBeta
					model.Omega = unconditionalVar * (1 - model.Alpha - model.Beta)
				}
			}
		}

		// Проверяем условие стационарности
		if model.Alpha+model.Beta >= 1.0 {
			model.Alpha = 0.1
			model.Beta = 0.85
			model.Omega = unconditionalVar * (1 - model.Alpha - model.Beta)
		}
	}

	return nil
}

// forecast прогнозирует волатильность на заданное количество шагов вперед
func (model *GARCHVolModel) forecast(steps int) []float64 {
	if len(model.Sigma2) == 0 || len(model.Returns) == 0 {
		return nil
	}

	forecasts := make([]float64, steps)

	// Текущая волатильность
	currentSigma2 := model.Sigma2[len(model.Sigma2)-1]
	currentReturn := model.Returns[len(model.Returns)-1] - model.Mu

	// Безусловная дисперсия
	unconditionalVar := model.Omega / (1 - model.Alpha - model.Beta)

	for i := 0; i < steps; i++ {
		if i == 0 {
			// Первый шаг: используем последнюю доходность
			forecasts[i] = model.Omega + model.Alpha*currentReturn*currentReturn + model.Beta*currentSigma2
		} else {
			// Последующие шаги: сходимость к безусловной дисперсии
			persistence := math.Pow(model.Alpha+model.Beta, float64(i))
			forecasts[i] = unconditionalVar + persistence*(forecasts[0]-unconditionalVar)
		}
	}

	return forecasts
}

// getVolatilityRegime определяет текущий режим волатильности
func (model *GARCHVolModel) getVolatilityRegime(currentVol, avgVol float64) string {
	if currentVol > avgVol*1.5 {
		return "HIGH"
	} else if currentVol < avgVol*0.7 {
		return "LOW"
	}
	return "NORMAL"
}

type GARCHVolatilityStrategy struct{ internal.BaseConfig }

func (s *GARCHVolatilityStrategy) Name() string {
	return "garch_volatility_strategy"
}

// calculateTrendStrength вычисляет силу тренда
func (s *GARCHVolatilityStrategy) calculateTrendStrength(prices []float64, window int) float64 {
	if len(prices) < window {
		return 0.0
	}

	recentPrices := prices[len(prices)-window:]
	n := float64(len(recentPrices))

	// Линейная регрессия для определения тренда
	sumX := n * (n - 1) / 2
	sumY := 0.0
	sumXY := 0.0
	sumXX := n * (n - 1) * (2*n - 1) / 6

	for i, price := range recentPrices {
		x := float64(i)
		sumY += price
		sumXY += x * price
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)
	avgPrice := sumY / n

	return slope / avgPrice // нормализованный наклон
}

func (s *GARCHVolatilityStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	garchConfig, ok := config.(*GARCHVolatilityConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := garchConfig.Validate(); err != nil {
		log.Printf("❌ Ошибка конфигурации GARCH Volatility: %v", err)
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < garchConfig.WindowSize+50 {
		log.Printf("⚠️ Недостаточно данных для GARCH Volatility: получено %d свечей, требуется минимум %d",
			len(candles), garchConfig.WindowSize+50)
		return make([]internal.SignalType, len(candles))
	}

	// Извлекаем ценовые данные
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	log.Printf("🚀 ЗАПУСК GARCH VOLATILITY СТРАТЕГИИ:")
	log.Printf("   Окно калибровки: %d свечей", garchConfig.WindowSize)
	log.Printf("   Горизонт прогноза: %d шагов", garchConfig.ForecastHorizon)
	log.Printf("   Порог волатильности: %.3f", garchConfig.VolatilityThreshold)
	log.Printf("   Режимы волатильности: %v", garchConfig.UseVolatilityRegime)

	signals := make([]internal.SignalType, len(candles))

	// Параметры для управления позицией
	inPosition := false
	minHoldBars := 2 // уменьшили с 5 до 2 для более активной торговли
	lastTradeIndex := -minHoldBars

	// Начинаем анализ после накопления достаточных данных
	startIndex := garchConfig.WindowSize + 10

	for i := startIndex; i < len(candles); i++ {
		// Окно для калибровки модели
		windowStart := i - garchConfig.WindowSize
		windowData := prices[windowStart:i]

		// Калибруем GARCH модель
		model := NewGARCHVolModel()
		if err := model.calibrate(windowData); err != nil {
			signals[i] = internal.HOLD
			continue
		}

		// Прогнозируем волатильность
		volForecasts := model.forecast(garchConfig.ForecastHorizon)
		if len(volForecasts) == 0 {
			signals[i] = internal.HOLD
			continue
		}

		// Текущая и прогнозируемая волатильность
		currentVol := math.Sqrt(model.Sigma2[len(model.Sigma2)-1])
		forecastVol := math.Sqrt(volForecasts[0])
		avgVol := math.Sqrt(calculateMean(model.Sigma2))

		// Определяем режим волатильности
		volRegime := model.getVolatilityRegime(currentVol, avgVol)

		// Вычисляем силу тренда
		trendStrength := s.calculateTrendStrength(prices, 20)

		// Вычисляем изменение волатильности
		volChange := (forecastVol - currentVol) / currentVol

		// Отладочная информация только в начале
		if i == startIndex {
			log.Printf("🔍 Начало анализа: порог_тренда=%.4f, порог_волат=%.4f",
				garchConfig.TrendThreshold, garchConfig.VolatilityThreshold)
		}

		// Генерируем сигналы на основе волатильности и тренда
		signal := internal.HOLD

		if garchConfig.UseVolatilityRegime {
			// Стратегия на основе режимов волатильности
			switch volRegime {
			case "LOW":
				// В периоды низкой волатильности следуем тренду (более мягкие условия)
				if !inPosition && trendStrength > garchConfig.TrendThreshold &&
					i-lastTradeIndex >= minHoldBars {
					signal = internal.BUY
					inPosition = true
					lastTradeIndex = i
					// log.Printf("📈 BUY (низкая волатильность, тренд=%.4f) на свече %d", trendStrength, i)
				}

			case "HIGH":
				// В периоды высокой волатильности - осторожность
				if inPosition && i-lastTradeIndex >= minHoldBars {
					signal = internal.SELL
					inPosition = false
					lastTradeIndex = i
					// log.Printf("📉 SELL (высокая волатильность) на свече %d", i)
				}

			case "NORMAL":
				// В нормальные периоды используем прогноз волатильности (упрощенные условия)
				if !inPosition && volChange < -garchConfig.VolatilityThreshold &&
					i-lastTradeIndex >= minHoldBars {
					signal = internal.BUY
					inPosition = true
					lastTradeIndex = i
					// log.Printf("📈 BUY (снижение волатильности=%.4f) на свече %d", volChange, i)
				} else if inPosition && volChange > garchConfig.VolatilityThreshold &&
					i-lastTradeIndex >= minHoldBars {
					signal = internal.SELL
					inPosition = false
					lastTradeIndex = i
					// log.Printf("📉 SELL (рост волатильности=%.4f) на свече %d", volChange, i)
				}
			}
		} else {
			// Простая стратегия на основе прогноза волатильности (еще более простая)
			if !inPosition && volChange < -garchConfig.VolatilityThreshold &&
				i-lastTradeIndex >= minHoldBars {
				signal = internal.BUY
				inPosition = true
				lastTradeIndex = i
				// log.Printf("📈 BUY (простая: волат=%.4f) на свече %d", volChange, i)
			} else if inPosition && volChange > garchConfig.VolatilityThreshold &&
				i-lastTradeIndex >= minHoldBars {
				signal = internal.SELL
				inPosition = false
				lastTradeIndex = i
				// log.Printf("📉 SELL (простая: волат=%.4f) на свече %d", volChange, i)
			}
		}

		signals[i] = signal
	}

	log.Printf("✅ GARCH Volatility анализ завершен")
	return signals
}

func (s *GARCHVolatilityStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := s.DefaultConfig().(*GARCHVolatilityConfig)
	bestProfit := -1.0

	// Оптимизируем параметры
	windowSizes := []int{50, 100, 150}
	horizons := []int{3, 5, 10}
	volThresholds := []float64{0.01, 0.02, 0.03}
	trendThresholds := []float64{0.005, 0.01, 0.02}
	regimeModes := []bool{true, false}

	for _, windowSize := range windowSizes {
		for _, horizon := range horizons {
			for _, volThresh := range volThresholds {
				for _, trendThresh := range trendThresholds {
					for _, useRegime := range regimeModes {
						config := &GARCHVolatilityConfig{
							WindowSize:          windowSize,
							ForecastHorizon:     horizon,
							VolatilityThreshold: volThresh,
							TrendThreshold:      trendThresh,
							UseVolatilityRegime: useRegime,
						}

						if config.Validate() != nil {
							continue
						}

						signals := s.GenerateSignalsWithConfig(candles, config)
						result := internal.Backtest(candles, signals, s.GetSlippage())

						if result.TotalProfit >= bestProfit {
							bestProfit = result.TotalProfit
							bestConfig = config
						}
					}
				}
			}
		}
	}

	fmt.Printf("Лучшие параметры GARCH Volatility: окно=%d, горизонт=%d, vol_thresh=%.3f, trend_thresh=%.3f, режимы=%v, профит=%.4f\n",
		bestConfig.WindowSize, bestConfig.ForecastHorizon, bestConfig.VolatilityThreshold,
		bestConfig.TrendThreshold, bestConfig.UseVolatilityRegime, bestProfit)

	return bestConfig
}

// Вспомогательные функции для статистических вычислений

func calculateMean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func calculateVariance(data []float64, mean float64) float64 {
	if len(data) <= 1 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		diff := v - mean
		sum += diff * diff
	}
	return sum / float64(len(data)-1)
}

func init() {
	// internal.RegisterStrategy("garch_volatility_strategy", &GARCHVolatilityStrategy{
	// 	BaseConfig: internal.BaseConfig{
	// 		Config: &GARCHVolatilityConfig{
	// 			WindowSize:          100,
	// 			ForecastHorizon:     5,
	// 			VolatilityThreshold: 0.005, // уменьшили с 0.02 до 0.005
	// 			TrendThreshold:      0.002, // уменьшили с 0.01 до 0.002
	// 			UseVolatilityRegime: true,
	// 		},
	// 	},
	// })
}
