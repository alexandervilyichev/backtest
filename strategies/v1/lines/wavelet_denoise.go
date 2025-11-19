package lines

import (
	"bt/internal"
	"errors"
	"fmt"
	"math"
)

// --- НОВЫЙ КОНФИГ ---
type WaveletDenoiseConfig struct {
	LookbackPeriod  int     `json:"lookback_period"`  // Период для скользящего минимума (уровня поддержки)
	DenoiseLookback int     `json:"denoise_lookback"` // Период для DWT (окно сглаживания)
	BuyThreshold    float64 `json:"buy_threshold"`    // Порог для покупки (% ниже поддержки)
	SellThreshold   float64 `json:"sell_threshold"`   // Порог для продажи (% ниже поддержки)
	TakeProfitPct   float64 `json:"take_profit_pct"`  // Фиксация прибыли в %
}

func (c *WaveletDenoiseConfig) Validate() error {
	if c.LookbackPeriod <= 0 {
		return errors.New("lookback period must be positive")
	}
	if c.DenoiseLookback <= 0 {
		return errors.New("denoise lookback must be positive")
	}
	if c.BuyThreshold <= 0 || c.BuyThreshold >= 1.0 {
		return errors.New("buy threshold must be between 0 and 1.0")
	}
	if c.SellThreshold <= 0 || c.SellThreshold >= 1.0 {
		return errors.New("sell threshold must be between 0 and 1.0")
	}
	if c.SellThreshold <= c.BuyThreshold {
		return errors.New("sell threshold must be greater than buy threshold")
	}
	if c.TakeProfitPct <= 0 || c.TakeProfitPct >= 1.0 {
		return errors.New("take profit pct must be between 0 and 1.0")
	}
	return nil
}

func (c *WaveletDenoiseConfig) DefaultConfigString() string {
	return fmt.Sprintf("WaveletDenoise(lookback=%d, denoise_lb=%d, buy_thresh=%.3f, sell_thresh=%.3f, tp=%.3f)",
		c.LookbackPeriod, c.DenoiseLookback, c.BuyThreshold, c.SellThreshold, c.TakeProfitPct)
}

// --- НОВАЯ СТРАТЕГИЯ ---
type WaveletDenoiseStrategy struct {
	internal.BaseConfig
}

func (s *WaveletDenoiseStrategy) Name() string {
	return "wavelet_denoise"
}

// --- ЭФФЕКТИВНЫЙ АЛГОРИТМ ДЛЯ СКОЛЬЗЯЩЕГО МИНИМУМА ---
// Использует двустороннюю очередь (deque) для O(n) сложности
func calculateRollingMin(values []float64, period int) []float64 {
	if len(values) < period {
		return nil
	}

	n := len(values)
	result := make([]float64, n)
	deque := make([]int, 0) // Хранит индексы

	for i := 0; i < n; i++ {
		// Удаляем индексы, которые вышли за пределы окна
		for len(deque) > 0 && deque[0] < i-period+1 {
			deque = deque[1:]
		}

		// Удаляем индексы, значения которых >= текущего (они не могут быть минимумом)
		for len(deque) > 0 && values[deque[len(deque)-1]] >= values[i] {
			deque = deque[:len(deque)-1]
		}

		deque = append(deque, i)

		// Минимум для окна, заканчивающегося на i, это values[deque[0]]
		if i >= period-1 {
			result[i] = values[deque[0]]
		} else {
			// Для индексов до period-1, мы не можем рассчитать минимум
			result[i] = math.NaN()
		}
	}
	return result
}

// --- ВСПОМОГАТЕЛЬНАЯ ФУНКЦИЯ ДЛЯ СГЛАЖИВАНИЯ ---
func applyDenoising(prices []float64, lookback int) ([]float64, error) {
	if len(prices) < lookback {
		return nil, fmt.Errorf("not enough data for denoise lookback %d, got %d", lookback, len(prices))
	}
	// Выбираем последние lookback цен
	windowStart := len(prices) - lookback
	window := prices[windowStart:]

	// Убедимся, что длина окна четная
	n := len(window)
	if n%2 != 0 {
		window = window[1:] // Отбрасываем первую точку, чтобы сделать четное
		n = len(window)
		if n == 0 {
			return nil, fmt.Errorf("window became empty after making length even")
		}
	}

	// Применяем DWT к окну
	approx, detail, err := internal.DWT(window)
	if err != nil {
		// Логируем ошибку DWT, если нужно
		// fmt.Printf("DWT Error for window: %v\n", err)
		return nil, err
	}

	// Денойзинг: обнуляем детали
	zeroDetail := make([]float64, len(detail))

	// Реконструируем
	denoisedWindow, err := internal.IDWT(approx, zeroDetail)
	if err != nil {
		// Логируем ошибку IDWT, если нужно
		// fmt.Printf("IDWT Error: %v\n", err)
		return nil, err
	}

	// Результат должен быть той же длины, что и входное окно (после выравнивания на четность)
	if len(denoisedWindow) != n {
		// Это может быть проблемой в реализации DWT/IDWT
		return nil, fmt.Errorf("reconstructed window length %d does not match expected length %d", len(denoisedWindow), n)
	}

	return denoisedWindow, nil
}

func (s *WaveletDenoiseStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	waveletConfig, ok := config.(*WaveletDenoiseConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := waveletConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < 8 {
		return make([]internal.SignalType, len(candles))
	}

	prices := make([]float64, len(candles))
	for i, c := range candles {
		prices[i] = c.Close.ToFloat64()
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false
	var entryPrice float64

	// Проходим по свечам
	for i := 0; i < len(candles); i++ {
		// 1. Рассчитываем сглаженные цены для текущего окна
		// denoisedWindow, err := applyDenoising(prices[:i+1], waveletConfig.DenoiseLookback)
		// if err != nil {
		// 	// fmt.Printf("Error applying denoising at index %d: %v\n", i, err)
		// 	signals[i] = internal.HOLD
		// 	continue
		// }

		// Вычисляем "очищенное" значение для текущей свечи i
		// Последняя точка в denoisedWindow соответствует цене i
		// currentDenoisedPrice := denoisedWindow[len(denoisedWindow)-1]

		// Создадим срез "очищенных" цен до текущего момента i
		// Это неэффективно, но корректно для избежания Look-Ahead Bias
		denoisedPricesUpToI := make([]float64, i+1)
		for j := 0; j <= i; j++ {
			startJ := j - waveletConfig.DenoiseLookback + 1
			if startJ < 0 {
				startJ = 0
			}
			windowJ := prices[startJ : j+1]
			denoisedWindowJ, err := applyDenoising(windowJ, len(windowJ))
			if err != nil || denoisedWindowJ == nil || len(denoisedWindowJ) == 0 {
				// fmt.Printf("Error applying denoising for j=%d: %v\n", j, err)
				denoisedPricesUpToI[j] = math.NaN()
			} else {
				denoisedPricesUpToI[j] = denoisedWindowJ[len(denoisedWindowJ)-1]
			}
		}

		// 2. Рассчитываем уровень поддержки на основе "очищенных" цен
		supportLevels := calculateRollingMin(denoisedPricesUpToI, waveletConfig.LookbackPeriod)
		if supportLevels == nil || math.IsNaN(supportLevels[i]) {
			// Недостаточно данных для расчета поддержки или ошибка
			signals[i] = internal.HOLD
			continue
		}
		currentSupport := supportLevels[i]

		realClosePrice := prices[i] // Реальная цена закрытия

		// 3. Генерация сигналов на основе реальной цены и "очищенного" уровня поддержки
		if !inPosition && realClosePrice <= currentSupport*(1+waveletConfig.BuyThreshold) {
			signals[i] = internal.BUY
			inPosition = true
			entryPrice = realClosePrice
			continue
		}

		if inPosition {
			// Условие 1: Цена падает ниже поддержки на порог
			if realClosePrice <= currentSupport*(1-waveletConfig.SellThreshold) {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
			// Условие 2: Фиксация прибыли
			takeProfitPrice := entryPrice * (1 + waveletConfig.TakeProfitPct)
			if realClosePrice >= takeProfitPrice {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		if !inPosition {
			signals[i] = internal.HOLD
		}
	}

	return signals
}

func (s *WaveletDenoiseStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := s.DefaultConfig().(*WaveletDenoiseConfig)
	bestProfit := -math.MaxFloat64 // Используем более подходящее начальное значение

	// Пример сеточного поиска (можно расширить диапазоны и шаги)
	for lookback := 20; lookback <= 30; lookback += 5 { // Уменьшил диапазон для демонстрации
		for denoiseLb := 20; denoiseLb <= 35; denoiseLb += 5 { // Добавлен параметр
			for buyThresh := 0.02; buyThresh <= 0.02; buyThresh += 0.005 {
				for sellThresh := 0.025; sellThresh <= 0.035; sellThresh += 0.005 {

					config := &WaveletDenoiseConfig{
						LookbackPeriod:  lookback,
						DenoiseLookback: denoiseLb,
						BuyThreshold:    buyThresh,
						SellThreshold:   sellThresh,
						TakeProfitPct:   0.04,
					}
					if config.Validate() != nil {
						continue
					}

					signals := s.GenerateSignalsWithConfig(candles, config)
					result := internal.Backtest(candles, signals, s.GetSlippage())

					fmt.Printf("Параметры Wavelet: lb=%d, dlb=%d, b_thresh=%.4f, s_thresh=%.4f, tp=%.4f, профит=%.4f\n",
						config.LookbackPeriod, config.DenoiseLookback, config.BuyThreshold, config.SellThreshold, config.TakeProfitPct, result.TotalProfit)

					if result.TotalProfit > bestProfit {
						bestProfit = result.TotalProfit
						bestConfig = config
					}
				}
			}

		}
	}

	fmt.Printf("Лучшие параметры Wavelet: lb=%d, dlb=%d, b_thresh=%.4f, s_thresh=%.4f, tp=%.4f, профит=%.4f\n",
		bestConfig.LookbackPeriod, bestConfig.DenoiseLookback, bestConfig.BuyThreshold, bestConfig.SellThreshold, bestConfig.TakeProfitPct, bestProfit)

	return bestConfig
}

// --- УСТАНОВКА СТРАТЕГИИ ---
func init() {
	// internal.RegisterStrategy("wavelet_denoise", &WaveletDenoiseStrategy{
	// 	BaseConfig: internal.BaseConfig{
	// 		Config: &WaveletDenoiseConfig{
	// 			LookbackPeriod:  20,
	// 			DenoiseLookback: 16,
	// 			BuyThreshold:    0.005,
	// 			SellThreshold:   0.01,
	// 			TakeProfitPct:   0.03, // Новый параметр
	// 		},
	// 	}})
}
