# Используем легкий официальный образ Python
FROM python:3.12

# Отключаем буферизацию вывода (чтобы логи сразу видны были)
ENV PYTHONUNBUFFERED=1

# Рабочая директория внутри контейнера
WORKDIR /app

# Копируем зависимости и устанавливаем их
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Копируем весь код
COPY . .

# Команда запуска (модульный запуск через -m)
CMD ["python", "-m", "src.main"]