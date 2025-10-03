# Backtest - Система бэктестирования торговых стратегий

Проект для тестирования и оптимизации торговых стратегий на исторических данных с использованием языка программирования Go.

## 🚀 Возможности

- **Множество стратегий**: Более 15 встроенных торговых стратегий (RSI, CCI, MACD, скользящие средние и др.)
- **Параллельное выполнение**: Одновременный запуск всех стратегий для сравнения
- **Автоматическая оптимизация**: Поиск оптимальных параметров для каждой стратегии
- **Сбор исторических данных**: Автоматический сбор данных из Tinkoff Invest API
- **Детальная аналитика**: Подробные отчеты о производительности стратегий

## 📋 Требования

- Go 1.24+
- Доступ к Tinkoff Invest API (для сбора данных)

## 🏗️ Структура проекта

```
backtest/
├── cmd/
│   ├── backtester/          # Основной бэктестер
│   │   └── main.go         # Запуск тестирования стратегий
│   └── fetcher/            # Сборщик данных
│       └── main.go         # Получение данных из Tinkoff API
├── internal/               # Внутренние модули
│   ├── backtest.go         # Логика бэктестирования
│   ├── candle.go           # Работа со свечами
│   ├── strategy.go         # Интерфейс стратегий
│   └── ...
├── strategies/             # Реализация торговых стратегий
│   ├── cci_oscillator.go   # CCI осциллятор
│   ├── rsi_oscillator.go   # RSI осциллятор
│   ├── macd.go             # MACD стратегия
│   └── ...
└── *.json                  # Файлы с историческими данными
```

## 🚀 Быстрый старт

### 1. Сборка проекта

```bash
# Сборка всех компонентов
go build ./...

# Или сборка конкретных исполняемых файлов
go build -o backtester ./cmd/backtester/
go build -o fetcher ./cmd/fetcher/
```

### 2. Сбор исторических данных

Перед запуском бэктестирования необходимо собрать исторические данные:

```bash
# Сбор данных из Tinkoff API (автоматически сохраняет в tmos_big.json)
./fetcher

# Или сбор с указанием конкретного файла
./fetcher -output=my_data.json
```

**Примечание**: Для работы сборщика данных необходим токен Tinkoff Invest API.

### 3. Запуск бэктестирования

#### Тестирование всех стратегий

```bash
# Запуск всех стратегий на данных из candles.json
./backtester -file candles.json -strategy all

# Запуск на пользовательском файле с данными
./backtester -file tmos_big.json -strategy all

# Запуск с автоматическим сравнением с Buy & Hold стратегией
./backtester -file data.json -strategy all
```

#### Тестирование конкретной стратегии

```bash
# Тестирование только CCI стратегии
./backtester -file candles.json -strategy cci_oscillator

# Тестирование RSI стратегии
./backtester -file candles.json -strategy rsi_oscillator

# Тестирование MACD стратегии
./backtester -file candles.json -strategy macd
```

#### Доступные стратегии

- `cci_oscillator` - Commodity Channel Index
- `rsi_oscillator` - Relative Strength Index
- `macd` - Moving Average Convergence Divergence
- `ma_crossover` - Пересечение скользящих средних
- `stochastic_oscillator` - Стохастический осциллятор
- `momentum_breakout` - Пробой импульса
- `elliott_wave_strategy` - Стратегия волн Эллиотта
- `arima_strategy` - ARIMA модель
- `buy_and_hold` - Покупка и удержание (бенчмарк)
- И многие другие...

## 📊 Примеры вывода

### Сравнение всех стратегий

```
====================================================================================================
📊 СРАВНЕНИЕ СТРАТЕГИЙ
====================================================================================================
Стратегия          Прибыль     Сделки    Финал, $       Время      Ранг
----------------------------------------------------------------------------------------------------
cci_oscillator     +15.23%     45        $1152.30       1.2s       🥇 1
rsi_oscillator     +12.87%     38        $1128.70       980ms      🥈 2
macd               +8.45%      52        $1084.50       1.5s       🥉 3
ma_crossover       +5.12%      29        $1051.20       890ms      4
buy_and_hold       +3.21%      1         $1032.10       45ms       5
```

### Оптимизация стратегии

При запуске конкретной стратегии автоматически выполняется оптимизация параметров:

```
Лучшие параметры CCI: период=18, покупка=-120.0, продажа=140.0, профит=0.1523
Лучшие параметры RSI: период=14, покупка=25.0, продажа=75.0, профит=0.1287
```

## ⚙️ Параметры командной строки

### backtester

```bash
Usage: ./backtester [options]

Options:
  -file string
        Путь к JSON-файлу со свечами (default "candles.json")
  -strategy string
        Стратегия: all (все стратегии) или название конкретной стратегии (default "all")
```

### fetcher

```bash
Usage: ./fetcher [options]

Options:
  -output string
        Имя выходного файла для сохранения данных (default "tmos_big.json")
```

## 🔧 Конфигурация

### Tinkoff API

Для сбора данных настройте следующие константы в `cmd/fetcher/main.go`:

```go
const (
    API_TOKEN     = "your_tinkoff_api_token"
    INSTRUMENT_ID = "TCS60A101X76"  // FIGI инструмента
    INTERVAL      = "CANDLE_INTERVAL_30_MIN"  // Интервал свечей
    OUTPUT_FILE   = "tmos_big.json"  // Выходной файл
)
```

### Доступные интервалы свечей

- `CANDLE_INTERVAL_1_MIN` - 1 минута
- `CANDLE_INTERVAL_5_MIN` - 5 минут
- `CANDLE_INTERVAL_15_MIN` - 15 минут
- `CANDLE_INTERVAL_30_MIN` - 30 минут
- `CANDLE_INTERVAL_HOUR` - 1 час
- `CANDLE_INTERVAL_DAY` - 1 день

## 📈 Добавление новой стратегии

1. Создайте файл в папке `strategies/`
2. Реализуйте интерфейс `Strategy`:
   ```go
   type Strategy interface {
       Name() string
       GenerateSignals(candles []Candle, params StrategyParams) []SignalType
       Optimize(candles []Candle) StrategyParams
   }
   ```
3. Зарегистрируйте стратегию в `init()`:
   ```go
   func init() {
       internal.RegisterStrategy("my_strategy", &MyStrategy{})
   }
   ```

## 🛠️ Разработка

### Запуск тестов

```bash
go test ./...
```

### Форматирование кода

```bash
go fmt ./...
```

### Проверка зависимостей

```bash
go mod tidy
go mod verify
```

## 📁 Формат данных

Файлы с историческими данными должны содержать JSON в формате:

```json
{
  "candles": [
    {
      "time": "2023-01-01T00:00:00Z",
      "open": {"units": "100", "nano": 0},
      "high": {"units": "105", "nano": 0},
      "low": {"units": "95", "nano": 0},
      "close": {"units": "103", "nano": 0},
      "volume": "1000"
    }
  ]
}
```

## 🤝 Поддержка

При возникновении проблем или предложений создайте Issue в репозитории проекта.

## 📄 Лицензия

Этот проект распространяется под лицензией MIT.
