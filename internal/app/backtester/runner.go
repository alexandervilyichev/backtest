package backtester

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"bt/internal"
)

// ParallelStrategyRunner — реализация параллельного запуска стратегий
type ParallelStrategyRunner struct {
	debug bool
}

// NewParallelStrategyRunner — конструктор для ParallelStrategyRunner
func NewParallelStrategyRunner(debug bool) *ParallelStrategyRunner {
	return &ParallelStrategyRunner{debug: debug}
}

// RunStrategy — запускает одну стратегию
func (r *ParallelStrategyRunner) RunStrategy(strategyName string, candles []internal.Candle) (*BenchmarkResult, error) {
	strategy := internal.GetStrategy(strategyName)
	if strategy == nil {
		return nil, fmt.Errorf("стратегия %s не найдена", strategyName)
	}

	strategyStartTime := time.Now()

	if r.debug {
		fmt.Printf("🐛 DEBUG: Запуск стратегии %s\n", strategyName)
	}

	// Оптимизация параметров и генерация сигналов
	config := strategy.OptimizeWithConfig(candles)
	signals := strategy.GenerateSignalsWithConfig(candles, config)
	result := internal.Backtest(candles, signals, 0.01) // 0.01 units проскальзывание

	executionTime := time.Since(strategyStartTime)

	return &BenchmarkResult{
		Name:           strategy.Name(),
		TotalProfit:    result.TotalProfit,
		TradeCount:     result.TradeCount,
		FinalPortfolio: result.FinalPortfolio,
		ExecutionTime:  executionTime,
	}, nil
}

// RunAllStrategies — запускает все доступные стратегии параллельно
func (r *ParallelStrategyRunner) RunAllStrategies(candles []internal.Candle) ([]BenchmarkResult, error) {
	fmt.Println("🎯 Запуск всех доступных стратегий для сравнения...")
	fmt.Printf("🔥 Параллельное выполнение на %d ядрах\n", runtime.NumCPU())

	startTime := time.Now()
	strategyNames := internal.GetStrategyNames()
	totalStrategies := len(strategyNames)

	if r.debug {
		fmt.Printf("🐛 DEBUG: Найдено %d стратегий для тестирования: %s\n",
			totalStrategies, strings.Join(strategyNames, ", "))
	}

	// Канал для результатов
	resultsChan := make(chan BenchmarkResult, totalStrategies)
	var wg sync.WaitGroup

	// Запускаем стратегии параллельно
	for _, name := range strategyNames {
		wg.Add(1)

		go func(strategyName string) {
			defer wg.Done()

			if result, err := r.RunStrategy(strategyName, candles); err != nil {
				fmt.Printf("❌ Ошибка при запуске стратегии %s: %v\n", strategyName, err)
				return
			} else {
				resultsChan <- *result
				fmt.Printf("✅ Завершена стратегия: %s (прибыль: %.2f%%, время: %v)\n",
					result.Name, result.TotalProfit*100, result.ExecutionTime)
			}
		}(name)
	}

	// Ждем завершения всех горутин
	wg.Wait()
	close(resultsChan)

	// Собираем результаты
	var results []BenchmarkResult
	completed := 0
	for result := range resultsChan {
		results = append(results, result)
		completed++

		// Показываем прогресс
		if completed%5 == 0 || completed == totalStrategies {
			fmt.Printf("📊 Прогресс: %d/%d стратегий завершено\n", completed, totalStrategies)
		}
	}

	elapsed := time.Since(startTime)
	fmt.Printf("⚡ Все стратегии выполнены за %v\n", elapsed)

	return results, nil
}

// SingleStrategyRunner — реализация запуска одной стратегии с бенчмарком
type SingleStrategyRunner struct {
	debug bool
}

// NewSingleStrategyRunner — конструктор для SingleStrategyRunner
func NewSingleStrategyRunner(debug bool) *SingleStrategyRunner {
	return &SingleStrategyRunner{debug: debug}
}

// RunStrategy — запускает одну стратегию с Buy & Hold бенчмарком
func (r *SingleStrategyRunner) RunStrategy(strategyName string, candles []internal.Candle) (*BenchmarkResult, error) {
	strategy := internal.GetStrategy(strategyName)
	if strategy == nil {
		return nil, fmt.Errorf("стратегия %s не найдена", strategyName)
	}

	fmt.Printf("🎯 Выбрана стратегия: %s\n", strategy.Name())

	startTime := time.Now()

	// Запуск основной стратегии
	config := strategy.OptimizeWithConfig(candles)
	signals := strategy.GenerateSignalsWithConfig(candles, config)
	result := internal.Backtest(candles, signals, 0.01)

	executionTime := time.Since(startTime)

	mainResult := &BenchmarkResult{
		Name:           strategy.Name(),
		TotalProfit:    result.TotalProfit,
		TradeCount:     result.TradeCount,
		FinalPortfolio: result.FinalPortfolio,
		ExecutionTime:  executionTime,
	}

	// Запуск Buy & Hold как бенчмарка
	bnhStrategy := internal.GetStrategy("buy_and_hold")

	bnhConfig := bnhStrategy.DefaultConfig()
	bnhSignals := bnhStrategy.GenerateSignalsWithConfig(candles, bnhConfig)
	internal.Backtest(candles, bnhSignals, 0.01)

	fmt.Printf("⚡ Стратегии выполнены за %v\n", executionTime)

	return mainResult, nil
}

// RunAllStrategies — для интерфейса совместимости (не используется для одиночной стратегии)
func (r *SingleStrategyRunner) RunAllStrategies(candles []internal.Candle) ([]BenchmarkResult, error) {
	return nil, fmt.Errorf("SingleStrategyRunner не поддерживает запуск всех стратегий")
}
