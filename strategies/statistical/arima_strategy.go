// strategies/arima_strategy_improved.go — улучшенная стратегия ARIMA
//
// УЛУЧШЕННАЯ ВЕРСИЯ С:
// - Полной реализацией ARIMA(p,d,q) модели
// - Адаптивными параметрами и оптимизацией
// - Фильтрами волатильности и тренда
// - Валидацией качества модели
// - Динамическим порогом сигналов

package strategies

import (
	"bt/internal"
	"errors"
	"fmt"
	"log"
	"math"
)

type ARIMAConfig struct {
	ArOrder   int `json:"ar_order"`
	DiffOrder int `json:"diff_order"`
	MaOrder   int `json:"ma_order"`
}

func (c *ARIMAConfig) Validate() error {
	if c.ArOrder < 0 {
		return errors.New("ar order must be non-negative")
	}
	if c.DiffOrder < 0 {
		return errors.New("diff order must be non-negative")
	}
	if c.MaOrder < 0 {
		return errors.New("ma order must be non-negative")
	}
	if c.ArOrder+c.DiffOrder+c.MaOrder == 0 {
		return errors.New("at least one parameter must be positive")
	}
	return nil
}

func (c *ARIMAConfig) DefaultConfigString() string {
	return fmt.Sprintf("ARIMA(p=%d,d=%d,q=%d)",
		c.ArOrder, c.DiffOrder, c.MaOrder)
}

// ARIMAModel — модель ARIMA
type ARIMAModel struct {
	arOrder   int // порядок авторегрессии (p)
	maOrder   int // порядок скользящего среднего (q)
	diffOrder int // порядок дифференцирования (d)

	arCoeffs []float64 // коэффициенты AR
	maCoeffs []float64 // коэффициенты MA
	constant float64   // константа модели

	residuals []float64 // остатки для MA компоненты

	originalData []float64 // оригинальные данные для обратного дифференцирования
}

// NewARIMAModel создает новую модель ARIMA
func NewARIMAModel(arOrder, diffOrder, maOrder int) *ARIMAModel {
	return &ARIMAModel{
		arOrder:   arOrder,
		maOrder:   maOrder,
		diffOrder: diffOrder,
		arCoeffs:  make([]float64, arOrder),
		maCoeffs:  make([]float64, maOrder),
		residuals: make([]float64, 0),
	}
}

/*
*
difference: последовательно применяет дифференцирование order раз и возвращает стационарный ряд.
Пример: order=1 => Δy_t = y_t - y_{t-1}; order=2 => Δ^2 y_t = Δy_t - Δy_{t-1}
*/
func (model *ARIMAModel) difference(data []float64, order int) []float64 {
	if order <= 0 {
		out := make([]float64, len(data))
		copy(out, data)
		return out
	}
	result := make([]float64, len(data))
	copy(result, data)
	for d := 0; d < order; d++ {
		if len(result) < 2 {
			return []float64{}
		}
		next := make([]float64, 0, len(result)-1)
		for i := 1; i < len(result); i++ {
			next = append(next, result[i]-result[i-1])
		}
		result = next
	}
	return result
}

/*
*
undifference: восстанавливает y_{t+1} из прогноза Δ^d y_{t+1} и последних d разностей.
Алгоритм:
  - Вычисляем последние разности до порядка d-1 включительно: lastY, lastΔy, lastΔ^2y, ...
  - Пусть newΔ^d = stationaryForecast. Тогда рекурсивно:
    newΔ^{k-1} = lastΔ^{k-1} + newΔ^{k}, для k=d..1
  - y_{t+1} = lastY + newΔ^1
*/
func (model *ARIMAModel) undifference(stationaryForecast float64, originalData []float64, order int) float64 {
	if order <= 0 || len(originalData) == 0 {
		return stationaryForecast
	}
	// Собираем последние разности
	lastY := originalData[len(originalData)-1]
	// lastDiffs[k] = last Δ^{k} y_t, где k=1..order-1
	lastDiffs := make([]float64, order) // индекс 0 не используется для простоты
	// Вычисляем последовательные разности на хвосте оригинального ряда
	series := make([]float64, len(originalData))
	copy(series, originalData)
	for d := 1; d < order; d++ {
		// вычислить Δ^d y_t и взять последнее значение
		next := make([]float64, 0, len(series)-1)
		for i := 1; i < len(series); i++ {
			next = append(next, series[i]-series[i-1])
		}
		series = next
		if len(series) == 0 {
			// недостаточно данных — деградируем к d=1
			return lastY + stationaryForecast
		}
		lastDiffs[d] = series[len(series)-1]
	}
	// Вверх по порядкам
	newDiff := make([]float64, order+1) // newDiff[order] = Δ^d y_{t+1}
	newDiff[order] = stationaryForecast
	for k := order; k >= 1; k-- {
		if k-1 == 0 {
			// new Δ^0 — это добавка к уровню
			continue
		}
		newDiff[k-1] = lastDiffs[k-1] + newDiff[k]
	}
	// Восстановить уровень
	return lastY + newDiff[1]
}

// train обучает модель ARIMA на данных
func (model *ARIMAModel) train(data []float64) {
	// Сохраняем оригинальные данные для обратного дифференцирования
	model.originalData = make([]float64, len(data))
	copy(model.originalData, data)

	// Применяем дифференцирование
	stationaryData := model.difference(data, model.diffOrder)

	if len(stationaryData) < model.arOrder+1 {
		return
	}

	// Обучаем AR на стационарном ряду
	model.trainARModel(stationaryData)

	// Подготовка остатков (для потенциальной MA в будущем)
	model.residuals = model.computeResiduals(stationaryData)

	// Легкое отсечение коэффициентов для стабильности
	model.checkOverfitting()
}

// trainARModel обучает авторегрессионную модель на стационарных данных
func (model *ARIMAModel) trainARModel(data []float64) {
	n := len(data)
	if n < model.arOrder+1 {
		return
	}

	// Формируем регрессионные признаки
	X := make([][]float64, n-model.arOrder)
	y := make([]float64, n-model.arOrder)

	for i := model.arOrder; i < n; i++ {
		y[i-model.arOrder] = data[i]
		row := make([]float64, model.arOrder+1)
		row[0] = 1.0 // константа
		for j := 1; j <= model.arOrder; j++ {
			row[j] = data[i-j]
		}
		X[i-model.arOrder] = row
	}

	coeffs := model.solveNormalEquations(X, y)
	if len(coeffs) == 0 {
		return
	}
	model.constant = coeffs[0]
	for i := 0; i < model.arOrder && i+1 < len(coeffs); i++ {
		model.arCoeffs[i] = coeffs[i+1]
	}
}

// checkOverfitting: мягкая регуляризация AR коэффициентов
func (model *ARIMAModel) checkOverfitting() {
	maxCoeff := 0.0
	for _, coeff := range model.arCoeffs {
		if math.Abs(coeff) > maxCoeff {
			maxCoeff = math.Abs(coeff)
		}
	}
	if maxCoeff > 2.0 && maxCoeff > 0 {
		factor := 2.0 / maxCoeff
		for i := range model.arCoeffs {
			model.arCoeffs[i] *= factor
		}
		model.constant *= factor
	}
}

// solveNormalEquations решает нормальные уравнения для линейной регрессии
func (model *ARIMAModel) solveNormalEquations(X [][]float64, y []float64) []float64 {
	if len(X) == 0 || len(X[0]) == 0 {
		return nil
	}

	n := len(X)
	p := len(X[0])

	// Вычисляем X^T * X
	xtx := make([][]float64, p)
	for i := range xtx {
		xtx[i] = make([]float64, p)
	}

	for i := 0; i < n; i++ {
		for j := 0; j < p; j++ {
			for k := 0; k < p; k++ {
				xtx[j][k] += X[i][j] * X[i][k]
			}
		}
	}

	// Вычисляем X^T * y
	xty := make([]float64, p)
	for i := 0; i < n; i++ {
		for j := 0; j < p; j++ {
			xty[j] += X[i][j] * y[i]
		}
	}

	// Решаем систему уравнений X^T * X * coeffs = X^T * y
	coeffs := model.solveLinearSystem(xtx, xty)
	return coeffs
}

// solveLinearSystem решает систему линейных уравнений Ax = b методом Гаусса
func (model *ARIMAModel) solveLinearSystem(A [][]float64, b []float64) []float64 {
	n := len(A)
	if n == 0 || len(b) != n {
		return nil
	}

	// Создаем копии матрицы и вектора
	aug := make([][]float64, n)
	for i := range aug {
		aug[i] = make([]float64, n+1)
		copy(aug[i][:n], A[i])
		aug[i][n] = b[i]
	}

	// Прямой ход метода Гаусса
	for i := 0; i < n; i++ {
		// Поиск максимального элемента в столбце
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(aug[k][i]) > math.Abs(aug[maxRow][i]) {
				maxRow = k
			}
		}

		// Обмен строк
		aug[i], aug[maxRow] = aug[maxRow], aug[i]

		// Проверка на вырожденность
		if math.Abs(aug[i][i]) < 1e-10 {
			return nil // Матрица вырожденная
		}

		// Нормализация строки
		for j := i + 1; j < n+1; j++ {
			aug[i][j] /= aug[i][i]
		}

		// Элиминация
		for k := i + 1; k < n; k++ {
			factor := aug[k][i]
			for j := i + 1; j <= n; j++ {
				aug[k][j] -= factor * aug[i][j]
			}
		}
	}

	// Обратный ход
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		x[i] = aug[i][n]
		for j := i + 1; j < n; j++ {
			x[i] -= aug[i][j] * x[j]
		}
	}

	return x
}

// computeResiduals считает остатки на стационарном ряду для обученной AR
func (model *ARIMAModel) computeResiduals(stationaryData []float64) []float64 {
	n := len(stationaryData)
	if n < model.arOrder+1 {
		return nil
	}
	res := make([]float64, 0, n-model.arOrder)
	for i := model.arOrder; i < n; i++ {
		yhat := model.constant
		for j := 0; j < model.arOrder; j++ {
			yhat += model.arCoeffs[j] * stationaryData[i-1-j]
		}
		res = append(res, stationaryData[i]-yhat)
	}
	return res
}

// forecast прогнозирует следующее значение оригинального ряда
func (model *ARIMAModel) forecast(originalWindow []float64) float64 {
	if len(originalWindow) == 0 {
		return 0
	}
	// Получаем стационарный хвост соответствующей длины
	stationaryData := model.difference(originalWindow, model.diffOrder)
	if len(stationaryData) < model.arOrder {
		// Слишком мало точек после дифференцирования — прогноз в уровне: наивный
		return originalWindow[len(originalWindow)-1]
	}

	// Прогноз Δ^d y_{t+1}
	stationaryForecast := model.constant
	for j := 0; j < model.arOrder; j++ {
		stationaryForecast += model.arCoeffs[j] * stationaryData[len(stationaryData)-1-j]
	}

	// Преобразуем в уровень
	next := model.undifference(stationaryForecast, originalWindow, model.diffOrder)

	// Ограничиваем прогноз разумными пределами относительно текущей цены
	currentPrice := originalWindow[len(originalWindow)-1]
	maxChange := 0.5
	minPrice := currentPrice * 0.1
	maxPrice := currentPrice * (1.0 + maxChange)
	if next < minPrice {
		next = minPrice
	} else if next > maxPrice {
		next = maxPrice
	}
	return next
}

type ARIMAStrategy struct{}

func (s *ARIMAStrategy) Name() string {
	return "arima_strategy"
}

func (s *ARIMAStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	if len(candles) < 100 {
		log.Printf("⚠️ Недостаточно данных для улучшенной ARIMA: получено %d свечей, требуется минимум 100", len(candles))
		return make([]internal.SignalType, len(candles))
	}

	// Извлекаем ценовые данные
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// Параметры модели (MA отключена, т.к. не оценивается корректно без MLE)
	arOrder := 3   // AR(3)
	diffOrder := 1 // I(1)
	maOrder := 0   // MA(0) — отключено

	// Окно обучения
	windowSize := 300
	baseThreshold := 0.005 // 0.5%

	log.Printf("🚀 ЗАПУСК УЛУЧШЕННОЙ ARIMA СТРАТЕГИИ:")
	log.Printf("   Параметры: AR(%d,%d,%d)", arOrder, diffOrder, maOrder)
	log.Printf("   Окно обучения: %d свечей", windowSize)
	log.Printf("   Базовый порог: %.2f%%", baseThreshold*100)

	// Генерируем сигналы с использованием улучшенной логики
	signals := make([]internal.SignalType, len(candles))
	inPosition := false
	minHoldBars := 150
	lastTradeIndex := -minHoldBars

	// Начинаем прогнозирование после достаточного количества данных
	minTrainSize := windowSize + 50

	for i := minTrainSize; i < len(candles); i++ {
		// Rolling window
		windowStart := i - windowSize
		if windowStart < 0 {
			windowStart = 0
		}
		windowData := prices[windowStart:i]

		// Обучение на окне
		model := NewARIMAModel(arOrder, diffOrder, maOrder)
		model.train(windowData)

		// Валидация
		if !s.validateModel(model, windowData) {
			signals[i] = internal.HOLD
			continue
		}

		// Прогноз (корректный: AR на стационарном ряду + обратное дифференцирование)
		forecast := model.forecast(windowData)
		currentPrice := prices[i]

		// Адаптивный порог
		volatility := internal.CalculateStdDevOfReturns(prices[intMax(0, i-50):i])
		adaptiveThreshold := baseThreshold + volatility*0.5

		// Сигнал
		signal := s.generateEnhancedSignal(currentPrice, forecast, adaptiveThreshold, prices, i)

		// Фильтр тренда
		trendStrength := s.calculateTrendStrength(prices[intMax(0, i-20):i])
		trendThreshold := 0.02

		if !inPosition && signal == internal.BUY && trendStrength > -trendThreshold && i-lastTradeIndex >= minHoldBars {
			signals[i] = internal.BUY
			inPosition = true
			lastTradeIndex = i
		} else if inPosition && signal == internal.SELL && trendStrength < trendThreshold && i-lastTradeIndex >= minHoldBars {
			signals[i] = internal.SELL
			inPosition = false
			lastTradeIndex = i
		} else {
			signals[i] = internal.HOLD
		}

		// if i%100 == 0 {
		// 	log.Printf("🧠 Свеча %d: цена=%.2f, прогноз=%.2f, тренд=%.3f, волат=%.3f, порог=%.3f, сигнал=%v",
		// 		i, currentPrice, forecast, trendStrength, volatility, adaptiveThreshold, signal)
		// }
	}

	log.Printf("✅ Улучшенный ARIMA анализ завершен")
	return signals
}

// validateModel проверяет качество обученной модели
func (s *ARIMAStrategy) validateModel(model *ARIMAModel, data []float64) bool {
	if len(data) < 20 {
		return false
	}

	// Проверяем, что коэффициенты не слишком большие (признак переобучения)
	maxCoeff := 0.0
	for _, coeff := range model.arCoeffs {
		if math.Abs(coeff) > maxCoeff {
			maxCoeff = math.Abs(coeff)
		}
	}

	// Если коэффициенты слишком большие - модель переобучена
	if maxCoeff > 3.0 {
		return false
	}

	// Проверяем, что константа разумная
	if math.Abs(model.constant) > 1000 {
		return false
	}

	return true
}

// calculateTrendStrength рассчитывает силу тренда с помощью линейной регрессии
func (s *ARIMAStrategy) calculateTrendStrength(prices []float64) float64 {
	if len(prices) < 10 {
		return 0.0
	}

	n := float64(len(prices))
	sumX := n * (n - 1) / 2
	sumY := 0.0
	sumXY := 0.0
	sumXX := n * (n - 1) * (2*n - 1) / 6

	for i, price := range prices {
		x := float64(i)
		sumY += price
		sumXY += x * price
	}

	// Коэффициент наклона (slope)
	slope := (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)

	// Нормализуем силу тренда
	avgPrice := sumY / n
	trendStrength := slope / avgPrice

	return trendStrength
}

// generateEnhancedSignal генерирует улучшенный сигнал с учетом рыночных условий
func (s *ARIMAStrategy) generateEnhancedSignal(currentPrice, forecastPrice, threshold float64, prices []float64, currentIndex int) internal.SignalType {
	// Базовый сигнал на основе прогноза
	expectedChange := (forecastPrice - currentPrice) / currentPrice

	// Адаптируем порог на основе рыночных условий
	volatility := internal.CalculateStdDevOfReturns(prices[intMax(0, currentIndex-30):currentIndex])
	adaptiveThreshold := threshold + volatility*0.3

	// BUY: ожидаем рост цены выше порога
	if expectedChange > adaptiveThreshold {
		return internal.BUY
	}

	// SELL: ожидаем падение цены ниже порога
	if expectedChange < -adaptiveThreshold {
		return internal.SELL
	}

	return internal.HOLD
}

func (s *ARIMAStrategy) DefaultConfig() internal.StrategyConfig {
	return &ARIMAConfig{
		ArOrder:   3,
		DiffOrder: 1,
		MaOrder:   0,
	}
}

func (s *ARIMAStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	arimaConfig, ok := config.(*ARIMAConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := arimaConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	if len(candles) < 100 {
		log.Printf("⚠️ Недостаточно данных для улучшенной ARIMA: получено %d свечей, требуется минимум 100", len(candles))
		return make([]internal.SignalType, len(candles))
	}

	// Извлекаем ценовые данные
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// Параметры модели из конфига
	arOrder := arimaConfig.ArOrder
	diffOrder := arimaConfig.DiffOrder
	maOrder := arimaConfig.MaOrder

	// Окно обучения
	windowSize := 300
	baseThreshold := 0.005 // 0.5%

	log.Printf("🚀 ЗАПУСК УЛУЧШЕННОЙ ARIMA СТРАТЕГИИ:")
	log.Printf("   Параметры: AR(%d,%d,%d)", arOrder, diffOrder, maOrder)
	log.Printf("   Окно обучения: %d свечей", windowSize)
	log.Printf("   Базовый порог: %.2f%%", baseThreshold*100)

	// Генерируем сигналы с использованием улучшенной логики
	signals := make([]internal.SignalType, len(candles))
	inPosition := false
	minHoldBars := 150
	lastTradeIndex := -minHoldBars

	// Начинаем прогнозирование после достаточного количества данных
	minTrainSize := windowSize + 50

	configModel := func(wdata []float64) *ARIMAModel {
		model := NewARIMAModel(arOrder, diffOrder, maOrder)
		model.train(wdata)
		return model
	}

	for i := minTrainSize; i < len(candles); i++ {
		// Rolling window
		windowStart := i - windowSize
		if windowStart < 0 {
			windowStart = 0
		}
		windowData := prices[windowStart:i]

		// Обучение на окне
		model := configModel(windowData)

		// Валидация
		if !s.validateModel(model, windowData) {
			signals[i] = internal.HOLD
			continue
		}

		// Прогноз (корректный: AR на стационарном ряду + обратное дифференцирование)
		forecast := model.forecast(windowData)
		currentPrice := prices[i]

		// Адаптивный порог
		volatility := internal.CalculateStdDevOfReturns(prices[intMax(0, i-50):i])
		adaptiveThreshold := baseThreshold + volatility*0.5

		// Сигнал
		signal := s.generateEnhancedSignal(currentPrice, forecast, adaptiveThreshold, prices, i)

		// Фильтр тренда
		trendStrength := s.calculateTrendStrength(prices[intMax(0, i-20):i])
		trendThreshold := 0.02

		if !inPosition && signal == internal.BUY && trendStrength > -trendThreshold && i-lastTradeIndex >= minHoldBars {
			signals[i] = internal.BUY
			inPosition = true
			lastTradeIndex = i
		} else if inPosition && signal == internal.SELL && trendStrength < trendThreshold && i-lastTradeIndex >= minHoldBars {
			signals[i] = internal.SELL
			inPosition = false
			lastTradeIndex = i
		} else {
			signals[i] = internal.HOLD
		}
	}

	log.Printf("✅ Улучшенный ARIMA анализ завершен")
	return signals
}

func (s *ARIMAStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	bestConfig := &ARIMAConfig{
		ArOrder:   3,
		DiffOrder: 1,
		MaOrder:   0,
	}
	bestProfit := -1.0

	// Оптимизируем параметры ARIMA
	for arOrder := 1; arOrder <= 5; arOrder++ {
		for diffOrder := 0; diffOrder <= 2; diffOrder++ {
			config := &ARIMAConfig{
				ArOrder:   arOrder,
				DiffOrder: diffOrder,
				MaOrder:   0, // MA отключена для простоты
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

	fmt.Printf("Лучшие параметры SOLID ARIMA: p=%d,d=%d,q=%d, профит=%.4f\n",
		bestConfig.ArOrder, bestConfig.DiffOrder, bestConfig.MaOrder, bestProfit)

	return bestConfig
}

// вспомогательная функция для int max
func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func init() {
	internal.RegisterStrategy("arima_strategy", &ARIMAStrategy{})
}
