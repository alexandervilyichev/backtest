# Руководство по добавлению стратегий V2

## Обзор

Новая архитектура V2 использует композицию и интерфейсы для создания более гибких и тестируемых стратегий.

## Как добавить новую стратегию V2

### 1. Создайте файл стратегии

Создайте файл в соответствующей папке (например, `strategies/trend/my_strategy_v2.go`).

### 2. Реализуйте компоненты

```go
package trend

import (
	"bt/internal"
	"errors"
	"fmt"
)

// Конфигурация
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

// Генератор сигналов
type MyStrategySignalGenerator struct{}

func NewMyStrategySignalGenerator() *MyStrategySignalGenerator {
	return &MyStrategySignalGenerator{}
}

func (sg *MyStrategySignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	cfg, ok := config.(*MyStrategyConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	// Ваша логика генерации сигналов
	signals := make([]internal.SignalType, len(candles))
	// ...
	return signals
}

// Генератор конфигураций для оптимизации
type MyStrategyConfigGenerator struct {
	minPeriod, maxPeriod, step int
}

func NewMyStrategyConfigGenerator(min, max, step int) *MyStrategyConfigGenerator {
	return &MyStrategyConfigGenerator{
		minPeriod: min,
		maxPeriod: max,
		step:      step,
	}
}

func (cg *MyStrategyConfigGenerator) Generate() []internal.StrategyConfigV2 {
	configs := []internal.StrategyConfigV2{}
	for period := cg.minPeriod; period <= cg.maxPeriod; period += cg.step {
		configs = append(configs, &MyStrategyConfig{Period: period})
	}
	return configs
}

// Фабричная функция
func NewMyStrategyV2(slippage float64) internal.TradingStrategy {
	slippageProvider := internal.NewSlippageProvider(slippage)
	signalGenerator := NewMyStrategySignalGenerator()
	
	configManager := internal.NewConfigManager(
		&MyStrategyConfig{Period: 20}, // default config
		func() internal.StrategyConfigV2 { return &MyStrategyConfig{} },
	)
	
	configGenerator := NewMyStrategyConfigGenerator(5, 100, 5)
	
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

// Регистрация
func init() {
	strategy := NewMyStrategyV2(0.01)
	internal.RegisterStrategyV2(strategy)
}
```

### 3. Импортируйте пакет

Убедитесь, что пакет импортирован в `cmd/backtester/main.go`:

```go
import (
	_ "bt/strategies/trend"  // Это загрузит все стратегии из папки
)
```

### 4. Запустите тесты

```bash
# Тест одной стратегии
./backtester --file candles.json --strategy my_strategy_v2

# Тест всех стратегий (включая V2)
./backtester --file candles.json --strategy all
```

## Преимущества V2 архитектуры

1. **Переиспользование кода**: `GridSearchOptimizer` работает для всех стратегий
2. **Тестируемость**: Каждый компонент можно тестировать отдельно
3. **Гибкость**: Легко заменить оптимизатор или генератор сигналов
4. **Читаемость**: Явные зависимости через конструкторы
5. **Расширяемость**: Добавление новых компонентов не требует изменения существующих

## Совместимость

Обе архитектуры (V1 и V2) работают одновременно. Система автоматически определяет тип стратегии и использует соответствующий runner.

## Пример: golden_cross_v2

См. `strategies/trend/golden_cross_strategy_v2.go` для полного примера миграции стратегии на V2 архитектуру.
