// strategies/statistical/heston_strategy.go
//
// Стратегия на основе модели Heston для стохастической волатильности
//
// Модель Heston описывает эволюцию цены актива и его волатильности:
// dS_t = μ S_t dt + √V_t S_t dW1_t
// dV_t = κ(θ - V_t)dt + σ √V_t dW2_t
//
// где:
// S_t - цена актива
// V_t - мгновенная дисперсия (волатильность²)
// μ - дрифт цены
// κ - скорость возврата волатильности к среднему
// θ - долгосрочная средняя волатильность²
// σ - волатильность волатильности
// W1_t, W2_t - коррелированные броуновские движения

package statistical

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
)

type HestonConfig struct {
	WindowSize      int     `json:"window_size"`      // размер окна для калибровки
	PredictionSteps int     `json:"prediction_steps"` // количество шагов прогноза
	NumSimulations  int     `json:"num_simulations"`  // количество симуляций Монте-Карло
	Threshold       float64 `json:"threshold"`        // порог для генерации сигналов
}

func (c *HestonConfig) Validate() error {
	if c.WindowSize < 50 {
		return errors.New("window size must be at least 50")
	}
	if c.PredictionSteps < 1 {
		return errors.New("prediction steps must be positive")
	}
	if c.NumSimulations < 100 {
		return errors.New("number of simulations must be at least 100")
	}
	if c.Threshold <= 0 {
		return errors.New("threshold must be positive")
	}
	return nil
}

func (c *HestonConfig) DefaultConfigString() string {
	return fmt.Sprintf("Heston(window=%d, sims=%d)",
		c.WindowSize, c.NumSimulations)
}

// HestonModel представляет модель Heston для стохастической волатильности
type HestonModel struct {
	Mu    float64 // дрифт цены
	Kappa float64 // скорость возврата волатильности
	Theta float64 // долгосрочная средняя волатильность²
	Sigma float64 // волатильность волатильности
	Rho   float64 // корреляция между ценой и волатильностью
	V0    float64 // начальная волатильность²
	S0    float64 // начальная цена
}

// calibrateHeston калибрует параметры модели Heston на исторических данных
func calibrateHeston(prices []float64) *HestonModel {
	if len(prices) < 10 {
		return nil
	}

	// Вычисляем логарифмические доходности
	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		returns[i-1] = math.Log(prices[i] / prices[i-1])
	}

	// Базовые статистики
	mu := mean(returns)
	variance := variance(returns, mu)

	// Простая калибровка параметров Heston
	// В реальности используются более сложные методы (MLE, характеристические функции)

	model := &HestonModel{
		Mu:    mu,
		Kappa: 2.0,                       // скорость возврата к среднему
		Theta: variance,                  // долгосрочная волатильность
		Sigma: math.Sqrt(variance) * 0.5, // волатильность волатильности
		Rho:   -0.3,                      // отрицательная корреляция (leverage effect)
		V0:    variance,                  // текущая волатильность
		S0:    prices[len(prices)-1],     // текущая цена
	}

	return model
}

// simulateHeston выполняет симуляцию Монте-Карло для модели Heston
func (model *HestonModel) simulateHeston(steps int, dt float64, numSims int) [][]float64 {
	simulations := make([][]float64, numSims)

	for sim := 0; sim < numSims; sim++ {
		prices := make([]float64, steps+1)
		volatilities := make([]float64, steps+1)

		prices[0] = model.S0
		volatilities[0] = model.V0

		for i := 1; i <= steps; i++ {
			// Генерируем коррелированные случайные числа
			z1 := rand.NormFloat64()
			z2 := rand.NormFloat64()
			w1 := z1
			w2 := model.Rho*z1 + math.Sqrt(1-model.Rho*model.Rho)*z2

			// Обновляем волатильность (схема Эйлера с отражением)
			vt := math.Max(volatilities[i-1], 0.0001) // избегаем отрицательных значений
			dv := model.Kappa*(model.Theta-vt)*dt + model.Sigma*math.Sqrt(vt)*w2*math.Sqrt(dt)
			volatilities[i] = math.Max(vt+dv, 0.0001)

			// Обновляем цену
			st := prices[i-1]
			ds := model.Mu*st*dt + math.Sqrt(vt)*st*w1*math.Sqrt(dt)
			prices[i] = st + ds

			// Избегаем отрицательных цен
			if prices[i] <= 0 {
				prices[i] = st * 0.99
			}
		}

		simulations[sim] = prices
	}

	return simulations
}

// analyzeSimulations анализирует результаты симуляций и возвращает статистики
func analyzeSimulations(simulations [][]float64, currentPrice float64) (float64, float64, float64) {
	if len(simulations) == 0 || len(simulations[0]) == 0 {
		return currentPrice, 0, 0
	}

	finalPrices := make([]float64, len(simulations))
	for i, sim := range simulations {
		finalPrices[i] = sim[len(sim)-1]
	}

	meanPrice := mean(finalPrices)
	stdPrice := math.Sqrt(variance(finalPrices, meanPrice))

	// Вероятность роста
	upCount := 0
	for _, price := range finalPrices {
		if price > currentPrice {
			upCount++
		}
	}
	probUp := float64(upCount) / float64(len(finalPrices))

	return meanPrice, stdPrice, probUp
}

type HestonStrategy struct{ internal.BaseConfig }

func (s *HestonStrategy) Name() string {
	return "heston_strategy"
}

func (s *HestonStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	hestonConfig, ok := config.(*HestonConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := hestonConfig.Validate(); err != nil {
		log.Printf("❌ Ошибка конфигурации Heston: %v", err)
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < hestonConfig.WindowSize+50 {
		log.Printf("⚠️ Недостаточно данных для Heston: получено %d свечей, требуется минимум %d",
			len(candles), hestonConfig.WindowSize+50)
		return make([]internal.SignalType, len(candles))
	}

	// Извлекаем ценовые данные
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	log.Printf("🚀 ЗАПУСК СТРАТЕГИИ HESTON:")
	log.Printf("   Окно калибровки: %d свечей", hestonConfig.WindowSize)
	log.Printf("   Шагов прогноза: %d", hestonConfig.PredictionSteps)
	log.Printf("   Симуляций: %d", hestonConfig.NumSimulations)
	log.Printf("   Порог сигнала: %.2f%%", hestonConfig.Threshold*100)

	signals := make([]internal.SignalType, len(candles))
	dt := 1.0 / 252.0 // дневной шаг (252 торговых дня в году)

	// Параметры для управления позицией
	inPosition := false
	minHoldBars := 3 // Уменьшаем минимальное время удержания
	lastTradeIndex := -minHoldBars

	// Счетчики для статистики
	buySignals := 0
	sellSignals := 0

	// Начинаем анализ после накопления достаточных данных
	startIndex := hestonConfig.WindowSize + 10 // Уменьшаем стартовый индекс

	for i := startIndex; i < len(candles); i++ {
		// Окно для калибровки модели
		windowStart := i - hestonConfig.WindowSize
		windowData := prices[windowStart:i]
		currentPrice := prices[i]

		// Калибруем и симулируем модель Heston
		hestonModel := calibrateHeston(windowData)
		if hestonModel == nil {
			signals[i] = internal.HOLD
			continue
		}

		simulations := hestonModel.simulateHeston(hestonConfig.PredictionSteps, dt, hestonConfig.NumSimulations)
		meanForecast, stdForecast, probUp := analyzeSimulations(simulations, currentPrice)

		// Вычисляем ожидаемое изменение цены
		expectedReturn := (meanForecast - currentPrice) / currentPrice

		// Более мягкий адаптивный порог
		volatility := internal.CalculateStdDevOfReturns(prices[max(0, i-20):i])
		adaptiveThreshold := hestonConfig.Threshold * (1 + volatility*0.3) // Менее агрессивная адаптация

		// Дополнительные сигналы на основе волатильности прогноза
		volatilitySignal := stdForecast / currentPrice

		// Генерируем сигналы
		signal := internal.HOLD

		// BUY сигнал: более мягкие условия
		buyCondition1 := probUp > 0.55 && expectedReturn > adaptiveThreshold                                // Основной сигнал
		buyCondition2 := probUp > 0.65 && expectedReturn > adaptiveThreshold*0.7                            // Высокая вероятность
		buyCondition3 := expectedReturn > adaptiveThreshold*1.5 && probUp > 0.5                             // Высокая ожидаемая доходность
		buyCondition4 := volatilitySignal > 0.02 && expectedReturn > adaptiveThreshold*0.8 && probUp > 0.52 // Волатильность + доходность

		if !inPosition && (buyCondition1 || buyCondition2 || buyCondition3 || buyCondition4) &&
			i-lastTradeIndex >= minHoldBars {
			signal = internal.BUY
			inPosition = true
			lastTradeIndex = i
			buySignals++
			// if buySignals <= 20 { // Логируем только первые 20 сигналов
			// 	log.Printf("📈 BUY сигнал на свече %d: ожидаемая доходность %.2f%%, вероятность роста %.1f%%",
			// 		i, expectedReturn*100, probUp*100)
			// }
		}

		// SELL сигнал: более мягкие условия
		sellCondition1 := probUp < 0.45 || expectedReturn < -adaptiveThreshold           // Основной сигнал
		sellCondition2 := probUp < 0.35                                                  // Очень низкая вероятность роста
		sellCondition3 := expectedReturn < -adaptiveThreshold*0.7 && probUp < 0.5        // Отрицательная доходность
		sellCondition4 := volatilitySignal > 0.03 && expectedReturn < 0 && probUp < 0.48 // Высокая волатильность + падение

		if inPosition && (sellCondition1 || sellCondition2 || sellCondition3 || sellCondition4) &&
			i-lastTradeIndex >= minHoldBars {
			signal = internal.SELL
			inPosition = false
			lastTradeIndex = i
			sellSignals++
			// log.Printf("📉 SELL сигнал на свече %d: ожидаемая доходность %.2f%%, вероятность роста %.1f%%, волатильность %.2f%%",
			// 	i, expectedReturn*100, probUp*100, volatilitySignal*100)
		}

		signals[i] = signal
	}

	log.Printf("📊 Статистика сигналов: BUY=%d, SELL=%d, Всего=%d", buySignals, sellSignals, buySignals+sellSignals)

	log.Printf("✅ Анализ Heston завершен")
	return signals
}

func (s *HestonStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := s.DefaultConfig().(*HestonConfig)
	bestProfit := -1.0

	// Оптимизируем параметры для более активной торговли
	windowSizes := []int{50, 80, 120}
	predictionSteps := []int{2, 3, 5}
	thresholds := []float64{0.008, 0.012, 0.018, 0.025}

	for _, windowSize := range windowSizes {
		for _, steps := range predictionSteps {
			for _, threshold := range thresholds {
				config := &HestonConfig{
					WindowSize:      windowSize,
					PredictionSteps: steps,
					NumSimulations:  300, // уменьшаем для оптимизации
					Threshold:       threshold,
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

	fmt.Printf("Лучшие параметры Heston: окно=%d, шаги=%d, порог=%.3f, профит=%.4f\n",
		bestConfig.WindowSize, bestConfig.PredictionSteps, bestConfig.Threshold, bestProfit)

	return bestConfig
}

// Вспомогательные функции для статистических вычислений

func mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func variance(data []float64, mean float64) float64 {
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
	// internal.RegisterStrategy("heston_strategy", &HestonStrategy{
	// 	BaseConfig: internal.BaseConfig{
	// 		Config: &HestonConfig{
	// 			WindowSize:      80,    // Уменьшаем окно для более быстрой адаптации
	// 			PredictionSteps: 3,     // Уменьшаем шаги прогноза для более частых сигналов
	// 			NumSimulations:  400,   // Немного уменьшаем для скорости
	// 			Threshold:       0.015, // Снижаем порог с 2% до 1.5%
	// 		},
	// 	},
	// })
}
