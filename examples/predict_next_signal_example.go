package main

import (
	"bt/internal"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	// Импортируем стратегии для регистрации
	_ "bt/strategies/v2/oscillators"
	_ "bt/strategies/v2/trend"
	_ "bt/strategies/v2/wave"
)

func main() {
	// Параметры командной строки
	strategyName := flag.String("strategy", "predictive_linear_spline_v2", "Название стратегии для предсказания")
	candlesFile := flag.String("file", "tmos_big.json", "Файл с данными свечей")
	flag.Parse()

	fmt.Printf("📊 Загрузка данных из %s...\n", *candlesFile)
	candles, err := loadCandles(*candlesFile)
	if err != nil {
		log.Fatalf("❌ Ошибка загрузки данных: %v", err)
	}

	fmt.Printf("✅ Загружено %d свечей\n", len(candles))
	fmt.Printf("📅 Период: %s - %s\n",
		candles[0].ToTime().Format("2006-01-02 15:04"),
		candles[len(candles)-1].ToTime().Format("2006-01-02 15:04"))

	// Получаем стратегию из реестра
	fmt.Printf("\n🔍 Поиск стратегии: %s\n", *strategyName)
	strategy, ok := internal.GetStrategyV2(*strategyName)
	if !ok {
		fmt.Printf("❌ Стратегия '%s' не найдена\n", *strategyName)
		fmt.Println("\n📋 Доступные стратегии V2:")
		for _, name := range internal.GetStrategyNamesV2() {
			fmt.Printf("   - %s\n", name)
		}
		os.Exit(1)
	}

	fmt.Printf("✅ Стратегия найдена: %s\n", strategy.Name())

	// Получаем конфигурацию по умолчанию
	config := strategy.DefaultConfig()
	fmt.Printf("\n⚙️  Конфигурация: %s\n", config.String())

	// Проверяем, поддерживает ли стратегия предсказание
	fmt.Println("\n🔮 Предсказание ближайшего сигнала...")
	
	// Используем метод из StrategyBase
	var futureSignal *internal.FutureSignal
	if strategyBase, ok := strategy.(*internal.StrategyBase); ok {
		futureSignal = strategyBase.PredictNextSignal(candles, config)
	} else {
		fmt.Println("❌ Стратегия не поддерживает интерфейс StrategyBase")
		os.Exit(1)
	}

	if futureSignal == nil {
		fmt.Println("❌ Не удалось предсказать ближайший сигнал")
		fmt.Println("   Возможные причины:")
		fmt.Println("   - Стратегия не поддерживает предсказание")
		fmt.Println("   - Недостаточно данных")
		fmt.Println("   - Текущий тренд слишком слабый")
		fmt.Println("   - Низкая уверенность в предсказании")
		fmt.Println("\n💡 Стратегии с поддержкой предсказания:")
		fmt.Println("   - predictive_linear_spline_v2")
		fmt.Println("   - elliott_wave_v2")
		fmt.Println("   - golden_cross_v2")
		fmt.Println("   - cci_oscillator_v2")
		return
	}

	// Выводим результат
	fmt.Println("\n✨ Предсказание успешно!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	fmt.Printf("📊 Стратегия:      %s\n", strategy.Name())
	
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
	fmt.Printf("� Дата сигснала:   %s\n", time.Unix(futureSignal.Date, 0).Format("2006-01-02 15:04:05"))
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
	priceChangeEmoji := "�"
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
	
	fmt.Println("\n📝 Использование:")
	fmt.Printf("   ./predict_signal -strategy %s -file %s\n", strategy.Name(), *candlesFile)
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
