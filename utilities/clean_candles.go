// clean_candles.go - утилита для очистки файла свечей от записей с пустым временем
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
)

// SimpleCandle - упрощенная структура свечи для работы с любым форматом
type SimpleCandle struct {
	Open         interface{} `json:"open"`
	High         interface{} `json:"high"`
	Low          interface{} `json:"low"`
	Close        interface{} `json:"close"`
	Volume       string      `json:"volume"`
	Time         string      `json:"time"`
	IsComplete   bool        `json:"isComplete"`
	CandleSource string      `json:"candleSource"`
}

func main() {
	inputFile := flag.String("input", "tmos_big.json", "Входной файл со свечами")
	outputFile := flag.String("output", "", "Выходной файл (по умолчанию перезаписывает входной)")
	flag.Parse()

	if *outputFile == "" {
		*outputFile = *inputFile
	}

	log.Printf("🔧 Очистка файла %s от свечей с пустым временем", *inputFile)

	// Читаем файл
	data, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("❌ Ошибка чтения файла: %v", err)
	}

	var wrapper struct {
		Candles []SimpleCandle `json:"candles"`
	}

	if err := json.Unmarshal(data, &wrapper); err != nil {
		log.Fatalf("❌ Ошибка парсинга JSON: %v", err)
	}

	originalCount := len(wrapper.Candles)
	log.Printf("📊 Исходное количество свечей: %d", originalCount)

	// Фильтруем свечи
	validCandles := make([]SimpleCandle, 0, len(wrapper.Candles))
	invalidCount := 0

	for _, candle := range wrapper.Candles {
		if candle.Time != "" {
			validCandles = append(validCandles, candle)
		} else {
			invalidCount++
		}
	}

	log.Printf("✅ Валидных свечей: %d", len(validCandles))
	log.Printf("❌ Свечей с пустым временем: %d", invalidCount)

	if invalidCount == 0 {
		log.Println("✨ Файл уже чистый, изменения не требуются")
		return
	}

	// Сохраняем очищенные данные
	outputData := struct {
		Candles []SimpleCandle `json:"candles"`
	}{
		Candles: validCandles,
	}

	outputJSON, err := json.MarshalIndent(outputData, "", "  ")
	if err != nil {
		log.Fatalf("❌ Ошибка сериализации: %v", err)
	}

	if err := os.WriteFile(*outputFile, outputJSON, 0644); err != nil {
		log.Fatalf("❌ Ошибка записи в файл: %v", err)
	}

	log.Printf("💾 Очищенные данные сохранены в %s", *outputFile)
	log.Printf("🎉 Удалено %d невалидных свечей", invalidCount)
}
