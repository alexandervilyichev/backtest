package main

import (
	"bt/internal"
	"bt/strategies/v2/trend"
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

	fmt.Printf("✅ Загружено %d свечей\n\n", len(candles))

	// Тест 1: Predictive Linear Spline с ОПТИМИЗИРОВАННОЙ конфигурацией
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("Тест 1: Predictive Linear Spline (ОПТИМИЗИРОВАННАЯ)")
	fmt.Println("═══════════════════════════════════════════════════════════")

	plsGenerator := trend.NewPredictiveLinearSplineSignalGenerator()
	plsConfig := &trend.PredictiveLinearSplineConfig{
		MinSegmentLength:      125,
		MaxSegmentLength:      445,
		PredictionHorizon:     5,
		MinR2Threshold:        0.65,
		SignalAdvance:         5,
		MinSlopeThreshold:     0.00055,
		TrendExhaustionFactor: 0.60,
		MinPriceChange:        0.008,
	}

	plsSignal := plsGenerator.PredictNextSignal(candles, plsConfig)
	if plsSignal != nil {
		fmt.Printf("Тип сигнала: %s\n", plsSignal.SignalType)
		fmt.Printf("Unix timestamp: %d\n", plsSignal.Date)
		fmt.Printf("Дата (RFC3339): %s\n", time.Unix(plsSignal.Date, 0).Format(time.RFC3339))
		fmt.Printf("Дата (02.01 15:04): %s\n", time.Unix(plsSignal.Date, 0).Format("02.01 15:04"))
		fmt.Printf("Цена: %.4f\n", plsSignal.Price)
		fmt.Printf("Уверенность: %.2f%%\n\n", plsSignal.Confidence*100)
	} else {
		fmt.Println("Предсказание не удалось\n")
	}

	// Тест 2: Elliott Wave с ОПТИМИЗИРОВАННОЙ конфигурацией
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("Тест 2: Elliott Wave (ОПТИМИЗИРОВАННАЯ)")
	fmt.Println("═══════════════════════════════════════════════════════════")

	ewGenerator := wave.NewElliottWaveSignalGenerator()
	ewConfig := &wave.ElliottWaveConfig{
		MinWaveLength:      3,
		MaxWaveLength:      30,
		FibonacciThreshold: 0.5,
		TrendStrength:      0.2,
	}

	ewSignal := ewGenerator.PredictNextSignal(candles, ewConfig)
	if ewSignal != nil {
		fmt.Printf("Тип сигнала: %s\n", ewSignal.SignalType)
		fmt.Printf("Unix timestamp: %d\n", ewSignal.Date)
		fmt.Printf("Дата (RFC3339): %s\n", time.Unix(ewSignal.Date, 0).Format(time.RFC3339))
		fmt.Printf("Дата (02.01 15:04): %s\n", time.Unix(ewSignal.Date, 0).Format("02.01 15:04"))
		fmt.Printf("Цена: %.4f\n", ewSignal.Price)
		fmt.Printf("Уверенность: %.2f%%\n\n", ewSignal.Confidence*100)
	} else {
		fmt.Println("Предсказание не удалось\n")
	}

	// Сравнение
	if plsSignal != nil && ewSignal != nil {
		fmt.Println("═══════════════════════════════════════════════════════════")
		fmt.Println("Сравнение")
		fmt.Println("═══════════════════════════════════════════════════════════")
		fmt.Printf("Разница во времени: %d секунд (%.1f часов)\n", 
			plsSignal.Date-ewSignal.Date, 
			float64(plsSignal.Date-ewSignal.Date)/3600.0)
		fmt.Printf("Разница в цене: %.4f\n", plsSignal.Price-ewSignal.Price)
		
		if plsSignal.Date == ewSignal.Date {
			fmt.Println("⚠️  ВНИМАНИЕ: Даты совпадают!")
		} else {
			fmt.Println("✅ Даты различаются")
		}
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
