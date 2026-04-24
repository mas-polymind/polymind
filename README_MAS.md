# Ансамбль агентов с Docker-оркестрацией

Полнофункциональная распределенная система интеллектуальных агентов с Telegram-интерфейсом, веб-поиском и локальными ИИ-моделями. Проект полностью контейнеризован с использованием Docker Compose.

## 🏗️ Архитектура

Система построена по принципу микросервисов с четким разделением ответственности:

### Компоненты системы

1. **Go-оркестратор** (`orchestrator/`)
   - Принимает сообщения от пользователей через Telegram-бота
   - Управляет очередью задач
   - Сохраняет задачи в PostgreSQL
   - Отправляет задачи Python-агенту
   - Обрабатывает callback с результатами
   - Отправляет ответы пользователям в Telegram

2. **Python-агент** (`agents/`)
   - FastAPI-сервер с endpoint `/enqueue`
   - Асинхронная обработка задач через BackgroundTasks
   - Веб-поиск через SearXNG и DuckDuckGo
   - Интеграция с Ollama (локальные ИИ-модели)
   - Формирование расширенных промптов на основе поиска
   - Отправка результатов обратно в оркестратор

3. **Инфраструктурные сервисы** (Docker контейнеры):
   - **PostgreSQL** - хранение задач и их статусов
   - **pgAdmin** - веб-интерфейс для управления БД
   - **SearXNG** - приватная мета-поисковая система
   - **Ollama** - сервис для запуска локальных ИИ-моделей

### Поток данных

```
Пользователь → Telegram → Go-оркестратор → PostgreSQL
                                      ↓
                                Python-агент
                                      ↓
                                SearXNG/Ollama
                                      ↓
                                Go-оркестратор → PostgreSQL
                                      ↓
                                Telegram → Пользователь
```

## 🛠️ Технологии

### Backend
- **Go 1.26** - высокопроизводительный оркестратор
  - Gin - веб-фреймворк
  - GORM - ORM для работы с PostgreSQL
  - Telegram Bot API - интеграция с мессенджером
- **Python 3.14** - интеллектуальный агент
  - FastAPI - асинхронный веб-фреймворк
  - uv - современный менеджер пакетов Python
  - AsyncOpenAI - клиент для Ollama API
  - httpx - асинхронные HTTP-запросы
  - DuckDuckGo Search - веб-поиск

### База данных
- **PostgreSQL 15** - реляционная БД для хранения задач
- **pgAdmin 4** - веб-интерфейс для администрирования

### ИИ и поиск
- **Ollama** - запуск локальных LLM моделей
  - Qwen 2.5 1.5B - легкая и умная модель для CPU
- **SearXNG** - мета-поисковая система с приватностью
- **DuckDuckGo** - резервный источник поиска

### Инфраструктура
- **Docker** - контейнеризация всех компонентов
- **Docker Compose** - оркестрация многоконтейнерного приложения
- **Alpine Linux** - минималистичные базовые образы

## 🚀 Быстрый старт

### Предварительные требования

1. **Docker** и **Docker Compose** установленные на системе
2. **Telegram-бот** созданный через @BotFather (получить токен)
3. **~4 ГБ свободной RAM** для Ollama и других сервисов

### Шаг 1: Клонирование и настройка

```bash
# Клонировать репозиторий
git clone <repository-url>
cd agents_app

# Создать конфигурационный файл
cp .env.docker .env

# Отредактировать .env, указав ваш Telegram токен
# TELEGRAM_BOT_TOKEN=ваш_токен_здесь
```

### Шаг 2: Запуск всех сервисов

```bash
# Собрать и запустить все контейнеры
docker-compose up -d --build

# Проверить статус
docker-compose ps

# Просмотр логов
docker-compose logs -f orchestrator
```

### Шаг 3: Инициализация Ollama

```bash
# Загрузить модель Qwen 2.5 1.5B (однократно)
docker exec ai_agency_ollama ollama pull qwen2.5:1.5b

# Проверить доступные модели
docker exec ai_agency_ollama ollama list
```

### Шаг 4: Тестирование системы

1. Найдите своего Telegram-бота по username
2. Отправьте ему любое сообщение (например, "Что такое ИИ?")
3. Дождитесь ответа (обычно 30-60 секунд)

## 📁 Структура проекта

```
agents_app/
├── docker-compose.yml          # Полная конфигурация Docker
├── .env.docker                 # Пример переменных окружения
├── README.md                   # Эта документация
│
├── orchestrator/              # Go-оркестратор
│   ├── Dockerfile             # Многостадийная сборка
│   ├── main.go                # Основная логика
│   ├── go.mod                 # Зависимости Go
│   ├── .env.example           # Пример конфигурации
│   └── internal/database/     # Работа с PostgreSQL
│       ├── db.go              # Модели GORM
│       └── init.go            # Инициализация БД
│
├── agents/                    # Python-агент
│   ├── Dockerfile             # Сборка на Python 3.14
│   ├── brain.py               # Основная логика FastAPI
│   ├── pyproject.toml         # Зависимости Python
│   ├── uv.lock                # Фиксация версий
│   └── .dockerignore          # Исключаемые файлы
│
└── data/                      # Данные и конфигурации
    └── searxng/               # Конфигурация SearXNG
        └── settings.yml       # Настройки поиска
```

## ⚙️ Конфигурация

### Ключевые переменные окружения (`.env`)

```env
# Telegram Bot
TELEGRAM_BOT_TOKEN=your_bot_token_here

# PostgreSQL
DB_HOST=db
DB_PORT=5432
DB_USER=user
DB_PASSWORD=password
DB_NAME=agency_db

# Python Agent
OLLAMA_HOST=ollama:11434
SEARXNG_URL=http://searxng:8080
ORCHESTRATOR_URL=http://orchestrator:8080
MODEL_NAME=qwen2.5:1.5b

# pgAdmin
PGADMIN_EMAIL=admin@admin.com
PGADMIN_PASSWORD=admin
```

### Порты сервисов

| Сервис | Порт | Назначение |
|--------|------|------------|
| PostgreSQL | 5432 | База данных |
| pgAdmin | 5050 | Веб-интерфейс БД |
| SearXNG | 8081 | Веб-поиск |
| Ollama | 11434 | API ИИ-моделей |
| Python-агент | 5000 | FastAPI сервер |
| Go-оркестратор | 8080 | HTTP сервер |

## 🔧 Управление

### Основные команды Docker Compose

```bash
# Запуск всех сервисов
docker-compose up -d

# Остановка всех сервисов
docker-compose down

# Перезапуск конкретного сервиса
docker-compose restart agent

# Просмотр логов в реальном времени
docker-compose logs -f orchestrator agent

# Проверка статуса
docker-compose ps

# Остановка с удалением volume (осторожно!)
docker-compose down -v
```

### Администрирование базы данных

```bash
# Подключение к PostgreSQL
docker exec -it ai_agency_db psql -U user -d agency_db

# Просмотр задач
SELECT * FROM tasks ORDER BY id DESC;

# Очистка завершенных задач
DELETE FROM tasks WHERE status = 'completed';
```

### Мониторинг

```bash
# Использование ресурсов
docker stats

# Логи Ollama
docker logs ai_agency_ollama

# Логи SearXNG
docker logs ai_agency_search
```

## 🧪 Тестирование и отладка

### Проверка здоровья системы

1. **PostgreSQL**: `http://localhost:5050` (pgAdmin)
2. **SearXNG**: `http://localhost:8081`
3. **FastAPI документация**: `http://localhost:5000/docs`
4. **Ollama API**: `http://localhost:11434/api/tags`

### Отладка проблем

```bash
# Проверить подключение между контейнерами
docker exec ai_agency_agent ping orchestrator
docker exec ai_agency_orchestrator ping db

# Проверить переменные окружения
docker exec ai_agency_agent env | grep OLLAMA
docker exec ai_agency_orchestrator env | grep DB

# Тестирование API агента
curl -X POST http://localhost:5000/enqueue \
  -H "Content-Type: application/json" \
  -d '{"id": 999, "query": "test"}'
```

## 📈 Масштабирование

### Горизонтальное масштабирование агентов

```yaml
# В docker-compose.yml можно добавить:
agent2:
  build: ./agents
  environment:
    - OLLAMA_HOST=ollama:11434
    - SEARXNG_URL=http://searxng:8080
    - ORCHESTRATOR_URL=http://orchestrator:8080
  depends_on:
    - ollama
    - searxng
```

### Оптимизация для продакшена

1. **Использование более мощных моделей**:
   ```bash
   docker exec ai_agency_ollama ollama pull qwen2.5:7b
   # Обновить MODEL_NAME в .env
   ```

2. **Настройка ресурсов** в `docker-compose.yml`:
   ```yaml
   orchestrator:
     deploy:
       resources:
         limits:
           cpus: '1'
           memory: 512M
   ```

3. **Внешний PostgreSQL** для высокой доступности

## 🚨 Устранение неполадок

### Проблема: "Не приходят ответы в Telegram"
- Проверьте токен бота в `.env`
- Убедитесь, что бот добавлен в контакты
- Проверьте логи оркестратора: `docker logs ai_agency_orchestrator`

### Проблема: "Ollama не отвечает"
- Проверьте загружена ли модель: `docker exec ai_agency_ollama ollama list`
- Увеличьте время ожидания в `brain.py`
- Проверьте доступность памяти

### Проблема: "SearXNG ошибки поиска"
- SearXNG может временно блокировать запросы при высокой нагрузке
- Используется резервный поиск через DuckDuckGo
- Можно отключить SearXNG, изменив `brain.py`

### Проблема: "База данных не подключена"
- Проверьте логи PostgreSQL: `docker logs ai_agency_db`
- Убедитесь, что volume не поврежден
- Проверьте credentials в `.env`

## 🔮 Дальнейшее развитие

### Планируемые улучшения

1. **Мультимодальность**:
   - Добавление поддержки изображений и документов
   - Интеграция с Whisper для распознавания голоса

2. **Расширение инструментов**:
   - Работа с файловой системой
   - Выполнение кода (Python, SQL)
   - Доступ к API внешних сервисов

3. **Улучшение оркестрации**:
   - Балансировка нагрузки между агентами
   - Приоритизация задач
   - Очередь с retry-механизмами

4. **Мониторинг и аналитика**:
   - Prometheus + Grafana для метрик
   - Централизованное логирование (ELK)
   - Дашборд для администратора

5. **Безопасность**:
   - Аутентификация пользователей
   - Шифрование данных
   - Аудит действий

### Миграция на сервер

Проект разработан для развертывания на сервере с:
- **Текущая среда**: Ноутбук (Ryzen 3 4300U, 8 ГБ RAM)
- **Целевая среда**: Сервер до 64 ГБ RAM, 16 CPU, 150 ГБ NVMe

**Шаги миграции**:
1. Перенести всю папку проекта на сервер
2. Установить Docker и Docker Compose
3. Настроить `.env` для production
4. Запустить `docker-compose up -d`
5. Настроить reverse proxy (nginx) с SSL
6. Настроить автоматические обновления и бэкапы

## 📄 Лицензия

Проект распространяется под лицензией MIT. Используйте свободно для коммерческих и некоммерческих проектов.

## 🤝 Вклад в проект

Приветствуются pull requests и issues. Перед внесением изменений:
1. Обсудите предлагаемые изменения в issue
2. Следуйте существующему стилю кода
3. Добавьте тесты для новой функциональности
4. Обновите документацию

## 📞 Поддержка

Для вопросов и предложений:
- Создайте issue в репозитории
- Опишите проблему с логами и шагами воспроизведения
- Укажите версии Docker и ОС

---

**Статус проекта**: Production-ready 🚀

Система успешно обрабатывает запросы пользователей, масштабируется и готова к промышленному использованию. Docker-оркестрация обеспечивает надежность и простоту развертывания в любой среде.




docker-compose build --no-cache agent-scout agent-analyst agent-writer agent-critic

docker-compose up -d agent-scout agent-analyst agent-writer agent-critic

$ docker-compose logs -f agent-scout