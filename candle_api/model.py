import numpy as np
import pandas as pd
from tensorflow.keras.models import Sequential, load_model
from tensorflow.keras.layers import LSTM, Dense, Dropout, Input
from tensorflow.keras.callbacks import EarlyStopping, ReduceLROnPlateau
from tensorflow.keras import mixed_precision
from sklearn.preprocessing import MinMaxScaler
import os
import joblib

# Set mixed precision policy for half precision
policy = mixed_precision.Policy('mixed_float16')
mixed_precision.set_global_policy(policy)

class CandlePredictor:
    def __init__(self, model_path: str = None, scaler_path: str = None, fields_path: str = None, seq_length_path: str = None):
        self.model_path = model_path or os.getenv('MODEL_PATH', 'model.keras')
        self.scaler_path = scaler_path or os.getenv('SCALER_PATH', 'scaler.pkl')
        self.fields_path = fields_path or os.getenv('FIELDS_PATH', 'fields.pkl')
        self.seq_length_path = seq_length_path or os.getenv('SEQ_LENGTH_PATH', 'seq_length.pkl')
        self.model = None
        self.scaler = MinMaxScaler()
        self.seq_length = 30  # default
        self.input_shape = (self.seq_length, 1)  # seq_length, features, default 1 for close only
        self.fields = ['close']  # default, will be set during training
    
    def train(self, X: np.ndarray, y: np.ndarray, epochs: int = 50, batch_size: int = 512):
        num_features = X.shape[2]
        self.input_shape = (X.shape[1], num_features)
        
        # Normalize data
        X_flat = X.reshape(-1, num_features)
        y_flat = y.reshape(-1, num_features)
        
        self.scaler.fit(X_flat)
        X_scaled = self.scaler.transform(X_flat).reshape(X.shape)
        y_scaled = self.scaler.transform(y_flat).reshape(y.shape)
        
        # Build model
        self.model = Sequential([
            Input(shape=self.input_shape),
            LSTM(50, return_sequences=True),
            Dropout(0.2),
            LSTM(50, return_sequences=False),
            Dropout(0.2),
            Dense(num_features)
        ])
        self.model.compile(optimizer='adam', loss='mse')
        
        #Define callbacks
        early_stopping = EarlyStopping(
            monitor='loss',
            patience=10,
            min_delta=0.000005,
            restore_best_weights=True,
            verbose=1
        )
        reduce_lr = ReduceLROnPlateau(
            monitor='loss',
            factor=0.5,
            patience=10,
            min_lr=1e-6,
            verbose=1
        )

        # Train
        self.model.fit(
            X_scaled, y_scaled,
            epochs=epochs,
            batch_size=batch_size,
            verbose=1,
            callbacks=[early_stopping, reduce_lr]
        )
        
        # Save
        self.save_model()
        joblib.dump(self.scaler, self.scaler_path)
        joblib.dump(self.fields, self.fields_path)
        joblib.dump(self.seq_length, self.seq_length_path)
    
    def predict_next(self, last_sequence: np.ndarray) -> np.ndarray:
        if self.model is None:
            if os.path.exists(self.model_path):
                self.load_model()
            else:
                raise ValueError("Model not trained or found")
        
        num_features = last_sequence.shape[1]
        last_sequence_flat = last_sequence.flatten().reshape(-1, num_features)
        
        # Normalize
        last_sequence_scaled = self.scaler.transform(last_sequence_flat).reshape(1, last_sequence.shape[0], num_features)
        
        # Predict
        pred_scaled = self.model.predict(last_sequence_scaled, verbose=0)
        
        # Denormalize
        pred = self.scaler.inverse_transform(pred_scaled)
        
        return pred.flatten()
    
    def predict_multiple(self, last_sequence: np.ndarray, steps: int) -> list[np.ndarray]:
        predictions = []
        current_sequence = last_sequence.copy()
        for _ in range(steps):
            pred = self.predict_next(current_sequence)
            predictions.append(pred)
            # Append to sequence for next prediction
            current_sequence = np.concatenate([current_sequence[1:], [pred]])
        return predictions
    
    def save_model(self):
        self.model.save(self.model_path)
    
    def load_model(self):
        self.model = load_model(self.model_path)
        if os.path.exists(self.scaler_path):
            self.scaler = joblib.load(self.scaler_path)
        if os.path.exists(self.fields_path):
            self.fields = joblib.load(self.fields_path)
        else:
            self.fields = ['close']  # fallback
        if os.path.exists(self.seq_length_path):
            self.seq_length = joblib.load(self.seq_length_path)
        else:
            self.seq_length = 60  # fallback
