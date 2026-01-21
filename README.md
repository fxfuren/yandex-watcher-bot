# 🤖 Yandex VM Watchdog Bot

Автоматический мониторинг и восстановление виртуальных машин Yandex Cloud с уведомлениями в Telegram.

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

---

## ✨ Возможности

- 🔍 **Ping First Strategy** - 90% экономия API запросов
- ⏸️ **Grace Period** - 85% сокращение запросов при запуске VM
- 🚀 **Параллельный мониторинг** - все VM проверяются одновременно
- ⚡ **Автозапуск** - автоматическое восстановление упавших VM
- 📱 **Telegram уведомления** - мгновенные алерты о состоянии
- 🐳 **Docker ready** - запуск в один клик
- 🔒 **Безопасность** - non-root пользователь, минимальный образ

---

## 🚀 Быстрый старт

### Docker (рекомендуется)

```bash
# 1. Клонируйте репозиторий
git clone https://github.com/your/yandex-watcher-bot.git
cd yandex-watcher-bot

# 2. Настройте конфигурацию
cp .env.example .env
nano .env  # Добавьте BOT_TOKEN и CHAT_ID

# 3. Настройте VM
nano vms.yaml  # Добавьте ваши VM

# 4. Запустите
docker-compose up -d

# 5. Смотрите логи
docker-compose logs -f
```

### Локально

```bash
# Установите зависимости
go mod download

# Соберите
go build -o watchdog ./cmd/watchdog

# Запустите
./watchdog
```

---

## 📋 Конфигурация

### .env файл

```bash
# Telegram
BOT_TOKEN=7958844485:AAHMFXxxxxxxxxxxxxxxxxxxxxxxxx
CHAT_ID=-1002279050000
TOPIC_ID=2  # Опционально для топиков

MIN_CHECK_INTERVAL=3s
MAX_CHECK_INTERVAL=59s
```

### vms.yaml файл

```yaml
telegram_workers: 3

vms:
  - name: ru-ya-01
    url: https://xxxxx.apigw.yandexcloud.net
    ip: 51.250.100.105 # Определяется автоматически

  - name: ru-ya-02
    url: https://yyyyy.apigw.yandexcloud.net
    ip: 51.250.108.169
```

---

## 🎯 Как это работает

### 1. Ping First Strategy (90% экономия API)

```
┌─────────────────────────────────────────┐
│ VM работает нормально                   │
├─────────────────────────────────────────┤
│ 1. Ping OK → Skip API ✅                │
│ 2. Повторяется каждые 60 секунд        │
│ 3. API используется: 0 запросов/час    │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│ VM упала                                │
├─────────────────────────────────────────┤
│ 1. Ping FAIL → API запрос ⚠️           │
│ 2. API используется: только при проблемах│
└─────────────────────────────────────────┘
```

### 2. Grace Period (85% экономия при запуске)

После запуска VM система ждет 60 секунд перед проверками:

```
10:00:00  Ping fails → API (Stopped) → Start VM
10:00:01  Grace period установлен (60 секунд)
10:00:05  ⏸️ Пропуск проверки (55s осталось)
10:00:20  ⏸️ Пропуск проверки (40s осталось)
10:00:40  ⏸️ Пропуск проверки (20s осталось)
10:01:00  Ping OK ✅ VM восстановлена

Результат: 1 API запрос вместо 6-8
```

### 3. Параллельный мониторинг

Все VM проверяются одновременно:

```go
for _, vm := range vms {
    go monitor.Start(vm)  // Каждая VM в своей горутине
}
```

**Преимущества:**

- Одновременное обнаружение проблем
- Параллельный запуск упавших VM
- Нет задержек между проверками

### 4. Интервалы проверки

| Состояние       | Интервал | Обоснование                 |
| --------------- | -------- | --------------------------- |
| Running         | 60s      | Стабильное, редкие проверки |
| Stopped/Crashed | 5s       | Критично, быстро запустить  |
| Starting        | 15s      | VM долго загружается        |
| Provisioning    | 15s      | Подготовка ресурсов         |
| Stopping        | 10s      | Обычно быстрая операция     |
| Updating        | 30s      | Долгая операция             |

---

## 📊 Статистика

### API запросы в месяц

**Оптимальный сценарий** (все VM стабильны):

- API запросов: **~0/месяц** ✅
- Ping проверок: ~130,000/месяц (бесплатно)

**Реалистичный сценарий** (1 падение/день/VM):

- Падений: 3 VM × 30 дней = 90 инцидентов
- API за инцидент: ~1-2 (с Grace Period)
- **Итого: ~100-200 API/месяц** из 100,000 лимита ✅

### Время восстановления

```
Обнаружение падения:    5-60 секунд
Команда запуска:        1-3 секунды
Загрузка VM:            60-120 секунд
Grace Period:           60 секунд (без лишних проверок)
────────────────────────────────────────────
Total:                  ~2-3 минуты
```

---

## 🐳 Docker

### Запуск

```bash
docker-compose up -d       # Запустить
docker-compose down        # Остановить
docker-compose restart     # Перезапустить
docker-compose logs -f     # Логи
docker-compose ps          # Статус
```

### Особенности

- ✅ Автоперезапуск при падении
- ✅ Health checks каждые 30 сек
- ✅ Ротация логов (10MB × 3 = 30MB макс)
- ✅ Graceful shutdown (15 сек)
- ✅ Минимальный образ (~20MB)
- ✅ Non-root пользователь

### Обновление

```bash
docker-compose down
git pull
docker-compose build
docker-compose up -d
```

Подробнее: [DOCKER.md](DOCKER.md)

---

## 📱 Telegram уведомления

### Типы уведомлений

1. **Сбой VM**

   ```
   🚨 СБОЙ: ВМ ru-ya-01 недоступна.
   Статус: Stopped
   ```

2. **Автозапуск**

   ```
   🚀 Автозапуск: ВМ ru-ya-01 запускается через API.
   ```

3. **Восстановление**

   ```
   ✅ ВОССТАНОВЛЕНИЕ: ВМ ru-ya-01 снова в строю.
   Проверка: Ping OK на 51.250.100.105
   ```

4. **Застревание в состоянии**
   ```
   ⚠️ ВНИМАНИЕ: ВМ ru-ya-01 застряла в статусе Starting более 5m
   ```

### Настройка Telegram

1. Создайте бота через [@BotFather](https://t.me/botfather)
2. Получите `BOT_TOKEN`
3. Добавьте бота в группу
4. Получите `CHAT_ID`:
   ```bash
   curl https://api.telegram.org/bot<YOUR_TOKEN>/getUpdates
   ```
5. Добавьте в `.env`

---

## 🔧 Troubleshooting

### Контейнер не запускается

```bash
# Проверьте логи
docker-compose logs

# Проверьте конфиг
docker-compose config

# Пересоберите
docker-compose down
docker-compose build
docker-compose up -d
```

### Telegram не работает

```bash
# Проверьте переменные
docker exec yandex-watchdog env | grep BOT_TOKEN

# Проверьте сеть
docker exec yandex-watchdog ping -c 3 api.telegram.org

# Проверьте токен
curl https://api.telegram.org/bot<YOUR_TOKEN>/getMe
```

### Permission denied для vms.yaml

```bash
# Исправьте права на хосте
chown 1000:1000 vms.yaml
# или
chmod 666 vms.yaml
```

### Ping не работает

Добавьте capability в docker-compose.yml:

```yaml
cap_add:
  - NET_RAW
```

---

## 🏗️ Архитектура

```
┌──────────────────────────────────────────┐
│            Coordinator                   │
│     (управляет мониторами)              │
└────────┬─────────────────────────────────┘
         │
    ┌────┴────┬──────────┬──────────┐
    │         │          │          │
┌───▼───┐ ┌──▼───┐ ┌────▼────┐ ┌───▼───┐
│VM-1   │ │VM-2  │ │  VM-3   │ │ VM-N  │
│Monitor│ │Monitor│ │ Monitor │ │Monitor│
└───┬───┘ └──┬───┘ └────┬────┘ └───┬───┘
    │        │           │          │
    └────────┴───────────┴──────────┘
             │           │
    ┌────────▼───┐  ┌───▼──────────┐
    │  Yandex    │  │  Telegram    │
    │  Cloud API │  │  Notifier    │
    └────────────┘  └──────────────┘
```

### Ключевые компоненты

- **Coordinator** - управляет мониторами VM
- **VMMonitor** - мониторинг одной VM (горутина)
- **YandexClient** - взаимодействие с API
- **NotificationQueue** - очередь уведомлений
- **Grace Period** - умная оптимизация запросов

---

## 📈 Сравнение с Python версией

### Python (старая версия)

```
✗ Последовательные проверки (1 VM за раз)
✗ Фиксированный интервал 60 секунд
✗ API запросы при каждой проверке
✗ Нет Grace Period
✗ Время реакции: 60-300 секунд
✗ Потребление памяти: 200MB+
```

### Go (текущая версия)

```
✓ Параллельные проверки (все VM сразу)
✓ Динамические интервалы (5-60 секунд)
✓ Ping First Strategy (90% экономия API)
✓ Grace Period (85% экономия при запуске)
✓ Время реакции: 5-15 секунд
✓ Потребление памяти: ~20MB
```

**Улучшение: 12-15x быстрее, 90% меньше API запросов**

---

## 🧪 Разработка

### Структура проекта

```
.
├── cmd/
│   └── watchdog/
│       └── main.go              # Entry point
├── internal/
│   ├── client/                  # Yandex Cloud API
│   ├── config/                  # Конфигурация
│   ├── monitoring/              # Логика мониторинга
│   │   ├── coordinator.go       # Координатор
│   │   ├── vm_monitor.go        # Монитор VM
│   │   └── status.go            # Состояния VM
│   ├── network/                 # Ping утилиты
│   ├── notification/            # Telegram алерты
│   └── types/                   # Типы данных
├── pkg/
│   └── logger/                  # Логирование
├── Dockerfile                   # Docker образ
├── docker-compose.yml           # Docker Compose
└── vms.yaml                     # Конфигурация VM
```

### Тесты

```bash
# Все тесты
go test ./...

# С race detector
go test -race ./...

# Конкретный пакет
go test -v ./internal/monitoring

# Бенчмарки
go test -bench=. ./internal/monitoring
```

---

## 📚 Документация

- [TECHNICAL_DOCS.md](TECHNICAL_DOCS.md) - Подробная техническая документация
- [DOCKER.md](DOCKER.md) - Docker setup и troubleshooting

---

## 🤝 Вклад в проект

1. Fork репозиторий
2. Создайте feature branch (`git checkout -b feature/amazing`)
3. Запустите тесты (`go test -race ./...`)
4. Commit изменения (`git commit -m 'Add feature'`)
5. Push в branch (`git push origin feature/amazing`)
6. Откройте Pull Request

---

## 📄 Лицензия

MIT License - используйте свободно!

---

## 🎯 Roadmap

- [ ] Prometheus метрики
- [ ] Web dashboard
- [ ] Поддержка других облачных провайдеров
- [ ] Slack интеграция
- [ ] Adaptive Grace Period на основе истории

---

**Сделано с ❤️ на Go для максимальной производительности**

⭐ Поставьте звезду, если проект был полезен!
