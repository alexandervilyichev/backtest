from flask import Flask, request, jsonify
from flask_cors import CORS
import logging
from datetime import datetime
from typing import Dict, Any
import json

from models import (
    TrainRequest, PredictRequest, ModelStatus, 
    TrainResponse, PredictResponse, Candle
)
from config_manager import ConfigManager
from lstm_model import LSTMCandlePredictor
from brownian_motion_model import create_brownian_motion_predictor, BrownianMotionConfig

# Настройка логирования
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Создаем Flask приложение
app = Flask(__name__)
CORS(app)  # Включаем CORS для всех маршрутов

# Инициализируем компоненты
config_manager = ConfigManager("config.yaml")
predictor = LSTMCandlePredictor(config_manager)

# Инициализируем предикторы броуновского движения
brownian_predictors = {
    "heston": create_brownian_motion_predictor("heston"),
    "garch": create_brownian_motion_predictor("garch"), 
    "gbm": create_brownian_motion_predictor("gbm")
}

def validate_candles(candles_data):
    """Валидация данных свечей"""
    try:
        candles = []
        for candle_data in candles_data:
            candle = Candle(**candle_data)
            candles.append(candle)
        return candles, None
    except Exception as e:
        return None, str(e)

def startup():
    """Инициализация при запуске"""
    logger.info("Starting LSTM Candle Predictor API...")
    
    # Пытаемся загрузить существующую модель
    if predictor.load_model():
        logger.info("Existing model loaded successfully")
    else:
        logger.info("No existing model found, ready for training")

@app.route("/")
def root():
    """Корневой эндпоинт"""
    return jsonify({
        "message": "LSTM Candle Predictor API",
        "version": "1.0.0",
        "status": "running"
    })

@app.route("/status", methods=["GET"])
def get_status():
    """
    Получить статус модели
    
    Возвращает информацию о том, загружена ли модель,
    с какими параметрами обучена и когда
    """
    try:
        status_data = predictor.get_status()
        
        # Преобразуем datetime в строку для JSON
        if status_data.get('trained_at'):
            status_data['trained_at'] = status_data['trained_at'].isoformat()
        
        return jsonify(status_data)
    except Exception as e:
        logger.error(f"Error getting status: {str(e)}")
        return jsonify({"error": str(e)}), 500

@app.route("/train", methods=["POST"])
def train_model():
    """
    Обучить модель на массиве свечей
    
    Параметры обучения берутся из конфига, который автоматически
    перечитывается при изменении. Можно переопределить параметры
    через config_override в запросе.
    """
    try:
        data = request.get_json()
        
        if not data or 'candles' not in data:
            return jsonify({"error": "No candles provided"}), 400
        
        # Валидируем свечи
        candles, error = validate_candles(data['candles'])
        if error:
            return jsonify({"error": f"Invalid candle data: {error}"}), 400
        
        if len(candles) == 0:
            return jsonify({"error": "No candles provided"}), 400
        
        logger.info(f"Starting training with {len(candles)} candles")
        
        # Обучаем модель
        config_override = data.get('config_override')
        result = predictor.train(candles, config_override)
        
        if not result['success']:
            return jsonify({"error": result['message']}), 400
        
        return jsonify(result)
        
    except Exception as e:
        logger.error(f"Training error: {str(e)}")
        return jsonify({"error": str(e)}), 500

@app.route("/predict", methods=["POST"])
def predict_candles():
    """
    Предсказать следующие N свечей
    
    Принимает последовательность свечей и возвращает предсказания
    для следующих N свечей. Количество предсказываемых свечей
    можно задать в параметре prediction_steps или использовать
    значение из конфига.
    """
    try:
        if not predictor.is_trained:
            return jsonify({
                "error": "Model is not trained. Please train the model first."
            }), 400
        
        data = request.get_json()
        
        if not data or 'candles' not in data:
            return jsonify({"error": "No candles provided"}), 400
        
        # Валидируем свечи
        candles, error = validate_candles(data['candles'])
        if error:
            return jsonify({"error": f"Invalid candle data: {error}"}), 400
        
        if len(candles) == 0:
            return jsonify({"error": "No candles provided"}), 400
        
        logger.info(f"Making prediction with {len(candles)} input candles")
        
        # Делаем предсказание
        prediction_steps = data.get('prediction_steps')
        predictions = predictor.predict(candles, prediction_steps)
        
        # Преобразуем предсказания в словари для JSON
        predictions_dict = []
        for pred in predictions:
            pred_dict = pred.model_dump()
            predictions_dict.append(pred_dict)
        
        return jsonify({
            "success": True,
            "predictions": predictions_dict,
            "message": f"Successfully predicted {len(predictions)} candles"
        })
        
    except Exception as e:
        logger.error(f"Prediction error: {str(e)}")
        return jsonify({"error": str(e)}), 500

@app.route("/config", methods=["GET"])
def get_config():
    """Получить текущую конфигурацию"""
    return jsonify(config_manager.config)

@app.route("/config", methods=["POST"])
def update_config():
    """
    Обновить конфигурацию
    
    Принимает частичные обновления конфигурации.
    Изменения применяются немедленно.
    """
    try:
        config_updates = request.get_json()
        
        if not config_updates:
            return jsonify({"error": "No config updates provided"}), 400
        
        config_manager.update(config_updates)
        
        return jsonify({
            "success": True,
            "message": "Configuration updated successfully",
            "config": config_manager.config
        })
    except Exception as e:
        logger.error(f"Config update error: {str(e)}")
        return jsonify({"error": str(e)}), 500

@app.route("/brownian/predict", methods=["POST"])
def predict_brownian():
    """
    Прогнозирование цен с использованием моделей броуновского движения
    
    Поддерживаемые модели:
    - heston: Модель Heston со стохастической волатильностью
    - garch: GARCH(1,1) модель с условной волатильностью
    - gbm: Геометрическое броуновское движение
    """
    try:
        data = request.get_json()
        
        if not data or 'candles' not in data:
            return jsonify({"error": "No candles provided"}), 400
        
        # Валидируем свечи
        candles, error = validate_candles(data['candles'])
        if error:
            return jsonify({"error": f"Invalid candle data: {error}"}), 400
        
        if len(candles) == 0:
            return jsonify({"error": "No candles provided"}), 400
        
        # Извлекаем цены закрытия
        prices = [float(candle.close) for candle in candles]
        
        # Получаем тип модели (по умолчанию heston)
        model_type = data.get('model_type', 'heston')
        if model_type not in brownian_predictors:
            return jsonify({"error": f"Unsupported model type: {model_type}"}), 400
        
        logger.info(f"Making Brownian prediction with {len(candles)} candles using {model_type} model")
        
        # Получаем предиктор
        predictor = brownian_predictors[model_type]
        
        # Обновляем конфигурацию если передана
        if 'config' in data:
            config_data = data['config']
            predictor.config.window_size = config_data.get('window_size', predictor.config.window_size)
            predictor.config.prediction_steps = config_data.get('prediction_steps', predictor.config.prediction_steps)
            predictor.config.num_simulations = config_data.get('num_simulations', predictor.config.num_simulations)
        
        # Делаем прогноз
        prediction = predictor.predict(prices)
        
        if "error" in prediction:
            return jsonify({"error": prediction["error"]}), 400
        
        return jsonify({
            "success": True,
            "prediction": prediction,
            "message": f"Successfully predicted using {model_type} model"
        })
        
    except Exception as e:
        logger.error(f"Brownian prediction error: {str(e)}")
        return jsonify({"error": str(e)}), 500

@app.route("/brownian/signals", methods=["POST"])
def generate_brownian_signals():
    """
    Генерация торговых сигналов на основе моделей броуновского движения
    """
    try:
        data = request.get_json()
        
        if not data or 'candles' not in data:
            return jsonify({"error": "No candles provided"}), 400
        
        # Валидируем свечи
        candles, error = validate_candles(data['candles'])
        if error:
            return jsonify({"error": f"Invalid candle data: {error}"}), 400
        
        if len(candles) == 0:
            return jsonify({"error": "No candles provided"}), 400
        
        # Извлекаем цены закрытия
        prices = [float(candle.close) for candle in candles]
        
        # Получаем параметры
        model_type = data.get('model_type', 'heston')
        threshold = data.get('threshold', 0.02)
        
        if model_type not in brownian_predictors:
            return jsonify({"error": f"Unsupported model type: {model_type}"}), 400
        
        logger.info(f"Generating Brownian signals for {len(candles)} candles using {model_type} model")
        
        # Получаем предиктор
        predictor = brownian_predictors[model_type]
        
        # Обновляем конфигурацию если передана
        if 'config' in data:
            config_data = data['config']
            predictor.config.window_size = config_data.get('window_size', predictor.config.window_size)
            predictor.config.prediction_steps = config_data.get('prediction_steps', predictor.config.prediction_steps)
            predictor.config.num_simulations = config_data.get('num_simulations', predictor.config.num_simulations)
        
        # Генерируем сигналы
        signals = predictor.generate_signals(prices, threshold)
        
        # Подсчитываем статистику сигналов
        signal_counts = {
            "BUY": signals.count("BUY"),
            "SELL": signals.count("SELL"), 
            "HOLD": signals.count("HOLD")
        }
        
        return jsonify({
            "success": True,
            "signals": signals,
            "signal_counts": signal_counts,
            "model_type": model_type,
            "threshold": threshold,
            "message": f"Successfully generated {len(signals)} signals using {model_type} model"
        })
        
    except Exception as e:
        logger.error(f"Brownian signals error: {str(e)}")
        return jsonify({"error": str(e)}), 500

@app.route("/brownian/models", methods=["GET"])
def get_brownian_models():
    """Получить список доступных моделей броуновского движения"""
    return jsonify({
        "models": {
            "heston": {
                "name": "Heston Model",
                "description": "Стохастическая волатильность с корреляцией",
                "parameters": ["mu", "kappa", "theta", "sigma", "rho", "v0"]
            },
            "garch": {
                "name": "GARCH(1,1) Model", 
                "description": "Условная волатильность с ARCH/GARCH эффектами",
                "parameters": ["omega", "alpha", "beta", "mu"]
            },
            "gbm": {
                "name": "Geometric Brownian Motion",
                "description": "Классическое геометрическое броуновское движение",
                "parameters": ["mu", "sigma"]
            }
        }
    })

@app.errorhandler(404)
def not_found(error):
    return jsonify({"error": "Endpoint not found"}), 404

@app.errorhandler(405)
def method_not_allowed(error):
    return jsonify({"error": "Method not allowed"}), 405

@app.errorhandler(500)
def internal_error(error):
    return jsonify({"error": "Internal server error"}), 500

if __name__ == "__main__":
    print("Starting LSTM Candle Predictor API on http://localhost:8000")
    print("Press Ctrl+C to stop")
    
    try:
        # Инициализация при запуске
        startup()
        
        # Запускаем Flask сервер
        app.run(host="0.0.0.0", port=8000, debug=False, threaded=True)
        
    except KeyboardInterrupt:
        print("\nShutting down server...")
    finally:
        # Останавливаем наблюдение за конфигом
        config_manager.stop_watching()