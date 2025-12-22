import requests
import socket
import platform
import logging
from typing import Optional

def ping_host(host: str, port: int = 22, timeout: int = 3) -> bool:
    """
    Проверяет доступность хоста, пытаясь подключиться к TCP-порту.
    По умолчанию проверяет порт 22 (SSH).
    """
    try:
        # Пытаемся установить соединение
        with socket.create_connection((host, port), timeout=timeout):
            return True
    except (socket.timeout, socket.error):
        # Если таймаут или ошибка соединения — хост недоступен
        return False
    except Exception as e:
        logging.error(f"Ошибка при проверке порта {host}:{port}: {e}")
        return False

def get_vm_ip(base_url: str) -> Optional[str]:
    """
    Пытается получить IP адрес ВМ.
    Добавляет /info к базовому URL.
    """
    # Формируем URL для получения инфо
    info_url = f"{base_url.rstrip('/')}/info"
    
    try:
        response = requests.get(info_url, timeout=5)
        if response.status_code == 200:
            data = response.json()
            
            interfaces = data.get("networkInterfaces", [])
            if interfaces:
                primary = interfaces[0].get("primaryV4Address", {})
                
                # Приоритет публичному IP
                public_ip = primary.get("oneToOneNat", {}).get("address")
                if public_ip:
                    return public_ip
                
                internal_ip = primary.get("address")
                if internal_ip:
                    return internal_ip
    except Exception as e:
        logging.warning(f"Не удалось получить IP через {info_url}: {e}")
    
    return None

def trigger_vm_start(base_url: str) -> tuple[bool, str, bool, Optional[str]]:
    """
    Делает запрос к шлюзу (добавляет /start).
    """
    # Формируем URL для запуска
    start_url = f"{base_url.rstrip('/')}/start"
    
    ip_address = None
    try:
        response = requests.post(start_url, timeout=10)

        if response.status_code == 200:
            return True, "", True, None

        try:
            data = response.json()
            code = data.get("code")
            message = data.get("message", "")
            
            if "ip" in data:
                ip_address = data["ip"]

            if code == 9 and "RUNNING" in message:
                return True, "", False, ip_address

            return False, f"⚠️ Ошибка API ({response.status_code}): {message}", False, None

        except ValueError:
            return False, f"❌ Критическая ошибка шлюза: {response.text[:100]}", False, None

    except requests.RequestException as e:
        return False, f"🚨 Ошибка сети: {e}", False, None