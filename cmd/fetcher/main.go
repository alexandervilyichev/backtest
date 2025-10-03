// main.go — Сбор свечей Tinkoff API: по месяцам, с автосохранением после каждого запроса
package main

import (
	"bt/internal"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	API_TOKEN     = "NEW_TOKEN"
	INSTRUMENT_ID = "TCS60A101X76"
	INTERVAL      = "CANDLE_INTERVAL_30_MIN"
	LIMIT         = 1000
	API_ENDPOINT  = "https://invest-public-api.tbank.ru/rest/tinkoff.public.invest.api.contract.v1.MarketDataService/GetCandles"
	OUTPUT_FILE   = "tmos_big.json"
	MONTH_STEP    = 30 * 24 * time.Hour // ~1 месяц (без учёта точного количества дней — достаточно)
)

var client = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Пропускаем проверку SSL сертификатов
		},
	},
}

func main() {
	log.Println("🚀 Запуск сборщика свечей Tinkoff Invest (месячные блоки + автосохранение)")

	// Начинаем с текущего времени
	toTime := time.Now().UTC()
	var allCandles []internal.Candle
	daysSkipped := 0

	// Пытаемся загрузить уже сохранённые данные
	if err := loadExistingCandles(&allCandles); err != nil {
		log.Printf("⚠️ Не удалось загрузить существующие данные из %s: %v", OUTPUT_FILE, err)
	} else {
		log.Printf("🔄 Загружено %d свечей из предыдущего сеанса", len(allCandles))
	}

	for {
		fromTime := toTime.Add(-MONTH_STEP)

		reqBody := internal.RequestBody{
			From:             fromTime.Format(time.RFC3339),
			To:               toTime.Format(time.RFC3339),
			Interval:         INTERVAL,
			InstrumentId:     INSTRUMENT_ID,
			CandleSourceType: "CANDLE_SOURCE_UNSPECIFIED",
			Limit:            LIMIT,
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			log.Fatal("❌ Ошибка сериализации запроса:", err)
		}

		req, err := http.NewRequestWithContext(context.Background(), "POST", API_ENDPOINT, bytes.NewBuffer(jsonBody))
		if err != nil {
			log.Fatal("❌ Ошибка создания запроса:", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+API_TOKEN)

		log.Printf("📥 Запрос: from=%s, to=%s, limit=%d", reqBody.From, reqBody.To, reqBody.Limit)

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("❌ HTTP ошибка при запросе: %v", err)
			log.Println("💾 Сохраняю накопленные данные перед выходом...")
			saveCandlesToFile(allCandles)
			log.Fatal("🛑 Прервано из-за сетевой ошибки")
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("❌ Ошибка чтения тела ответа: %v", err)
			log.Println("💾 Сохраняю накопленные данные перед выходом...")
			saveCandlesToFile(allCandles)
			log.Fatal("🛑 Прервано из-за ошибки чтения ответа")
		}

		if resp.StatusCode != 200 {
			log.Printf("⚠️ HTTP %d: %s", resp.StatusCode, string(body))
			if strings.Contains(string(body), "not found") || strings.Contains(string(body), "no data") {
				log.Println("✅ Данные закончились — завершаем сбор")
				break
			}
			log.Printf("⚠️ Неожиданный статус: %d — пробуем продолжить...", resp.StatusCode)
			// Не останавливаемся — возможно, временный сбой
			toTime = fromTime
			time.Sleep(2 * time.Second)
			continue
		}

		var response internal.GetCandlesResponse
		if err := json.Unmarshal(body, &response); err != nil {
			log.Printf("❌ Ошибка парсинга JSON: %v", err)
			log.Println("💾 Сохраняю накопленные данные перед выходом...")
			saveCandlesToFile(allCandles)
			log.Fatal("🛑 Прервано из-за ошибки парсинга ответа")
		}

		candles := response.Candles

		if len(candles) == 0 {
			daysSkipped++
			log.Printf("ℹ️ Месяц %s–%s: 0 свечей (выходные/праздники?) — пропущено (%d всего)",
				fromTime.Format("2006-01"), toTime.Format("2006-01"), daysSkipped)
			toTime = fromTime
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Добавляем новые свечи в начало списка (хронологический порядок: старые → новые)
		allCandles = append(candles, allCandles...)
		processedCount := len(allCandles)

		// 🚨 КЛЮЧЕВОЙ ШАГ: сохраняем ВСЁ в файл сразу после успешного запроса
		saveCandlesToFile(allCandles)

		// Сдвигаем верхнюю границу на самую старую свечу
		oldestCandleTime, err := time.Parse(time.RFC3339, candles[0].Time)
		if err != nil {
			log.Fatal("❌ Невозможно распарсить время самой старой свечи:", candles[0].Time)
		}
		toTime = oldestCandleTime

		log.Printf("✅ Получено %d свечей (всего: %d). Следующий запрос до %s",
			len(candles), processedCount, toTime.Format("2006-01-02"))

		// Защита от бесконечности
		if processedCount > 500000 {
			log.Println("⚠️ Достигнут лимит в 500k свечей — остановка для защиты")
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("🎉 Успешно собрано и сохранено %d свечей в файл %s", len(allCandles), OUTPUT_FILE)
}

// saveCandlesToFile сохраняет свечи в JSON-файл
func saveCandlesToFile(candles []internal.Candle) error {
	outputData := struct {
		Candles []internal.Candle `json:"candles"`
	}{
		Candles: candles,
	}

	outputJSON, err := json.MarshalIndent(outputData, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка сериализации: %w", err)
	}

	if err := os.WriteFile(OUTPUT_FILE, outputJSON, 0644); err != nil {
		return fmt.Errorf("ошибка записи в файл: %w", err)
	}

	log.Printf("💾 Сохранено %d свечей в %s", len(candles), OUTPUT_FILE)
	return nil
}

// loadExistingCandles загружает уже сохранённые свечи из файла
func loadExistingCandles(candles *[]internal.Candle) error {
	data, err := os.ReadFile(OUTPUT_FILE)
	if err != nil {
		return err
	}

	var wrapper struct {
		Candles []internal.Candle `json:"candles"`
	}

	if err := json.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("ошибка парсинга существующих данных: %w", err)
	}

	*candles = wrapper.Candles
	return nil
}
