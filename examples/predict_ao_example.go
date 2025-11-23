package main

import (
	"bt/internal"
	"bt/strategies/v2/oscillators"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

func main() {
	fmt.Println("🚀 Тестирование предсказания сигналов Awesome Oscillator")
	fmt.Println(strings.Repeat("=", 80))

	// Загружаем данные свечей
	candlesFile := "tmos_big.json"
	if len(os.Args) > 1 {
		candlesFile = os.Args[1]
	}

	candles, err := loadCandlesAO(candlesFile)
	if err != nil {
		log.Fatalf("❌ Ошибка загрузки свечей: %v", err)
	}

	fmt.Printf("📊 Загружено %d свечей из файла %s\n", len(candles), candlesFile)
	fmt.Printf("📅 Период: %s - %s\n\n",
		candles[0].ToTime().Format("02.01.2006"),
		candles[len(candles)-1].ToTime().Format("02.01.2006"))

	// Создаем генератор сигналов
	generator := oscillators.NewAOSignalGenerator()

	// Тестируем разные конфигурации
	configs := []struct {
		name   string
		config *oscillators.AOConfig
	}{
		{
			name: "Классические параметры Билла Вильямса",
			config: &oscillators.AOConfig{
				FastPeriod:          5,
				SlowPeriod:          34,
				ConfirmByTwoCandles: false,
			},
		},
		{
			name: "Более чувствительный вариант",
			config: &oscillators.AOConfig{
				FastPeriod:          3,
				SlowPeriod:          21,
				ConfirmByTwoCandles: false,
			},
		},
		{
			name: "С подтверждением двумя свечами",
			config: &oscillators.AOConfig{
				FastPeriod:          5,
				SlowPeriod:          34,
				ConfirmByTwoCandles: true,
			},
		},
	}

	for i, cfg := range configs {
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("📋 Конфигурация %d: %s\n", i+1, cfg.name)
		fmt.Printf("⚙️  Параметры: %s\n", cfg.config.String())
		fmt.Println(strings.Repeat("-", 80))

		// Генерируем сигналы
		signals := generator.GenerateSignals(candles, cfg.config)

		// Запускаем бэктест
		result := internal.Backtest(candles, signals, 0.01)

		// Выводим результаты бэктеста
		fmt.Printf("💵 Финальный портфель:   $%.2f\n", result.FinalPortfolio)
		fmt.Printf("📊 Общая прибыль:        %+.2f%%\n", result.TotalProfit*100)
		fmt.Printf("🔄 Количество сделок:    %d\n", result.TradeCount)
		if result.TradeCount > 0 {
			fmt.Printf("📈 Прибыль на сделку:    %+.2f%%\n", (result.TotalProfit/float64(result.TradeCount))*100)
		}

		// Предсказываем следующий сигнал
		fmt.Println()
		fmt.Println("🔮 ПРЕДСКАЗАНИЕ СЛЕДУЮЩЕГО СИГНАЛА")
		fmt.Println(strings.Repeat("-", 80))

		futureSignal := generator.PredictNextSignal(candles, cfg.config)

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
				fmt.Println("✅ Высокая уверенность - AO быстро движется к нулю")
			} else if futureSignal.Confidence >= 0.5 {
				fmt.Println("⚠️  Средняя уверенность - движение стабильное, но медленное")
			} else {
				fmt.Println("⚠️  Низкая уверенность - слабое движение или далеко от нуля")
			}

			// Текущая медианная цена для сравнения
			currentCandle := candles[len(candles)-1]
			currentMedian := (currentCandle.High.ToFloat64() + currentCandle.Low.ToFloat64()) / 2
			priceChange := ((futureSignal.Price - currentMedian) / currentMedian) * 100
			fmt.Printf("\n📊 Текущая медианная цена: $%.4f\n", currentMedian)
			fmt.Printf("📈 Ожидаемое изменение:    %+.2f%%\n", priceChange)

		} else {
			fmt.Println("⚠️  Предсказание невозможно")
			fmt.Println("Возможные причины:")
			fmt.Println("  • Недостаточно данных для анализа")
			fmt.Println("  • AO движется от нулевой линии")
			fmt.Println("  • Слишком слабое движение AO")
			fmt.Println("  • Низкая уверенность в предсказании")
		}

		fmt.Println()
	}

	// Анализ последних сигналов
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("📋 ПОСЛЕДНИЕ СИГНАЛЫ (классическая конфигурация)")
	fmt.Println(strings.Repeat("=", 80))

	classicConfig := &oscillators.AOConfig{
		FastPeriod:          5,
		SlowPeriod:          34,
		ConfirmByTwoCandles: false,
	}
	signals := generator.GenerateSignals(candles, classicConfig)

	lastSignals := getLastSignalsAO(candles, signals, 5)
	if len(lastSignals) > 0 {
		fmt.Printf("%-15s %-12s %-12s\n", "Дата", "Тип", "Медианная цена")
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

type signalInfoAO struct {
	date       time.Time
	signalType internal.SignalType
	price      float64
}

func getLastSignalsAO(candles []internal.Candle, signals []internal.SignalType, count int) []signalInfoAO {
	result := []signalInfoAO{}

	for i := len(signals) - 1; i >= 0 && len(result) < count; i-- {
		if signals[i] != internal.HOLD {
			medianPrice := (candles[i].High.ToFloat64() + candles[i].Low.ToFloat64()) / 2
			result = append(result, signalInfoAO{
				date:       candles[i].ToTime(),
				signalType: signals[i],
				price:      medianPrice,
			})
		}
	}

	// Разворачиваем, чтобы показать от старых к новым
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

func loadCandlesAO(filename string) ([]internal.Candle, error) {
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
