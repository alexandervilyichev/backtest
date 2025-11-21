package main

import (
	"bt/internal"
	"bt/strategies/v2/trend"
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
	fmt.Printf("📅 Период: %s - %s\n",
		candles[0].ToTime().Format("2006-01-02 15:04"),
		candles[len(candles)-1].ToTime().Format("2006-01-02 15:04"))

	// Создаем генератор сигналов
	generator := trend.NewGoldenCrossSignalGenerator()

	// Конфигурация стратегии
	config := &trend.GoldenCrossConfig{
		FastPeriod: 12, // Быстрая EMA
		SlowPeriod: 26, // Медленная EMA
	}

	fmt.Printf("\n⚙️  Конфигурация: %s\n", config.String())

	// Предсказываем ближайший сигнал
	fmt.Println("\n🔮 Предсказание ближайшего Golden/Death Cross...")
	futureSignal := generator.PredictNextSignal(candles, config)

	if futureSignal == nil {
		fmt.Println("❌ Не удалось предсказать ближайший сигнал")
		fmt.Println("   Возможные причины:")
		fmt.Println("   - Недостаточно данных")
		fmt.Println("   - EMA линии расходятся (нет пересечения в обозримом будущем)")
		fmt.Println("   - Скорость сближения слишком мала")
		fmt.Println("   - Пересечение слишком далеко в будущем")
		return
	}

	// Выводим результат
	fmt.Println("\n✨ Предсказание успешно!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	signalTypeStr := "HOLD"
	signalEmoji := "⏸️"
	signalDescription := ""
	switch futureSignal.SignalType {
	case internal.BUY:
		signalTypeStr = "BUY (Golden Cross)"
		signalEmoji = "🟢"
		signalDescription = "Быстрая EMA пересечет медленную снизу вверх"
	case internal.SELL:
		signalTypeStr = "SELL (Death Cross)"
		signalEmoji = "🔴"
		signalDescription = "Быстрая EMA пересечет медленную сверху вниз"
	}

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

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Дополнительная информация
	fmt.Println("\n💡 Рекомендации:")
	if futureSignal.Confidence >= 0.7 {
		fmt.Println("   ✅ Высокая уверенность - сигнал надежный")
	} else if futureSignal.Confidence >= 0.5 {
		fmt.Println("   ⚠️  Средняя уверенность - рекомендуется подтверждение")
	} else {
		fmt.Println("   ⚠️  Низкая уверенность - используйте с осторожностью")
	}

	if timeUntilSignal.Hours() < 24 {
		fmt.Println("   ⏰ Сигнал ожидается в ближайшее время")
	} else if timeUntilSignal.Hours() < 168 {
		fmt.Println("   📅 Сигнал ожидается на этой неделе")
	} else {
		fmt.Println("   📅 Сигнал ожидается через длительное время")
	}

	fmt.Println("\n📖 О стратегии Golden Cross:")
	fmt.Println("   Golden Cross - бычий сигнал, когда быстрая EMA пересекает медленную снизу вверх")
	fmt.Println("   Death Cross - медвежий сигнал, когда быстрая EMA пересекает медленную сверху вниз")
	fmt.Printf("   Текущие параметры: Fast EMA = %d, Slow EMA = %d\n", config.FastPeriod, config.SlowPeriod)
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
