import telebot
from loguru import logger

from src.config import BOT_TOKEN, GROUP_CHAT_ID, TOPIC_ID

bot = telebot.TeleBot(BOT_TOKEN, parse_mode="Markdown")


def send_alert(message: str):
    """Отправляет алерт в группу."""
    try:
        bot.send_message(
            GROUP_CHAT_ID,
            message,
            parse_mode="Markdown",
            message_thread_id=TOPIC_ID if TOPIC_ID else None,
        )
    except Exception as e:
        logger.critical(f"Не удалось отправить алерт в группу: {e}")
        logger.exception(e)


# --- Для запуска без Docker ---
if __name__ == "__main__":
    logger.info("🤖 Бот запускается в режиме polling...")
    try:
        bot.polling(non_stop=True, timeout=60)
    except Exception as e:
        logger.critical(f"Бот остановлен: {e}")
        logger.exception(e)
