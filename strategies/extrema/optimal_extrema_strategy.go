// strategies/optimal_extrema_strategy.go — стратегия поиска оптимальных пар точек покупки/продажи
//
// Описание стратегии:
// Стратегия находит оптимальные пары точек покупки/продажи на основе локальных экстремумов.
// Алгоритм гарантирует, что между точкой покупки и продажи нет более выгодных точек.
//
// Как работает:
// 1. Находит все локальные минимумы (точки покупки) и максимумы (точки продажи)
// 2. Формирует последовательность чередующихся экстремумов
// 3. Проверяет оптимальность интервалов между парами
// 4. Устраняет пересечения и дубликаты
//
// Преимущества подхода:
// - Гарантирует отсутствие более выгодных точек в интервале
// - Простая и понятная логика принятия решений
// - Минимизирует количество ложных сигналов
// - Работает с любыми финансовыми инструментами
//
// Параметры:
// - Нет настраиваемых параметров (алгоритм детерминирован)

package extrema

import (
	"bt/internal"
	"log"
)

type OptimalExtremaConfig struct{}

func (c *OptimalExtremaConfig) Validate() error {
	return nil // No parameters to validate
}

func (c *OptimalExtremaConfig) DefaultConfigString() string {
	return "OptimalExtrema()"
}

// OptimalExtremaPoint представляет точку экстремума
type OptimalExtremaPoint struct {
	Index int     // индекс свечи
	Price float64 // цена (Low для минимума, High для максимума)
	IsBuy bool    // true для точки покупки (минимум), false для продажи (максимум)
}

// OptimalExtremaStrategy реализует стратегию поиска оптимальных пар точек
type OptimalExtremaStrategy struct{}

func (s *OptimalExtremaStrategy) Name() string {
	return "optimal_extrema_strategy"
}

// findPotentialExtrema находит потенциальные локальные экстремумы
func (s *OptimalExtremaStrategy) findPotentialExtrema(candles []internal.Candle) ([]OptimalExtremaPoint, []OptimalExtremaPoint) {
	var potentialMinima []OptimalExtremaPoint
	var potentialMaxima []OptimalExtremaPoint

	// Находим локальные минимумы и максимумы
	for i := 1; i < len(candles)-1; i++ {
		currentLow := candles[i].Low.ToFloat64()
		currentHigh := candles[i].High.ToFloat64()
		prevLow := candles[i-1].Low.ToFloat64()
		prevHigh := candles[i-1].High.ToFloat64()
		nextLow := candles[i+1].Low.ToFloat64()
		nextHigh := candles[i+1].High.ToFloat64()

		// Локальный минимум (точка покупки)
		if currentLow < prevLow && currentLow < nextLow {
			potentialMinima = append(potentialMinima, OptimalExtremaPoint{
				Index: i,
				Price: currentLow,
				IsBuy: true,
			})
		}

		// Локальный максимум (точка продажи)
		if currentHigh > prevHigh && currentHigh > nextHigh {
			potentialMaxima = append(potentialMaxima, OptimalExtremaPoint{
				Index: i,
				Price: currentHigh,
				IsBuy: false,
			})
		}
	}

	return potentialMinima, potentialMaxima
}

// createAlternatingSequence формирует последовательность чередующихся экстремумов
func (s *OptimalExtremaStrategy) createAlternatingSequence(minima, maxima []OptimalExtremaPoint) []OptimalExtremaPoint {
	var sequence []OptimalExtremaPoint

	// Находим индексы начала последовательности
	minIdx := 0
	maxIdx := 0

	// Находим первый минимум, который раньше первого максимума
	if len(minima) > 0 && len(maxima) > 0 {
		if minima[0].Index < maxima[0].Index {
			sequence = append(sequence, minima[0])
			minIdx = 1
		}
	} else if len(minima) > 0 {
		sequence = append(sequence, minima[0])
		minIdx = 1
	}

	// Чередуем минимумы и максимумы
	for len(sequence) > 0 {
		lastIsBuy := sequence[len(sequence)-1].IsBuy

		if lastIsBuy {
			// Последний был минимум, ищем следующий максимум
			found := false
			for maxIdx < len(maxima) {
				if maxima[maxIdx].Index > sequence[len(sequence)-1].Index {
					sequence = append(sequence, maxima[maxIdx])
					maxIdx++
					found = true
					break
				}
				maxIdx++
			}
			if !found {
				break
			}
		} else {
			// Последний был максимум, ищем следующий минимум
			found := false
			for minIdx < len(minima) {
				if minima[minIdx].Index > sequence[len(sequence)-1].Index {
					sequence = append(sequence, minima[minIdx])
					minIdx++
					found = true
					break
				}
				minIdx++
			}
			if !found {
				break
			}
		}
	}

	return sequence
}

// validateOptimalInterval проверяет оптимальность интервала между парой экстремумов
func (s *OptimalExtremaStrategy) validateOptimalInterval(candles []internal.Candle, buyPoint, sellPoint OptimalExtremaPoint) bool {
	buyIndex := buyPoint.Index
	sellIndex := sellPoint.Index

	if buyIndex >= sellIndex {
		return false
	}

	buyPrice := buyPoint.Price

	// Проверяем, что в интервале нет цен ниже точки покупки
	for i := buyIndex; i <= sellIndex; i++ {
		if candles[i].Low.ToFloat64() < buyPrice {
			return false
		}
	}

	sellPrice := sellPoint.Price

	// Проверяем, что в интервале нет цен выше точки продажи
	for i := buyIndex; i <= sellIndex; i++ {
		if candles[i].High.ToFloat64() > sellPrice {
			return false
		}
	}

	return true
}

// removeOverlapsAndDuplicates удаляет пересечения и дубликаты из списка пар
func (s *OptimalExtremaStrategy) removeOverlapsAndDuplicates(pairs []OptimalExtremaPoint) []OptimalExtremaPoint {
	if len(pairs) < 2 {
		return pairs
	}

	var filtered []OptimalExtremaPoint
	var lastSellIndex = -1

	for _, point := range pairs {
		if point.IsBuy {
			// Точка покупки - проверяем, что она после последней продажи
			if point.Index > lastSellIndex {
				filtered = append(filtered, point)
			}
		} else {
			// Точка продажи - всегда добавляем и обновляем lastSellIndex
			filtered = append(filtered, point)
			lastSellIndex = point.Index
		}
	}

	return filtered
}

// Optimize выполняет оптимизацию параметров стратегии (в данной стратегии параметры не требуются)
func (s *OptimalExtremaStrategy) DefaultConfig() internal.StrategyConfig {
	return &OptimalExtremaConfig{}
}

func (s *OptimalExtremaStrategy) GenerateSignalsWithConfig(candles []internal.Candle, config internal.StrategyConfig) []internal.SignalType {
	optimalExtremaConfig, ok := config.(*OptimalExtremaConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := optimalExtremaConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	// Шаг 1: Подготовка данных
	if len(candles) < 3 {
		log.Printf("⚠️ Недостаточно данных для анализа: получено %d свечей, требуется минимум 3", len(candles))
		return make([]internal.SignalType, len(candles))
	}

	// Шаг 2: Поиск потенциальных экстремумов
	potentialMinima, potentialMaxima := s.findPotentialExtrema(candles)

	log.Printf("🔍 Найдено потенциальных минимумов: %d, максимумов: %d", len(potentialMinima), len(potentialMaxima))

	// Шаг 3: Фильтрация и чередование экстремумов
	sequence := s.createAlternatingSequence(potentialMinima, potentialMaxima)

	// Удаляем некорректные начальные точки (если первый экстремум - максимум)
	if len(sequence) > 0 && !sequence[0].IsBuy {
		sequence = sequence[1:]
	}

	log.Printf("📊 Сформирована последовательность из %d экстремумов", len(sequence))

	// Шаг 4: Проверка оптимальности интервалов
	var optimalPairs []OptimalExtremaPoint
	for i := 0; i < len(sequence)-1; i++ {
		if sequence[i].IsBuy && !sequence[i+1].IsBuy {
			// Пара: покупка -> продажа
			if s.validateOptimalInterval(candles, sequence[i], sequence[i+1]) {
				optimalPairs = append(optimalPairs, sequence[i], sequence[i+1])
			}
		}
	}

	log.Printf("✅ Найдено %d оптимальных пар (покупка -> продажа)", len(optimalPairs)/2)

	// Шаг 5: Устранение пересечений и повторов
	optimalPairs = s.removeOverlapsAndDuplicates(optimalPairs)

	// Шаг 6: Генерация сигналов
	signals := make([]internal.SignalType, len(candles))

	for i := 0; i < len(optimalPairs); i += 2 {
		if i+1 < len(optimalPairs) {
			buyIndex := optimalPairs[i].Index
			sellIndex := optimalPairs[i+1].Index

			if buyIndex < len(signals) && sellIndex < len(signals) {
				signals[buyIndex] = internal.BUY
				signals[sellIndex] = internal.SELL
			}
		}
	}

	// Вывод статистики
	buyCount := 0
	sellCount := 0
	for _, signal := range signals {
		switch signal {
		case internal.BUY:
			buyCount++
		case internal.SELL:
			sellCount++
		}
	}

	log.Printf("📈 Сгенерировано сигналов: BUY=%d, SELL=%d", buyCount, sellCount)

	return signals
}

func (s *OptimalExtremaStrategy) OptimizeWithConfig(candles []internal.Candle) internal.StrategyConfig {
	log.Printf("🔧 Оптимизация параметров для optimal_extrema_strategy (параметры не требуются)")
	var bestConfig *OptimalExtremaConfig
	var bestProfit float64 = -1.0

	// Single configuration since no parameters
	config := &OptimalExtremaConfig{}
	if config.Validate() == nil {
		signals := s.GenerateSignalsWithConfig(candles, config)
		result := internal.Backtest(candles, signals, 0.01)
		if result.TotalProfit >= bestProfit {
			bestProfit = result.TotalProfit
			bestConfig = config
		}
	}

	log.Printf("Лучшие параметры OptimalExtrema: профит=%.4f", bestProfit)
	return bestConfig
}

func init() {
	internal.RegisterStrategy("optimal_extrema_strategy", &OptimalExtremaStrategy{})
}
