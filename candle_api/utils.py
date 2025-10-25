import json
from typing import List, Dict, Any, Optional
import numpy as np

class Candle:
    def __init__(self, open: float, high: float, low: float, close: float, volume: float, time: str, is_complete: bool, candle_source: str):
        self.open = open
        self.high = high
        self.low = low
        self.close = close
        self.volume = volume
        self.time = time
        self.is_complete = is_complete
        self.candle_source = candle_source

def parse_candles(candles_json: List[Dict[str, Any]]) -> List[Candle]:
    candles = []
    for c in candles_json:
        candle = Candle(
            open=float(c['open']['units']) + float(c['open']['nano']) / 1e9 if isinstance(c['open'], dict) else c['open'],
            high=float(c['high']['units']) + float(c['high']['nano']) / 1e9 if isinstance(c['high'], dict) else c['high'],
            low=float(c['low']['units']) + float(c['low']['nano']) / 1e9 if isinstance(c['low'], dict) else c['low'],
            close=float(c['close']['units']) + float(c['close']['nano']) / 1e9 if isinstance(c['close'], dict) else c['close'],
            volume=float(c['volume']) if 'volume' in c and isinstance(c['volume'], str) else c.get('volume', 0),
            time=c.get('time', ''),
            is_complete=c.get('isComplete', False),
            candle_source=c.get('candleSource', '')
        )
        candles.append(candle)
    return candles

# Assuming no time for prediction, but we can use timestamp
import datetime

def timestamp_from_str(time_str: str) -> float:
    # Simple parser, assuming RFC3339
    try:
        dt = datetime.datetime.fromisoformat(time_str.replace('Z', '+00:00'))
        return dt.timestamp()
    except:
        return 0.0

def prepare_data(candles: List[Candle], fields: Optional[List[str]] = None, seq_length: int = 30) -> tuple[np.ndarray, np.ndarray]:
    """
    Prepares data for LSTM training.
    fields: list of field names to use, e.g. ['close'], default all relevant numeric fields
    """
    if fields is None:
        fields = ['open', 'high', 'low', 'close', 'volume']  # exclude time and metadata for now
    
    data = []
    for c in candles:
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
                row.append(timestamp_from_str(c.time))
        data.append(row)
    
    data = np.array(data)
    
    X, y = [], []
    for i in range(len(data) - seq_length):
        X.append(data[i:i+seq_length])
        y.append(data[i+seq_length])
    
    return np.array(X), np.array(y)

def candles_to_json(candles: List[Candle]) -> List[Dict]:
    return [{
        'open': c.open,
        'high': c.high,
        'low': c.low,
        'close': c.close,
        'volume': c.volume,
        'time': c.time,
        'isComplete': c.is_complete,
        'candleSource': c.candle_source
    } for c in candles]
