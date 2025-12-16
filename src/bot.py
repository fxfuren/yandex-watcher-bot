import telebot
from telebot.types import InlineKeyboardMarkup, InlineKeyboardButton
from src.config import BOT_TOKEN, ADMIN_ID, VMS, TOPIC_ID
from src.client import trigger_vm_start

bot = telebot.TeleBot(BOT_TOKEN, parse_mode="Markdown")

def check_admin(message) -> bool:
    """Проверяет, является ли пользователь админом."""
    if message.from_user.id != ADMIN_ID:
        bot.reply_to(message, "⛔️ У вас нет доступа к этому боту.",
                     message_thread_id=TOPIC_ID if TOPIC_ID else None)
        return False
    return True

def send_alert(message: str):
    """Отправляет алерт администратору."""
    try:
        bot.send_message(ADMIN_ID, message,
                         message_thread_id=TOPIC_ID if TOPIC_ID else None)
    except Exception as e:
        # Логируем ошибку, если не удалось отправить сообщение
        print(f"CRITICAL: Failed to send alert to admin: {e}")

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
    if not check_admin(message): return
    
    thread_id = TOPIC_ID if TOPIC_ID else None
    
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

@bot.message_handler(commands=['ping'])
def handle_ping(message):
    if not check_admin(message): return
    bot.reply_to(message, "🏓 Понг!")

@bot.callback_query_handler(func=lambda call: call.data.startswith('vm_'))
def handle_vm_callback(call):
    if not check_admin(call): return

    vm_index_str = call.data.split('_')[1]
    
    bot.answer_callback_query(call.id, "🚀 Отправляю команду...")
    
    if vm_index_str == "all":
        results = []
        for vm in VMS:
            success, text = trigger_vm_start(vm['url'])
            status_icon = "✅" if success else "❌"
            results.append(f"*{vm['name']}*: {status_icon} {text}")
        
        final_message = "\n".join(results)
        bot.send_message(call.message.chat.id, f"📡 *Результаты проверки всех машин:*\n\n{final_message}")
    else:
        try:
            vm_index = int(vm_index_str)
            if 0 <= vm_index < len(VMS):
                vm = VMS[vm_index]
                success, text = trigger_vm_start(vm['url'])
                status_icon = "✅" if success else "❌"
                bot.send_message(call.message.chat.id, f"*{vm['name']}*: {status_icon} {text}")
            else:
                bot.send_message(call.message.chat.id, "❌ Неверный индекс ВМ.")
        except (ValueError, IndexError):
            bot.send_message(call.message.chat.id, "❌ Ошибка при обработке вашего выбора.")

    # Обновляем сообщение, чтобы убрать "часики" на кнопке
    bot.edit_message_reply_markup(call.message.chat.id, call.message.message_id)

# --- Для запуска без Docker ---
if __name__ == '__main__':
    print("🤖 Бот запускается...")
    bot.polling(non_stop=True)