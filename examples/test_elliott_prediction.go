package main

import (
	"bt/internal"
	"bt/strategies/v2/wave"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
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

	// Создаем генератор сигналов
	generator := wave.NewElliottWaveSignalGenerator()

	// Конфигурация стратегии
	config := &wave.ElliottWaveConfig{
		MinWaveLength:      3,
		MaxWaveLength:      30,
		FibonacciThreshold: 0.5,
		TrendStrength:      0.2,
	}

	fmt.Printf("\n⚙️  Конфигурация: %s\n", config.String())

	// Предсказываем ближайший сигнал
	fmt.Println("\n🔮 Предсказание ближайшего сигнала Elliott Wave...")
	futureSignal := generator.PredictNextSignal(candles, config)

	if futureSignal == nil {
		fmt.Println("❌ Не удалось предсказать ближайший сигнал")
		fmt.Println("   Возможные причины:")
		fmt.Println("   - Недостаточно волновых точек")
		fmt.Println("   - Волновая структура не определена")
		fmt.Println("   - Текущая позиция не подходит для предсказания")
		return
	}

	// Выводим результат
	fmt.Println("\n✨ Предсказание успешно!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	signalTypeStr := "HOLD"
	signalEmoji := "⏸️"
	switch futureSignal.SignalType {
	case internal.BUY:
		signalTypeStr = "BUY"
		signalEmoji = "🟢"
	case internal.SELL:
		signalTypeStr = "SELL"
		signalEmoji = "🔴"
	}

	fmt.Printf("%s Тип сигнала:    %s\n", signalEmoji, signalTypeStr)
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
}

func loadCandles(filename string) ([]internal.Candle, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл: %w", err)
	}

	var candles []internal.Candle
	if err := json.Unmarshal(data, &candles); err == nil {
		return candles, nil
	}

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
