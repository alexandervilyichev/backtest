from pydantic import BaseModel, validator
from typing import List, Optional, Dict, Any, Union
from datetime import datetime

class Price(BaseModel):
    """Модель для представления цены в формате units/nano"""
    units: str
    nano: int
    
    def to_float(self) -> float:
        """Преобразует цену в float"""
        return float(self.units) + float(self.nano) / 1_000_000_000.0

class Candle(BaseModel):
    open: Union[float, Price, Dict[str, Any]]
    high: Union[float, Price, Dict[str, Any]]
    low: Union[float, Price, Dict[str, Any]]
    close: Union[float, Price, Dict[str, Any]]
    volume: Union[float, str]
    time: str
    isComplete: bool = True
    candleSource: str = ""
    
    @validator('open', 'high', 'low', 'close', pre=True)
    def convert_price(cls, v):
        """Преобразует цену из различных форматов в float"""
        if isinstance(v, dict):
            # Формат {"units": "7", "nano": 240000000}
            if 'units' in v and 'nano' in v:
                units = float(v['units'])
                nano = float(v['nano'])
                return units + nano / 1_000_000_000.0
            else:
                raise ValueError(f"Invalid price format: {v}")
        elif isinstance(v, (int, float)):
            return float(v)
        elif isinstance(v, str):
            return float(v)
        else:
            raise ValueError(f"Unsupported price type: {type(v)}")
    
    @validator('volume', pre=True)
    def convert_volume(cls, v):
        """Преобразует объем в float"""
        if isinstance(v, str):
            return float(v)
        elif isinstance(v, (int, float)):
            return float(v)
        else:
            raise ValueError(f"Unsupported volume type: {type(v)}")
    
    def to_dict(self) -> Dict[str, float]:
        """Возвращает словарь с числовыми значениями"""
        return {
            'open': float(self.open),
            'high': float(self.high),
            'low': float(self.low),
            'close': float(self.close),
            'volume': float(self.volume),
            'time': self.time,
            'isComplete': self.isComplete,
            'candleSource': self.candleSource
        }

class TrainRequest(BaseModel):
    candles: List[Candle]
    config_override: Optional[Dict[str, Any]] = None

class PredictRequest(BaseModel):
    candles: List[Candle]
    prediction_steps: Optional[int] = None

class ModelStatus(BaseModel):
    is_loaded: bool
    training_params: Optional[Dict[str, Any]] = None
    trained_at: Optional[datetime] = None
    model_path: Optional[str] = None
    features_used: Optional[List[str]] = None
    sequence_length: Optional[int] = None

class TrainResponse(BaseModel):
    success: bool
    message: str
    training_history: Optional[Dict[str, List[float]]] = None
    model_path: str

class PredictResponse(BaseModel):
    success: bool
    predictions: List[Candle]
    message: str = ""