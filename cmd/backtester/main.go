// main.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	"bt/internal"

	"bt/internal/app/backtester"

	_ "bt/strategies/v1/extrema"
	_ "bt/strategies/v1/lines"
	_ "bt/strategies/v1/momentum"
	_ "bt/strategies/v1/moving_averages"
	_ "bt/strategies/v1/oscillators"
	_ "bt/strategies/v1/rebalance"
	_ "bt/strategies/v1/sell"
	_ "bt/strategies/v1/simple"
	_ "bt/strategies/v1/spline"
	_ "bt/strategies/v1/statistical"
	_ "bt/strategies/v1/trend"
	_ "bt/strategies/v1/volatility"
	_ "bt/strategies/v1/volume"

	_ "bt/strategies/v2/lines"
	_ "bt/strategies/v2/oscillators"
	_ "bt/strategies/v2/trend"
	_ "bt/strategies/v2/volatility"
	_ "bt/strategies/v2/wave"
)

func LoadCandlesFromFile(filename string) []internal.Candle {
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal("❌ Не удалось прочитать файл:", err)
	}

	var wrapper struct {
		Candles []internal.Candle `json:"candles"`
	}

	err = json.Unmarshal(data, &wrapper)
	if err != nil {
		log.Fatal("❌ Ошибка парсинга JSON:", err)
	}

	// Precompute ParsedTime to optimize ToTime() calls for better performance
	// Handle empty time strings gracefully to avoid parsing errors
	for i := range wrapper.Candles {
		if wrapper.Candles[i].Time != "" {
			// Compute and store time.Time with multiple format fallbacks
			t, err := time.Parse(time.RFC3339, wrapper.Candles[i].Time)
			if err != nil {
				// Try RFC3339Nano format
				t, err = time.Parse(time.RFC3339Nano, wrapper.Candles[i].Time)
				if err != nil {
					// Try format without timezone
					t, err = time.Parse("2006-01-02T15:04:05", wrapper.Candles[i].Time)
					if err != nil {
						log.Printf("❌ Все форматы времени провалились для: '%s', используем zero time", wrapper.Candles[i].Time)
						t = time.Time{} // Use zero time for invalid formats
					}
				}
			}
			wrapper.Candles[i].ParsedTime = t
		}
		// If Time is empty, ParsedTime remains as zero time (already set by UnmarshalJSON)
	}

	sort.Slice(wrapper.Candles, func(i, j int) bool {
		return wrapper.Candles[i].ParsedTime.Before(wrapper.Candles[j].ParsedTime)
	})

	fmt.Printf("✅ Загружено %d свечей из %s\n", len(wrapper.Candles), filename)
	return wrapper.Candles
}

func main() {
	// Парсинг командной строки
	config := parseFlags()

	// Запуск realtime профилирования если указано
	if config.ProfPort > 0 {
		go func() {
			addr := fmt.Sprintf(":%d", config.ProfPort)
			log.Printf("🚀 HTTP профилирование доступно на http://localhost%s/debug/pprof/", addr)
			if err := http.ListenAndServe(addr, nil); err != nil {
				log.Printf("❌ Ошибка запуска HTTP сервера профилирования: %v", err)
			}
		}()
	}

	// Запуск CPU профилирования если указано
	if config.CpuProfile != "" {
		f, err := os.Create(config.CpuProfile)
		if err != nil {
			log.Fatal("❌ Не удалось создать файл CPU профиля:", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("❌ Не удалось запустить CPU профилирование:", err)
		}
		defer pprof.StopCPUProfile()
	}

	// Загрузка данных
	candles := LoadCandlesFromFile(config.Filename)
	if len(candles) == 0 {
		log.Fatal("Нет данных для анализа")
	}

	// Инициализация компонентов
	printer := backtester.NewCombinedPrinter() // Используем комбинированный принтер для автоматической генерации MD отчетов
	runner := createRunner(config, printer)
	saver := backtester.NewFileSaver()

	// Запуск стратегий
	results, err := runStrategies(config, runner, candles)
	if err != nil {
		log.Fatalf("Ошибка при запуске стратегий: %v", err)
	}

	// Результаты уже выведены через принтер в runner

	// Сохранение данных для графиков
	if config.SaveSignals > 0 {
		fmt.Printf("%s", "\n"+strings.Repeat("=", 100)+"\n")
		fmt.Printf("💾 СОХРАНЕНИЕ ТОП-%d СТРАТЕГИЙ ДЛЯ ГРАФИКОВ\n", config.SaveSignals)
		fmt.Println(strings.Repeat("=", 100))

		if err := saver.SaveTopStrategies(candles, results, config.Filename, config.SaveSignals); err != nil {
			log.Printf("❌ Ошибка при сохранении данных: %v", err)
		}
	} else if config.Debug {
		fmt.Println("\n💡 Сохранение сигналов отключено флагом --save_signals=0")
	}

	// Memory профилирование
	if config.MemProfile != "" {
		f, err := os.Create(config.MemProfile)
		if err != nil {
			log.Fatal("❌ Не удалось создать файл memory профиля:", err)
		}
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("❌ Не удалось записать memory профиль:", err)
		}
		f.Close()
	}
}

// parseFlags — парсит командную строку и возвращает конфигурацию
func parseFlags() backtester.Config {
	filename := flag.String("file", "candles.json", "Путь к JSON-файлу со свечами")
	strategyName := flag.String("strategy", "all", "Стратегия: all (все стратегии) или "+strings.Join(internal.GetStrategyNames(), ", "))
	debug := flag.Bool("debug", false, "Включить детальное логирование")
	saveSignals := flag.Int("save_signals", 0, "Сохранить топ-N стратегий с сигналами (0 = не сохранять)")
	cpuProfile := flag.String("cpu_profile", "", "Файл для CPU профилирования (пусто = отключено)")
	memProfile := flag.String("mem_profile", "", "Файл для памяти профилирования (пусто = отключено)")
	configFile := flag.String("config", "", "Путь к JSON-файлу с конфигурациями стратегий (пусто = оптимизация)")
	profPort := flag.Int("prof_port", 0, "Порт для realtime профилирования (0 = отключено)")
	flag.Parse()

	return backtester.Config{
		Filename:    *filename,
		Strategy:    *strategyName,
		Debug:       *debug,
		SaveSignals: *saveSignals,
		CpuProfile:  *cpuProfile,
		MemProfile:  *memProfile,
		ConfigFile:  *configFile,
		ProfPort:    *profPort,
	}
}

// createRunner — создает подходящий runner в зависимости от стратегии
func createRunner(config backtester.Config, printer backtester.ResultPrinter) backtester.StrategyRunner {
	if config.Strategy == "all" {
		if config.ConfigFile != "" {
			return backtester.NewParallelStrategyRunnerWithConfig(config.Debug, printer, config)
		}
		return backtester.NewParallelStrategyRunnerWithPrinter(config.Debug, printer)
	}
	if config.ConfigFile != "" {
		return backtester.NewSingleStrategyRunnerWithConfig(config.Debug, config)
	}
	return backtester.NewSingleStrategyRunner(config.Debug)
}

// runStrategies — запускает стратегии с помощью runner
func runStrategies(config backtester.Config, runner backtester.StrategyRunner, candles []internal.Candle) ([]backtester.BenchmarkResult, error) {
	if config.Strategy == "all" {
		return runner.RunAllStrategies(candles)
	}

	// Для одиночной стратегии создаем результаты вручную
	mainResult, err := runner.RunStrategy(config.Strategy, candles)
	if err != nil {
		return nil, err
	}

	// Добавляем Buy & Hold как бенчмарк
	bnhStrategy := internal.GetStrategy("buy_and_hold")
	var bnhResult internal.BacktestResult

	bnhConfig := bnhStrategy.DefaultConfig()
	bnhSignals := bnhStrategy.GenerateSignalsWithConfig(candles, bnhConfig)

	// Получаем значение проскальзывания из runner
	slipping := 0.01
	if parallelRunner, ok := runner.(*backtester.ParallelStrategyRunner); ok {
		slipping = parallelRunner.GetSlipping()
	} else if singleRunner, ok := runner.(*backtester.SingleStrategyRunner); ok {
		slipping = singleRunner.GetSlipping()
	}

	bnhResult = internal.Backtest(candles, bnhSignals, slipping)

	results := []backtester.BenchmarkResult{
		*mainResult,
		{
			Name:           bnhStrategy.Name(),
			TotalProfit:    bnhResult.TotalProfit,
			TradeCount:     bnhResult.TradeCount,
			FinalPortfolio: bnhResult.FinalPortfolio,
			ExecutionTime:  mainResult.ExecutionTime, // Используем то же время для простоты
			NextSignal:     nil,                      // Buy & Hold не предсказывает сигналы
		},
	}

	// Выводим результаты через принтер для одиночной стратегии
	printer := backtester.NewCombinedPrinter()
	printer.PrintComparison(results)

	return results, nil
}
