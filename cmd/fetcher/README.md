# Fetcher - Сборщик свечей Tinkoff Invest API

Утилита для загрузки исторических данных свечей через Tinkoff Invest API.

## Настройка

1. Скопируйте файл примера конфигурации:
```bash
cp cmd/fetcher/config.example.json cmd/fetcher/config.json
```

2. Отредактируйте `cmd/fetcher/config.json` и укажите ваши параметры:
   - `api_token` - ваш токен Tinkoff Invest API
   - `instrument_id` - ID инструмента (например, "TCS60A101X76")
   - `interval` - интервал свечей (например, "CANDLE_INTERVAL_30_MIN")
   - `output_file` - имя выходного файла для сохранения данных

## Запуск

```bash
go run cmd/fetcher/main.go
```

Или с указанием пути к конфигурации:

```bash
go run cmd/fetcher/main.go -config=/path/to/config.json
```

## Параметры конфигурации

- `api_token` (обязательно) - токен API Tinkoff Invest
- `instrument_id` (обязательно) - идентификатор инструмента
- `interval` - интервал свечей (по умолчанию: "CANDLE_INTERVAL_30_MIN")
- `limit` - максимальное количество свечей за запрос (по умолчанию: 1000)
- `api_endpoint` - URL API endpoint
- `output_file` - имя выходного файла (по умолчанию: "candles.json")
- `month_step_days` - шаг в днях для запросов (по умолчанию: 30)
- `request_timeout_seconds` - таймаут запроса в секундах (по умолчанию: 15)
- `request_delay_ms` - задержка между запросами в миллисекундах (по умолчанию: 100)
- `max_candles_limit` - максимальное количество свечей для загрузки (по умолчанию: 500000)

## Очистка данных

Если в файле со свечами есть записи с пустым полем `time`, используйте утилиту очистки:

```bash
go run utilities/clean_candles.go -input=tmos_big.json
```

Или с указанием выходного файла:

```bash
go run utilities/clean_candles.go -input=tmos_big.json -output=tmos_big_clean.json
```

## Примечания

- Файл `config.json` добавлен в `.gitignore` для защиты вашего API токена
- Программа автоматически сохраняет прогресс после каждого успешного запроса
- При повторном запуске продолжает загрузку с места остановки
- Программа автоматически фильтрует свечи с пустым полем `time` при загрузке и сохранении
