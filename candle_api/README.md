# Advanced Candle Predictor API

REST API для прогнозирования цен с использованием различных моделей машинного обучения.



## Установка

1. Установите зависимости:
```bash
pip install -r requirements.txt
```

2. Запустите API:
```bash
python main.py
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
├── config_manager.py            # Управление конфигурацией
├── config.yaml                  # Файл конфигурации
├── requirements.txt             # Зависимости Python
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
