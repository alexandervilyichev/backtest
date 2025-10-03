// main.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"bt/internal"
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

// BenchmarkResult — для сравнения стратегий
type BenchmarkResult struct {
	Name           string
	TotalProfit    float64
	TradeCount     int
	FinalPortfolio float64
	ExecutionTime  time.Duration
}

func main() {
	filename := flag.String("file", "C:/Users/alexa/OneDrive/Документы/Projects/backtest/candles.json", "Путь к JSON-файлу со свечами")
	strategyName := flag.String("strategy", "all", "Стратегия: all (все стратегии) или "+strings.Join(internal.GetStrategyNames(), ", "))
	flag.Parse()

	candles := LoadCandlesFromFile(*filename)
	if len(candles) == 0 {
		log.Fatal("Нет данных для анализа")
	}

	var results []BenchmarkResult

	// Если выбрана стратегия "all" или не указана конкретная стратегия, запускаем все
	if *strategyName == "all" {
		fmt.Println("🎯 Запуск всех доступных стратегий для сравнения...")
		fmt.Printf("🔥 Параллельное выполнение на %d ядрах\n", runtime.NumCPU())

		startTime := time.Now()

		strategyNames := internal.GetStrategyNames()
		totalStrategies := len(strategyNames)

		// Канал для результатов
		resultsChan := make(chan BenchmarkResult, totalStrategies)

		// WaitGroup для синхронизации горутин
		var wg sync.WaitGroup

		// Запускаем стратегии параллельно
		for _, name := range strategyNames {
			wg.Add(1)

			go func(strategyName string) {
				defer wg.Done()

				strategy := internal.GetStrategy(strategyName)
				strategyStartTime := time.Now()

				fmt.Printf("🚀 Запущена стратегия: %s\n", strategy.Name())

				// Обучение и генерация сигналов
				params := strategy.Optimize(candles)
				signals := strategy.GenerateSignals(candles, params)
				result := internal.Backtest(candles, signals, 0.01) // 0.01 units проскальзывание

				executionTime := time.Since(strategyStartTime)

				resultsChan <- BenchmarkResult{
					Name:           strategy.Name(),
					TotalProfit:    result.TotalProfit,
					TradeCount:     result.TradeCount,
					FinalPortfolio: result.FinalPortfolio,
					ExecutionTime:  executionTime,
				}

				fmt.Printf("✅ Завершена стратегия: %s (прибыль: %.2f%%, время: %v)\n",
					strategy.Name(), result.TotalProfit*100, executionTime)
			}(name)
		}

		// Ждем завершения всех горутин
		wg.Wait()

		// Закрываем канал после завершения всех стратегий
		close(resultsChan)

		// Собираем результаты
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
	} else {
		// Запускаем только выбранную стратегию

		mainStrategy := internal.GetStrategy(*strategyName)
		fmt.Printf("🎯 Выбрана стратегия: %s\n", mainStrategy.Name())

		mainStartTime := time.Now()

		// Обучение и генерация сигналов для основной стратегии
		mainParams := mainStrategy.Optimize(candles)
		mainSignals := mainStrategy.GenerateSignals(candles, mainParams)

		// fmt.Printf("Применена квантизация\n")
		// mainParams.QuantizationEnabled = true
		// candles = strategies.ApplyQuantizationToCandles(candles, mainParams)

		mainResult := internal.Backtest(candles, mainSignals, 0.01) // 0.01 units проскальзывание

		mainExecutionTime := time.Since(mainStartTime)

		// Автоматически запускаем Buy & Hold как бенчмарк
		bnhStrategy := internal.GetStrategy("buy_and_hold")
		bnhStartTime := time.Now()
		bnhSignals := bnhStrategy.GenerateSignals(candles, internal.StrategyParams{})
		bnhResult := internal.Backtest(candles, bnhSignals, 0.01) // 0.01 units проскальзывание
		bnhExecutionTime := time.Since(bnhStartTime)

		// Собираем результаты для сравнения
		results = []BenchmarkResult{
			{
				Name:           mainStrategy.Name(),
				TotalProfit:    mainResult.TotalProfit,
				TradeCount:     mainResult.TradeCount,
				FinalPortfolio: mainResult.FinalPortfolio,
				ExecutionTime:  mainExecutionTime,
			},
			{
				Name:           bnhStrategy.Name(),
				TotalProfit:    bnhResult.TotalProfit,
				TradeCount:     bnhResult.TradeCount,
				FinalPortfolio: bnhResult.FinalPortfolio,
				ExecutionTime:  bnhExecutionTime,
			},
		}

		fmt.Printf("⚡ Стратегии выполнены за %v\n", mainExecutionTime+bnhExecutionTime)
	}

	// Сортируем результаты по доходности (лучшие вверху)
	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalProfit > results[j].TotalProfit
	})

	// Выводим сравнительную таблицу
	fmt.Println("\n" + strings.Repeat("=", 100))
	fmt.Println("📊 СРАВНЕНИЕ СТРАТЕГИЙ")
	fmt.Println(strings.Repeat("=", 100))
	fmt.Printf("%-18s %-12s %-10s %-15s %-10s %-12s\n", "Стратегия", "Прибыль", "Сделки", "Финал, $", "Время", "Ранг")
	fmt.Println(strings.Repeat("-", 100))

	rank := 1
	for i, r := range results {
		rankStr := fmt.Sprintf("%d", rank)
		if i == 0 {
			rankStr = "🥇 " + rankStr
		} else if i == 1 {
			rankStr = "🥈 " + rankStr
		} else if i == 2 {
			rankStr = "🥉 " + rankStr
		} else {
			rankStr = "  " + rankStr
		}

		profitColor := ""
		if r.TotalProfit > 0 {
			profitColor = fmt.Sprintf("+%.2f%%", r.TotalProfit*100)
		} else {
			profitColor = fmt.Sprintf("%.2f%%", r.TotalProfit*100)
		}

		// Форматируем время выполнения
		timeStr := fmt.Sprintf("%v", r.ExecutionTime)
		if r.ExecutionTime > time.Second {
			timeStr = fmt.Sprintf("%.1fs", r.ExecutionTime.Seconds())
		} else {
			timeStr = fmt.Sprintf("%.0fms", float64(r.ExecutionTime.Nanoseconds())/1e6)
		}

		fmt.Printf("%-18s %-12s %-10d $%-14.2f %-10s %-12s\n",
			r.Name,
			profitColor,
			r.TradeCount,
			r.FinalPortfolio,
			timeStr,
			rankStr)
		rank++
	}

	// Выводим динамику основной стратегии (опционально)
	// fmt.Println("\n" + strings.Repeat("=", 50))
	// fmt.Printf("📉 Динамика портфеля (%s)\n", mainStrategy.Name())
	// fmt.Println(strings.Repeat("=", 50))
	// for i, val := range mainResult.PortfolioValues {
	// 	if i < len(candles) {
	// 		t := candles[i].ToTime().Format("15:04")
	// 		fmt.Printf("[%s] $%.2f\n", t, val)
	// 	} else if i == len(candles) {
	// 		fmt.Printf("[FINAL] $%.2f\n", val)
	// 	}
	// }
}
