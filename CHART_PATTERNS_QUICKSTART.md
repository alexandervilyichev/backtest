# Chart Patterns V2 - Быстрый старт

## Что это?

Стратегия распознавания классических ценовых паттернов с предсказанием будущих сигналов.

## Поддерживаемые паттерны

- 🔄 **Разворотные**: Голова и плечи, Двойное/тройное дно/вершина
- ➡️ **Продолжения**: Флаги, Вымпелы
- 🔺 **Треугольники**: Восходящий, Нисходящий, Симметричный
- 📐 **Клинья**: Восходящий, Нисходящий

## Быстрый запуск

### 1. Тестирование стратегии

```bash
# Только chart_patterns_v2
./backtester -file tmos_big.json -strategy chart_patterns_v2

# Все стратегии (включая chart_patterns_v2)
./backtester -file tmos_big.json -strategy all
```

### 2. Использование в коде

```go
package main

import (
    "bt/internal"
    "bt/strategies/v2/patterns"
)

func main() {
    // Создание стратегии
    strategy := patterns.NewChartPatternsStrategyV2(0.01)
    
    // Генерация сигналов
    config := strategy.DefaultConfig()
    signals := strategy.GenerateSignals(candles, config)
    
    // Предсказание следующего сигнала
    if sb, ok := strategy.(*internal.StrategyBase); ok {
        future := sb.PredictNextSignal(candles, config)
        if future != nil {
            fmt.Printf("Следующий сигнал: %v\n", future.SignalType)
            fmt.Printf("Уверенность: %.1f%%\n", future.Confidence*100)
        }
    }
    
    // Бэктестинг
    result := internal.Backtest(candles, signals, 0.01)
    fmt.Printf("Прибыль: %.2f%%\n", result.TotalProfit*100)
}
```

## Результаты на тестовых данных

```
Данные: TMOS, 23,000 свечей
Прибыль: +4.42%
Сделки: 40
Время: 35s
Ранг: 28/42

Предсказание:
  Сигнал: BUY
  Уверенность: 84.9%
```

## Параметры по умолчанию

```go
MinPatternLength:      8     // Минимум свечей в паттерне
MaxPatternLength:      30    // Максимум свечей в паттерне
ConfirmationThreshold: 0.5   // Порог уверенности (0-1)
LookbackPeriod:        100   // Период анализа
PriceTolerance:        0.03  // Допуск цены (3%)
```

## Оптимизация параметров

Стратегия автоматически оптимизирует параметры при запуске:

```bash
./backtester -file tmos_big.json -strategy chart_patterns_v2
# Выведет: Best config found: ChartPatterns(min_len=5, max_len=20, threshold=0.70, lookback=50, tolerance=2.00%)
```

## Пример вывода

```
════════════════════════════════════════════════════════════
📊 ИТОГОВЫЙ ОТЧЕТ ПО СТРАТЕГИЯМ
════════════════════════════════════════════════════════════
│ Ранг │ Стратегия         │ Прибыль   │ След.сигнал │ Уверен. │
├──────┼───────────────────┼───────────┼─────────────┼─────────┤
│  28  │ chart_patterns_v2 │ +4.42%    │ 🟢 BUY      │ 84.9%   │
└──────┴───────────────────┴───────────┴─────────────┴─────────┘
```

## Полная документация

См. [CHART_PATTERNS_V2_SUMMARY.md](CHART_PATTERNS_V2_SUMMARY.md)

## Пример кода

См. [examples/chart_patterns_example.go](examples/chart_patterns_example.go)

---

**Категория**: Ценовые паттерны  
**Версия**: V2  
**Файл**: `strategies/v2/patterns/chart_patterns_strategy.go`
