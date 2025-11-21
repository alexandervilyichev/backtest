package main

import (
	"bt/internal"
	"encoding/json"
	"fmt"
	"log"
	"os"
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

	fmt.Printf("✅ Загружено %d свечей\n\n", len(candles))

	// Извлекаем цены
	prices := make([]float64, len(candles))
	for i, candle := range candles {
		prices[i] = candle.Close.ToFloat64()
	}

	// Тестируем разные периоды
	testConfigs := []struct {
		fast, slow int
	}{
		{12, 26},
		{20, 50},
		{50, 200},
	}

	for _, cfg := range testConfigs {
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("Тестирование: Fast=%d, Slow=%d\n", cfg.fast, cfg.slow)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

		fastEMA := internal.CalculateEMAForValues(prices, cfg.fast)
		slowEMA := internal.CalculateEMAForValues(prices, cfg.slow)

		if fastEMA == nil || slowEMA == nil {
			fmt.Println("❌ Не удалось рассчитать EMA")
			continue
		}

		currentIdx := len(candles) - 1
		currFast := fastEMA[currentIdx]
		currSlow := slowEMA[currentIdx]

		fmt.Printf("Текущая Fast EMA: %.4f\n", currFast)
		fmt.Printf("Текущая Slow EMA: %.4f\n", currSlow)
		fmt.Printf("Разница: %.4f (%.2f%%)\n", currFast-currSlow, (currFast-currSlow)/currSlow*100)

		if currFast > currSlow {
			fmt.Println("Состояние: Fast выше Slow (бычий тренд)")
		} else {
			fmt.Println("Состояние: Fast ниже Slow (медвежий тренд)")
		}

		// Вычисляем скорости
		lookback := 5
		if currentIdx >= lookback {
			fastVelocity := (fastEMA[currentIdx] - fastEMA[currentIdx-lookback]) / float64(lookback)
			slowVelocity := (slowEMA[currentIdx] - slowEMA[currentIdx-lookback]) / float64(lookback)
			relativeVelocity := fastVelocity - slowVelocity

			fmt.Printf("\nСкорость Fast EMA: %.6f\n", fastVelocity)
			fmt.Printf("Скорость Slow EMA: %.6f\n", slowVelocity)
			fmt.Printf("Относительная скорость: %.6f\n", relativeVelocity)

			if relativeVelocity > 0 {
				fmt.Println("Направление: Fast ускоряется относительно Slow (расхождение вверх)")
			} else if relativeVelocity < 0 {
				fmt.Println("Направление: Fast замедляется относительно Slow (сближение/расхождение вниз)")
			} else {
				fmt.Println("Направление: Параллельное движение")
			}

			// Проверяем условия для предсказания
			isFastAbove := currFast > currSlow
			distance := currFast - currSlow

			fmt.Printf("\nАнализ возможности предсказания:\n")

			if isFastAbove && relativeVelocity > 0 {
				fmt.Println("❌ Fast выше и расходится вверх - пересечение невозможно")
			} else if !isFastAbove && relativeVelocity < 0 {
				fmt.Println("❌ Fast ниже и расходится вниз - пересечение невозможно")
			} else if internal.Abs(relativeVelocity) < 0.0001 {
				fmt.Printf("❌ Скорость сближения слишком мала (%.6f < 0.0001)\n", internal.Abs(relativeVelocity))
			} else {
				candlesUntilCross := internal.Abs(distance / relativeVelocity)
				fmt.Printf("✅ Предсказание возможно!\n")
				fmt.Printf("   Свечей до пересечения: %.1f\n", candlesUntilCross)

				maxHorizon := float64(cfg.slow)
				if candlesUntilCross > maxHorizon {
					fmt.Printf("   ❌ Но слишком далеко (%.1f > %.1f)\n", candlesUntilCross, maxHorizon)
				} else {
					if isFastAbove {
						fmt.Println("   Ожидается: Death Cross (SELL)")
					} else {
						fmt.Println("   Ожидается: Golden Cross (BUY)")
					}
				}
			}
		}

		fmt.Println()
	}
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
