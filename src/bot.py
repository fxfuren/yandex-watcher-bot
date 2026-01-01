import telebot
from loguru import logger
from telebot.types import InlineKeyboardMarkup, InlineKeyboardButton
from src.config import BOT_TOKEN, GROUP_CHAT_ID, VMS, TOPIC_ID
from src.client import trigger_vm_start

bot = telebot.TeleBot(BOT_TOKEN, parse_mode="Markdown")

def check_group(message_or_call) -> bool:
    """Проверяет, что команда вызвана из нужной группы."""
    # Определяем тип объекта (message или callback_query)
    chat_id = None
    is_callback = False
    
    if hasattr(message_or_call, 'chat'):
        # Это message
        chat_id = message_or_call.chat.id
    elif hasattr(message_or_call, 'message'):
        # Это callback_query
        chat_id = message_or_call.message.chat.id
        is_callback = True
    
    # Проверяем, что это именно наша группа
    if chat_id != GROUP_CHAT_ID:
        try:
            # Для callback обязательно нужно ответить, иначе кнопка зависнет
            if is_callback:
                bot.answer_callback_query(message_or_call.id, "⛔️ Бот работает только в настроенной группе.", show_alert=True)
            else:
                bot.send_message(chat_id, "⛔️ Бот работает только в настроенной группе.",
                               message_thread_id=TOPIC_ID if TOPIC_ID else None)
        except Exception as e:
            logger.error(f"Ошибка при отправке сообщения об отказе в доступе: {e}")
        return False
    return True

def send_alert(message: str):
    """Отправляет алерт в группу."""
    try:
        bot.send_message(GROUP_CHAT_ID, message, parse_mode="Markdown",
                         message_thread_id=TOPIC_ID if TOPIC_ID else None)
        logger.debug(f"Алерт отправлен в группу: {message[:50]}...")
    except Exception as e:
        logger.critical(f"Не удалось отправить алерт в группу: {e}")
        logger.exception(e)

def create_vm_keyboard() -> InlineKeyboardMarkup:
    """Создает клавиатуру со списком ВМ."""
    keyboard = InlineKeyboardMarkup()
    for i, vm in enumerate(VMS):
        keyboard.add(InlineKeyboardButton(f"🖥 {vm['name']}", callback_data=f"vm_{i}"))
    if len(VMS) > 1:
        keyboard.add(InlineKeyboardButton("🚀 Все сразу", callback_data="vm_all"))
    return keyboard

@bot.message_handler(commands=['start', 'help'])
def handle_start(message):
    if not check_group(message): return
    
    logger.info(f"Команда /start от пользователя {message.from_user.id} (@{message.from_user.username})")
    thread_id = TOPIC_ID if TOPIC_ID else None
    
    try:
        if not VMS:
            bot.reply_to(message, "⚠️ **Конфигурация пуста!**\n\nНе найдено ни одной виртуальной машины. Пожалуйста, настройте переменную окружения `VM_CONFIG` и перезапустите бота.",
                         message_thread_id=thread_id)
            return

        bot.reply_to(
            message,
            "🤖 *Yandex VM Watchdog*\n\n"
            "Выберите машину, чтобы проверить ее статус или отправить команду на запуск.",
            reply_markup=create_vm_keyboard(),
            message_thread_id=thread_id
        )
    except Exception as e:
        logger.error(f"Ошибка в handle_start: {e}")
        logger.exception(e)

@bot.message_handler(commands=['ping'])
def handle_ping(message):
    if not check_group(message): return
    
    logger.info(f"Команда /ping от пользователя {message.from_user.id} (@{message.from_user.username})")
    try:
        bot.reply_to(message, "🏓 Понг!", message_thread_id=TOPIC_ID if TOPIC_ID else None)
    except Exception as e:
        logger.error(f"Ошибка в handle_ping: {e}")

@bot.callback_query_handler(func=lambda call: call.data.startswith('vm_'))
def handle_vm_callback(call):
    if not check_group(call): return

    vm_index_str = call.data.split('_')[1]
    logger.info(f"Callback от пользователя {call.from_user.id} (@{call.from_user.username}): {call.data}")
    
    try:
        bot.answer_callback_query(call.id, "🚀 Отправляю команду...")
        thread_id = TOPIC_ID if TOPIC_ID else None
        
        if vm_index_str == "all":
            results = []
            for vm in VMS:
                success, text, start_initiated, _ = trigger_vm_start(vm['url'])
                status_icon = "✅" if success else "❌"
                status_text = text if text else ("Запускается..." if start_initiated else "OK")
                results.append(f"*{vm['name']}*: {status_icon} {status_text}")
            
            final_message = "\n\n".join(results)
            bot.send_message(call.message.chat.id, 
                           f"📡 *Результаты проверки всех машин:*\n\n{final_message}",
                           message_thread_id=thread_id)
        else:
            vm_index = int(vm_index_str)
            if 0 <= vm_index < len(VMS):
                vm = VMS[vm_index]
                success, text, start_initiated, _ = trigger_vm_start(vm['url'])
                status_icon = "✅" if success else "❌"
                status_text = text if text else ("Запускается..." if start_initiated else "Машина работает")
                bot.send_message(call.message.chat.id, 
                               f"*{vm['name']}*: {status_icon} {status_text}",
                               message_thread_id=thread_id)
            else:
                bot.send_message(call.message.chat.id, "❌ Неверный индекс ВМ.",
                               message_thread_id=thread_id)
    except (ValueError, IndexError) as e:
        logger.error(f"Ошибка парсинга индекса ВМ: {e}")
        bot.send_message(call.message.chat.id, "❌ Ошибка при обработке вашего выбора.",
                       message_thread_id=TOPIC_ID if TOPIC_ID else None)
    except Exception as e:
        logger.error(f"Ошибка в handle_vm_callback: {e}")
        logger.exception(e)
        try:
            bot.send_message(call.message.chat.id, "❌ Произошла ошибка при обработке команды.",
                           message_thread_id=TOPIC_ID if TOPIC_ID else None)
        except:
            pass

    # Обновляем сообщение, чтобы убрать "часики" на кнопке
    try:
        bot.edit_message_reply_markup(call.message.chat.id, call.message.message_id)
    except Exception as e:
        logger.debug(f"Не удалось обновить markup: {e}")

# --- Для запуска без Docker ---
if __name__ == '__main__':
    logger.info("🤖 Бот запускается в режиме polling...")
    try:
        bot.polling(non_stop=True, timeout=60)
    except Exception as e:
        logger.critical(f"Бот остановлен: {e}")
        logger.exception(e)