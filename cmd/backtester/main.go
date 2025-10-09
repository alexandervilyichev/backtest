// main.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"bt/internal"

	"bt/internal/app/backtester"
	_ "bt/strategies" // Импортируем для регистрации стратегий
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

	sort.Slice(wrapper.Candles, func(i, j int) bool {
		return wrapper.Candles[i].ToTime().Before(wrapper.Candles[j].ToTime())
	})

	fmt.Printf("✅ Загружено %d свечей из %s\n", len(wrapper.Candles), filename)
	return wrapper.Candles
}

func main() {
	// Парсинг командной строки
	config := parseFlags()

	// Загрузка данных
	candles := LoadCandlesFromFile(config.Filename)
	if len(candles) == 0 {
		log.Fatal("Нет данных для анализа")
	}

	// Инициализация компонентов
	runner := createRunner(config)
	printer := backtester.NewConsolePrinter()
	saver := backtester.NewFileSaver()

	// Запуск стратегий
	results, err := runStrategies(config, runner, candles)
	if err != nil {
		log.Fatalf("Ошибка при запуске стратегий: %v", err)
	}

	// Вывод результатов
	printer.PrintComparison(results)

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
}

// parseFlags — парсит командную строку и возвращает конфигурацию
func parseFlags() backtester.Config {
	filename := flag.String("file", "C:/Users/alexa/OneDrive/Документы/Projects/backtest/candles.json", "Путь к JSON-файлу со свечами")
	strategyName := flag.String("strategy", "all", "Стратегия: all (все стратегии) или "+strings.Join(internal.GetStrategyNames(), ", "))
	debug := flag.Bool("debug", false, "Включить детальное логирование")
	saveSignals := flag.Int("save_signals", 0, "Сохранить топ-N стратегий с сигналами (0 = не сохранять)")
	flag.Parse()

	return backtester.Config{
		Filename:    *filename,
		Strategy:    *strategyName,
		Debug:       *debug,
		SaveSignals: *saveSignals,
	}
}

// createRunner — создает подходящий runner в зависимости от стратегии
func createRunner(config backtester.Config) backtester.StrategyRunner {
	if config.Strategy == "all" {
		return backtester.NewParallelStrategyRunner(config.Debug)
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
	bnhSignals := bnhStrategy.GenerateSignals(candles, internal.StrategyParams{})
	bnhResult := internal.Backtest(candles, bnhSignals, 0.01)

	results := []backtester.BenchmarkResult{
		*mainResult,
		{
			Name:           bnhStrategy.Name(),
			TotalProfit:    bnhResult.TotalProfit,
			TradeCount:     bnhResult.TradeCount,
			FinalPortfolio: bnhResult.FinalPortfolio,
			ExecutionTime:  mainResult.ExecutionTime, // Используем то же время для простоты
		},
	}

	return results, nil
}
