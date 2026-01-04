import signal
import threading
import time

from loguru import logger

from src.bot import bot, send_alert
from src.client import get_vm_ip, ping_host, trigger_vm_start
from src.config import CHECK_INTERVAL, VMS, update_vms_file

# --- Graceful Shutdown ---
shutdown_event = threading.Event()


def signal_handler(signum, frame):
    """Обработчик сигналов завершения."""
    sig_name = signal.Signals(signum).name
    logger.info(f"⚠️ Получен сигнал {sig_name}, завершаем работу...")
    shutdown_event.set()


# Регистрируем обработчики
signal.signal(signal.SIGINT, signal_handler)  # Ctrl+C
signal.signal(signal.SIGTERM, signal_handler)  # Docker stop


def clean_for_log(text: str) -> str:
    """Убирает переносы строк для красивого лога."""
    if not text:
        return ""
    return text.replace("\n", " ").replace("\r", "").strip()


def watchdog_loop():
    """Фоновый процесс для проверки ВМ."""
    if not VMS:
        logger.warning("Watchdog не запускается: список ВМ пуст.")
        return

    logger.info(
        f"👀 Watchdog запущен. Интервал: {CHECK_INTERVAL} сек. Машин: {len(VMS)}"
    )

    vm_states = {}

    # Инициализация состояний
    for vm in VMS:
        vm_states[vm["name"]] = True

    while not shutdown_event.is_set():
        try:
            config_changed = False

            for vm in VMS:
                if shutdown_event.is_set():
                    break

                vm_name = vm["name"]
                vm_url = vm["url"]

                known_ip = vm.get("ip")

                last_known_is_up = vm_states.get(vm_name, True)
                is_currently_up = False
                check_details = ""

                # 1. Пинг
                ping_success = False
                if known_ip:
                    ping_success = ping_host(known_ip)

                if ping_success:
                    is_currently_up = True
                    if not last_known_is_up:
                        check_details = (
                            f"Машина снова доступна по IP {known_ip} (Ping OK)"
                        )
                else:
                    # 2. API (Check/Start)
                    success_api, text, start_initiated, new_ip = (
                        trigger_vm_start(vm_url)
                    )

                    if success_api and not new_ip and not known_ip:
                        new_ip = get_vm_ip(vm_url)

                    # --- СОХРАНЕНИЕ IP ---
                    if new_ip and new_ip != known_ip:
                        vm["ip"] = new_ip
                        config_changed = True
                        logger.info(f"Обнаружен IP для {vm_name}: {new_ip}")
                        known_ip = new_ip

                    # --- ЛОГИКА ЗАПУСКА ---
                    if start_initiated:
                        logger.info(
                            f"🚀 Автозапуск: ВМ {vm_name} запущена через API"
                        )
                        send_alert(
                            f"🚀 Автозапуск: ВМ *{vm_name}* запускается через API.\n\n{text}"
                        )

                        vm_states[vm_name] = False
                        continue

                    elif success_api:
                        is_currently_up = True
                        if not last_known_is_up:
                            check_details = "Статус API: RUNNING. (Ping не прошел, но API отвечает)"
                    else:
                        is_currently_up = False
                        check_details = text

                # --- ЛОГИКА УВЕДОМЛЕНИЙ ---

                # 1. Восстановление
                if is_currently_up and not last_known_is_up:
                    logger.info(
                        f"✅ Восстановление: ВМ {vm_name} снова доступна ({clean_for_log(check_details)})"
                    )
                    send_alert(
                        f"✅ ВОССТАНОВЛЕНИЕ: ВМ *{vm_name}* снова в строю.\n\n{check_details}"
                    )

                # 2. Сбой
                elif not is_currently_up and last_known_is_up:
                    logger.error(
                        f"🚨 Сбой: ВМ {vm_name} недоступна - {clean_for_log(check_details)}"
                    )
                    send_alert(
                        f"🚨 СБОЙ: ВМ *{vm_name}* недоступна.\n\n{check_details}"
                    )

                vm_states[vm_name] = is_currently_up

            if config_changed:
                update_vms_file()

        except Exception as e:
            err_text = clean_for_log(str(e))
            logger.critical(f"Критическая ошибка в цикле watchdog: {err_text}")
            logger.exception(e)

        # Graceful sleep с проверкой shutdown
        shutdown_event.wait(timeout=CHECK_INTERVAL)

    logger.info("👋 Watchdog завершён корректно")


if __name__ == "__main__":
    watchdog_thread = threading.Thread(target=watchdog_loop, daemon=True)
    watchdog_thread.start()

    logger.info("🤖 Бот запущен...")
    try:
        # Polling с проверкой shutdown
        while not shutdown_event.is_set():
            try:
                bot.polling(non_stop=False, timeout=30, long_polling_timeout=30)
            except Exception as e:
                if not shutdown_event.is_set():
                    logger.error(f"Ошибка polling: {e}")
                    time.sleep(5)
    except KeyboardInterrupt:
        pass
    finally:
        logger.info("⏳ Ожидание завершения потоков...")
        shutdown_event.set()
        watchdog_thread.join(timeout=10)
        logger.info("✅ Бот остановлен корректно")
