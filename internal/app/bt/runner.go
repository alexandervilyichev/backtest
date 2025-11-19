package backtester

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"bt/internal"
)

// BaseStrategyRunner — базовая структура с общей логикой для запуска стратегий
type BaseStrategyRunner struct {
	debug    bool
	config   Config
	configs  map[string]json.RawMessage // Загруженные конфигурации из файла
	slipping float64                    // Глобальный параметр проскальзывания
}

// loadConfigsFromFile — загружает конфигурации стратегий из JSON файла
func (r *BaseStrategyRunner) loadConfigsFromFile() {
	data, err := os.ReadFile(r.config.ConfigFile)
	if err != nil {
		fmt.Printf("❌ Ошибка чтения файла конфигурации %s: %v\n", r.config.ConfigFile, err)
		return
	}

	var allConfigs map[string]json.RawMessage
	err = json.Unmarshal(data, &allConfigs)
	if err != nil {
		fmt.Printf("❌ Ошибка парсинга JSON файла конфигурации: %v\n", err)
		return
	}

	r.slipping = 0.02
	// Извлекаем глобальный параметр проскальзывания
	if slippingVal, exists := allConfigs["slipping"]; exists {
		if err := json.Unmarshal(slippingVal, &r.slipping); err != nil {
			r.slipping = 0.02 // значение по умолчанию
			fmt.Printf("⚠️  Неверный тип параметра проскальзывания, используем значение по умолчанию: %.4f\n", r.slipping)

		}
	}

	// Удаляем глобальный параметр из конфигураций стратегий
	r.configs = make(map[string]json.RawMessage)
	for key, value := range allConfigs {
		if key != "slipping" {
			r.configs[key] = value
		}
	}

	fmt.Printf("✅ Загружены конфигурации для %d стратегий из %s\n", len(r.configs), r.config.ConfigFile)
}

// runSingleStrategy — общая логика запуска одной стратегии (поддержка V1 и V2)
func (r *BaseStrategyRunner) runSingleStrategy(strategyName string, candles []internal.Candle) (*BenchmarkResult, internal.StrategyConfig, error) {
	// Сначала пробуем V2 стратегию
	if strategyV2, ok := internal.GetStrategyV2(strategyName); ok {
		return r.runStrategyV2(strategyName, strategyV2, candles)
	}

	// Если не найдена V2, используем V1
	strategy := internal.GetStrategy(strategyName)
	strategy.SetSlippage(r.slipping)
	if strategy == nil {
		return nil, nil, fmt.Errorf("стратегия %s не найдена", strategyName)
	}

	strategyStartTime := time.Now()

	if r.debug {
		fmt.Printf("🐛 DEBUG: Запуск стратегии V1 %s\n", strategyName)
	}

	var config internal.StrategyConfig

	// Если есть загруженная конфигурация из файла, используем её
	if r.configs != nil {
		if loadedConfig, exists := r.configs[strategyName]; exists {
			config = strategy.LoadConfigFromMap(loadedConfig)
			if r.debug {
				fmt.Printf("🐛 DEBUG: Используем загруженную конфигурацию для %s\n", strategyName)
			}
		} else {
			if r.debug {
				fmt.Printf("🐛 DEBUG: Конфигурация для %s имеет неверный тип, используем оптимизацию\n", strategyName)
			}
			config = strategy.OptimizeWithConfig(candles)
		}
	} else {
		if r.debug {
			fmt.Printf("🐛 DEBUG: Конфигурация для %s не найдена в файле, используем оптимизацию\n", strategyName)
		}
		config = strategy.OptimizeWithConfig(candles)
	}

	signals := strategy.GenerateSignalsWithConfig(candles, config)
	result := internal.Backtest(candles, signals, strategy.GetSlippage())

	executionTime := time.Since(strategyStartTime)

	return &BenchmarkResult{
		Name:           strategy.Name(),
		TotalProfit:    result.TotalProfit,
		TradeCount:     result.TradeCount,
		FinalPortfolio: result.FinalPortfolio,
		ExecutionTime:  executionTime,
	}, config, nil
}

// runStrategyV2 — запуск стратегии V2 (новая архитектура)
func (r *BaseStrategyRunner) runStrategyV2(strategyName string, strategy internal.TradingStrategy, candles []internal.Candle) (*BenchmarkResult, internal.StrategyConfig, error) {
	strategyStartTime := time.Now()

	if r.debug {
		fmt.Printf("🐛 DEBUG: Запуск стратегии V2 %s\n", strategyName)
	}

	var config internal.StrategyConfigV2

	// Если есть загруженная конфигурация из файла, используем её
	if r.configs != nil {
		if loadedConfig, exists := r.configs[strategyName]; exists {
			var err error
			config, err = strategy.LoadFromJSON(loadedConfig)
			if err != nil {
				if r.debug {
					fmt.Printf("🐛 DEBUG: Ошибка загрузки конфигурации для %s: %v, используем оптимизацию\n", strategyName, err)
				}
				config = strategy.Optimize(candles, strategy)
			} else if r.debug {
				fmt.Printf("🐛 DEBUG: Используем загруженную конфигурацию для %s\n", strategyName)
			}
		} else {
			if r.debug {
				fmt.Printf("🐛 DEBUG: Конфигурация для %s не найдена, используем оптимизацию\n", strategyName)
			}
			config = strategy.Optimize(candles, strategy)
		}
	} else {
		if r.debug {
			fmt.Printf("🐛 DEBUG: Конфигурация для %s не найдена в файле, используем оптимизацию\n", strategyName)
		}
		config = strategy.Optimize(candles, strategy)
	}

	signals := strategy.GenerateSignals(candles, config)
	result := internal.Backtest(candles, signals, r.slipping)

	executionTime := time.Since(strategyStartTime)

	// Конвертируем V2 config в интерфейс для совместимости
	var v1Config internal.StrategyConfig
	if config != nil {
		// Создаем обертку для V2 конфига
		v1Config = &strategyConfigV2Wrapper{config: config}
	}

	return &BenchmarkResult{
		Name:           strategy.Name(),
		TotalProfit:    result.TotalProfit,
		TradeCount:     result.TradeCount,
		FinalPortfolio: result.FinalPortfolio,
		ExecutionTime:  executionTime,
	}, v1Config, nil
}

// strategyConfigV2Wrapper — обертка для совместимости V2 конфига с V1 интерфейсом
type strategyConfigV2Wrapper struct {
	config internal.StrategyConfigV2
}

func (w *strategyConfigV2Wrapper) DefaultConfigString() string {
	if w.config != nil {
		return w.config.String()
	}
	return ""
}

func (w *strategyConfigV2Wrapper) Validate() error {
	if w.config != nil {
		return w.config.Validate()
	}
	return nil
}

// MarshalJSON — сериализация V2 конфига в JSON
func (w *strategyConfigV2Wrapper) MarshalJSON() ([]byte, error) {
	if w.config != nil {
		return json.Marshal(w.config)
	}
	return []byte("{}"), nil
}

// GetSlipping — возвращает значение параметра проскальзывания
func (r *BaseStrategyRunner) GetSlipping() float64 {
	return r.slipping
}

// ParallelStrategyRunner — реализация параллельного запуска стратегий
type ParallelStrategyRunner struct {
	BaseStrategyRunner
	printer ResultPrinter
}

// NewParallelStrategyRunner — конструктор для ParallelStrategyRunner
func NewParallelStrategyRunner(debug bool) *ParallelStrategyRunner {
	return &ParallelStrategyRunner{
		BaseStrategyRunner: BaseStrategyRunner{debug: debug, slipping: 0.01},
		printer:            NewConsolePrinter(), // По умолчанию консольный принтер
	}
}

// NewParallelStrategyRunnerWithPrinter — конструктор с кастомным принтером
func NewParallelStrategyRunnerWithPrinter(debug bool, printer ResultPrinter) *ParallelStrategyRunner {
	return &ParallelStrategyRunner{
		BaseStrategyRunner: BaseStrategyRunner{debug: debug, slipping: 0.01},
		printer:            printer,
	}
}

// NewParallelStrategyRunnerWithConfig — конструктор с конфигурацией
func NewParallelStrategyRunnerWithConfig(debug bool, printer ResultPrinter, config Config) *ParallelStrategyRunner {
	runner := &ParallelStrategyRunner{
		BaseStrategyRunner: BaseStrategyRunner{
			debug:    debug,
			config:   config,
			slipping: 0.01,
		},
		printer: printer,
	}

	// Загружаем конфигурации из файла если указан
	if config.ConfigFile != "" {
		runner.loadConfigsFromFile()
	}

	return runner
}

// saveOptimizedConfigs — сохраняет оптимизированные конфигурации в JSON файл
func (r *ParallelStrategyRunner) saveOptimizedConfigs(configs map[string]internal.StrategyConfig) {
	filename := "optimized_configs.json"
	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		fmt.Printf("❌ Ошибка сериализации конфигураций: %v\n", err)
		return
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		fmt.Printf("❌ Ошибка сохранения файла конфигураций %s: %v\n", filename, err)
		return
	}

	fmt.Printf("💾 Оптимизированные конфигурации сохранены в %s\n", filename)
}

// RunStrategyWithConfig — запускает одну стратегию и возвращает результат с конфигурацией
func (r *ParallelStrategyRunner) RunStrategyWithConfig(strategyName string, candles []internal.Candle) (*BenchmarkResult, internal.StrategyConfig, error) {
	return r.runSingleStrategy(strategyName, candles)
}

// RunStrategy — запускает одну стратегию
func (r *ParallelStrategyRunner) RunStrategy(strategyName string, candles []internal.Candle) (*BenchmarkResult, error) {
	result, _, err := r.runSingleStrategy(strategyName, candles)
	return result, err
}

// RunAllStrategies — запускает все доступные стратегии параллельно (V1 + V2)
func (r *ParallelStrategyRunner) RunAllStrategies(candles []internal.Candle) ([]BenchmarkResult, error) {
	fmt.Println("\n" + strings.Repeat("═", 80))
	if r.config.ConfigFile != "" {
		fmt.Println("🚀 ЗАПУСК СТРАТЕГИЙ С КОНФИГУРАЦИЯМИ ИЗ ФАЙЛА")
	} else {
		fmt.Println("🚀 ЗАПУСК МАССОВОГО ТЕСТИРОВАНИЯ СТРАТЕГИЙ")
	}
	fmt.Println(strings.Repeat("═", 80))
	fmt.Printf("🔥 Параллельное выполнение на %d ядрах\n", runtime.NumCPU())
	fmt.Printf("📊 Данных для анализа: %d свечей\n", len(candles))

	startTime := time.Now()
	
	// Получаем стратегии из обоих реестров (V1 + V2)
	strategyNamesV1 := internal.GetStrategyNames()
	strategyNamesV2 := internal.GetStrategyNamesV2()
	
	// Объединяем списки стратегий
	strategyNames := append(strategyNamesV1, strategyNamesV2...)
	totalStrategies := len(strategyNames)

	if r.debug {
		fmt.Printf("🐛 DEBUG: Найдено %d стратегий V1 и %d стратегий V2 для тестирования\n",
			len(strategyNamesV1), len(strategyNamesV2))
		fmt.Printf("🐛 DEBUG: V1: %s\n", strings.Join(strategyNamesV1, ", "))
		fmt.Printf("🐛 DEBUG: V2: %s\n", strings.Join(strategyNamesV2, ", "))
	}

	fmt.Printf("🎯 Всего стратегий к запуску: %d (V1: %d, V2: %d)\n", totalStrategies, len(strategyNamesV1), len(strategyNamesV2))
	fmt.Println(strings.Repeat("─", 80))

	// Канал для результатов
	resultsChan := make(chan BenchmarkResult, totalStrategies)
	configsChan := make(chan map[string]internal.StrategyConfig, totalStrategies)
	var wg sync.WaitGroup

	// Запускаем стратегии параллельно
	for _, name := range strategyNames {
		wg.Add(1)

		go func(strategyName string) {
			defer wg.Done()

			if result, config, err := r.RunStrategyWithConfig(strategyName, candles); err != nil {
				fmt.Printf("❌ Ошибка при запуске стратегии %s: %v\n", strategyName, err)
				return
			} else {
				resultsChan <- *result
				configsChan <- map[string]internal.StrategyConfig{strategyName: config}
				fmt.Printf("✅ %-25s │ Прибыль: %+7.2f%% │ Сделки: %4d │ Время: %8v\n",
					result.Name, result.TotalProfit*100, result.TradeCount, result.ExecutionTime)
			}
		}(name)
	}

	// Ждем завершения всех горутин
	wg.Wait()
	close(resultsChan)
	close(configsChan)

	// Собираем результаты
	var results []BenchmarkResult
	completed := 0
	for result := range resultsChan {
		results = append(results, result)
		completed++
	}

	// Собираем конфигурации для сохранения
	optimizedConfigs := make(map[string]internal.StrategyConfig)
	for configMap := range configsChan {
		for name, config := range configMap {
			optimizedConfigs[name] = config
		}
	}

	elapsed := time.Since(startTime)
	fmt.Println(strings.Repeat("─", 80))
	fmt.Printf("⚡ Все %d стратегий выполнены за %v\n", totalStrategies, elapsed)
	fmt.Printf("⏱️  Среднее время на стратегию: %v\n", elapsed/time.Duration(totalStrategies))

	// Сохраняем оптимизированные конфигурации если не используется файл конфигурации
	if r.config.ConfigFile == "" && len(optimizedConfigs) > 0 {
		r.saveOptimizedConfigs(optimizedConfigs)
	}

	// Выводим результаты через принтер
	if r.printer != nil {
		r.printer.PrintComparison(results)
	}

	return results, nil
}

// GetSlipping — возвращает значение параметра проскальзывания
// func (r *ParallelStrategyRunner) GetSlipping() float64 {
// 	return r.slipping
// }

// SingleStrategyRunner — реализация запуска одной стратегии с бенчмарком
type SingleStrategyRunner struct {
	BaseStrategyRunner
}

// NewSingleStrategyRunner — конструктор для SingleStrategyRunner
func NewSingleStrategyRunner(debug bool) *SingleStrategyRunner {
	return &SingleStrategyRunner{
		BaseStrategyRunner: BaseStrategyRunner{debug: debug, slipping: 0.01},
	}
}

// NewSingleStrategyRunnerWithConfig — конструктор с конфигурацией
func NewSingleStrategyRunnerWithConfig(debug bool, config Config) *SingleStrategyRunner {
	runner := &SingleStrategyRunner{
		BaseStrategyRunner: BaseStrategyRunner{
			debug:    debug,
			config:   config,
			slipping: 0.01,
		},
	}

	// Загружаем конфигурации из файла если указан
	if config.ConfigFile != "" {
		runner.loadConfigsFromFile()
	}

	return runner
}

// RunStrategy — запускает одну стратегию с Buy & Hold бенчмарком
func (r *SingleStrategyRunner) RunStrategy(strategyName string, candles []internal.Candle) (*BenchmarkResult, error) {
	fmt.Println("\n" + strings.Repeat("═", 80))
	if r.config.ConfigFile != "" {
		fmt.Println("🎯 ТЕСТИРОВАНИЕ СТРАТЕГИИ С КОНФИГУРАЦИЕЙ ИЗ ФАЙЛА")
	} else {
		fmt.Println("🎯 ТЕСТИРОВАНИЕ ОДИНОЧНОЙ СТРАТЕГИИ")
	}
	fmt.Println(strings.Repeat("═", 80))
	fmt.Printf("📈 Стратегия: %s\n", strategyName)
	fmt.Printf("📊 Данных для анализа: %d свечей\n", len(candles))
	fmt.Println(strings.Repeat("─", 80))

	startTime := time.Now()

	// Проверяем, используем ли конфигурацию из файла
	useConfigFromFile := false
	if r.configs != nil {
		if _, exists := r.configs[strategyName]; exists {
			useConfigFromFile = true
		}
	}

	if useConfigFromFile {
		fmt.Println("📋 Используем конфигурацию из файла...")
	} else {
		fmt.Println("🔄 Оптимизация параметров...")
	}

	result, _, err := r.runSingleStrategy(strategyName, candles)
	if err != nil {
		return nil, err
	}

	fmt.Println("📡 Генерация торговых сигналов...")
	fmt.Println("💹 Выполнение бэктестинга...")

	// Запуск Buy & Hold как бенчмарка
	bnhStrategy := internal.GetStrategy("buy_and_hold")
	bnhConfig := bnhStrategy.DefaultConfig()
	bnhSignals := bnhStrategy.GenerateSignalsWithConfig(candles, bnhConfig)
	internal.Backtest(candles, bnhSignals, r.slipping)

	executionTime := time.Since(startTime)

	fmt.Println(strings.Repeat("─", 80))
	fmt.Printf("⚡ Тестирование завершено за %v\n", executionTime)

	return result, nil
}

// RunAllStrategies — для интерфейса совместимости (не используется для одиночной стратегии)
func (r *SingleStrategyRunner) RunAllStrategies(candles []internal.Candle) ([]BenchmarkResult, error) {
	return nil, fmt.Errorf("SingleStrategyRunner не поддерживает запуск всех стратегий")
}
