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
go run ./cmd/backtester/ -file tmos_big.json -strategy all -cpu_profile cpu.prof -mem_profile mem.prof

```

#### Тестирование конкретной стратегии

```bash
# Тестирование только CCI стратегии

go run ./cmd/backtester/ -file tmos_big.json -strategy cci_oscillator -cpu_profile cpu.prof -mem_profile mem.prof

```

#### Расширенные возможности

```bash
# Включить детальное логирование
go run ./cmd/backtester/ -file tmos_big.json -strategy all -debug

# Сохранить только топ-5 стратегий с сигналами
go run ./cmd/backtester/ -file tmos_big.json -strategy all  -save_signals=5

# Отключить сохранение файлов с сигналами
go run ./cmd/backtester/ -file tmos_big.json -strategy all -save_signals=0

# Ограничить количество параллельных воркеров (для экономии памяти)
go run ./cmd/backtester/ -file tmos_big.json -strategy all -workers=4

# Использовать только 1 воркер (последовательное выполнение)
go run ./cmd/backtester/ -file tmos_big.json -strategy all -workers=1

# Комбинированные параметры
go run ./cmd/backtester/ -file tmos_big.json -strategy all -debug -save_signals=1 -workers=6
```

**💡 Совет**: При работе с большими файлами (>50k свечей) используйте флаг `--workers` для ограничения потребления памяти. Подробнее см. [MEMORY_OPTIMIZATION.md](MEMORY_OPTIMIZATION.md)

#### Доступные стратегии

**V2 Стратегии (рекомендуется):**
- `linear_spline_v2` - Линейные сплайны без прогнозирования (NEW!)
- `predictive_linear_spline_v2` - Прогнозирующие линейные сплайны (NEW!)
- `predictive_spline_v2` - Прогнозирующие квадратичные сплайны
- `elliott_wave_v2` - Волны Эллиотта V2

**V1 Стратегии:**
- `cci_oscillator` - Commodity Channel Index
- `rsi_oscillator` - Relative Strength Index
- `macd` - Moving Average Convergence Divergence
- `ma_crossover` - Пересечение скользящих средних
- `stochastic_oscillator` - Стохастический осциллятор
- `momentum_breakout` - Пробой импульса
- `linear_alternating_spline` - Линейные чередующиеся сплайны V1
- `buy_and_hold` - Покупка и удержание (бенчмарк)
- И многие другие...

📊 СРАВНЕНИЕ СТРАТЕГИЙ
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
## 📊 Примеры вывода

### Сравнение всех стратегий

```
📊 СРАВНЕНИЕ СТРАТЕГИЙ
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

### Сохранение данных для графиков

После сравнения стратегий автоматически сохраняются топ-3 стратегии с сигналами:

```
💾 СОХРАНЕНИЕ ДАННЫХ ДЛЯ ГРАФИКОВ
💾 Сохранены данные с сигналами: tmos_big_cci_oscillator_signals.json (прибыль: +15.23%, сигналов: 45)
💾 Сохранены данные с сигналами: tmos_big_rsi_oscillator_signals.json (прибыль: +12.87%, сигналов: 38)
💾 Сохранены данные с сигналами: tmos_big_macd_signals.json (прибыль: +8.45%, сигналов: 52)
```

Формат файлов для графиков:
```json
{
  "strategy": "cci_oscillator",
  "params": {
    "CciPeriod": 18,
    "CciBuyLevel": -120.0,
    "CciSellLevel": 140.0
  },
  "profit": 0.1523,
  "candles": [
    {
      "time": "2023-01-01T00:00:00Z",
      "open": 100.50,
      "high": 105.25,
      "low": 99.75,
      "close": 103.20,
      "volume": 1000,
      "signal": 1
    }
  ]
}
```

### Значения сигналов:
- `0` = HOLD (удерживать позицию)
- `1` = BUY (сигнал на покупку)
- `2` = SELL (сигнал на продажу)
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
  -debug
        Включить детальное логирование
  -save_signals int
        Сохранить топ-N стратегий с сигналами (0 = не сохранять) (default 0)
  -workers int
        Количество параллельных воркеров (0 = auto = NumCPU) (default 0)
  -config string
        Путь к JSON-файлу с конфигурациями стратегий (пусто = оптимизация)
  -cpu_profile string
        Файл для CPU профилирования (пусто = отключено)
  -mem_profile string
        Файл для памяти профилирования (пусто = отключено)
  -prof_port int
        Порт для realtime профилирования (0 = отключено) (default 0)
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

## 🆕 Новая стратегия: Predictive Spline V2

**Предсказательная стратегия на основе квадратичных сплайнов** - преобразует ретроспективный анализ в предсказательную модель для реальной торговли.

### Результаты на больших данных (15323 свечи)

```bash
./backtester -file tmos_big.json -strategy predictive_spline_v2
```

| Метрика | Значение |
|---------|----------|
| **Прибыль** | +33.39% |
| **Сделок** | 5 |
| **Buy & Hold** | -1.26% |
| **Время** | 1.1s |

### Ключевые особенности

- ✅ **Предсказывает развороты заранее** (не постфактум)
- ✅ **Фильтры качества** - отсеивает слабые тренды
- ✅ **Оптимизирована для больших данных** (>5000 свечей)
- ✅ **Консервативный подход** - мало сделок, но качественных
- ✅ **Быстрая работа** - кэширование и оптимизация

### Документация

- [Быстрый старт](docs/PREDICTIVE_SPLINE_QUICKSTART.md)
- [Полное руководство](docs/PREDICTIVE_SPLINE_STRATEGY.md)
- [Сравнение V1 vs V2](docs/SPLINE_STRATEGY_COMPARISON.md)
- [Улучшения для больших данных](docs/PREDICTIVE_SPLINE_IMPROVEMENTS.md)
- [Итоговое резюме](PREDICTIVE_SPLINE_V2_SUMMARY.md)

---

## 📈 Добавление новой стратегии

### Архитектура V2 (рекомендуется)

Новая архитектура использует композицию и интерфейсы для создания более гибких и тестируемых стратегий.

**Преимущества V2:**
- ✅ Переиспользование кода (универсальный `GridSearchOptimizer`)
- ✅ Тестируемость (каждый компонент независим)
- ✅ Гибкость (легко заменить оптимизатор)
- ✅ Читаемость (явные зависимости)

**Пример создания стратегии V2:**

```go
package trend

import (
	"bt/internal"
	"errors"
	"fmt"
)

// 1. Конфигурация
type MyStrategyConfig struct {
	Period int `json:"period"`
}

func (c *MyStrategyConfig) Validate() error {
	if c.Period <= 0 {
		return errors.New("period must be positive")
	}
	return nil
}

func (c *MyStrategyConfig) String() string {
	return fmt.Sprintf("MyStrategy(period=%d)", c.Period)
}

// 2. Генератор сигналов
type MyStrategySignalGenerator struct{}

func (sg *MyStrategySignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	// Ваша логика генерации сигналов
	signals := make([]internal.SignalType, len(candles))
	// ...
	return signals
}

// 3. Генератор конфигураций для оптимизации
type MyStrategyConfigGenerator struct {
	minPeriod, maxPeriod, step int
}

func (cg *MyStrategyConfigGenerator) Generate() []internal.StrategyConfigV2 {
	configs := []internal.StrategyConfigV2{}
	for period := cg.minPeriod; period <= cg.maxPeriod; period += cg.step {
		configs = append(configs, &MyStrategyConfig{Period: period})
	}
	return configs
}

// 4. Фабричная функция (композиция компонентов)
func NewMyStrategyV2(slippage float64) internal.TradingStrategy {
	slippageProvider := internal.NewSlippageProvider(slippage)
	signalGenerator := &MyStrategySignalGenerator{}
	
	configManager := internal.NewConfigManager(
		&MyStrategyConfig{Period: 20}, // default config
		func() internal.StrategyConfigV2 { return &MyStrategyConfig{} },
	)
	
	configGenerator := &MyStrategyConfigGenerator{
		minPeriod: 5, maxPeriod: 100, step: 5,
	}
	
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)
	
	return internal.NewStrategyBase(
		"my_strategy_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

// 5. Регистрация
func init() {
	strategy := NewMyStrategyV2(0.01)
	internal.RegisterStrategyV2(strategy)
}
```

**Подробнее**: См. `docs/V2_STRATEGY_GUIDE.md` и пример `strategies/trend/golden_cross_strategy_v2.go`

### Архитектура V1 (legacy)

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

**Примечание**: Обе архитектуры работают одновременно. Система автоматически определяет тип стратегии.

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

## 📊 Визуализация результатов

Проект включает интерактивный HTML/JavaScript визуализатор для графического представления торговых сигналов.

### Запуск визуализатора

**Важно**: Из-за политики CORS браузеры блокируют прямые запросы к локальным файлам. Используйте один из следующих способов:

#### Способ 1: Загрузка файла через input
1. Откройте `visualizer.html` в браузере
2. Выберите файл с сигналами через кнопку "Обзор..." (input[type="file"])
3. График автоматически отобразит данные

#### Способ 2: Локальный сервер
```bash
# Используйте любой локальный сервер, например:
python -m http.server 8000
# или
npx serve .
# или
php -S localhost:8000
```
Затем откройте: `http://localhost:8000/visualizer.html`

#### Способ 3: Прямая загрузка по URL
```
http://localhost:8000/visualizer.html?file=tmos_cci_oscillator_signals.json
```

### Возможности визуализатора

- **Интерактивный график свечей** с маркерами сигналов покупки/продажи
- **Масштабирование и навигация**:
  - Колесико мыши для масштабирования относительно курсора
  - Ползунок масштаба в интерфейсе (10% - 500%)
  - Кнопки "По размеру экрана" и "Сбросить"
- **Детальная информация**:
  - Подсказки при наведении курсора с OHLCV данными
  - Перекрестие для точного позиционирования
  - Отображение сигналов и параметров стратегии
- **Цветовая кодировка**:
  - Зеленые маркеры - сигналы BUY
  - Красные маркеры - сигналы SELL
  - Зеленые свечи - рост цены
  - Красные свечи - падение цены

### Управление

- **Колесико мыши** - масштабирование относительно курсора
- **Движение курсора** - перекрестие и подсказки
- **Кнопки управления** - предустановленные действия
- **Ползунок масштаба** - точная настройка увеличения

### Пример использования

1. **Запустите бэктестер** с сохранением сигналов:
   ```bash
   ./backtester -file tmos_big.json -strategy cci_oscillator -save_signals=1
   ```

2. **Запустите локальный сервер** (способ 2):
   ```bash
   python -m http.server 8000
   ```

3. **Откройте визуализатор**:
   ```
   http://localhost:8000/visualizer.html
   ```

4. **Загрузите файл** через интерфейс или используйте прямую ссылку:
   ```
   http://localhost:8000/visualizer.html?file=tmos_cci_oscillator_signals.json
   ```

5. **Навигация по графику**:
   - Масштабируйте колесиком мыши
   - Наведите курсор для детальной информации
   - Используйте кнопки для управления видом

## 📚 Документация стратегий

Подробная документация по стратегиям доступна в папке `docs/`:

- [Linear Spline Strategy V2](docs/LINEAR_SPLINE_STRATEGY.md) - Линейные сплайны без прогнозирования
- [Linear Spline Quick Start](docs/LINEAR_SPLINE_QUICKSTART.md) - Быстрый старт с Linear Spline
- [Predictive Linear Spline Strategy V2](docs/PREDICTIVE_LINEAR_SPLINE_STRATEGY.md) - Прогнозирующие линейные сплайны (NEW!)
- [Predictive Spline Strategy](docs/PREDICTIVE_SPLINE_STRATEGY.md) - Прогнозирующие квадратичные сплайны
- [Predictive Spline Quick Start](docs/PREDICTIVE_SPLINE_QUICKSTART.md) - Быстрый старт с Predictive Spline
- [Spline Strategy Comparison](docs/SPLINE_STRATEGY_COMPARISON.md) - Сравнение стратегий на основе сплайнов

## 📄 Лицензия

Этот проект распространяется под лицензией MIT.
