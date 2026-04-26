# AI Agents Ensemble

Полнофункциональная распределённая система интеллектуальных агентов с поддержкой Telegram и веб-интерфейса. Проект полностью контейнеризован с использованием Docker Compose и построен на микросервисной архитектуре с RabbitMQ в качестве брокера сообщений.

## ✨ Особенности

- **Мультиканальное взаимодействие**: Telegram-бот и веб-интерфейс (WebSocket + REST API).
- **Ансамбль специализированных агентов**: scout (поиск), analyst (анализ), writer (генерация), critic (оценка).
- **Гибкая LLM-интеграция**: поддержка локальных моделей (Ollama) и облачных провайдеров (OpenRouter).
- **Приватный поиск**: встроенная мета-поисковая система SearXNG.
- **Автоматическая оркестрация**: Go-оркестратор управляет очередями задач через RabbitMQ.
- **Готовность к продакшену**: HTTPS, обратный прокси, мониторинг, health checks.

## 🏗️ Архитектура

### Компоненты системы

1. **Оркестратор (Go)**
   - Принимает сообщения от пользователей через Telegram-бота и веб-API.
   - Сохраняет задачи в PostgreSQL.
   - Публикует задачи в RabbitMQ (очередь `task.scout`).
   - Обрабатывает результаты от агентов и отправляет ответы пользователям.

2. **Агенты (Python)**
   - **Scout**: выполняет веб-поиск через SearXNG/DuckDuckGo.
   - **Analyst**: анализирует собранную информацию.
   - **Writer**: генерирует текст на основе анализа.
   - **Critic**: оценивает качество текста и возвращает на доработку при необходимости.
   - Каждый агент подписан на свою очередь RabbitMQ и публикует результаты в очередь `task.result`.

3. **Инфраструктурные сервисы**
   - **PostgreSQL**: хранение задач и контекста.
   - **RabbitMQ**: брокер сообщений между оркестратором и агентами.
   - **SearXNG**: приватная мета-поисковая система.
   - **Ollama** (опционально): сервис для запуска локальных LLM.
   - **pgAdmin**: веб-интерфейс для администрирования БД.

4. **Веб-интерфейс**
   - Статический HTML/JS чат (`static/chat_modified.html`).
   - REST API (`/api/message`) для отправки запросов.
   - WebSocket (`/ws`) для получения реальных ответов.

### Поток данных

```
Пользователь → [Telegram] → Оркестратор → PostgreSQL
                    ↓
             [Веб-интерфейс] → REST API → Оркестратор
                    ↓
                RabbitMQ (task.scout)
                    ↓
                Агенты (scout → analyst → writer → critic)
                    ↓
                RabbitMQ (task.result)
                    ↓
                Оркестратор → PostgreSQL
                    ↓
        [Telegram / WebSocket] → Пользователь
```

## 🚀 Быстрый старт

### Предварительные требования

- **Docker** и **Docker Compose** (Docker Desktop или standalone).
- **Telegram-бот** (получите токен у [@BotFather](https://t.me/BotFather)).
- **~2 ГБ свободной RAM** (при использовании OpenRouter) или **~4 ГБ** (с Ollama).

### Шаг 1: Клонирование и настройка

```bash
git clone <repository-url>
cd agents_app

# Скопируйте пример переменных окружения
cp .env.example .env

# Отредактируйте .env, указав ваш Telegram токен
# TELEGRAM_BOT_TOKEN=ваш_токен_здесь
```

### Шаг 2: Запуск всех сервисов

```bash
docker-compose up -d --build

# Проверка статуса
docker-compose ps

# Просмотр логов оркестратора
docker-compose logs -f orchestrator
```

### Шаг 3: Инициализация Ollama (если используется)

```bash
# Загрузите модель (однократно)
docker exec ai_agency_ollama ollama pull qwen2.5:1.5b

# Проверьте доступные модели
docker exec ai_agency_ollama ollama list
```

### Шаг 4: Тестирование

1. **Telegram**: найдите своего бота, отправьте любое сообщение (например, «Что такое ИИ?»).
2. **Веб-интерфейс**: откройте `http://localhost:8080/static/chat_modified.html`, введите запрос.

## 🌐 Настройка веб-интерфейса

Веб-интерфейс уже встроен в оркестратор и доступен на порту 8080.

### Использование модифицированного чата

Файл `static/chat_modified.html` содержит готовый интерфейс, который:
- Подключается к WebSocket (`ws://localhost:8080/ws`).
- Отправляет запросы через REST API (`POST /api/message`).
- Отображает ответы в реальном времени.

Чтобы использовать его на другом домене, измените `API_BASE` в файле:

```javascript
const API_BASE = 'https://ваш-домен.com';
```

### Размещение на GitHub Pages

Статический чат можно разместить на GitHub Pages, но backend должен работать на отдельном сервере (VPS, Render, Railway).

## 📦 Деплой

### Вариант 1: VPS (рекомендуется)

Подробное руководство в [DEPLOY.md](DEPLOY.md). Кратко:

1. Выберите VPS с 2+ ГБ RAM (например, Hetzner CX21, DigitalOcean 4 ГБ).
2. Установите Docker и Docker Compose.
3. Скопируйте проект, настройте `.env`.
4. Запустите `docker-compose up -d`.
5. Настройте nginx и SSL (Let's Encrypt).

### Вариант 2: Облачные платформы (Render, Railway, Fly.io)

Эти платформы поддерживают Docker Compose. Пример для Render:

1. Создайте новый **Web Service**.
2. Подключите репозиторий.
3. Укажите `docker-compose.yml` как конфигурацию.
4. Добавьте переменные окружения (TELEGRAM_BOT_TOKEN и др.).
5. Деплой автоматический.

### Вариант 3: Маломощный VPS (1 ГБ RAM)

Если RAM ограничена, выполните оптимизации:

1. Отключите ненужные сервисы в `docker-compose.yml`:
   ```yaml
   searxng:
     restart: "no"
   pgadmin:
     restart: "no"
   ollama:
     restart: "no"  # используйте OpenRouter
   ```

2. Установите лимиты памяти для контейнеров:
   ```yaml
   mem_limit: 256m
   ```

3. Добавьте swap-файл (2 ГБ):
   ```bash
   sudo fallocate -l 2G /swapfile
   sudo chmod 600 /swapfile
   sudo mkswap /swapfile
   sudo swapon /swapfile
   ```

## ⚙️ Конфигурация

### Переменные окружения

Ключевые переменные (полный список в `.env.example`):

| Переменная | Описание | Пример |
|------------|----------|--------|
| `TELEGRAM_BOT_TOKEN` | Токен Telegram бота (обязательно) | `123456:ABC-DEF1234` |
| `LLM_PROVIDER` | Провайдер LLM: `ollama` или `openrouter` | `openrouter` |
| `OPENROUTER_API_KEY` | Ключ API OpenRouter (если используется) | `sk-...` |
| `OPENROUTER_MODEL` | Модель OpenRouter | `meta-llama/llama-3.2-3b-instruct:free` |
| `DB_USER`, `DB_PASSWORD` | Учётные данные PostgreSQL | `user`, `password` |
| `RABBITMQ_USER`, `RABBITMQ_PASSWORD` | Учётные данные RabbitMQ | `guest`, `guest` |

### Выбор LLM-провайдера

**Ollama** (локально):
- Установите `LLM_PROVIDER=ollama`.
- Загрузите модель: `docker exec ai_agency_ollama ollama pull qwen2.5:1.5b`.
- Модель по умолчанию: `qwen2.5:1.5b`.

**OpenRouter** (облачно):
- Установите `LLM_PROVIDER=openrouter`.
- Получите API-ключ на [openrouter.ai](https://openrouter.ai).
- Укажите `OPENROUTER_API_KEY` и `OPENROUTER_MODEL`.

## 📁 Структура проекта

```
agents_app/
├── docker-compose.yml          # Полная конфигурация Docker
├── .env.example                # Пример переменных окружения
├── DEPLOY.md                   # Подробное руководство по деплою
├── README.md                   # Эта документация
│
├── orchestrator/               # Go-оркестратор
│   ├── cmd/orchestrator/main.go
│   ├── internal/
│   │   ├── telegram/bot.go     # Telegram-бот
│   │   ├── web/server.go       # Веб-сервер и WebSocket
│   │   ├── handlers/           # Обработчики RabbitMQ
│   │   └── database/           # Модели PostgreSQL
│   └── Dockerfile
│
├── agents/                     # Python-агенты
│   ├── main.py                 # Точка входа (запуск агента)
│   ├── brain.py                # Логика агента
│   ├── roles/                  # Реализации scout, analyst, writer, critic
│   ├── tools/                  # LLM, поиск, память
│   └── Dockerfile
│
├── static/                     # Веб-интерфейс
│   └── chat_modified.html      # Готовый чат
│
└── data/                       Конфигурация SearXNG
```

## 🔧 Разработка

### Локальная сборка

```bash
# Сборка оркестратора
cd orchestrator
go build ./cmd/orchestrator

# Сборка агентов
cd ../agents
uv sync  # установка зависимостей
```

### Добавление нового агента

1. Создайте файл в `agents/roles/` (наследуйте от `BaseAgent`).
2. Добавьте роль в `agents/brain.py`.
3. Обновите `docker-compose.yml`, добавив сервис с переменной `AGENT_ROLE=новая_роль`.
4. Обновите оркестратор (при необходимости).

## ❓ Часто задаваемые вопросы

### 1. Telegram-бот не отвечает

- Проверьте, что `TELEGRAM_BOT_TOKEN` указан правильно.
- Убедитесь, что оркестратор запущен: `docker-compose logs orchestrator`.
- Проверьте, что бот не заблокирован.

### 2. Веб-интерфейс не подключается по WebSocket

- Убедитесь, что оркестратор слушает порт 8080.
- Проверьте, что nginx (если используется) проксирует WebSocket (заголовки `Upgrade`, `Connection`).
- Откройте консоль браузера (F12) для диагностики ошибок.

### 3. Агенты не обрабатывают задачи

- Проверьте RabbitMQ: откройте `http://localhost:15672` (логин/пароль из .env).
- Убедитесь, что очереди созданы.
- Проверьте логи агентов: `docker-compose logs agent-scout`.

### 4. Не хватает памяти на VPS

- Отключите SearXNG, pgAdmin, Ollama.
- Используйте OpenRouter вместо Ollama.
- Добавьте swap.
- Установите `mem_limit` для контейнеров.

### 5. Как сменить модель LLM?

- Для Ollama: измените `MODEL_NAME` в `docker-compose.yml` (например, `llama3.2:3b`).
- Для OpenRouter: измените `OPENROUTER_MODEL` в `.env`.

## 📄 Лицензия

Проект распространяется под лицензией MIT. Подробности в файле LICENSE.

## 🤝 Вклад

Приветствуются пул-реквесты и issue. Перед внесением изменений обсудите их в issue.

## 📞 Контакты

- Репозиторий: [GitHub](https://github.com/your-repo)
- Telegram: [@your_contact](https://t.me/your_contact)

---

**Примечание**: Проект находится в активной разработке. API и конфигурация могут меняться.