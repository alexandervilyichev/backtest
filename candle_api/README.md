# Advanced Candle Predictor API

REST API для прогнозирования цен с использованием различных моделей машинного обучения и стохастических процессов.

## Возможности

### LSTM модель
- Обучение LSTM модели на исторических данных свечей
- Предсказание следующих N свечей
- Гибкая конфигурация через YAML файл с автоматической перезагрузкой
- Выбор подмножества полей свечи для обучения
- Callback'и для раннего выхода, логирования и адаптации скорости обучения

### Модели броуновского движения
- **Модель Heston** - стохастическая волатильность с корреляцией
- **GARCH(1,1)** - условная волатильность с ARCH/GARCH эффектами
- **Геометрическое броуновское движение** - классическая модель с переменной волатильностью
- Симуляции Монте-Карло для прогнозирования
- Генерация торговых сигналов на основе вероятностных прогнозов

## Установка

1. Установите зависимости:
```bash
pip install -r requirements.txt
```

2. Запустите API:
```bash
python flask_main.py
```

API будет доступен по адресу: http://localhost:8000

## Конфигурация

Настройки находятся в файле `config.yaml`. Файл автоматически перечитывается при изменении.

### Основные параметры:

- `model.sequence_length` - количество предыдущих свечей для предсказания
- `model.prediction_steps` - количество свечей для предсказания
- `model.features` - поля свечи для обучения (open, high, low, close, volume)
- `training.epochs` - количество эпох обучения
- `training.batch_size` - размер батча
- `lstm.units` - количество нейронов в LSTM слоях
- `callbacks` - настройки callback'ов для обучения

## API Endpoints

### LSTM модель

#### GET /status
Получить статус модели (загружена ли, параметры обучения, время обучения)

#### POST /train
Обучить модель на массиве свечей

Пример запроса:
```json
{
  "candles": [
    {
      "open": {"units": "100", "nano": 500000000},
      "high": {"units": "105", "nano": 0},
      "low": {"units": "99", "nano": 0},
      "close": {"units": "103", "nano": 0},
      "volume": "1000",
      "time": "2023-01-01T00:00:00Z",
      "isComplete": true,
      "candleSource": "CANDLE_SOURCE_EXCHANGE"
    }
  ],
  "config_override": {
    "training": {
      "epochs": 50
    }
  }
}
```

#### POST /predict
Предсказать следующие N свечей

Пример запроса:
```json
{
  "candles": [
    {
      "open": {"units": "100", "nano": 500000000},
      "high": {"units": "105", "nano": 0},
      "low": {"units": "99", "nano": 0},
      "close": {"units": "103", "nano": 0},
      "volume": "1000",
      "time": "2023-01-01T00:00:00Z",
      "isComplete": true,
      "candleSource": "CANDLE_SOURCE_EXCHANGE"
    }
  ],
  "prediction_steps": 5
}
```

### Модели броуновского движения

#### POST /brownian/predict
Прогнозирование цен с использованием стохастических моделей

Пример запроса:
```json
{
  "candles": [...],
  "model_type": "heston",
  "config": {
    "window_size": 100,
    "prediction_steps": 5,
    "num_simulations": 1000
  }
}
```

Ответ:
```json
{
  "success": true,
  "prediction": {
    "model_type": "heston",
    "current_price": 101.0,
    "predicted_price": 102.5,
    "expected_return": 0.0148,
    "volatility": 2.1,
    "confidence_interval": [98.2, 106.8],
    "probability_up": 0.67,
    "price_paths": [[101.0, 101.2, 102.1, 102.5, 103.0]],
    "num_simulations": 1000,
    "prediction_steps": 5
  }
}
```

#### POST /brownian/signals
Генерация торговых сигналов на основе стохастических моделей

Пример запроса:
```json
{
  "candles": [...],
  "model_type": "garch",
  "threshold": 0.02,
  "config": {
    "window_size": 100,
    "num_simulations": 500
  }
}
```

#### GET /brownian/models
Получить список доступных моделей броуновского движения

### Общие эндпоинты

#### GET /config
Получить текущую конфигурацию

#### POST /config
Обновить конфигурацию

## Структура проекта

```
candle_api/
├── main.py                      # Flask приложение
├── models.py                    # Pydantic модели для API
├── lstm_model.py                # LSTM модель для предсказания
├── brownian_motion_model.py     # Модели броуновского движения
├── config_manager.py            # Управление конфигурацией
├── config.yaml                  # Файл конфигурации
├── requirements.txt             # Зависимости Python
├── example_brownian_usage.py    # Пример использования API
├── BROWNIAN_MOTION_GUIDE.md     # Подробное руководство
├── models/                      # Директория для сохранения моделей
└── README.md                   # Документация
```

## Callback'и для обучения

- **Early Stopping** - останавливает обучение при отсутствии улучшений
- **Reduce Learning Rate** - уменьшает скорость обучения при выходе на плато
- **Model Checkpoint** - сохраняет лучшую модель во время обучения

## Логирование

Подробное логирование всех операций с настраиваемым уровнем детализации.

## Примеры использования

### LSTM модель

#### Обучение модели только на поле Close:

Измените в `config.yaml`:
```yaml
model:
  features:
    - "close"
```

#### Предсказание с пользовательскими параметрами:

```python
import requests

# Обучение
response = requests.post("http://localhost:8000/train", json={
    "candles": candles_data,
    "config_override": {
        "model": {"features": ["close", "volume"]},
        "training": {"epochs": 100}
    }
})

# Предсказание
response = requests.post("http://localhost:8000/predict", json={
    "candles": recent_candles,
    "prediction_steps": 10
})
```

### Модели броуновского движения

#### Прогнозирование с моделью Heston:

```python
import requests

response = requests.post("http://localhost:8000/brownian/predict", json={
    "candles": candles_data,
    "model_type": "heston",
    "config": {
        "window_size": 150,
        "prediction_steps": 10,
        "num_simulations": 2000
    }
})

prediction = response.json()["prediction"]
print(f"Ожидаемая цена: {prediction['predicted_price']}")
print(f"Вероятность роста: {prediction['probability_up']:.2%}")
```

#### Генерация торговых сигналов:

```python
response = requests.post("http://localhost:8000/brownian/signals", json={
    "candles": candles_data,
    "model_type": "garch",
    "threshold": 0.015
})

signals = response.json()["signals"]
print(f"Последние сигналы: {signals[-10:]}")
```

#### Запуск примера:

```bash
python example_brownian_usage.py
```

## Документация

- [Подробное руководство по моделям броуновского движения](BROWNIAN_MOTION_GUIDE.md)
- [Конфигурация и параметры](config.yaml)

## Go стратегии

В папке `../strategies/` также реализованы стратегии для Go бэктестера:

- `statistical/brownian_motion_strategy.go` - стратегия броуновского движения
- `volatility/garch_volatility_strategy.go` - стратегия на основе GARCH волатильности