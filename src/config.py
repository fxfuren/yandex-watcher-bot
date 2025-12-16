import os
import sys
import yaml
from dotenv import load_dotenv
from typing import List, Dict

# Загружаем переменные из .env файла
load_dotenv()

def get_env_var(name: str, default: str = None) -> str:
    """Получает переменную окружения или вызывает ошибку, если она не найдена."""
    value = os.getenv(name, default)
    if value is None:
        print(f"❌ ОШИБКА: Не найдена обязательная переменная окружения: {name}")
        sys.exit(1)
    return value

def get_env_var_int(name: str, default: int = None) -> int:
    """Получает числовую переменную окружения."""
    try:
        return int(get_env_var(name, str(default)))
    except (ValueError, TypeError):
        print(f"❌ ОШИБКА: Переменная окружения {name} должна быть целым числом.")
        sys.exit(1)

def load_vms_from_yaml(config_path: str = "vms.yaml") -> List[Dict[str, str]]:
    """Загружает список ВМ из YAML-файла."""
    try:
        with open(config_path, "r", encoding="utf-8") as f:
            data = yaml.safe_load(f)
            if not data or 'vms' not in data:
                print(f"⚠️  ПРЕДУПРЕЖДЕНИЕ: 'vms' не найден в {config_path}. Мониторинг ВМ не будет работать.")
                return []
            
            vms = data['vms']
            if not isinstance(vms, list):
                print(f"❌ ОШИБКА: 'vms' в файле {config_path} должен быть списком.")
                return []

            # Валидация списка ВМ
            for vm in vms:
                if not isinstance(vm, dict) or "name" not in vm or "url" not in vm:
                    print(f"❌ ОШИБКА: Неверный формат элемента в списке 'vms'. Каждый элемент должен содержать 'name' и 'url'.")
                    continue # Пропускаем невалидную запись
            return vms

    except FileNotFoundError:
        print(f"ℹ️  ИНФО: Файл {config_path} не найден. Мониторинг ВМ отключен.")
        return []
    except yaml.YAMLError as e:
        print(f"❌ ОШИБКА: Не удалось разобрать YAML из файла {config_path}: {e}")
        return []

# --- Основные настройки бота из .env ---
BOT_TOKEN: str = get_env_var("BOT_TOKEN")
ADMIN_ID: int = get_env_var_int("ADMIN_ID")
CHECK_INTERVAL: int = get_env_var_int("CHECK_INTERVAL", 60)

# --- Конфигурация виртуальных машин из vms.yaml ---
VMS: List[Dict[str, str]] = load_vms_from_yaml()

if not VMS:
    print("⚠️ ПРЕДУПРЕЖДЕНИЕ: Список виртуальных машин пуст. Бот будет работать без мониторинга ВМ.")