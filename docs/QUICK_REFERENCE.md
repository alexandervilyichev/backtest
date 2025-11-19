# 🚀 Быстрая шпаргалка: Композиция vs Встраивание

## 📌 Основные правила

### ✅ ДЕЛАТЬ

```go
// 1. Зависеть от интерфейсов
type Strategy struct {
    generator SignalGenerator  // интерфейс
}

// 2. Использовать композицию
type Strategy struct {
    generator SignalGenerator  // поле
}

// 3. Dependency Injection
func NewStrategy(gen SignalGenerator) *Strategy {
    return &Strategy{generator: gen}
}

// 4. Маленькие интерфейсы
type SignalGenerator interface {
    GenerateSignals(...) []SignalType
}

// 5. Явные зависимости
func NewStrategy(
    gen SignalGenerator,
    opt ConfigOptimizer,
) *Strategy { ... }
```

### ❌ НЕ ДЕЛАТЬ

```go
// 1. Зависеть от конкретных типов
type Strategy struct {
    generator *SMAGenerator  // конкретный тип
}

// 2. Использовать встраивание для поведения
type Strategy struct {
    BaseStrategy  // встраивание
}

// 3. Создавать зависимости внутри
func NewStrategy() *Strategy {
    return &Strategy{
        generator: NewSMAGenerator(),  // создание внутри
    }
}

// 4. Большие интерфейсы
type Strategy interface {
    Method1()
    Method2()
    // ... 10 методов
}

// 5. Скрытые зависимости
func NewStrategy() *Strategy {
    // Зависимости создаются внутри
}
```

---

## 🔄 Шаблон рефакторинга

### Шаг 1: Было

```go
type MyStrategy struct {
    internal.BaseConfig
    internal.BaseStrategy
}

func (s *MyStrategy) GenerateSignalsWithConfig(...) { /* ... */ }
func (s *MyStrategy) OptimizeWithConfig(...) { /* дублирование */ }
```

### Шаг 2: Стало

```go
// Компонент 1: Генератор сигналов
type MySignalGenerator struct{}
func (sg *MySignalGenerator) GenerateSignals(...) { /* ... */ }

// Компонент 2: Генератор конфигураций
type MyConfigGenerator struct{}
func (cg *MyConfigGenerator) Generate() []StrategyConfigV2 { /* ... */ }

// Фабрика
func NewMyStrategy(slippage float64) TradingStrategy {
    return NewStrategyBase(
        "my_strategy",
        NewMySignalGenerator(),
        NewConfigManager(...),
        NewGridSearchOptimizer(...),  // переиспользуется!
        NewSlippageProvider(slippage),
    )
}
```

---

## 🎯 Быстрые примеры

### Создание стратегии

```go
// Простой способ (с дефолтами)
strategy := NewGoldenCrossStrategy(0.01)

// Гибкий способ (кастомные компоненты)
strategy := NewStrategyBase(
    "custom",
    NewMLSignalGenerator(),      // ML генератор
    NewConfigManager(...),
    NewGeneticOptimizer(...),    // Генетический оптимизатор
    NewSlippageProvider(0.01),
)
```

### Использование стратегии

```go
// Генерация сигналов
config := strategy.DefaultConfig()
signals := strategy.GenerateSignals(candles, config)

// Оптимизация
bestConfig := strategy.Optimize(candles, strategy)

// Бэктест
result := Backtest(candles, signals, strategy.GetSlippage())
```

### Тестирование

```go
// Mock генератор
type MockGen struct{ signals []SignalType }
func (m *MockGen) GenerateSignals(...) []SignalType { return m.signals }

// Тест
func TestOptimizer(t *testing.T) {
    mock := &MockGen{signals: []SignalType{BUY, HOLD, SELL}}
    optimizer := NewGridSearchOptimizer(sp, cg)
    result := optimizer.Optimize(candles, mock)
    assert.NotNil(t, result)
}
```

---

## 📊 Сравнительная таблица

| Аспект | Встраивание | Композиция |
|--------|-------------|------------|
| Связанность | Высокая 🔴 | Низкая 🟢 |
| Гибкость | Низкая 🔴 | Высокая 🟢 |
| Тестируемость | Сложно 🔴 | Легко 🟢 |
| Переиспользование | 20% 🔴 | 80% 🟢 |
| Дублирование | Есть 🔴 | Нет 🟢 |
| SOLID | Нарушает 🔴 | Соблюдает 🟢 |

---

## 🔍 Когда использовать что?

### Композиция (почти всегда)

✅ Для поведения (генераторы, оптимизаторы)
✅ Когда нужна гибкость
✅ Когда компоненты независимы
✅ Для бизнес-логики

### Встраивание (редко)

⚠️ Для простых структур данных (Point, Color)
⚠️ Когда действительно "is-a", а не "has-a"
⚠️ Для утилитарных типов

---

## 💡 Ключевые принципы

1. **"Accept interfaces, return structs"**
   ```go
   func NewStrategy(gen SignalGenerator) *Strategy { ... }
   ```

2. **"Composition over inheritance"**
   ```go
   type Strategy struct {
       generator SignalGenerator  // has-a
   }
   ```

3. **"Depend on abstractions"**
   ```go
   type Strategy struct {
       optimizer ConfigOptimizer  // интерфейс
   }
   ```

4. **"Small interfaces"**
   ```go
   type SignalGenerator interface {
       GenerateSignals(...) []SignalType
   }
   ```

5. **"Explicit dependencies"**
   ```go
   func NewStrategy(gen SignalGenerator, opt ConfigOptimizer) { ... }
   ```

---

## 🚨 Частые ошибки

### ❌ Ошибка 1
```go
type Strategy struct {
    generator *SMAGenerator  // конкретный тип
}
```
**Исправление:**
```go
type Strategy struct {
    generator SignalGenerator  // интерфейс
}
```

### ❌ Ошибка 2
```go
func NewStrategy() *Strategy {
    return &Strategy{
        generator: NewSMAGenerator(),  // создание внутри
    }
}
```
**Исправление:**
```go
func NewStrategy(gen SignalGenerator) *Strategy {
    return &Strategy{generator: gen}  // передача извне
}
```

### ❌ Ошибка 3
```go
type Strategy struct {
    BaseStrategy  // встраивание для поведения
}
```
**Исправление:**
```go
type Strategy struct {
    generator SignalGenerator  // композиция
}
```

---

## 📚 Полезные ссылки

- [strategy_refactored.go](../internal/strategy_refactored.go) - Новая архитектура
- [strategy_example.go](../internal/strategy_example.go) - Примеры
- [golden_cross_strategy_v2.go](../strategies/trend/golden_cross_strategy_v2.go) - Миграция
- [REFACTORING_GUIDE.md](./REFACTORING_GUIDE.md) - Детальное руководство
- [ARCHITECTURE_COMPARISON.md](./ARCHITECTURE_COMPARISON.md) - Сравнение

---

## ✨ Итог

**Композиция + Интерфейсы = Гибкость + Тестируемость + SOLID**

Используйте композицию для поведения, интерфейсы для контрактов, и dependency injection для гибкости! 🚀
