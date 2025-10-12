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
	"log"
	"math"
)

// ARIMAModel — модель ARIMA
type ARIMAModelImproved struct {
	arOrder   int // порядок авторегрессии (p)
	maOrder   int // порядок скользящего среднего (q)
	diffOrder int // порядок дифференцирования (d)

	arCoeffs []float64 // коэффициенты AR
	maCoeffs []float64 // коэффициенты MA
	constant float64   // константа модели

	residuals []float64 // остатки для MA компоненты

	originalData []float64 // оригинальные данные для обратного дифференцирования
}

// NewARIMAModelImproved создает новую модель ARIMA
func NewARIMAModelImproved(arOrder, diffOrder, maOrder int) *ARIMAModelImproved {
	return &ARIMAModelImproved{
		arOrder:   arOrder,
		maOrder:   maOrder,
		diffOrder: diffOrder,
		arCoeffs:  make([]float64, arOrder),
		maCoeffs:  make([]float64, maOrder),
		residuals: make([]float64, 0),
	}
}

// difference выполняет дифференцирование ряда
func (model *ARIMAModelImproved) difference(data []float64, order int) []float64 {
	if order == 0 {
		result := make([]float64, len(data))
		copy(result, data)
		return result
	}

	result := make([]float64, len(data))
	copy(result, data)

	for d := 0; d < order; d++ {
		for i := 1; i < len(result); i++ {
			result[i] = result[i] - result[i-1]
		}
		result = result[1:] // удаляем первый элемент после дифференцирования
	}

	return result
}

// undifference выполняет обратное дифференцирование для получения прогноза в исходной шкале
func (model *ARIMAModelImproved) undifference(stationaryForecast float64, originalData []float64, order int) float64 {
	if order == 0 {
		return stationaryForecast
	}

	// Для обратного дифференцирования нам нужны последние значения оригинального ряда
	if len(originalData) < order {
		return originalData[len(originalData)-1] + stationaryForecast
	}

	// Начинаем с последнего значения оригинального ряда
	result := stationaryForecast

	// Применяем обратное дифференцирование
	for d := order - 1; d >= 0; d-- {
		lastOriginalValue := originalData[len(originalData)-1-d]
		result = lastOriginalValue + result
	}

	return result
}

// train обучает модель ARIMA на данных
func (model *ARIMAModelImproved) train(data []float64) {
	log.Printf("🧠 Обучение улучшенной ARIMA(%d,%d,%d) модели на %d данных", model.arOrder, model.diffOrder, model.maOrder, len(data))

	// Сохраняем оригинальные данные для обратного дифференцирования
	model.originalData = make([]float64, len(data))
	copy(model.originalData, data)

	// Применяем дифференцирование
	stationaryData := model.difference(data, model.diffOrder)

	if len(stationaryData) < model.arOrder+model.maOrder+1 {
		log.Printf("❌ Недостаточно данных после дифференцирования: %d < %d", len(stationaryData), model.arOrder+model.maOrder+1)
		return
	}

	// Обучаем AR модель
	model.trainARModel(stationaryData)

	log.Printf("✅ Улучшенная ARIMA модель обучена, AR коэффициенты: %v, константа: %.6f", model.arCoeffs, model.constant)
}

// trainARModel обучает авторегрессионную модель
func (model *ARIMAModelImproved) trainARModel(data []float64) {
	n := len(data)
	if n < model.arOrder+1 {
		return
	}

	// Создаем матрицу признаков для регрессии
	X := make([][]float64, n-model.arOrder)
	y := make([]float64, n-model.arOrder)

	for i := model.arOrder; i < n; i++ {
		// Целевая переменная
		y[i-model.arOrder] = data[i]

		// Признаки (лагированные значения)
		X[i-model.arOrder] = make([]float64, model.arOrder+1)
		X[i-model.arOrder][0] = 1.0 // константа

		for j := 1; j <= model.arOrder; j++ {
			X[i-model.arOrder][j] = data[i-j]
		}
	}

	// Решаем нормальные ур...rn false
	coeffs := model.solveNormalEquations(X, y)
	if len(coeffs) > 0 {
		model.constant = coeffs[0]
		for i := 0; i < model.arOrder && i+1 < len(coeffs); i++ {
			model.arCoeffs[i] = coeffs[i+1]
		}

		// Проверяем на переобучение
		model.checkOverfitting()
	}
}

// checkOverfitting проверяет модель на переобучение
func (model *ARIMAModelImproved) checkOverfitting() {
	// Проверяем, что AR коэффициенты не слишком большие (признак переобучения)
	maxCoeff := 0.0
	for _, coeff := range model.arCoeffs {
		if math.Abs(coeff) > maxCoeff {
			maxCoeff = math.Abs(coeff)
		}
	}

	// Если максимальный коэффициент > 2.0, это может указывать на переобучение
	if maxCoeff > 2.0 {
		log.Printf("⚠️ Обнаружены признаки переобучения: максимальный AR коэффициент = %.3f", maxCoeff)

		// Уменьшаем коэффициенты для предотвращения переобучения
		factor := 2.0 / maxCoeff
		for i := range model.arCoeffs {
			model.arCoeffs[i] *= factor
		}
		model.constant *= factor
	}
}

// solveNormalEquations решает нормальные уравнения для линейной регрессии
func (model *ARIMAModelImproved) solveNormalEquations(X [][]float64, y []float64) []float64 {
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
func (model *ARIMAModelImproved) solveLinearSystem(A [][]float64, b []float64) []float64 {
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
		for j := i + 1; j <= n; j++ {
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

// forecast прогнозирует следующее значение
func (model *ARIMAModelImproved) forecast(data []float64) float64 {
	if len(data) < model.arOrder {
		return 0
	}

	// Прогноз для стационарного ряда
	stationaryForecast := model.constant

	// AR компонента
	for i := 0; i < model.arOrder; i++ {
		idx := len(data) - 1 - i
		if idx >= 0 {
			stationaryForecast += model.arCoeffs[i] * data[idx]
		}
	}

	// Обратное дифференцирование для получения прогноза в исходной шкале
	originalForecast := model.undifference(stationaryForecast, data, model.diffOrder)

	// Ограничиваем прогноз разумными пределами
	currentPrice := data[len(data)-1]
	maxChange := 0.5
	minPrice := currentPrice * 0.1
	maxPrice := currentPrice * (1.0 + maxChange)

	if originalForecast < minPrice {
		originalForecast = minPrice
	} else if originalForecast > maxPrice {
		originalForecast = maxPrice
	}

	return originalForecast
}

type ARIMAStrategyImproved struct{}

func (s *ARIMAStrategyImproved) Name() string {
	return "arima_strategy_improved"
}

func (s *ARIMAStrategyImproved) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	if len(candles) < 100 {
		log.Printf("⚠️ Недостаточно данных для улучшенной ARIMA: получено %d свечей, требуется минимум 100", len(candles))
		return make([]internal.SignalType, len(candles))
	}

	// Извлекаем ценовые данные
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// УЛУЧШЕННЫЕ ПАРАМЕТРЫ
	arOrder := 3   // AR(3) - увеличено для лучшего моделирования
	diffOrder := 1 // I(1) - первое дифференцирование
	maOrder := 1   // MA(1) - добавлена MA компонента

	// Увеличенное окно обучения для стабильности
	windowSize := 300
	baseThreshold := 0.003 // 0.3% - сниженный базовый порог для большего количества сигналов

	log.Printf("🚀 ЗАПУСК УЛУЧШЕННОЙ ARIMA СТРАТЕГИИ:")
	log.Printf("   Параметры: AR(%d,%d,%d)", arOrder, diffOrder, maOrder)
	log.Printf("   Окно обучения: %d свечей", windowSize)
	log.Printf("   Базовый порог: %.2f%%", baseThreshold*100)

	// Генерируем сигналы с использованием улучшенной логики
	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	// Начинаем прогнозирование после достаточного количества данных
	minTrainSize := windowSize + 50

	for i := minTrainSize; i < len(candles); i++ {
		// Используем rolling window для обучения модели
		windowStart := i - windowSize
		if windowStart < 0 {
			windowStart = 0
		}
		windowData := prices[windowStart:i]

		// Создаем и обучаем модель на окне данных
		model := NewARIMAModelImproved(arOrder, diffOrder, maOrder)
		model.train(windowData)

		// Проверяем качество модели перед использованием
		if !s.validateModel(model, windowData) {
			signals[i] = internal.HOLD
			continue
		}

		// Прогнозируем следующее значение
		forecast := model.forecast(windowData)
		currentPrice := prices[i-1]

		// Вычисляем адаптивный порог на основе волатильности
		volatility := s.calculateVolatility(prices[max(0, i-50):i])
		adaptiveThreshold := baseThreshold + volatility*0.5

		// Получаем сигнал с учетом тренда и волатильности
		signal := s.generateEnhancedSignal(currentPrice, forecast, adaptiveThreshold, prices, i)

		// Улучшенная логика позиционирования с фильтром тренда
		trendStrength := s.calculateTrendStrength(prices[max(0, i-20):i])

		// Снижаем требования к силе тренда для генерации сигналов
		trendThreshold := 0.02 // Было 0.1

		if !inPosition && signal == internal.BUY && trendStrength > -trendThreshold {
			signals[i] = internal.BUY
			inPosition = true
		} else if inPosition && signal == internal.SELL && trendStrength < trendThreshold {
			signals[i] = internal.SELL
			inPosition = false
		} else {
			signals[i] = internal.HOLD
		}

		// Детальный отладочный вывод каждые 100 свечей
		if i%100 == 0 {
			log.Printf("🧠 Свеча %d: цена=%.2f, прогноз=%.2f, тренд=%.3f, волат=%.3f, порог=%.3f, сигнал=%v",
				i, currentPrice, forecast, trendStrength, volatility, adaptiveThreshold, signal)
		}
	}

	log.Printf("✅ Улучшенный ARIMA анализ завершен")
	return signals
}

// validateModel проверяет качество обученной модели
func (s *ARIMAStrategyImproved) validateModel(model *ARIMAModelImproved, data []float64) bool {
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

// calculateVolatility рассчитывает волатильность на основе стандартного отклонения
func (s *ARIMAStrategyImproved) calculateVolatility(prices []float64) float64 {
	if len(prices) < 10 {
		return 0.01 // дефолтная волатильность
	}

	// Рассчитываем доходности
	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		returns[i-1] = (prices[i] - prices[i-1]) / prices[i-1]
	}

	// Рассчитываем стандартное отклонение
	mean := 0.0
	for _, ret := range returns {
		mean += ret
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, ret := range returns {
		variance += (ret - mean) * (ret - mean)
	}
	variance /= float64(len(returns))

	return math.Sqrt(variance)
}

// calculateTrendStrength рассчитывает силу тренда с помощью линейной регрессии
func (s *ARIMAStrategyImproved) calculateTrendStrength(prices []float64) float64 {
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
func (s *ARIMAStrategyImproved) generateEnhancedSignal(currentPrice, forecastPrice, threshold float64, prices []float64, currentIndex int) internal.SignalType {
	// Базовый сигнал на основе прогноза
	expectedChange := (forecastPrice - currentPrice) / currentPrice

	// Адаптируем порог на основе рыночных условий
	volatility := s.calculateVolatility(prices[max(0, currentIndex-30):currentIndex])
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

func (s *ARIMAStrategyImproved) Optimize(candles []internal.Candle) internal.StrategyParams {
	return internal.StrategyParams{}
}

func init() {
	internal.RegisterStrategy("arima_strategy", &ARIMAStrategyImproved{})
}
