package backtester

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ConsolePrinter — реализация вывода результатов в консоль
type ConsolePrinter struct{}

// NewConsolePrinter — конструктор для ConsolePrinter
func NewConsolePrinter() *ConsolePrinter {
	return &ConsolePrinter{}
}

// PrintComparison — выводит сравнительную таблицу стратегий
func (p *ConsolePrinter) PrintComparison(results []BenchmarkResult) {
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
		timeStr := p.formatDuration(r.ExecutionTime)

		fmt.Printf("%-18s %-12s %-10d $%-14.2f %-10s %-12s\n",
			r.Name,
			profitColor,
			r.TradeCount,
			r.FinalPortfolio,
			timeStr,
			rankStr)
		rank++
	}
}

// PrintProgress — выводит прогресс выполнения стратегий
func (p *ConsolePrinter) PrintProgress(current, total int) {
	fmt.Printf("📊 Прогресс: %d/%d стратегий завершено\n", current, total)
}

// formatDuration — форматирует длительность в читаемый вид
func (p *ConsolePrinter) formatDuration(d time.Duration) string {
	if d > time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.0fms", float64(d.Nanoseconds())/1e6)
}
