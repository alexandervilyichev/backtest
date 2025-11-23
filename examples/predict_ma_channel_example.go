package main

import (
	"bt/internal"
	"bt/strategies/v2/momentum"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	// Загружаем данные свечей из файла
	candlesFile := "tmos_big.json"
	if len(os.Args) > 1 {
		candlesFile = os.Args[1]
	}

	fmt.Printf("📊 Загрузка данных из %s...\n", candlesFile)
	candles, err := loadCandles(candlesFile)
	if err != nil {
		log.Fatalf("❌ Ошибка загрузки данных: %v", err)
	}

	fmt.Printf("✅ Загружено %d свечей\n", len(candles))
	fmt.Printf("📅 Период: %s - %s\n\n",
		candles[0].ToTime().Format("2006-01-02 15:04"),
		candles[len(candles)-1].ToTime().Format("2006-01-02 15:04"))

	// Создаем генератор сигналов
	generator := momentum.NewMAChannelSignalGenerator()

	// Тестируем разные конфигурации
	configs := []*momentum.MAChannelConfig{
		{FastPeriod: 10, SlowPeriod: 20, Multiplier: 1.0},
		{FastPeriod: 8, SlowPeriod: 21, Multiplier: 1.5},
		{FastPeriod: 12, SlowPeriod: 26, Multiplier: 0.8},
		{FastPeriod: 5, SlowPeriod: 15, Multiplier: 2.0},
	}

	fmt.Println("=== Тестирование MA Channel V2 с предсказанием ===\n")

	for _, config := range configs {
		fmt.Printf("⚙️  Конфигурация: %s\n", config.String())
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		// Генерируем сигналы
		signals := generator.GenerateSignals(candles, config)

		// Бэктест
		result := internal.Backtest(candles, signals, 0.01)

		fmt.Printf("\n📈 Результаты бэктеста:\n")
		fmt.Printf("   Прибыль: %.2f%%\n", result.TotalProfit*100)

		// Предсказываем следующий сигнал
		fmt.Println("\n🔮 Предсказание следующего сигнала...")
		futureSignal := generator.PredictNextSignal(candles, config)

		if futureSignal != nil {
			signalTypeStr := "HOLD"
			signalEmoji := "⏸️"
			signalDescription := ""
			switch futureSignal.SignalType {
			case internal.BUY:
				signalTypeStr = "BUY (Пробой верхнего канала)"
				signalEmoji = "🟢"
				signalDescription = "Цена пробьет верхнюю границу канала"
			case internal.SELL:
				signalTypeStr = "SELL (Пробой нижнего канала)"
				signalEmoji = "🔴"
				signalDescription = "Цена пробьет нижнюю границу канала"
			}

			fmt.Printf("\n✨ Предсказание успешно!\n")
			fmt.Printf("%s Тип сигнала:    %s\n", signalEmoji, signalTypeStr)
			fmt.Printf("📝 Описание:       %s\n", signalDescription)
			fmt.Printf("📅 Дата сигнала:   %s\n", time.Unix(futureSignal.Date, 0).Format("2006-01-02 15:04:05"))
			fmt.Printf("💰 Цена:           %.4f\n", futureSignal.Price)
			fmt.Printf("📊 Уверенность:    %.2f%%\n", futureSignal.Confidence*100)

			// Вычисляем время до сигнала
			lastCandleTime := candles[len(candles)-1].ToTime()
			signalTime := time.Unix(futureSignal.Date, 0)
			timeUntilSignal := signalTime.Sub(lastCandleTime)
			fmt.Printf("⏰ Время до сигнала: %s\n", formatDuration(timeUntilSignal))

			// Вычисляем изменение цены
			lastPrice := candles[len(candles)-1].Close.ToFloat64()
			priceChange := (futureSignal.Price - lastPrice) / lastPrice * 100
			priceChangeEmoji := "📈"
			if priceChange < 0 {
				priceChangeEmoji = "📉"
			}
			fmt.Printf("%s Изменение цены:  %+.2f%%\n", priceChangeEmoji, priceChange)

			// Рекомендации
			fmt.Println("\n💡 Рекомендации:")
			if futureSignal.Confidence >= 0.7 {
				fmt.Println("   ✅ Высокая уверенность - сигнал надежный")
			} else if futureSignal.Confidence >= 0.5 {
				fmt.Println("   ⚠️  Средняя уверенность - рекомендуется подтверждение")
			} else {
				fmt.Println("   ⚠️  Низкая уверенность - используйте с осторожностью")
			}
		} else {
			fmt.Println("❌ Не удалось предсказать ближайший сигнал")
			fmt.Println("   Возможные причины:")
			fmt.Println("   - Недостаточно данных")
			fmt.Println("   - Цена не движется к границам канала")
			fmt.Println("   - Пробой слишком далеко в будущем")
		}

		fmt.Println()
	}

	// Оптимизация
	fmt.Println("\n=== Оптимизация параметров ===")
	fmt.Println("Запуск grid search оптимизации...")

	strategy := momentum.NewMAChannelStrategyV2(0.01)
	optimizedConfig := strategy.Optimize(candles, generator)

	if optimizedConfig != nil {
		fmt.Printf("\n✅ Оптимальная конфигурация: %s\n", optimizedConfig.String())

		// Тестируем оптимизированную конфигурацию
		signals := generator.GenerateSignals(candles, optimizedConfig)
		result := internal.Backtest(candles, signals, 0.01)

		fmt.Printf("\n📈 Результаты с оптимизированными параметрами:\n")
		fmt.Printf("   Прибыль: %.2f%%\n", result.TotalProfit*100)

		// Предсказание с оптимизированными параметрами
		futureSignal := generator.PredictNextSignal(candles, optimizedConfig)
		if futureSignal != nil {
			fmt.Printf("\n🔮 Предсказание с оптимизированными параметрами:\n")
			fmt.Printf("   Тип: %s\n", futureSignal.SignalType)
			fmt.Printf("   Дата: %s\n", time.Unix(futureSignal.Date, 0).Format("2006-01-02 15:04"))
			fmt.Printf("   Цена: %.4f\n", futureSignal.Price)
			fmt.Printf("   Уверенность: %.2f%%\n", futureSignal.Confidence*100)
		}
	}

	fmt.Println("\n📖 О стратегии MA Channel:")
	fmt.Println("   MA Channel использует канал на основе быстрой и медленной EMA")
	fmt.Println("   BUY - при пробое цены выше верхнего канала (бычий breakout)")
	fmt.Println("   SELL - при пробое цены ниже нижнего канала (медвежий breakout)")
	fmt.Println("   Ширина канала = |FastEMA - SlowEMA| × Multiplier")
}

func loadCandles(filename string) ([]internal.Candle, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл: %w", err)
	}

	// Пробуем сначала как массив
	var candles []internal.Candle
	if err := json.Unmarshal(data, &candles); err == nil {
		return candles, nil
	}

	// Если не получилось, пробуем как объект с полем candles
	var response internal.GetCandlesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("не удалось распарсить JSON: %w", err)
	}

	return response.Candles, nil
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "в прошлом"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours == 0 {
		return fmt.Sprintf("%d минут", minutes)
	}

	days := hours / 24
	hours = hours % 24

	if days == 0 {
		return fmt.Sprintf("%d часов %d минут", hours, minutes)
	}

	if hours == 0 {
		return fmt.Sprintf("%d дней", days)
	}

	return fmt.Sprintf("%d дней %d часов", days, hours)
}
