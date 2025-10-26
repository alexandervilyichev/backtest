from dotenv import load_dotenv

load_dotenv()

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import List, Dict, Optional, Any
from utils import parse_candles, prepare_data
from model import CandlePredictor
import numpy as np

app = FastAPI(title="Candle Prediction API", description="LSTM-based candle prediction service", version="1.0.0")

predictor = CandlePredictor()

class TrainRequest(BaseModel):
    candles: List[Dict[str, Any]]
    fields: List[str] = ["close"]  # required, default close
    seq_length: int = 30  # sequence length for LSTM
    epochs: int = 50

class PredictRequest(BaseModel):
    last_sequence: List[Dict[str, Any]]  # last seq_length candles
    steps: int = 1  # number of candles to predict

@app.post("/train")
def train_model(request: TrainRequest):
    try:
        candles = parse_candles(request.candles)
        predictor.fields = request.fields  # set fields for prediction
        predictor.seq_length = request.seq_length  # set seq_length
        X, y = prepare_data(candles, request.fields, seq_length=request.seq_length)
        if len(X) == 0:
            raise HTTPException(status_code=400, detail="Not enough candles for training with given seq_length.")
        predictor.train(X, y, epochs=request.epochs)
        return {"message": "Model trained successfully"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/predict")
def predict_multiple(request: PredictRequest):
    try:
        seq_candles = parse_candles(request.last_sequence)
        data = prepare_data_for_prediction(seq_candles, predictor.fields)
        if data.shape[0] < predictor.seq_length:
            raise HTTPException(status_code=400, detail=f"Not enough data: need at least {predictor.seq_length} candles.")
        
        last_sequence = data[-predictor.seq_length:]  # take last seq_length
        predictions = predictor.predict_multiple(last_sequence, request.steps)
        
        predicted_candles = []
        for pred in predictions:
            pred_dict = {}
            for i, field in enumerate(predictor.fields):
                pred_dict[field] = float(pred[i])
            predicted_candles.append(pred_dict)
        
        return {"predicted_candles": predicted_candles}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

def prepare_data_for_prediction(seq_candles: list, fields: Optional[List[str]] = None) -> np.ndarray:
    # Since we don't have y for prediction, we do minimal prep
    if fields is None:
        fields = ['open', 'high', 'low', 'close', 'volume']

    data = []
    for c in seq_candles:
        row = []
        for field in fields:
            if field == 'open':
                row.append(c.open)
            elif field == 'high':
                row.append(c.high)
            elif field == 'low':
                row.append(c.low)
            elif field == 'close':
                row.append(c.close)
            elif field == 'volume':
                row.append(c.volume)
            elif field == 'time':
                from utils import timestamp_from_str
                row.append(timestamp_from_str(c.time))
        data.append(row)

    return np.array(data)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
