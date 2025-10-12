package backtester

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"bt/internal"
)

// FileSaver — реализация сохранения результатов в файлы
type FileSaver struct{}

// NewFileSaver — конструктор для FileSaver
func NewFileSaver() *FileSaver {
	return &FileSaver{}
}

// SaveTopStrategies — сохраняет топ-N стратегии с сигналами в отдельные файлы
func (s *FileSaver) SaveTopStrategies(candles []internal.Candle, results []BenchmarkResult, inputFilename string, topN int) error {
	if len(results) < topN || topN <= 0 {
		if topN > 0 {
			return fmt.Errorf("недостаточно стратегий для сохранения топ-%d (доступно: %d)", topN, len(results))
		}
		return nil
	}

	// Получаем базовое имя файла без расширения
	baseName := strings.TrimSuffix(filepath.Base(inputFilename), filepath.Ext(inputFilename))

	for i := 0; i < topN && i < len(results); i++ {
		strategyName := results[i].Name

		// Получаем стратегию и генерируем сигналы
		strategy := internal.GetStrategy(strategyName)
		solidStrategy, ok := strategy.(internal.SolidStrategy)
		if !ok {
			log.Printf("⚠️  Стратегия %s не поддерживает SOLID архитектуру, пропускаем", strategyName)
			continue
		}

		config := solidStrategy.OptimizeWithConfig(candles)
		signals := solidStrategy.GenerateSignalsWithConfig(candles, config)

		// Создаем массив свечей с сигналами
		candlesWithSignals := make([]CandleWithSignal, len(candles))
		for j, candle := range candles {
			candlesWithSignals[j] = CandleWithSignal{
				Time:   candle.Time,
				Open:   candle.Open.ToFloat64(),
				High:   candle.High.ToFloat64(),
				Low:    candle.Low.ToFloat64(),
				Close:  candle.Close.ToFloat64(),
				Volume: candle.VolumeFloat64(),
				Signal: getSignalAtIndex(signals, j),
			}
		}

		// Создаем имя файла с постфиксом стратегии
		outputFilename := fmt.Sprintf("%s_%s_signals.json", baseName, strategyName)

		// Сохраняем в файл
		data := struct {
			Strategy string                  `json:"strategy"`
			Config   internal.StrategyConfig `json:"config"`
			Profit   float64                 `json:"profit"`
			Candles  []CandleWithSignal      `json:"candles"`
		}{
			Strategy: strategyName,
			Config:   config,
			Profit:   results[i].TotalProfit,
			Candles:  candlesWithSignals,
		}

		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			log.Printf("❌ Ошибка сериализации данных для %s: %v", strategyName, err)
			continue
		}

		err = os.WriteFile(outputFilename, jsonData, 0644)
		if err != nil {
			log.Printf("❌ Ошибка сохранения файла %s: %v", outputFilename, err)
			continue
		}

		fmt.Printf("💾 Сохранены данные с сигналами: %s (прибыль: %.2f%%, сигналов: %d)\n",
			outputFilename, results[i].TotalProfit*100, countSignals(signals))
	}

	return nil
}

// getSignalAtIndex — возвращает сигнал по индексу с проверкой границ
func getSignalAtIndex(signals []internal.SignalType, index int) internal.SignalType {
	if index < 0 || index >= len(signals) {
		return internal.HOLD
	}
	return signals[index]
}

// countSignals — подсчитывает количество ненулевых сигналов
func countSignals(signals []internal.SignalType) int {
	count := 0
	for _, signal := range signals {
		if signal != internal.HOLD {
			count++
		}
	}
	return count
}
