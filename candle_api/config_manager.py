import yaml
import logging
from pathlib import Path
from typing import Dict, Any
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler

logger = logging.getLogger(__name__)

class ConfigHandler(FileSystemEventHandler):
    def __init__(self, config_manager):
        self.config_manager = config_manager
    
    def on_modified(self, event):
        if not event.is_directory and event.src_path.endswith('config.yaml'):
            logger.info("Config file changed, reloading...")
            self.config_manager.reload_config()

class ConfigManager:
    def __init__(self, config_path: str = "config.yaml"):
        self.config_path = Path(config_path)
        self.config: Dict[str, Any] = {}
        self.observer = None
        self.load_config()
        self.start_watching()
    
    def load_config(self):
        """Загружает конфигурацию из YAML файла"""
        try:
            with open(self.config_path, 'r', encoding='utf-8') as f:
                self.config = yaml.safe_load(f)
            logger.info(f"Configuration loaded from {self.config_path}")
        except Exception as e:
            logger.error(f"Failed to load config: {e}")
            self.config = self._get_default_config()
    
    def reload_config(self):
        """Перезагружает конфигурацию"""
        self.load_config()
    
    def start_watching(self):
        """Запускает наблюдение за изменениями конфига"""
        if self.observer:
            self.observer.stop()
        
        self.observer = Observer()
        handler = ConfigHandler(self)
        self.observer.schedule(handler, str(self.config_path.parent), recursive=False)
        self.observer.start()
        logger.info("Started watching config file for changes")
    
    def stop_watching(self):
        """Останавливает наблюдение за конфигом"""
        if self.observer:
            self.observer.stop()
            self.observer.join()
    
    def get(self, key: str, default=None):
        """Получает значение из конфига по ключу с поддержкой точечной нотации"""
        keys = key.split('.')
        value = self.config
        
        for k in keys:
            if isinstance(value, dict) and k in value:
                value = value[k]
            else:
                return default
        
        return value
    
    def update(self, updates: Dict[str, Any]):
        """Обновляет конфигурацию новыми значениями"""
        def deep_update(base_dict, update_dict):
            for key, value in update_dict.items():
                if key in base_dict and isinstance(base_dict[key], dict) and isinstance(value, dict):
                    deep_update(base_dict[key], value)
                else:
                    base_dict[key] = value
        
        deep_update(self.config, updates)
    
    def _get_default_config(self) -> Dict[str, Any]:
        """Возвращает конфигурацию по умолчанию"""
        return {
            "model": {
                "sequence_length": 60,
                "prediction_steps": 5,
                "features": ["close", "volume", "high", "low", "open"]
            },
            "training": {
                "epochs": 100,
                "batch_size": 32,
                "validation_split": 0.2,
                "learning_rate": 0.001
            },
            "lstm": {
                "units": [50, 50],
                "dropout": 0.2,
                "recurrent_dropout": 0.2
            },
            "callbacks": {
                "early_stopping": {
                    "patience": 10,
                    "monitor": "val_loss",
                    "restore_best_weights": True
                },
                "reduce_lr": {
                    "factor": 0.5,
                    "patience": 5,
                    "min_lr": 0.0001,
                    "monitor": "val_loss"
                }
            }
        }