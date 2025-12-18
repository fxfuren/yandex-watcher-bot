import time
import threading
import logging
import requests
import sys
from src.config import CHECK_INTERVAL, VMS
from src.client import trigger_vm_start
from src.bot import bot, send_alert

# Настройка логов, чтобы видеть их в docker logs
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)


def compose_message(base: str, details: str) -> str:
    """Возвращает строку с деталями только при их наличии."""
    details = details.strip()
    return base if not details else f"{base}\n\n{details}"

def watchdog_loop():
    """Фоновый процесс для периодической проверки состояния всех ВМ."""
    if not VMS:
        logging.warning("Watchdog не запускается: список ВМ пуст.")
        return
        
    logging.info(f"👀 Watchdog запущен. Интервал: {CHECK_INTERVAL} сек. Машин в списке: {len(VMS)}")
    
    vm_states = {} # Словарь для хранения последнего состояния ВМ: { "vm_name": True (is_up) }

    while True:
        try:
            for vm in VMS:
                vm_name = vm['name']
                vm_url = vm['url']

                last_known_is_up = vm_states.get(vm_name, True) # По умолчанию считаем, что ВМ в порядке
                is_currently_up, text, start_initiated = trigger_vm_start(vm_url)

                if start_initiated:
                    if last_known_is_up:
                        restart_msg = compose_message(
                            f"🚀 Автозапуск: ВМ *{vm_name}* запускается.", text
                        )
                        logging.info(restart_msg)
                        send_alert(restart_msg)

                    vm_states[vm_name] = False
                    continue

                # Случай 1: ВМ была недоступна и восстановилась (или была только что запущена)
                if is_currently_up and not last_known_is_up:
                    log_msg = compose_message(
                        f"✅ ВОССТАНОВЛЕНИЕ: ВМ *{vm_name}* снова в строю.", text
                    )
                    logging.warning(log_msg)
                    send_alert(log_msg)
                
                # Случай 2: ВМ была доступна и упала
                elif not is_currently_up and last_known_is_up:
                    # Если шлюз сообщает, что ВМ уже в состоянии STARTING, не дублируем запуск
                    if "STARTING" in text.upper():
                        log_msg = compose_message(
                            f"ℹ️ ВМ *{vm_name}* уже находится в процессе запуска. Повторный старт не требуется.",
                            text,
                        )
                        logging.info(log_msg)
                        send_alert(log_msg)
                    else:
                        log_msg = compose_message(
                            f"🚨 СБОЙ: ВМ *{vm_name}* недоступна.", text
                        )
                        logging.error(log_msg)
                        send_alert(log_msg)

                        # При первом обнаружении простоя пробуем запустить ВМ сразу, не дожидаясь следующего цикла
                        restart_success, restart_text, _ = trigger_vm_start(vm_url)
                        if restart_success:
                            restart_msg = compose_message(
                                f"🚀 Автозапуск: ВМ *{vm_name}* запускается.", restart_text
                            )
                            logging.info(restart_msg)
                        else:
                            restart_msg = compose_message(
                                f"⚠️ Не удалось автоматически запустить ВМ *{vm_name}*.", restart_text
                            )
                            logging.warning(restart_msg)
                        send_alert(restart_msg)
                
                # Обновляем состояние ВМ в словаре
                vm_states[vm_name] = is_currently_up

        except Exception as e:
            logging.critical(f"Критическая ошибка в цикле watchdog: {e}", exc_info=True)
        
        time.sleep(CHECK_INTERVAL)

if __name__ == "__main__":
    # Запуск фонового потока для мониторинга
    watchdog_thread = threading.Thread(target=watchdog_loop, daemon=True)
    watchdog_thread.start()

    # Запуск бота
    logging.info("🤖 Бот запущен и готов к работе.")
    try:
        # bot.polling() из bot.py теперь используется для локального запуска.
        # Для Docker используем infinity_polling.
        bot.infinity_polling(timeout=60, logger_level=logging.WARNING)
    except requests.exceptions.ConnectionError as e:
        logging.error("="*50)
        logging.error("❌ КРИТИЧЕСКАЯ ОШИБКА: Не удалось подключиться к серверам Telegram.")
        logging.error("Пожалуйста, проверьте ваше интернет-соединение и настройки DNS/файрвола.")
        logging.error(f"Подробности: {e.args[0]}")
        logging.error("="*50)
        sys.exit(1)
    except Exception as e:
        logging.critical(f"Бот остановлен с критической ошибкой: {e}", exc_info=True)