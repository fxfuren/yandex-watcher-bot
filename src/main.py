import time
import threading
import logging
import sys
import requests
from src.config import CHECK_INTERVAL, VMS, update_vms_file
from src.client import trigger_vm_start, ping_host, get_vm_ip
from src.bot import bot, send_alert

# Настройка логов
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)

def clean_for_log(text: str) -> str:
    """Убирает переносы строк для красивого лога."""
    if not text:
        return ""
    return text.replace('\n', ' ').replace('\r', '').strip()

def watchdog_loop():
    """Фоновый процесс для проверки ВМ."""
    if not VMS:
        logging.warning("Watchdog не запускается: список ВМ пуст.")
        return
        
    logging.info(f"👀 Watchdog запущен. Интервал: {CHECK_INTERVAL} сек. Машин: {len(VMS)}")
    
    vm_states = {} 
    
    # Инициализация состояний
    for vm in VMS:
        vm_states[vm['name']] = True

    while True:
        try:
            config_changed = False 

            for vm in VMS:
                vm_name = vm['name']
                vm_url = vm['url']
                
                known_ip = vm.get('ip') 
                
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
                        check_details = f"Машина снова доступна по IP {known_ip} (Ping OK)"
                else:
                    # 2. API (Check/Start)
                    success_api, text, start_initiated, new_ip = trigger_vm_start(vm_url)
                    
                    if success_api and not new_ip and not known_ip:
                        new_ip = get_vm_ip(vm_url)
                    
                    # --- СОХРАНЕНИЕ IP ---
                    if new_ip and new_ip != known_ip:
                        vm['ip'] = new_ip 
                        config_changed = True
                        logging.info(f"Обнаружен IP для {vm_name}: {new_ip}")
                        known_ip = new_ip

                    # --- ЛОГИКА ЗАПУСКА ---
                    if start_initiated:
                        base_msg = f"🚀 Автозапуск: ВМ *{vm_name}* запускается через API."
                        
                        # В ЛОГ: чистим от переносов
                        logging.info(f"{base_msg} | {clean_for_log(text)}")
                        
                        # В ТЕЛЕГРАМ: шлем как есть (с переносами)
                        send_alert(f"{base_msg}\n\n{text}")
                        
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
                    base_msg = f"✅ ВОССТАНОВЛЕНИЕ: ВМ *{vm_name}* снова в строю."
                    logging.info(f"{base_msg} | {clean_for_log(check_details)}")
                    send_alert(f"{base_msg}\n\n{check_details}")
                
                # 2. Сбой
                elif not is_currently_up and last_known_is_up:
                    base_msg = f"🚨 СБОЙ: ВМ *{vm_name}* недоступна."
                    logging.error(f"{base_msg} | {clean_for_log(check_details)}")
                    send_alert(f"{base_msg}\n\n{check_details}")

                vm_states[vm_name] = is_currently_up

            if config_changed:
                update_vms_file()

        except Exception as e:
            err_text = clean_for_log(str(e))
            logging.critical(f"Критическая ошибка в цикле watchdog: {err_text}", exc_info=True)
        
        time.sleep(CHECK_INTERVAL)

if __name__ == "__main__":
    watchdog_thread = threading.Thread(target=watchdog_loop, daemon=True)
    watchdog_thread.start()

    logging.info("🤖 Бот запущен...")
    try:
        bot.infinity_polling(timeout=60, logger_level=logging.WARNING)
    except Exception as e:
        logging.critical(f"Бот остановлен: {clean_for_log(str(e))}", exc_info=True)