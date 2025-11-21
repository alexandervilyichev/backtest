package backtester

import (
	"bt/internal"
	"fmt"
	"os"
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
	fmt.Println("\n" + strings.Repeat("═", 120))
	fmt.Println("📊 ИТОГОВЫЙ ОТЧЕТ ПО СТРАТЕГИЯМ")
	fmt.Println(strings.Repeat("═", 120))

	// Заголовок таблицы с улучшенным выравниванием
	fmt.Printf("│ %-4s │ %-25s │ %-12s │ %-8s │ %-15s │ %-10s │ %-8s │ %-12s │ %-15s │ %-12s │ %-10s │\n",
		"Ранг", "Стратегия", "Прибыль", "Сделки", "Финал, $", "Время", "Статус", "След.сигнал", "Дата сигнала", "Цена", "Уверен.")
	fmt.Println("├" + strings.Repeat("─", 6) + "┼" + strings.Repeat("─", 27) + "┼" +
		strings.Repeat("─", 14) + "┼" + strings.Repeat("─", 10) + "┼" +
		strings.Repeat("─", 17) + "┼" + strings.Repeat("─", 12) + "┼" +
		strings.Repeat("─", 10) + "┼" + strings.Repeat("─", 14) + "┼" +
		strings.Repeat("─", 17) + "┼" + strings.Repeat("─", 14) + "┼" +
		strings.Repeat("─", 12) + "┤")

	rank := 1
	for i, r := range results {
		// Определяем ранг с медалями
		rankStr := ""
		switch i {
		case 0:
			rankStr = "🥇 1"
		case 1:
			rankStr = "🥈 2"
		case 2:
			rankStr = "🥉 3"
		default:
			rankStr = fmt.Sprintf("   %d", rank)
		}

		// Форматируем прибыль с цветовыми индикаторами
		profitStr := ""
		statusStr := ""
		if r.TotalProfit > 0.05 { // > 5%
			profitStr = fmt.Sprintf("🟢 +%.2f%%", r.TotalProfit*100)
			statusStr = "Отлично"
		} else if r.TotalProfit > 0 {
			profitStr = fmt.Sprintf("🟡 +%.2f%%", r.TotalProfit*100)
			statusStr = "Хорошо"
		} else if r.TotalProfit > -0.05 { // > -5%
			profitStr = fmt.Sprintf("🟠 %.2f%%", r.TotalProfit*100)
			statusStr = "Слабо"
		} else {
			profitStr = fmt.Sprintf("🔴 %.2f%%", r.TotalProfit*100)
			statusStr = "Убыток"
		}

		// Форматируем время выполнения
		timeStr := p.formatDuration(r.ExecutionTime)

		// Форматируем финальную сумму
		finalStr := fmt.Sprintf("$%.2f", r.FinalPortfolio)

		// Форматируем информацию о следующем сигнале
		nextSignalStr := "⏸️ HOLD"
		nextSignalDateStr := "Нет данных"
		nextSignalPriceStr := "-"
		nextSignalConfStr := "-"
		if r.NextSignal != nil {
			switch r.NextSignal.SignalType {
			case internal.BUY:
				nextSignalStr = "🟢 BUY"
			case internal.SELL:
				nextSignalStr = "🔴 SELL"
			default:
				nextSignalStr = "⏸️ HOLD"
			}
			signalTime := time.Unix(r.NextSignal.Date, 0)
			nextSignalDateStr = signalTime.Format("02.01 15:04")
			nextSignalPriceStr = fmt.Sprintf("$%.4f", r.NextSignal.Price)
			nextSignalConfStr = fmt.Sprintf("%.1f%%", r.NextSignal.Confidence*100)
		}

		// Выводим строку таблицы
		fmt.Printf("│ %-4s │ %-25s │ %-12s │ %-8d │ %-15s │ %-10s │ %-8s │ %-12s │ %-15s │ %-12s │ %-10s │\n",
			rankStr,
			p.truncateString(r.Name, 25),
			profitStr,
			r.TradeCount,
			finalStr,
			timeStr,
			statusStr,
			nextSignalStr,
			nextSignalDateStr,
			nextSignalPriceStr,
			nextSignalConfStr)

		rank++
	}

	// Нижняя граница таблицы
	fmt.Println("└" + strings.Repeat("─", 6) + "┴" + strings.Repeat("─", 27) + "┴" +
		strings.Repeat("─", 14) + "┴" + strings.Repeat("─", 10) + "┴" +
		strings.Repeat("─", 17) + "┴" + strings.Repeat("─", 12) + "┴" +
		strings.Repeat("─", 10) + "┴" + strings.Repeat("─", 14) + "┴" +
		strings.Repeat("─", 17) + "┴" + strings.Repeat("─", 14) + "┴" +
		strings.Repeat("─", 12) + "┘")

	// Добавляем статистику
	p.printSummaryStats(results)
}

// PrintProgress — выводит прогресс выполнения стратегий
func (p *ConsolePrinter) PrintProgress(current, total int) {
	percent := float64(current) / float64(total) * 100

	// Создаем прогресс-бар
	barWidth := 30
	filled := int(float64(barWidth) * percent / 100)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	fmt.Printf("📊 Прогресс: [%s] %d/%d (%.1f%%) стратегий завершено\n",
		bar, current, total, percent)
}

// formatDuration — форматирует длительность в читаемый вид
func (p *ConsolePrinter) formatDuration(d time.Duration) string {
	if d > time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.0fms", float64(d.Nanoseconds())/1e6)
}

// truncateString — обрезает строку до указанной длины
func (p *ConsolePrinter) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// printSummaryStats — выводит сводную статистику
func (p *ConsolePrinter) printSummaryStats(results []BenchmarkResult) {
	if len(results) == 0 {
		return
	}

	fmt.Println("\n" + strings.Repeat("═", 60))
	fmt.Println("📈 СВОДНАЯ СТАТИСТИКА")
	fmt.Println(strings.Repeat("═", 60))

	// Подсчитываем статистику
	profitable := 0
	totalProfit := 0.0
	totalTrades := 0
	bestProfit := results[0].TotalProfit
	worstProfit := results[len(results)-1].TotalProfit

	for _, r := range results {
		if r.TotalProfit > 0 {
			profitable++
		}
		totalProfit += r.TotalProfit
		totalTrades += r.TradeCount
	}

	avgProfit := totalProfit / float64(len(results))
	profitablePercent := float64(profitable) / float64(len(results)) * 100

	// Подсчитываем стратегии с предсказаниями
	withPredictions := 0
	buySignals := 0
	sellSignals := 0
	for _, r := range results {
		if r.NextSignal != nil {
			withPredictions++
			if r.NextSignal.SignalType == internal.BUY {
				buySignals++
			} else if r.NextSignal.SignalType == internal.SELL {
				sellSignals++
			}
		}
	}

	fmt.Printf("🎯 Всего стратегий:      %d\n", len(results))
	fmt.Printf("💰 Прибыльных:          %d (%.1f%%)\n", profitable, profitablePercent)
	fmt.Printf("📊 Средняя прибыль:     %.2f%%\n", avgProfit*100)
	fmt.Printf("🚀 Лучший результат:    %.2f%% (%s)\n", bestProfit*100, results[0].Name)
	fmt.Printf("📉 Худший результат:    %.2f%% (%s)\n", worstProfit*100, results[len(results)-1].Name)
	fmt.Printf("🔄 Всего сделок:        %d\n", totalTrades)
	
	if withPredictions > 0 {
		fmt.Printf("\n🔮 Предсказания:\n")
		fmt.Printf("   Стратегий с предсказаниями: %d\n", withPredictions)
		if buySignals > 0 {
			fmt.Printf("   🟢 BUY сигналов:  %d\n", buySignals)
		}
		if sellSignals > 0 {
			fmt.Printf("   🔴 SELL сигналов: %d\n", sellSignals)
		}
	}

	fmt.Println(strings.Repeat("═", 60))
}

// MarkdownPrinter — реализация вывода результатов в Markdown файл
type MarkdownPrinter struct{}

// NewMarkdownPrinter — конструктор для MarkdownPrinter
func NewMarkdownPrinter() *MarkdownPrinter {
	return &MarkdownPrinter{}
}

// PrintComparison — генерирует Markdown отчет и сохраняет в файл
func (p *MarkdownPrinter) PrintComparison(results []BenchmarkResult) {
	// Сортируем результаты по доходности (лучшие вверху)
	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalProfit > results[j].TotalProfit
	})

	var content strings.Builder

	// Заголовок отчета
	content.WriteString("# Отчет прогона всех торговых стратегий\n\n")
	content.WriteString("## Обзор тестирования\n\n")
	content.WriteString(fmt.Sprintf("**Дата проведения:** %s  \n", time.Now().Format("2 January 2006")))
	content.WriteString("**Система:** Параллельное выполнение на многоядерной архитектуре  \n")
	content.WriteString("**Метод тестирования:** Бэктестинг с оптимизацией параметров  \n")
	content.WriteString("**Проскальзывание:** 0.01 единиц  \n\n")
	content.WriteString("---\n\n")
	content.WriteString("## Результаты по стратегиям\n\n")

	// Создаем основную таблицу результатов
	content.WriteString("| Ранг | Стратегия | Категория | Прибыль | Сделки | Финальный портфель | Время | Статус | След.сигнал | Дата | Цена | Уверенность |\n")
	content.WriteString("|------|-----------|-----------|---------|--------|-------------------|-------|--------|-------------|------|------|-------------|\n")

	for i, r := range results {
		rank := i + 1
		category := p.getStrategyCategory(r.Name)
		profitStr := fmt.Sprintf("%+.2f%%", r.TotalProfit*100)
		finalStr := fmt.Sprintf("$%.2f", r.FinalPortfolio)
		timeStr := p.formatDurationMD(r.ExecutionTime)
		status := p.getStatusText(r.TotalProfit)

		// Форматируем информацию о следующем сигнале
		nextSignalStr := "⏸️ HOLD"
		nextSignalDateStr := "Нет данных"
		nextSignalPriceStr := "-"
		nextSignalConfStr := "-"
		if r.NextSignal != nil {
			switch r.NextSignal.SignalType {
			case internal.BUY:
				nextSignalStr = "🟢 BUY"
			case internal.SELL:
				nextSignalStr = "🔴 SELL"
			default:
				nextSignalStr = "⏸️ HOLD"
			}
			signalTime := time.Unix(r.NextSignal.Date, 0)
			nextSignalDateStr = signalTime.Format("02.01.2006 15:04")
			nextSignalPriceStr = fmt.Sprintf("$%.4f", r.NextSignal.Price)
			nextSignalConfStr = fmt.Sprintf("%.1f%%", r.NextSignal.Confidence*100)
		}

		content.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %d | %s | %s | %s | %s | %s | %s | %s |\n",
			rank, r.Name, category, profitStr, r.TradeCount, finalStr, timeStr, status,
			nextSignalStr, nextSignalDateStr, nextSignalPriceStr, nextSignalConfStr))
	}

	content.WriteString("\n")

	// Добавляем аналитические таблицы
	p.writeAnalyticsTables(&content, results)

	// Добавляем технические детали
	p.writeTechnicalDetails(&content, results)

	// Сохраняем в файл
	filename := fmt.Sprintf("strategy_report_%s.md", time.Now().Format("2006-01-02_15-04-05"))
	err := os.WriteFile(filename, []byte(content.String()), 0644)
	if err != nil {
		fmt.Printf("❌ Ошибка сохранения отчета: %v\n", err)
		return
	}

	fmt.Printf("📄 Markdown отчет сохранен: %s\n", filename)
}

// writeTechnicalDetails — записывает технические детали в Markdown
func (p *MarkdownPrinter) writeTechnicalDetails(content *strings.Builder, results []BenchmarkResult) {
	content.WriteString("---\n\n")
	content.WriteString("## Технические детали\n\n")

	content.WriteString("### Параметры тестирования\n")
	content.WriteString("- **Начальный капитал:** $1,000.00\n")
	content.WriteString("- **Комиссия за сделку:** Включена в расчет проскальзывания\n")
	content.WriteString("- **Проскальзывание:** 0.01 единиц на сделку\n")
	content.WriteString("- **Оптимизация:** Автоматическая оптимизация параметров для каждой стратегии\n\n")

	// Подсчитываем общее время выполнения
	totalTime := time.Duration(0)
	for _, r := range results {
		totalTime += r.ExecutionTime
	}
	avgTime := totalTime / time.Duration(len(results))

	content.WriteString("### Производительность системы\n")
	content.WriteString(fmt.Sprintf("- **Общее время выполнения:** %s\n", p.formatDurationMD(totalTime)))
	content.WriteString(fmt.Sprintf("- **Среднее время на стратегию:** %s\n", p.formatDurationMD(avgTime)))
	content.WriteString("- **Параллельное выполнение:** Использованы все доступные ядра процессора\n")
	content.WriteString("- **Обработано данных:** Полный набор исторических свечей\n\n")

	// Подсчитываем категории
	categories := p.countCategories(results)
	content.WriteString("### Категории стратегий\n")
	for category, count := range categories {
		content.WriteString(fmt.Sprintf("- **%s:** %d стратегий\n", category, count))
	}

	content.WriteString("\n---\n\n")
	content.WriteString("*Отчет сгенерирован автоматически системой бэктестинга*\n")
}

// getStrategyCategory — определяет категорию стратегии по имени
func (p *MarkdownPrinter) getStrategyCategory(name string) string {
	categoryMap := map[string]string{
		"elliott_wave":          "Волновой анализ",
		"arima":                 "Статистические методы",
		"heston":                "Статистические методы",
		"golden_cross":          "Трендовые стратегии",
		"ma_crossover":          "Трендовые стратегии",
		"supertrend":            "Трендовые стратегии",
		"fomo":                  "Трендовые стратегии",
		"rsi_oscillator":        "Осцилляторы",
		"cci_oscillator":        "Осцилляторы",
		"stochastic_oscillator": "Осцилляторы",
		"ao_oscillator":         "Осцилляторы",
		"qstick_oscillator":     "Осцилляторы",
		"momentum_breakout":     "Волатильность",
		"bollinger_bands":       "Волатильность",
		"garch_volatility":      "Волатильность",
		"ulcer_index":           "Волатильность",
		"macd":                  "Моментум",
		"ma_channel":            "Моментум",
		"volume_breakout":       "Объемные стратегии",
		"obv":                   "Объемные стратегии",
		"extrema":               "Экстремумы",
		"optimal_extrema":       "Экстремумы",
		"ma_ema_correlation":    "Скользящие средние",
		"buy_and_hold":          "Простые стратегии",
		"monthly_rebalance":     "Ребалансировка",
		"pullback_sell":         "Стратегии продажи",
		"support_line":          "Линии поддержки/сопротивления",
		"wavelet_denoise":       "Линии поддержки/сопротивления",
	}

	// Ищем по частичному совпадению имени
	for key, category := range categoryMap {
		if strings.Contains(strings.ToLower(name), key) {
			return category
		}
	}

	return "Прочие стратегии"
}

// getStatusText — возвращает статус без эмодзи для таблиц
func (p *MarkdownPrinter) getStatusText(profit float64) string {
	if profit > 0.05 {
		return "🟢 Отлично"
	} else if profit > 0 {
		return "🟡 Хорошо"
	} else if profit > -0.05 {
		return "🟠 Слабо"
	} else {
		return "🔴 Убыток"
	}
}

// writeAnalyticsTables — записывает аналитические таблицы
func (p *MarkdownPrinter) writeAnalyticsTables(content *strings.Builder, results []BenchmarkResult) {
	content.WriteString("---\n\n")
	content.WriteString("## Анализ по категориям\n\n")

	// Сводная таблица по категориям
	content.WriteString("### Сводная таблица по категориям стратегий\n\n")
	p.writeCategoryAnalysis(content, results)

	// Топ-5 по эффективности сделок
	content.WriteString("### Топ-5 стратегий по эффективности сделок\n\n")
	p.writeEfficiencyTable(content, results)

	// Анализ производительности по времени
	content.WriteString("### Анализ производительности по времени выполнения\n\n")
	p.writePerformanceAnalysis(content, results)
}

// writeCategoryAnalysis — создает таблицу анализа по категориям
func (p *MarkdownPrinter) writeCategoryAnalysis(content *strings.Builder, results []BenchmarkResult) {
	categoryStats := make(map[string]struct {
		count       int
		bestProfit  float64
		worstProfit float64
		totalProfit float64
		bestName    string
		worstName   string
	})

	// Собираем статистику по категориям
	for _, r := range results {
		category := p.getStrategyCategory(r.Name)
		stats := categoryStats[category]

		if stats.count == 0 {
			stats.bestProfit = r.TotalProfit
			stats.worstProfit = r.TotalProfit
			stats.bestName = r.Name
			stats.worstName = r.Name
		} else {
			if r.TotalProfit > stats.bestProfit {
				stats.bestProfit = r.TotalProfit
				stats.bestName = r.Name
			}
			if r.TotalProfit < stats.worstProfit {
				stats.worstProfit = r.TotalProfit
				stats.worstName = r.Name
			}
		}

		stats.count++
		stats.totalProfit += r.TotalProfit
		categoryStats[category] = stats
	}

	content.WriteString("| Категория | Количество | Лучший результат | Худший результат | Средняя прибыль |\n")
	content.WriteString("|-----------|------------|------------------|------------------|----------------|\n")

	for category, stats := range categoryStats {
		avgProfit := stats.totalProfit / float64(stats.count)
		bestStr := fmt.Sprintf("%+.2f%% (%s)", stats.bestProfit*100, stats.bestName)
		worstStr := fmt.Sprintf("%+.2f%% (%s)", stats.worstProfit*100, stats.worstName)
		avgStr := fmt.Sprintf("%+.2f%%", avgProfit*100)

		content.WriteString(fmt.Sprintf("| %s | %d | %s | %s | %s |\n",
			category, stats.count, bestStr, worstStr, avgStr))
	}
	content.WriteString("\n")
}

// writeEfficiencyTable — создает таблицу эффективности сделок
func (p *MarkdownPrinter) writeEfficiencyTable(content *strings.Builder, results []BenchmarkResult) {
	// Создаем копию для сортировки по эффективности
	efficiency := make([]struct {
		name           string
		profitPerTrade float64
		totalProfit    float64
		tradeCount     int
	}, 0)

	for _, r := range results {
		if r.TradeCount > 0 {
			efficiency = append(efficiency, struct {
				name           string
				profitPerTrade float64
				totalProfit    float64
				tradeCount     int
			}{
				name:           r.Name,
				profitPerTrade: r.TotalProfit / float64(r.TradeCount),
				totalProfit:    r.TotalProfit,
				tradeCount:     r.TradeCount,
			})
		}
	}

	// Сортируем по прибыли на сделку
	sort.Slice(efficiency, func(i, j int) bool {
		return efficiency[i].profitPerTrade > efficiency[j].profitPerTrade
	})

	content.WriteString("| Стратегия | Прибыль на сделку | Общая прибыль | Количество сделок |\n")
	content.WriteString("|-----------|-------------------|---------------|-------------------|\n")

	// Берем топ-5
	limit := 5
	if len(efficiency) < limit {
		limit = len(efficiency)
	}

	for i := 0; i < limit; i++ {
		e := efficiency[i]
		profitPerTradeStr := fmt.Sprintf("%+.2f%%", e.profitPerTrade*100)
		totalProfitStr := fmt.Sprintf("%+.2f%%", e.totalProfit*100)

		content.WriteString(fmt.Sprintf("| %s | %s | %s | %d |\n",
			e.name, profitPerTradeStr, totalProfitStr, e.tradeCount))
	}
	content.WriteString("\n")
}

// writePerformanceAnalysis — создает таблицу анализа производительности
func (p *MarkdownPrinter) writePerformanceAnalysis(content *strings.Builder, results []BenchmarkResult) {
	fast := []BenchmarkResult{}
	medium := []BenchmarkResult{}
	slow := []BenchmarkResult{}

	for _, r := range results {
		if r.ExecutionTime < 100*time.Millisecond {
			fast = append(fast, r)
		} else if r.ExecutionTime < time.Second {
			medium = append(medium, r)
		} else {
			slow = append(slow, r)
		}
	}

	content.WriteString("| Категория времени | Количество стратегий | Средняя прибыль |\n")
	content.WriteString("|-------------------|---------------------|----------------|\n")

	categories := []struct {
		name       string
		strategies []BenchmarkResult
	}{
		{"Быстрые (< 100ms)", fast},
		{"Средние (100ms - 1s)", medium},
		{"Медленные (> 1s)", slow},
	}

	for _, cat := range categories {
		if len(cat.strategies) > 0 {
			totalProfit := 0.0
			for _, s := range cat.strategies {
				totalProfit += s.TotalProfit
			}
			avgProfit := totalProfit / float64(len(cat.strategies))
			avgProfitStr := fmt.Sprintf("%+.2f%%", avgProfit*100)

			content.WriteString(fmt.Sprintf("| %s | %d | %s |\n",
				cat.name, len(cat.strategies), avgProfitStr))
		}
	}
	content.WriteString("\n")
}

// formatDurationMD — форматирует длительность для Markdown
func (p *MarkdownPrinter) formatDurationMD(d time.Duration) string {
	if d > time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.0fms", float64(d.Nanoseconds())/1e6)
}

// countCategories — подсчитывает количество стратегий по категориям
func (p *MarkdownPrinter) countCategories(results []BenchmarkResult) map[string]int {
	categories := make(map[string]int)
	for _, r := range results {
		category := p.getStrategyCategory(r.Name)
		categories[category]++
	}
	return categories
}

// PrintProgress — заглушка для совместимости с интерфейсом
func (p *MarkdownPrinter) PrintProgress(current, total int) {
	// Markdown принтер не выводит прогресс в консоль
}

// CombinedPrinter — принтер, который выводит и в консоль, и в Markdown
type CombinedPrinter struct {
	consolePrinter  *ConsolePrinter
	markdownPrinter *MarkdownPrinter
}

// NewCombinedPrinter — конструктор для CombinedPrinter
func NewCombinedPrinter() *CombinedPrinter {
	return &CombinedPrinter{
		consolePrinter:  NewConsolePrinter(),
		markdownPrinter: NewMarkdownPrinter(),
	}
}

// PrintComparison — выводит результаты и в консоль, и в Markdown файл
func (p *CombinedPrinter) PrintComparison(results []BenchmarkResult) {
	// Сначала выводим в консоль
	p.consolePrinter.PrintComparison(results)

	// Затем сохраняем в Markdown
	p.markdownPrinter.PrintComparison(results)
}

// PrintProgress — выводит прогресс в консоль
func (p *CombinedPrinter) PrintProgress(current, total int) {
	p.consolePrinter.PrintProgress(current, total)
}
