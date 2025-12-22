import os
import sys
from dotenv import load_dotenv
from typing import List, Dict
from ruamel.yaml import YAML

load_dotenv()

CONFIG_PATH = "vms.yaml"

# Настройка парсера YAML
yaml = YAML()
yaml.preserve_quotes = True
yaml.indent(mapping=2, sequence=4, offset=2)
# -------------------------

def get_env_var(name: str, default: str = None) -> str:
    value = os.getenv(name, default)
    if value is None:
        print(f"❌ ОШИБКА: Не найдена обязательная переменная окружения: {name}")
        sys.exit(1)
    return value

def get_env_var_int(name: str, default: int = None) -> int:
    try:
        return int(get_env_var(name, str(default)))
    except (ValueError, TypeError):
        print(f"❌ ОШИБКА: Переменная окружения {name} должна быть целым числом.")
        sys.exit(1)

# --- Глобальная переменная для хранения всей структуры YAML ---
_config_data = None

def load_config():
    """Загружает конфиг целиком."""
    global _config_data
    try:
        with open(CONFIG_PATH, "r", encoding="utf-8") as f:
            _config_data = yaml.load(f)
            if not _config_data or 'vms' not in _config_data:
                print(f"⚠️ ПРЕДУПРЕЖДЕНИЕ: Структура 'vms' не найдена в {CONFIG_PATH}")
                return []
            return _config_data['vms']
    except FileNotFoundError:
        print(f"ℹ️ ИНФО: Файл {CONFIG_PATH} не найден.")
        return []
    except Exception as e:
        print(f"❌ ОШИБКА YAML: {e}")
        return []

def update_vms_file():
    """Сохраняет текущее состояние _config_data в файл, сохраняя комментарии."""
    if _config_data:
        try:
            with open(CONFIG_PATH, 'w', encoding="utf-8") as f:
                yaml.dump(_config_data, f)
        except Exception as e:
            print(f"❌ ОШИБКА сохранения конфига: {e}")

# --- Инициализация ---
BOT_TOKEN: str = get_env_var("BOT_TOKEN")
ADMIN_ID: int = get_env_var_int("ADMIN_ID")
TOPIC_ID: int = get_env_var_int("TOPIC_ID", default=None)
CHECK_INTERVAL: int = get_env_var_int("CHECK_INTERVAL", 60)

# Загружаем VMS при старте
VMS = load_config()