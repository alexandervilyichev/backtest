package main

import (
	"bt/internal"
	"bt/strategies/v2/trend"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

func main() {
	fmt.Println("🚀 Тестирование предсказания сигналов SuperTrend")
	fmt.Println(strings.Repeat("=", 80))

	// Загружаем данные свечей
	candlesFile := "tmos_big.json"
	if len(os.Args) > 1 {
		candlesFile = os.Args[1]
	}

	candles, err := loadCandlesSupertrend(candlesFile)
	if err != nil {
		log.Fatalf("❌ Ошибка загрузки свечей: %v", err)
	}

	fmt.Printf("📊 Загружено %d свечей из файла %s\n", len(candles), candlesFile)
	fmt.Printf("📅 Период: %s - %s\n\n",
		candles[0].ToTime().Format("02.01.2006"),
		candles[len(candles)-1].ToTime().Format("02.01.2006"))

	// Создаем генератор сигналов
	generator := trend.NewSupertrendSignalGenerator()

	// Используем оптимизированную конфигурацию
	config := &trend.SupertrendConfig{
		Period:     10,
		Multiplier: 3.0,
	}

	fmt.Printf("⚙️  Конфигурация: %s\n\n", config.String())

	// Генерируем сигналы
	signals := generator.GenerateSignals(candles, config)

	// Запускаем бэктест
	result := internal.Backtest(candles, signals, 0.01)

	// Выводим результаты бэктеста
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("📈 РЕЗУЛЬТАТЫ БЭКТЕСТА")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("💵 Финальный портфель:   $%.2f\n", result.FinalPortfolio)
	fmt.Printf("📊 Общая прибыль:        %+.2f%%\n", result.TotalProfit*100)
	fmt.Printf("🔄 Количество сделок:    %d\n", result.TradeCount)
	if result.TradeCount > 0 {
		fmt.Printf("📈 Прибыль на сделку:    %+.2f%%\n", (result.TotalProfit/float64(result.TradeCount))*100)
	}
	fmt.Println()

	// Предсказываем следующий сигнал
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("🔮 ПРЕДСКАЗАНИЕ СЛЕДУЮЩЕГО СИГНАЛА")
	fmt.Println(strings.Repeat("=", 80))

	futureSignal := generator.PredictNextSignal(candles, config)

	if futureSignal != nil {
		signalTime := time.Unix(futureSignal.Date, 0)
		lastCandleTime := candles[len(candles)-1].ToTime()
		daysUntil := signalTime.Sub(lastCandleTime).Hours() / 24

		signalTypeStr := ""
		signalEmoji := ""
		switch futureSignal.SignalType {
		case internal.BUY:
			signalTypeStr = "BUY (Покупка)"
			signalEmoji = "🟢"
		case internal.SELL:
			signalTypeStr = "SELL (Продажа)"
			signalEmoji = "🔴"
		default:
			signalTypeStr = "HOLD (Удержание)"
			signalEmoji = "⏸️"
		}

		fmt.Printf("%s Тип сигнала:         %s\n", signalEmoji, signalTypeStr)
		fmt.Printf("📅 Ожидаемая дата:      %s\n", signalTime.Format("02.01.2006 15:04"))
		fmt.Printf("⏱️  Дней до сигнала:     %.1f\n", daysUntil)
		fmt.Printf("💵 Ожидаемая цена:      $%.4f\n", futureSignal.Price)
		fmt.Printf("🎯 Уверенность:         %.1f%%\n", futureSignal.Confidence*100)

		// Интерпретация уверенности
		fmt.Println()
		if futureSignal.Confidence >= 0.7 {
			fmt.Println("✅ Высокая уверенность - сигнал очень вероятен")
		} else if futureSignal.Confidence >= 0.5 {
			fmt.Println("⚠️  Средняя уверенность - сигнал вероятен, но требует подтверждения")
		} else {
			fmt.Println("⚠️  Низкая уверенность - сигнал возможен, но ненадежен")
		}

		// Текущая цена для сравнения
		currentPrice := candles[len(candles)-1].Close.ToFloat64()
		priceChange := ((futureSignal.Price - currentPrice) / currentPrice) * 100
		fmt.Printf("\n📊 Текущая цена:        $%.4f\n", currentPrice)
		fmt.Printf("📈 Ожидаемое изменение: %+.2f%%\n", priceChange)

	} else {
		fmt.Println("⚠️  Предсказание невозможно")
		fmt.Println("Возможные причины:")
		fmt.Println("  • Недостаточно данных для анализа")
		fmt.Println("  • Цена движется в направлении от линии SuperTrend")
		fmt.Println("  • Слишком слабое движение цены")
		fmt.Println("  • Низкая уверенность в предсказании")
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))

	// Анализ последних сигналов
	fmt.Println("\n📋 ПОСЛЕДНИЕ СИГНАЛЫ")
	fmt.Println(strings.Repeat("=", 80))

	lastSignals := getLastSignals(candles, signals, 5)
	if len(lastSignals) > 0 {
		fmt.Printf("%-15s %-12s %-12s\n", "Дата", "Тип", "Цена")
		fmt.Println(strings.Repeat("-", 40))
		for _, sig := range lastSignals {
			signalStr := ""
			switch sig.signalType {
			case internal.BUY:
				signalStr = "🟢 BUY"
			case internal.SELL:
				signalStr = "🔴 SELL"
			default:
				signalStr = "⏸️ HOLD"
			}
			fmt.Printf("%-15s %-12s $%-10.4f\n",
				sig.date.Format("02.01.2006"),
				signalStr,
				sig.price)
		}
	} else {
		fmt.Println("Нет сигналов в истории")
	}

	fmt.Println()
	fmt.Println("✅ Тестирование завершено")
}

type signalInfo struct {
	date       time.Time
	signalType internal.SignalType
	price      float64
}

func getLastSignals(candles []internal.Candle, signals []internal.SignalType, count int) []signalInfo {
	result := []signalInfo{}

	for i := len(signals) - 1; i >= 0 && len(result) < count; i-- {
		if signals[i] != internal.HOLD {
			result = append(result, signalInfo{
				date:       candles[i].ToTime(),
				signalType: signals[i],
				price:      candles[i].Close.ToFloat64(),
			})
		}
	}

	// Разворачиваем, чтобы показать от старых к новым
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

func loadCandlesSupertrend(filename string) ([]internal.Candle, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл: %w", err)
	}

	var wrapper struct {
		Candles []internal.Candle `json:"candles"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("не удалось распарсить JSON: %w", err)
	}

	return wrapper.Candles, nil
}
