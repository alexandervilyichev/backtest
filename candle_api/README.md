# Candle Prediction API

A Python-based REST API for predicting the next candle in a financial time series using LSTM neural networks.

## Features

- Train an LSTM model on historical candle data
- Predict the next candle based on the last sequence of candles
- Optional: Train and predict using only selected fields (e.g., only `close`)
- Built with FastAPI, runs in Docker

## Structure

Based on the `internal/candle.go` structure from the parent project:

- `open`, `high`, `low`, `close`: floating-point prices (can be dict {"units":, "nano":})
- `volume`: string/float volume
- `time`: timestamp string
- `isComplete`: boolean
- `candleSource`: string

## Running with Docker

1. Build the image:
   ```bash
   docker build -t candle-api .
   ```

2. Run the container:
   ```bash
   docker run -p 8000:8000 candle-api
   ```

3. Access the API at `http://localhost:8000`

Interactive docs: `http://localhost:8000/docs`

## API Endpoints

### POST /train

Train the model.

**Request:**
```json
{
  "candles": [
    {
      "open": 100.0,
      "high": 101.0,
      "low": 99.0,
      "close": 101.0,
      "volume": 1000.0,
      "time": "2023-01-01T00:00:00Z",
      "isComplete": true,
      "candleSource": "source"
    },
    ...
  ],
  "fields": ["close"],  // list of fields to train on, default ["close"]
  "epochs": 50  // optional
}
```

**Response:**
```json
{
  "message": "Model trained successfully"
}
```

### POST /predict

Predict multiple next candles based on the last sequence.

**Request:**
```json
{
  "last_sequence": [
    {
      "open": 100.0,
      "high": 101.0,
      "low": 99.0,
      "close": 101.0,
      "volume": 1000.0,
      "time": "2023-01-01T00:00:00Z",
      "isComplete": true,
      "candleSource": "source"
    },
    ...  // provide at least 30 candles for default seq_length
  ],
  "steps": 5  // number of candles to predict, default 1
}
```

**Response:**
```json
{
  "predicted_candles": [
    {
      "close": 102.5
    },
    {
      "close": 103.1
    },
    ...
  ]
}
```

## Notes

- Default sequence length is 30 candles.
- Model and scaler are saved inside the container (persisted until container restart).
- Supports any subset of fields for training/prediction.
