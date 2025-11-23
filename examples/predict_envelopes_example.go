package main

import (
	"bt/internal"
	_ "bt/strategies/v2/volatility"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Использование: predict_envelopes <файл_с_данными.json>")
		fmt.Println("Пример: predict_envelopes tmos_big.json")
		os.Exit(1)
	}

	filename := os.Args[1]

	// Загружаем данные
	fmt.Printf("📂 Загрузка данных из %s...\n", filename)
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("❌ Ошибка чтения файла: %v", err)
	}

	var wrapper struct {
		Candles []internal.Candle `json:"candles"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		log.Fatalf("❌ Ошибка парсинга JSON: %v", err)
	}
	candles := wrapper.Candles

	fmt.Printf("✅ Загружено %d свечей\n", len(candles))
	fmt.Printf("📅 Период: %s - %s\n\n",
		candles[0].ToTime().Format("02.01.2006"),
		candles[len(candles)-1].ToTime().Format("02.01.2006"))

	// Получаем стратегию
	strategy, ok := internal.GetStrategyV2("envelopes_v2")
	if !ok {
		log.Fatal("❌ Стратегия envelopes_v2 не найдена")
	}

	fmt.Println("🔧 Оптимизация параметров стратегии...")
	config := strategy.Optimize(candles, strategy)

	fmt.Printf("✅ Оптимальная конфигурация: %s\n\n", config.String())

	// Генерируем сигналы
	fmt.Println("📊 Генерация торговых сигналов...")
	signals := strategy.GenerateSignals(candles, config)

	// Подсчитываем сигналы
	buyCount := 0
	sellCount := 0
	for _, sig := range signals {
		if sig == internal.BUY {
			buyCount++
		} else if sig == internal.SELL {
			sellCount++
		}
	}

	fmt.Printf("   🟢 BUY сигналов:  %d\n", buyCount)
	fmt.Printf("   🔴 SELL сигналов: %d\n\n", sellCount)

	// Выполняем бэктест
	fmt.Println("💹 Выполнение бэктестинга...")
	result := internal.Backtest(candles, signals, 0.01)

	fmt.Printf("   💰 Прибыль: %+.2f%%\n", result.TotalProfit*100)
	fmt.Printf("   💵 Финальный портфель: $%.2f\n", result.FinalPortfolio)
	fmt.Printf("   🔄 Количество сделок: %d\n\n", result.TradeCount)

	// Предсказываем следующий сигнал
	fmt.Println(string([]rune{0x1F52E}) + " Предсказание следующего сигнала...")
	fmt.Println("─────────────────────────────────────────────────────────")

	// Проверяем, поддерживает ли стратегия предсказание
	strategyBase, ok := strategy.(*internal.StrategyBase)
	if !ok {
		log.Fatal("❌ Стратегия не поддерживает предсказание")
	}

	futureSignal := strategyBase.PredictNextSignal(candles, config)

	if futureSignal == nil {
		fmt.Println("⚠️  Не удалось предсказать следующий сигнал")
		fmt.Println("    Возможные причины:")
		fmt.Println("    - Недостаточно данных")
		fmt.Println("    - Слабое направленное движение")
		fmt.Println("    - Низкая уверенность в предсказании")
		return
	}

	// Выводим информацию о предсказании
	signalTime := time.Unix(futureSignal.Date, 0)
	lastCandleTime := candles[len(candles)-1].ToTime()
	daysUntilSignal := signalTime.Sub(lastCandleTime).Hours() / 24

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

	fmt.Printf("\n%s Тип сигнала:     %s\n", signalEmoji, signalTypeStr)
	fmt.Printf("📅 Ожидаемая дата:  %s\n", signalTime.Format("02.01.2006 15:04"))
	fmt.Printf("⏳ Через:           %.1f дней\n", daysUntilSignal)
	fmt.Printf("💵 Ожидаемая цена:  $%.4f\n", futureSignal.Price)
	fmt.Printf("🎯 Уверенность:     %.1f%%\n", futureSignal.Confidence*100)

	// Интерпретация уверенности
	fmt.Println("\n📊 Интерпретация уверенности:")
	if futureSignal.Confidence >= 0.70 {
		fmt.Println("   ✅ Высокая уверенность - сильный сигнал")
	} else if futureSignal.Confidence >= 0.50 {
		fmt.Println("   ⚠️  Средняя уверенность - умеренный сигнал")
	} else {
		fmt.Println("   ⚠️  Низкая уверенность - слабый сигнал")
	}

	// Дополнительная информация
	lastPrice := candles[len(candles)-1].Close.ToFloat64()
	priceChange := (futureSignal.Price - lastPrice) / lastPrice * 100

	fmt.Println("\n📈 Дополнительная информация:")
	fmt.Printf("   Текущая цена:        $%.4f\n", lastPrice)
	fmt.Printf("   Ожидаемое изменение: %+.2f%%\n", priceChange)

	fmt.Println("\n─────────────────────────────────────────────────────────")
	fmt.Println("✅ Анализ завершен")
}
