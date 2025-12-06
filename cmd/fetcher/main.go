// main.go — Сбор свечей Tinkoff API: по месяцам, с автосохранением после каждого запроса
package main

import (
	"bt/internal"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Config содержит настройки для fetcher
type Config struct {
	APIToken          string `json:"api_token"`
	InstrumentID      string `json:"instrument_id"`
	Interval          string `json:"interval"`
	Limit             int    `json:"limit"`
	APIEndpoint       string `json:"api_endpoint"`
	OutputFile        string `json:"output_file"`
	MonthStepDays     int    `json:"month_step_days"`
	RequestTimeoutSec int    `json:"request_timeout_seconds"`
	RequestDelayMs    int    `json:"request_delay_ms"`
	MaxCandlesLimit   int    `json:"max_candles_limit"`
}

var (
	configPath = flag.String("config", "cmd/fetcher/config.json", "Путь к файлу конфигурации")
	config     Config
	client     *http.Client
)

func main() {
	flag.Parse()

	// Загружаем конфигурацию
	if err := loadConfig(*configPath); err != nil {
		log.Fatalf("❌ Ошибка загрузки конфигурации из %s: %v", *configPath, err)
	}

	// Инициализируем HTTP клиент с настройками из конфига
	client = &http.Client{
		Timeout: time.Duration(config.RequestTimeoutSec) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Пропускаем проверку SSL сертификатов
			},
		},
	}

	log.Println("🚀 Запуск сборщика свечей Tinkoff Invest (месячные блоки + автосохранение)")
	log.Printf("📋 Конфигурация: инструмент=%s, интервал=%s, выходной файл=%s",
		config.InstrumentID, config.Interval, config.OutputFile)

	// Начинаем с текущего времени
	toTime := time.Now().UTC()
	var allCandles []internal.Candle
	daysSkipped := 0

	// Всегда начинаем с нуля, перезаписывая файл
	log.Printf("🔄 Начинаем сбор данных с нуля, файл %s будет перезаписан", config.OutputFile)

	monthStep := time.Duration(config.MonthStepDays) * 24 * time.Hour

	for {
		fromTime := toTime.Add(-monthStep)

		reqBody := internal.RequestBody{
			From:             fromTime.Format(time.RFC3339),
			To:               toTime.Format(time.RFC3339),
			Interval:         config.Interval,
			InstrumentId:     config.InstrumentID,
			CandleSourceType: "CANDLE_SOURCE_UNSPECIFIED",
			Limit:            config.Limit,
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			log.Fatal("❌ Ошибка сериализации запроса:", err)
		}

		req, err := http.NewRequestWithContext(context.Background(), "POST", config.APIEndpoint, bytes.NewBuffer(jsonBody))
		if err != nil {
			log.Fatal("❌ Ошибка создания запроса:", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+config.APIToken)

		log.Printf("📥 Запрос: from=%s, to=%s, limit=%d", reqBody.From, reqBody.To, reqBody.Limit)

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("❌ HTTP ошибка при запросе: %v", err)
			log.Println("💾 Сохраняю накопленные данные перед выходом...")
			err = saveCandlesToFile(allCandles)
			if err != nil {
				log.Fatal("❌ Невозможно сохранить свечи в файл")
			}
			log.Fatal("🛑 Прервано из-за сетевой ошибки")
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("❌ Ошибка чтения тела ответа: %v", err)
			log.Println("💾 Сохраняю накопленные данные перед выходом...")
			err = saveCandlesToFile(allCandles)
			if err != nil {
				log.Fatal("❌ Невозможно сохранить свечи в файл")
			}
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
			err = saveCandlesToFile(allCandles)
			if err != nil {
				log.Fatal("❌ Невозможно сохранить свечи в файл")
			}
			log.Fatal("🛑 Прервано из-за ошибки парсинга ответа")
		}

		candles := response.Candles

		// Фильтруем свечи с пустым временем
		candles = filterValidCandles(candles)

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
		err = saveCandlesToFile(allCandles)
		if err != nil {
			log.Fatal("❌ Невозможно сохранить свечи в файл")
		}

		// Сдвигаем верхнюю границу на самую старую свечу
		oldestCandleTime, err := time.Parse(time.RFC3339, candles[0].Time)
		if err != nil {
			log.Printf("❌ Невозможно распарсить время самой старой свечи: '%s', ошибка: %v", candles[0].Time, err)
			log.Printf("⚠️ Сдвигаемся назад на monthStep для продолжения")
			// Сдвигаемся назад на monthStep, чтобы гарантировать прогресс и избежать бесконечного цикла
			toTime = fromTime.Add(-monthStep)
		} else {
			toTime = oldestCandleTime
		}

		log.Printf("✅ Получено %d свечей (всего: %d). Следующий запрос до %s",
			len(candles), processedCount, toTime.Format("2006-01-02"))

		// Защита от бесконечности
		if processedCount > config.MaxCandlesLimit {
			log.Printf("⚠️ Достигнут лимит в %d свечей — остановка для защиты", config.MaxCandlesLimit)
			break
		}

		time.Sleep(time.Duration(config.RequestDelayMs) * time.Millisecond)
	}

	log.Printf("🎉 Успешно собрано и сохранено %d свечей в файл %s", len(allCandles), config.OutputFile)
}

// loadConfig загружает конфигурацию из JSON файла
func loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("не удалось прочитать файл конфигурации: %w", err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("не удалось распарсить конфигурацию: %w", err)
	}

	// Валидация обязательных полей
	if config.APIToken == "" || config.APIToken == "YOUR_TINKOFF_API_TOKEN_HERE" {
		return fmt.Errorf("необходимо указать api_token в файле конфигурации")
	}
	if config.InstrumentID == "" {
		return fmt.Errorf("необходимо указать instrument_id в файле конфигурации")
	}
	if config.OutputFile == "" {
		config.OutputFile = "candles.json"
	}
	if config.MonthStepDays == 0 {
		config.MonthStepDays = 30
	}
	if config.RequestTimeoutSec == 0 {
		config.RequestTimeoutSec = 15
	}
	if config.RequestDelayMs == 0 {
		config.RequestDelayMs = 100
	}
	if config.MaxCandlesLimit == 0 {
		config.MaxCandlesLimit = 500000
	}

	return nil
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

	if err := os.WriteFile(config.OutputFile, outputJSON, 0644); err != nil {
		return fmt.Errorf("ошибка записи в файл: %w", err)
	}

	log.Printf("💾 Сохранено %d свечей в %s", len(candles), config.OutputFile)
	return nil
}

// filterValidCandles фильтрует свечи, оставляя только те, у которых есть время
func filterValidCandles(candles []internal.Candle) []internal.Candle {
	validCandles := make([]internal.Candle, 0, len(candles))
	invalidCount := 0

	for _, candle := range candles {
		if candle.Time != "" {
			validCandles = append(validCandles, candle)
		} else {
			invalidCount++
		}
	}

	if invalidCount > 0 {
		log.Printf("⚠️ Отфильтровано %d свечей с пустым полем time", invalidCount)
	}

	return validCandles
}
