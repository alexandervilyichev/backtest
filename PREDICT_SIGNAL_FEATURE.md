# Функция предсказания будущих сигналов

## Описание

Добавлена возможность предсказывать ближайший торговый сигнал в будущем для стратегий, поддерживающих эту функциональность.

## Изменения в коде

### 1. Новый интерфейс `PredictiveSignalGenerator`

В файле `internal/strategy_v2.go` добавлен новый интерфейс:

```go
// FutureSignal - будущий сигнал с датой
type FutureSignal struct {
    SignalType SignalType  // Тип сигнала (BUY/SELL/HOLD)
    Date       int64       // Unix timestamp
    Price      float64     // Предсказанная цена
    Confidence float64     // Уверенность (0.0 - 1.0)
}

// PredictiveSignalGenerator - генератор с возможностью предсказания
type PredictiveSignalGenerator interface {
    SignalGenerator
    PredictNextSignal(candles []Candle, config StrategyConfigV2) *FutureSignal
}
```

### 2. Реализация для стратегий

#### Predictive Linear Spline Strategy

В файле `strategies/v2/trend/predictive_linear_spline_strategy.go` добавлен метод:

```go
func (sg *PredictiveLinearSplineSignalGenerator) PredictNextSignal(
    candles []internal.Candle, 
    config internal.StrategyConfigV2,
) *internal.FutureSignal
```

Метод:
- Анализирует текущий тренд
- Предсказывает точку разворота
- Вычисляет дату и цену будущего сигнала
- Оценивает уверенность в предсказании

#### Golden Cross Strategy

В файле `strategies/v2/trend/golden_cross_strategy.go` добавлен метод:

```go
func (sg *GoldenCrossSignalGenerator) PredictNextSignal(
    candles []internal.Candle, 
    config internal.StrategyConfigV2,
) *internal.FutureSignal
```

Метод:
- Рассчитывает быструю и медленную EMA
- Анализирует скорость изменения каждой EMA
- Вычисляет относительную скорость сближения/расхождения
- Предсказывает точку пересечения (Golden Cross или Death Cross)
- Оценивает уверенность на основе скорости сближения и стабильности

### 3. Поддержка в StrategyBase

Метод `PredictNextSignal` добавлен в `StrategyBase` с автоматической проверкой поддержки:

```go
func (sb *StrategyBase) PredictNextSignal(candles []Candle, config StrategyConfigV2) *FutureSignal {
    if predictive, ok := sb.signalGenerator.(PredictiveSignalGenerator); ok {
        return predictive.PredictNextSignal(candles, config)
    }
    return nil
}
```

## Использование

### Пример кода

```go
// Создаем генератор сигналов
generator := trend.NewPredictiveLinearSplineSignalGenerator()

// Конфигурация
config := &trend.PredictiveLinearSplineConfig{
    MinSegmentLength:      10,
    MaxSegmentLength:      50,
    PredictionHorizon:     5,
    MinR2Threshold:        0.50,
    SignalAdvance:         3,
    MinSlopeThreshold:     0.0003,
    TrendExhaustionFactor: 0.60,
    MinPriceChange:        0.003,
}

// Предсказываем сигнал
futureSignal := generator.PredictNextSignal(candles, config)

if futureSignal != nil {
    fmt.Printf("Тип: %s\n", futureSignal.SignalType)
    fmt.Printf("Дата: %s\n", time.Unix(futureSignal.Date, 0))
    fmt.Printf("Цена: %.4f\n", futureSignal.Price)
    fmt.Printf("Уверенность: %.2f%%\n", futureSignal.Confidence*100)
}
```

### Готовые примеры

#### Predictive Linear Spline

См. `examples/predict_next_signal_example.go` для полного рабочего примера.

Запуск:
```bash
go build -o predict_signal examples/predict_next_signal_example.go
./predict_signal tmos_big.json
```

#### Golden Cross

См. `examples/predict_golden_cross_example.go` для полного рабочего примера.

Запуск:
```bash
go build -o predict_golden_cross examples/predict_golden_cross_example.go
./predict_golden_cross tmos_big.json
```

## Алгоритмы предсказания

### Predictive Linear Spline

1. **Анализ текущего тренда**
   - Строит линейную регрессию на последних свечах
   - Вычисляет коэффициент детерминации (R²)
   - Определяет направление тренда

2. **Предсказание разворота**
   - Анализирует историческую длину трендов
   - Вычисляет импульс тренда (ускорение/замедление)
   - Определяет расстояние до разворота

3. **Вычисление параметров сигнала**
   - Экстраполирует цену в точке сигнала
   - Вычисляет дату сигнала
   - Оценивает уверенность на основе:
     - Качества модели (R²)
     - Истощения тренда
     - Импульса тренда
     - Изменения цены

### Golden Cross

1. **Расчет скользящих средних**
   - Вычисляет быструю EMA (Fast Period)
   - Вычисляет медленную EMA (Slow Period)
   - Определяет текущее положение линий

2. **Анализ динамики**
   - Вычисляет скорость изменения каждой EMA
   - Определяет относительную скорость сближения/расхождения
   - Проверяет направление движения (сближение или расхождение)

3. **Предсказание пересечения**
   - Вычисляет расстояние между линиями
   - Определяет количество свечей до пересечения
   - Экстраполирует цену в точке пересечения
   - Оценивает уверенность на основе:
     - Скорости сближения
     - Близости пересечения
     - Стабильности скорости изменения

## Возвращаемые значения

- **nil** - если предсказание невозможно:
  - Недостаточно данных
  - Слабый тренд (низкий R²)
  - Низкая уверенность в предсказании

- **FutureSignal** - структура с информацией о предсказанном сигнале:
  - `SignalType` - тип сигнала (BUY/SELL)
  - `Date` - Unix timestamp ожидаемого сигнала
  - `Price` - предсказанная цена
  - `Confidence` - уверенность (0.0 - 1.0)

## Расширение на другие стратегии

Чтобы добавить поддержку предсказания в другую стратегию:

1. Реализуйте метод `PredictNextSignal` в генераторе сигналов
2. Генератор автоматически будет реализовывать `PredictiveSignalGenerator`
3. Метод будет доступен через `StrategyBase`

Пример:

```go
func (sg *MySignalGenerator) PredictNextSignal(
    candles []internal.Candle,
    config internal.StrategyConfigV2,
) *internal.FutureSignal {
    // Ваша логика предсказания
    return &internal.FutureSignal{
        SignalType: internal.BUY,
        Date:       futureTimestamp,
        Price:      predictedPrice,
        Confidence: confidence,
    }
}
```

## Ограничения

- Предсказание основано на экстраполяции текущего тренда
- Не учитывает внешние факторы и новости
- Точность зависит от качества исторических данных
- Работает лучше на стабильных рынках с предсказуемыми трендами
