import requests

def trigger_vm_start(url: str) -> tuple[bool, str]:
    """
    Делает запрос к шлюзу по указанному URL.
    Возвращает: (Успех_операции, Сообщение_для_лога)
    """
    try:
        response = requests.post(url, timeout=10)
        
        # 1. Сервер лежал и начал включаться
        if response.status_code == 200:
            return True, "🚀 Сервер был выключен. Команда на старт отправлена успешно."

        # 2. Обработка ответа от Яндекса
        try:
            data = response.json()
            code = data.get("code")
            message = data.get("message", "")
            
            # Код 9 + RUNNING = Всё хорошо
            if code == 9 and "RUNNING" in message:
                return True, "✅ Сервер уже работает."
            
            return False, f"⚠️ Ошибка API ({response.status_code}): {message}"
            
        except ValueError:
            return False, f"❌ Критическая ошибка шлюза: {response.text[:100]}"

    except requests.RequestException as e:
        return False, f"🚨 Ошибка сети: {e}"