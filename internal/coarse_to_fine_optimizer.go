// coarse_to_fine_optimizer.go
// Двухэтапный оптимизатор: грубая сетка -> мелкая сетка в области оптимума

package internal

import (
	"fmt"
	"log"

	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
)

// CoarseToFineOptimizer - двухэтапный оптимизатор
type CoarseToFineOptimizer struct {
	slippageProvider      *SlippageProvider
	coarseConfigGenerator func() []StrategyConfigV2                      // грубая сетка
	fineConfigGenerator   func(best StrategyConfigV2) []StrategyConfigV2 // мелкая сетка вокруг оптимума
	topN                  int                                            // количество лучших конфигураций для второго этапа
}

// NewCoarseToFineOptimizer создает новый двухэтапный оптимизатор
func NewCoarseToFineOptimizer(
	slippageProvider *SlippageProvider,
	coarseConfigGenerator func() []StrategyConfigV2,
	fineConfigGenerator func(best StrategyConfigV2) []StrategyConfigV2,
	topN int,
) *CoarseToFineOptimizer {
	if topN <= 0 {
		topN = 3 // по умолчанию берем топ-3
	}
	return &CoarseToFineOptimizer{
		slippageProvider:      slippageProvider,
		coarseConfigGenerator: coarseConfigGenerator,
		fineConfigGenerator:   fineConfigGenerator,
		topN:                  topN,
	}
}

// Optimize выполняет двухэтапную оптимизацию
func (ctfo *CoarseToFineOptimizer) Optimize(candles []Candle, generator SignalGenerator) StrategyConfigV2 {
	// ========== ЭТАП 1: ГРУБАЯ СЕТКА ==========
	log.Printf("🔍 Этап 1: Грубая сетка поиска...")
	coarseConfigs := ctfo.coarseConfigGenerator()

	// Фильтруем валидные конфигурации
	validCoarseConfigs := lo.Filter(coarseConfigs, func(cfg StrategyConfigV2, _ int) bool {
		return cfg.Validate() == nil
	})

	if len(validCoarseConfigs) == 0 {
		log.Println("⚠️ Нет валидных конфигураций для грубой сетки")
		return nil
	}

	log.Printf("   Тестируем %d конфигураций грубой сетки...", len(validCoarseConfigs))

	// Параллельно тестируем все конфигурации грубой сетки
	coarseResults := lop.Map(validCoarseConfigs, func(cfg StrategyConfigV2, _ int) lo.Tuple2[StrategyConfigV2, float64] {
		signals := generator.GenerateSignals(candles, cfg)
		result := Backtest(candles, signals, ctfo.slippageProvider.GetSlippage())
		return lo.Tuple2[StrategyConfigV2, float64]{A: cfg, B: result.TotalProfit}
	})

	// Сортируем по прибыли и берем топ-N
	sortedCoarse := make([]lo.Tuple2[StrategyConfigV2, float64], len(coarseResults))
	copy(sortedCoarse, coarseResults)

	// Сортируем вручную по убыванию прибыли
	for i := 0; i < len(sortedCoarse); i++ {
		for j := i + 1; j < len(sortedCoarse); j++ {
			if sortedCoarse[j].B > sortedCoarse[i].B {
				sortedCoarse[i], sortedCoarse[j] = sortedCoarse[j], sortedCoarse[i]
			}
		}
	}

	topNConfigs := sortedCoarse
	if len(sortedCoarse) > ctfo.topN {
		topNConfigs = sortedCoarse[:ctfo.topN]
	}

	log.Printf("   ✅ Топ-%d конфигураций грубой сетки:", len(topNConfigs))
	for i, cfg := range topNConfigs {
		log.Printf("      %d. %s -> прибыль: %.4f", i+1, cfg.A.String(), cfg.B)
	}

	// ========== ЭТАП 2: МЕЛКАЯ СЕТКА ==========
	log.Printf("🔬 Этап 2: Мелкая сетка вокруг оптимумов...")

	allFineConfigs := make([]StrategyConfigV2, 0)

	// Генерируем мелкую сетку вокруг каждой из топ-N конфигураций
	for i, topConfig := range topNConfigs {
		log.Printf("   Генерируем мелкую сетку вокруг конфигурации #%d...", i+1)
		fineConfigs := ctfo.fineConfigGenerator(topConfig.A)

		// Фильтруем валидные
		validFineConfigs := lo.Filter(fineConfigs, func(cfg StrategyConfigV2, _ int) bool {
			return cfg.Validate() == nil
		})

		log.Printf("      Добавлено %d конфигураций мелкой сетки", len(validFineConfigs))
		allFineConfigs = append(allFineConfigs, validFineConfigs...)
	}

	if len(allFineConfigs) == 0 {
		log.Println("   ⚠️ Нет валидных конфигураций для мелкой сетки, возвращаем лучшую из грубой")
		return topNConfigs[0].A
	}

	log.Printf("   Тестируем %d конфигураций мелкой сетки...", len(allFineConfigs))

	// Параллельно тестируем все конфигурации мелкой сетки
	fineResults := lop.Map(allFineConfigs, func(cfg StrategyConfigV2, _ int) lo.Tuple2[StrategyConfigV2, float64] {
		signals := generator.GenerateSignals(candles, cfg)
		result := Backtest(candles, signals, ctfo.slippageProvider.GetSlippage())
		return lo.Tuple2[StrategyConfigV2, float64]{A: cfg, B: result.TotalProfit}
	})

	// Находим лучшую конфигурацию из мелкой сетки
	bestFine := lo.MaxBy(fineResults, func(a, b lo.Tuple2[StrategyConfigV2, float64]) bool {
		return a.B > b.B
	})

	// Сравниваем с лучшей из грубой сетки
	bestCoarse := topNConfigs[0]

	var finalBest lo.Tuple2[StrategyConfigV2, float64]
	if bestFine.B > bestCoarse.B {
		finalBest = bestFine
		log.Printf("   ✅ Мелкая сетка улучшила результат: %.4f -> %.4f (+%.4f)", bestCoarse.B, bestFine.B, bestFine.B-bestCoarse.B)
	} else {
		finalBest = bestCoarse
		log.Printf("   ℹ️ Грубая сетка дала лучший результат: %.4f (мелкая: %.4f)", bestCoarse.B, bestFine.B)
	}

	fmt.Printf("🎯 Лучшая конфигурация: %s с прибылью: %.4f\n", finalBest.A.String(), finalBest.B)
	return finalBest.A
}

// AdaptiveCoarseToFineOptimizer - адаптивный оптимизатор с автоматическим определением области поиска
type AdaptiveCoarseToFineOptimizer struct {
	slippageProvider      *SlippageProvider
	coarseConfigGenerator func() []StrategyConfigV2
	fineConfigGenerator   func(best StrategyConfigV2) []StrategyConfigV2
	topN                  int
	improvementThreshold  float64 // порог улучшения для продолжения поиска
}

// NewAdaptiveCoarseToFineOptimizer создает адаптивный оптимизатор
func NewAdaptiveCoarseToFineOptimizer(
	slippageProvider *SlippageProvider,
	coarseConfigGenerator func() []StrategyConfigV2,
	fineConfigGenerator func(best StrategyConfigV2) []StrategyConfigV2,
	topN int,
	improvementThreshold float64,
) *AdaptiveCoarseToFineOptimizer {
	if topN <= 0 {
		topN = 3
	}
	if improvementThreshold <= 0 {
		improvementThreshold = 0.01 // 1% улучшение по умолчанию
	}
	return &AdaptiveCoarseToFineOptimizer{
		slippageProvider:      slippageProvider,
		coarseConfigGenerator: coarseConfigGenerator,
		fineConfigGenerator:   fineConfigGenerator,
		topN:                  topN,
		improvementThreshold:  improvementThreshold,
	}
}

// Optimize выполняет адаптивную многоэтапную оптимизацию
func (actfo *AdaptiveCoarseToFineOptimizer) Optimize(candles []Candle, generator SignalGenerator) StrategyConfigV2 {
	// Этап 1: грубая сетка (как обычно)
	log.Printf("🔍 Этап 1: Грубая сетка поиска...")
	coarseConfigs := actfo.coarseConfigGenerator()

	validCoarseConfigs := lo.Filter(coarseConfigs, func(cfg StrategyConfigV2, _ int) bool {
		return cfg.Validate() == nil
	})

	if len(validCoarseConfigs) == 0 {
		log.Println("⚠️ Нет валидных конфигураций")
		return nil
	}

	log.Printf("   Тестируем %d конфигураций...", len(validCoarseConfigs))

	coarseResults := lop.Map(validCoarseConfigs, func(cfg StrategyConfigV2, _ int) lo.Tuple2[StrategyConfigV2, float64] {
		signals := generator.GenerateSignals(candles, cfg)
		result := Backtest(candles, signals, actfo.slippageProvider.GetSlippage())
		return lo.Tuple2[StrategyConfigV2, float64]{A: cfg, B: result.TotalProfit}
	})

	sortedCoarse := make([]lo.Tuple2[StrategyConfigV2, float64], len(coarseResults))
	copy(sortedCoarse, coarseResults)

	// Сортируем вручную по убыванию прибыли
	for i := 0; i < len(sortedCoarse); i++ {
		for j := i + 1; j < len(sortedCoarse); j++ {
			if sortedCoarse[j].B > sortedCoarse[i].B {
				sortedCoarse[i], sortedCoarse[j] = sortedCoarse[j], sortedCoarse[i]
			}
		}
	}

	currentBest := sortedCoarse[0]
	log.Printf("   ✅ Лучшая конфигурация грубой сетки: %s -> %.4f", currentBest.A.String(), currentBest.B)

	// Этап 2+: итеративное уточнение
	iteration := 1
	maxIterations := 3

	for iteration <= maxIterations {
		log.Printf("🔬 Этап %d: Уточнение вокруг текущего оптимума...", iteration+1)

		fineConfigs := actfo.fineConfigGenerator(currentBest.A)
		validFineConfigs := lo.Filter(fineConfigs, func(cfg StrategyConfigV2, _ int) bool {
			return cfg.Validate() == nil
		})

		if len(validFineConfigs) == 0 {
			log.Printf("   ⚠️ Нет конфигураций для уточнения, останавливаемся")
			break
		}

		log.Printf("   Тестируем %d конфигураций...", len(validFineConfigs))

		fineResults := lop.Map(validFineConfigs, func(cfg StrategyConfigV2, _ int) lo.Tuple2[StrategyConfigV2, float64] {
			signals := generator.GenerateSignals(candles, cfg)
			result := Backtest(candles, signals, actfo.slippageProvider.GetSlippage())
			return lo.Tuple2[StrategyConfigV2, float64]{A: cfg, B: result.TotalProfit}
		})

		bestFine := lo.MaxBy(fineResults, func(a, b lo.Tuple2[StrategyConfigV2, float64]) bool {
			return a.B > b.B
		})

		improvement := (bestFine.B - currentBest.B) / Max(Abs(currentBest.B), 0.0001)

		if improvement > actfo.improvementThreshold {
			log.Printf("   ✅ Улучшение: %.4f -> %.4f (+%.2f%%)", currentBest.B, bestFine.B, improvement*100)
			currentBest = bestFine
			iteration++
		} else {
			log.Printf("   ℹ️ Улучшение незначительно (%.2f%%), останавливаемся", improvement*100)
			break
		}
	}

	fmt.Printf("🎯 Финальная конфигурация: %s с прибылью: %.4f\n", currentBest.A.String(), currentBest.B)
	return currentBest.A
}
