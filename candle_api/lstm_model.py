import numpy as np
import pandas as pd
import tensorflow as tf
import keras
from keras.models import Sequential, load_model
from keras.layers import LSTM, Dense, Dropout, Input
from keras.callbacks import EarlyStopping, ReduceLROnPlateau, ModelCheckpoint
from sklearn.preprocessing import MinMaxScaler
import logging
import pickle
from pathlib import Path
from typing import List, Tuple, Dict, Any, Optional
from datetime import datetime, timedelta
import json

from models import Candle
from config_manager import ConfigManager

logger = logging.getLogger(__name__)

class LSTMCandlePredictor:
    def __init__(self, config_manager: ConfigManager):
        self.config_manager = config_manager
        self.model = None
        self.scaler = None
        self.feature_columns = None
        self.is_trained = False
        self.training_params = None
        self.trained_at = None
        self.timeframe_minutes = None  # Таймфрейм в минутах
        self.model_path = "models/lstm_candle_model.keras"
        self.scaler_path = "models/scaler.pkl"
        self.metadata_path = "models/metadata.json"
        
        # Создаем директорию для моделей
        Path("models").mkdir(exist_ok=True)
    
    def _detect_timeframe(self, candles: List[Candle]) -> int:
        """Определяет таймфрейм свечей в минутах"""
        if len(candles) < 2:
            logger.warning("Not enough candles to detect timeframe, assuming 1 minute")
            return 1
        
        # Преобразуем время в datetime объекты
        times = []
        for candle in candles[:10]:  # Берем первые 10 свечей для анализа
            time_str = candle.time
            try:
                dt = pd.to_datetime(time_str, format='ISO8601')
                times.append(dt)
            except:
                logger.warning(f"Failed to parse time: {time_str}")
                continue
        
        if len(times) < 2:
            logger.warning("Could not parse enough timestamps, assuming 1 minute")
            return 1
        
        # Вычисляем разности между соседними свечами
        intervals = []
        for i in range(1, len(times)):
            diff = times[i] - times[i-1]
            intervals.append(diff.total_seconds() / 60)  # В минутах
        
        # Находим наиболее частый интервал
        if intervals:
            # Округляем до ближайшего стандартного таймфрейма
            avg_interval = np.median(intervals)
            
            # Стандартные таймфреймы в минутах
            standard_timeframes = [1, 5, 15, 30, 60, 240, 1440]  # 1m, 5m, 15m, 30m, 1h, 4h, 1d
            
            # Находим ближайший стандартный таймфрейм
            closest_tf = min(standard_timeframes, key=lambda x: abs(x - avg_interval))
            
            logger.info(f"Detected timeframe: {closest_tf} minutes (avg interval: {avg_interval:.1f})")
            return closest_tf
        
        logger.warning("Could not detect timeframe, assuming 1 minute")
        return 1
    
    def _validate_timeframe_compatibility(self, candles: List[Candle]) -> bool:
        """Проверяет совместимость таймфрейма входных данных с обученной моделью"""
        if not self.is_trained or self.timeframe_minutes is None:
            return True  # Модель не обучена, проверка не нужна
        
        detected_tf = self._detect_timeframe(candles)
        
        if detected_tf != self.timeframe_minutes:
            logger.error(f"Timeframe mismatch: model trained on {self.timeframe_minutes}m, "
                        f"but input data has {detected_tf}m timeframe")
            return False
        
        return True
    
    def _add_technical_indicators(self, df: pd.DataFrame) -> pd.DataFrame:
        """Добавляет технические индикаторы"""
        # Простые скользящие средние
        for window in [5, 10, 20]:
            df[f'sma_{window}'] = df['close'].rolling(window=window).mean()
        
        # Относительные изменения
        df['price_change'] = df['close'].pct_change()
        df['volume_change'] = df['volume'].pct_change()
        
        # Высокие и низкие за период
        df['high_low_ratio'] = df['high'] / df['low']
        df['open_close_ratio'] = df['open'] / df['close']
        
        # Заполняем NaN значения
        df = df.bfill().fillna(0)
        
        return df
        
    def _prepare_data(self, candles: List[Candle]) -> pd.DataFrame:
        """Преобразует список свечей в DataFrame"""
        data = []
        for candle in candles:
            data.append({
                'open': float(candle.open),
                'high': float(candle.high),
                'low': float(candle.low),
                'close': float(candle.close),
                'volume': float(candle.volume),
                'time': candle.time
            })
        
        df = pd.DataFrame(data)
        # Обрабатываем время с учетом Z суффикса
        df['time'] = pd.to_datetime(df['time'], format='ISO8601')
        df = df.sort_values('time').reset_index(drop=True)
        
        # Добавляем технические индикаторы
        df = self._add_technical_indicators(df)
        
        return df
    
    def _create_sequences(self, data: np.ndarray, sequence_length: int, prediction_steps: int) -> Tuple[np.ndarray, np.ndarray]:
        """Создает последовательности для обучения LSTM"""
        X, y = [], []
        
        for i in range(len(data) - sequence_length - prediction_steps + 1):
            X.append(data[i:(i + sequence_length)])
            y.append(data[(i + sequence_length):(i + sequence_length + prediction_steps)])
        
        return np.array(X), np.array(y)
    
    def _build_model(self, input_shape: Tuple[int, int], output_shape: int) -> Sequential:
        """Создает LSTM модель"""
        model = Sequential()
        
        lstm_units = self.config_manager.get('lstm.units', [50, 50])
        dropout = self.config_manager.get('lstm.dropout', 0.2)
        recurrent_dropout = self.config_manager.get('lstm.recurrent_dropout', 0.2)
        
        # Входной слой (рекомендуемый способ для Keras 3.x)
        model.add(Input(shape=input_shape))
        
        # Первый LSTM слой
        model.add(LSTM(
            lstm_units[0],
            return_sequences=len(lstm_units) > 1,
            dropout=dropout,
            recurrent_dropout=recurrent_dropout
        ))
        
        # Дополнительные LSTM слои
        for i in range(1, len(lstm_units)):
            return_sequences = i < len(lstm_units) - 1
            model.add(LSTM(
                lstm_units[i],
                return_sequences=return_sequences,
                dropout=dropout,
                recurrent_dropout=recurrent_dropout
            ))
        
        # Выходной слой
        model.add(Dense(output_shape))
        
        # Компиляция модели
        learning_rate = self.config_manager.get('training.learning_rate', 0.001)
        model.compile(
            optimizer=keras.optimizers.Adam(learning_rate=learning_rate),
            loss='mse',
            metrics=['mae']
        )
        
        return model
    
    def _get_callbacks(self) -> List:
        """Создает callback'и для обучения"""
        callbacks = []
        
        # Early Stopping
        early_stopping_config = self.config_manager.get('callbacks.early_stopping', {})
        if early_stopping_config:
            callbacks.append(EarlyStopping(
                patience=early_stopping_config.get('patience', 10),
                monitor=early_stopping_config.get('monitor', 'val_loss'),
                restore_best_weights=early_stopping_config.get('restore_best_weights', True),
                verbose=1
            ))
        
        # Reduce Learning Rate
        reduce_lr_config = self.config_manager.get('callbacks.reduce_lr', {})
        if reduce_lr_config:
            callbacks.append(ReduceLROnPlateau(
                factor=reduce_lr_config.get('factor', 0.5),
                patience=reduce_lr_config.get('patience', 5),
                min_lr=reduce_lr_config.get('min_lr', 0.0001),
                monitor=reduce_lr_config.get('monitor', 'val_loss'),
                verbose=1
            ))
        
        # Model Checkpoint
        checkpoint_config = self.config_manager.get('callbacks.model_checkpoint', {})
        if checkpoint_config:
            callbacks.append(ModelCheckpoint(
                filepath=self.model_path,
                save_best_only=checkpoint_config.get('save_best_only', True),
                monitor=checkpoint_config.get('monitor', 'val_loss'),
                verbose=1
            ))
        
        return callbacks
    
    def train(self, candles: List[Candle], config_override: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """Обучает модель на данных свечей"""
        try:
            # Применяем переопределения конфига если есть
            if config_override:
                self.config_manager.update(config_override)
            
            logger.info("Starting model training...")
            
            # Определяем и сохраняем таймфрейм
            self.timeframe_minutes = self._detect_timeframe(candles)
            logger.info(f"Training on {self.timeframe_minutes}-minute timeframe")
            
            # Подготавливаем данные
            df = self._prepare_data(candles)
            
            # Получаем список признаков для обучения
            self.feature_columns = self.config_manager.get('model.features', ['close'])
            
            # Проверяем наличие всех признаков в данных
            missing_features = [f for f in self.feature_columns if f not in df.columns]
            if missing_features:
                raise ValueError(f"Missing features in data: {missing_features}")
            
            # Извлекаем признаки
            feature_data = df[self.feature_columns].values
            
            # Нормализация данных
            self.scaler = MinMaxScaler()
            scaled_data = self.scaler.fit_transform(feature_data)
            
            # Параметры модели
            sequence_length = self.config_manager.get('model.sequence_length', 60)
            prediction_steps = self.config_manager.get('model.prediction_steps', 5)
            
            # Создаем последовательности
            X, y = self._create_sequences(scaled_data, sequence_length, prediction_steps)
            
            if len(X) == 0:
                raise ValueError("Not enough data to create training sequences")
            
            logger.info(f"Created {len(X)} training sequences")
            
            # Создаем модель
            input_shape = (sequence_length, len(self.feature_columns))
            output_shape = prediction_steps * len(self.feature_columns)
            
            self.model = self._build_model(input_shape, output_shape)
            
            # Параметры обучения
            epochs = self.config_manager.get('training.epochs', 100)
            batch_size = self.config_manager.get('training.batch_size', 32)
            validation_split = self.config_manager.get('training.validation_split', 0.2)
            
            # Получаем callback'и
            callbacks = self._get_callbacks()
            
            # Обучение модели
            logger.info("Training model...")
            history = self.model.fit(
                X, y.reshape(y.shape[0], -1),
                epochs=epochs,
                batch_size=batch_size,
                validation_split=validation_split,
                callbacks=callbacks,
                verbose=1
            )
            
            # Сохраняем модель и метаданные
            self._save_model_and_metadata()
            
            self.is_trained = True
            self.trained_at = datetime.now()
            self.training_params = {
                'features': self.feature_columns,
                'sequence_length': sequence_length,
                'prediction_steps': prediction_steps,
                'epochs': epochs,
                'batch_size': batch_size
            }
            
            logger.info("Model training completed successfully")
            
            return {
                'success': True,
                'message': 'Model trained successfully',
                'training_history': {
                    'loss': history.history['loss'],
                    'val_loss': history.history['val_loss'],
                    'mae': history.history['mae'],
                    'val_mae': history.history['val_mae']
                },
                'model_path': self.model_path
            }
            
        except Exception as e:
            logger.error(f"Training failed: {str(e)}")
            return {
                'success': False,
                'message': f'Training failed: {str(e)}',
                'model_path': ''
            }
    
    def predict(self, candles: List[Candle], prediction_steps: Optional[int] = None) -> List[Candle]:
        """Предсказывает следующие свечи"""
        if not self.is_trained or self.model is None:
            raise ValueError("Model is not trained")
        
        # Проверяем совместимость таймфрейма
        if not self._validate_timeframe_compatibility(candles):
            raise ValueError(f"Timeframe mismatch: model trained on {self.timeframe_minutes}-minute candles, "
                           f"but input data has {self._detect_timeframe(candles)}-minute timeframe")
        
        if prediction_steps is None:
            prediction_steps = self.config_manager.get('model.prediction_steps', 5)
        
        # Подготавливаем данные
        df = self._prepare_data(candles)
        
        # Извлекаем признаки
        feature_data = df[self.feature_columns].values
        
        # Нормализация
        scaled_data = self.scaler.transform(feature_data)
        
        # Берем последнюю последовательность
        sequence_length = self.config_manager.get('model.sequence_length', 60)
        
        if len(scaled_data) < sequence_length:
            raise ValueError(f"Need at least {sequence_length} candles for prediction")
        
        last_sequence = scaled_data[-sequence_length:].reshape(1, sequence_length, len(self.feature_columns))
        
        # Предсказание
        prediction = self.model.predict(last_sequence, verbose=0)
        
        # Проверяем и корректируем размерность предсказания
        expected_size = prediction_steps * len(self.feature_columns)
        prediction_flat = prediction.flatten()
        
        if prediction_flat.size != expected_size:
            logger.warning(f"Prediction size mismatch: got {prediction_flat.size}, expected {expected_size}")
            # Если размер не совпадает, повторяем или обрезаем
            if prediction_flat.size < expected_size:
                # Дополняем предсказание повторением последних значений
                num_features = len(self.feature_columns)
                last_values = prediction_flat[-num_features:] if prediction_flat.size >= num_features else prediction_flat
                while prediction_flat.size < expected_size:
                    remaining = expected_size - prediction_flat.size
                    to_add = min(remaining, len(last_values))
                    prediction_flat = np.concatenate([prediction_flat, last_values[:to_add]])
            else:
                # Обрезаем до нужного размера
                prediction_flat = prediction_flat[:expected_size]
        
        prediction = prediction_flat.reshape(prediction_steps, len(self.feature_columns))
        
        # Денормализация
        prediction = self.scaler.inverse_transform(prediction)
        
        # Создаем свечи из предсказаний
        predicted_candles = []
        last_time = pd.to_datetime(candles[-1].time)
        
        for i, pred in enumerate(prediction):
            # Приращение времени с учетом таймфрейма модели
            next_time = last_time + timedelta(minutes=self.timeframe_minutes) * (i + 1)
            
            candle_data = {}
            for j, feature in enumerate(self.feature_columns):
                candle_data[feature] = float(pred[j])
            
            # Заполняем недостающие поля значениями по умолчанию
            open_val = candle_data.get('open', candle_data.get('close', 0))
            close_val = candle_data.get('close', 0)
            high_val = max(candle_data.get('high', close_val), open_val, close_val)
            low_val = min(candle_data.get('low', close_val), open_val, close_val)
            volume_val = max(0, candle_data.get('volume', 0))
            
            predicted_candle = Candle(
                open=open_val,
                high=high_val,
                low=low_val,
                close=close_val,
                volume=volume_val,
                time=next_time.isoformat() + "Z",
                isComplete=False,
                candleSource="CANDLE_SOURCE_PREDICTION"
            )
            
            predicted_candles.append(predicted_candle)
        
        return predicted_candles
    
    def _save_model_and_metadata(self):
        """Сохраняет модель, скейлер и метаданные"""
        # Сохраняем модель
        self.model.save(self.model_path)
        
        # Сохраняем скейлер
        with open(self.scaler_path, 'wb') as f:
            pickle.dump(self.scaler, f)
        
        # Сохраняем метаданные
        metadata = {
            'feature_columns': self.feature_columns,
            'trained_at': datetime.now().isoformat(),
            'training_params': self.training_params,
            'timeframe_minutes': self.timeframe_minutes
        }
        
        with open(self.metadata_path, 'w') as f:
            json.dump(metadata, f, indent=2)
    
    def load_model(self) -> bool:
        """Загружает сохраненную модель"""
        try:
            if not Path(self.model_path).exists():
                return False
            
            # Загружаем модель
            self.model = load_model(self.model_path)
            
            # Загружаем скейлер
            with open(self.scaler_path, 'rb') as f:
                self.scaler = pickle.load(f)
            
            # Загружаем метаданные
            with open(self.metadata_path, 'r') as f:
                metadata = json.load(f)
                self.feature_columns = metadata['feature_columns']
                self.trained_at = datetime.fromisoformat(metadata['trained_at'])
                self.training_params = metadata['training_params']
                self.timeframe_minutes = metadata.get('timeframe_minutes', 1)  # По умолчанию 1 минута
            
            self.is_trained = True
            logger.info("Model loaded successfully")
            return True
            
        except Exception as e:
            logger.error(f"Failed to load model: {str(e)}")
            return False
    
    def get_status(self) -> Dict[str, Any]:
        """Возвращает статус модели"""
        return {
            'is_loaded': self.is_trained,
            'training_params': self.training_params,
            'trained_at': self.trained_at,
            'model_path': self.model_path if self.is_trained else None,
            'features_used': self.feature_columns,
            'sequence_length': self.config_manager.get('model.sequence_length') if self.is_trained else None,
            'timeframe_minutes': self.timeframe_minutes
        }