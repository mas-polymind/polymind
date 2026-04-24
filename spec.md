# Техническое описание проекта: Ансамбль AI-агентов с Docker-оркестрацией

## Содержание
1. [Обзор системы](#обзор-системы)
2. [Архитектура](#архитектура)
3. [Компоненты системы](#компоненты-системы)
4. [Потоки данных](#потоки-данных)
5. [Конфигурация](#конфигурация)
6. [Развертывание](#развертывание)
7. [Мониторинг и отладка](#мониторинг-и-отладка)
8. [Планы развития](#планы-развития)

## Обзор системы

**Ансамбль AI-агентов с Docker-оркестрацией** — это распределенная система интеллектуальных агентов, работающих совместно для создания качественного контента. Система использует микросервисную архитектуру с четким разделением ответственности между компонентами.

### Ключевые характеристики
- **Микросервисная архитектура**: 9 независимых сервисов в Docker Compose
- **Асинхронная обработка**: RabbitMQ как единая точка коммуникации
- **Итеративная доработка**: 4 специализированных агента (scout → analyst → writer → critic)
- **Поддержка multiple LLM**: Ollama (локальный) и OpenRouter (облачный) с автоматическим fallback
- **Приватный поиск**: SearXNG как основной источник информации
- **Telegram-интеграция**: полный цикл от запроса до результата через бота

### Технологический стек
| Компонент | Технология | Версия | Назначение |
|-----------|------------|--------|------------|
| Оркестратор | Go | 1.24 | Управление задачами, работа с БД, Telegram-бот |
| Агенты | Python | 3.14 | Интеллектуальная обработка контента |
| Брокер | RabbitMQ | 3.12 | Асинхронная коммуникация между сервисами |
| База данных | PostgreSQL | 15 | Хранение задач и контекста |
| LLM сервис | Ollama | latest | Локальные LLM модели (Qwen 2.5 1.5B) |
| Поиск | SearXNG | latest | Приватная мета-поисковая система |
| Резервный поиск | DuckDuckGo | - | Fallback поисковый механизм |
| Управление БД | pgAdmin | latest | Веб-интерфейс для PostgreSQL |

## Архитектура

### Архитектурная диаграмма

```mermaid
graph TB
    User[Пользователь Telegram] --> TelegramBot[Telegram Bot API]
    TelegramBot --> Orchestrator[Go-оркестратор]
    
    Orchestrator --> PostgreSQL[(PostgreSQL)]
    Orchestrator --> RabbitMQ[RabbitMQ]
    
    RabbitMQ --> Scout[Агент-разведчик]
    RabbitMQ --> Analyst[Агент-аналитик]
    RabbitMQ --> Writer[Агент-писатель]
    RabbitMQ --> Critic[Агент-критик]
    
    Scout --> SearXNG[SearXNG]
    Scout --> DuckDuckGo[DuckDuckGo]
    
    Analyst --> Ollama[Ollama LLM]
    Writer --> Ollama
    Critic --> Ollama
    
    Ollama --> Models[Qwen 2.5 1.5B]
    
    Orchestrator --> Result[Результат пользователю]
```

### Принципы проектирования

1. **Разделение ответственности**: Каждый агент выполняет строго определенную функцию
2. **Асинхронность**: Все коммуникации через RabbitMQ, нет синхронных HTTP-вызовов
3. **Итеративность**: Контент проходит через 4 этапа обработки с возможностью доработки
4. **Отказоустойчивость**: Fallback механизмы для поиска и LLM провайдеров
5. **Контейнеризация**: Полная изоляция сервисов через Docker

### Очереди RabbitMQ

| Очередь | Назначение | Отправитель | Получатель |
|---------|------------|-------------|------------|
| `tasks.scout` | Задачи для разведчика | Оркестратор | Агент-разведчик |
| `tasks.analyst` | Задачи для аналитика | Оркестратор | Агент-аналитик |
| `tasks.writer` | Задачи для писателя | Оркестратор | Агент-писатель |
| `tasks.critic` | Задачи для критика | Оркестратор | Агент-критик |
| `results` | Результаты от агентов | Все агенты | Оркестратор |

## Компоненты системы

### 1. Go-оркестратор (`orchestrator/`)

**Основные функции:**
- Прием запросов от Telegram-бота
- Создание и управление задачами в PostgreSQL
- Отправка задач в соответствующие очереди RabbitMQ
- Обработка результатов от агентов
- Управление состоянием задач (scout → analyst → writer → critic → done)

**Ключевые модули:**
- `cmd/orchestrator/main.go` (130 строк) — точка входа, инициализация компонентов
- `internal/database/` — модели GORM и работа с PostgreSQL
- `internal/broker/connection.go` (82 строк) — подключение к RabbitMQ
- `internal/telegram/bot.go` (139 строк) — интеграция с Telegram Bot API
- `internal/handlers/` — обработчики для каждого типа задач

**Особенности реализации:**
- Использование `FOR UPDATE` в PostgreSQL для предотвращения race conditions
- Автоматическое разбиение длинных сообщений Telegram (до 4096 символов)
- Транзакционная обработка смены состояний задач

### 2. Python-агенты (`agents/`)

**Архитектура агентов:**
- Базовый класс `BaseAgent` с абстрактным методом `process()`
- 4 специализированных агента, наследуемых от `BaseAgent`
- Унифицированный интерфейс через RabbitMQ

**Роли агентов:**

#### Агент-разведчик (Scout)
- **Температура**: 0.7 (творческий поиск)
- **Задача**: Поиск информации в интернете по запросу
- **Источники**: SearXNG (основной) → DuckDuckGo (резервный)
- **Выходные данные**: Суммаризированная информация с источниками

#### Агент-аналитик (Analyst)
- **Температура**: 0.3 (точный анализ)
- **Задача**: Обработка найденной информации, выделение ключевых фактов
- **Выходные данные**: Структурированный анализ с тезисами

#### Агент-писатель (Writer)
- **Температура**: 0.5 (баланс креативности и структуры)
- **Задача**: Генерация поста в блог на основе анализа
- **Выходные данные**: Черновик поста с заголовком и структурой

#### Агент-критик (Critic)
- **Температура**: 0.2 (строгая оценка)
- **Задача**: Рецензирование черновика, оценка качества
- **Критерии оценки**: Актуальность, структура, стиль, грамматика
- **Выходные данные**: Оценка (1-10) и предложения по улучшению

**Техническая реализация:**
- `brain.py` (409 строк) — основная логика обработки сообщений
- `tools/llm.py` (228 строк) — универсальный LLM клиент с retry механизмом
- `tools/search.py` — поисковые инструменты
- `roles/` — реализации конкретных агентов
- `prompts/` — системные промпты для каждой роли

### 3. Инфраструктурные сервисы

#### PostgreSQL (`db`)
- **Порт**: 5432
- **База данных**: `agency_db`
- **Модели**: `Task` с полями ID, статус, контекст (JSON), шаг, итерация
- **Индексы**: По статусу и шагу для быстрого поиска задач

#### RabbitMQ (`rabbitmq`)
- **Порт приложений**: 5672
- **Веб-интерфейс**: 15672
- **Пользователь/пароль**: настраиваемые через переменные окружения
- **Healthcheck**: автоматическая проверка доступности

#### Ollama (`ollama`)
- **Порт**: 11434
- **Модель по умолчанию**: `qwen2.5:1.5b`
- **Объем данных**: сохраняется в volume `ollama_data`
- **Поддерживаемые модели**: любые модели Ollama

#### SearXNG (`searxng`)
- **Порт**: 8081 (наружу) → 8080 (внутри)
- **Конфигурация**: `data/searxng/settings.yml`
- **Особенности**: отключен лимитер для бесперебойной работы

#### pgAdmin (`pgadmin`)
- **Порт**: 5050
- **Доступ**: веб-интерфейс для управления БД
- **Аутентификация**: настраиваемый email/пароль

## Потоки данных

### 1. Поток создания контента

```mermaid
sequenceDiagram
    participant User as Пользователь
    participant TBot as Telegram Bot
    participant Orch as Оркестратор
    participant DB as PostgreSQL
    participant RMQ as RabbitMQ
    participant Scout as Разведчик
    participant Analyst as Аналитик
    participant Writer as Писатель
    participant Critic as Критик
    participant LLM as Ollama
    
    User->>TBot: /start Запрос
    TBot->>Orch: Создание задачи
    Orch->>DB: INSERT Task (step=scout)
    Orch->>RMQ: Отправка в queue=tasks.scout
    RMQ->>Scout: Получение задачи
    
    Scout->>SearXNG: Поиск информации
    SearXNG-->>Scout: Результаты поиска
    Scout->>LLM: Суммаризация
    LLM-->>Scout: Суммаризированная информация
    Scout->>RMQ: Результат в queue=results
    
    RMQ->>Orch: Обработка результата
    Orch->>DB: UPDATE Task (step=analyst)
    Orch->>RMQ: Отправка в queue=tasks.analyst
    
    RMQ->>Analyst: Получение задачи
    Analyst->>LLM: Анализ информации
    LLM-->>Analyst: Структурированный анализ
    Analyst->>RMQ: Результат в queue=results
    
    RMQ->>Orch: Обработка результата
    Orch->>DB: UPDATE Task (step=writer)
    Orch->>RMQ: Отправка в queue=tasks.writer
    
    RMQ->>Writer: Получение задачи
    Writer->>LLM: Генерация поста
    LLM-->>Writer: Черновик поста
    Writer->>RMQ: Результат в queue=results
    
    RMQ->>Orch: Обработка результата
    Orch->>DB: UPDATE Task (step=critic)
    Orch->>RMQ: Отправка в queue=tasks.critic
    
    RMQ->>Critic: Получение задачи
    Critic->>LLM: Оценка качества
    LLM-->>Critic: Оценка и предложения
    Critic->>RMQ: Результат в queue=results
    
    RMQ->>Orch: Обработка результата
    Orch->>DB: UPDATE Task (step=done)
    Orch->>TBot: Отправка результата пользователю
    TBot->>User: Финальный пост с оценкой
```

### 2. Механизм повторных попыток (Retry)

**Для LLM-запросов:**
- Максимум 3 попытки при ошибках сети или таймаутах
- Экспоненциальная задержка между попытками (1s → 2s → 4s)
- Автоматический fallback между провайдерами (OpenRouter → Ollama)

**Для поиска:**
- SearXNG как основной источник
- DuckDuckGo как резервный при недоступности SearXNG
- Таймаут 10 секунд на поисковый запрос

### 3. Форматы сообщений

#### Задача от оркестратора к агенту:
```json
{
  "task_id": 123,
  "step": "scout",
  "context": {
    "query": "искусственный интеллект в медицине",
    "iteration": 1,
    "previous_results": {}
  }
}
```

#### Результат от агента к оркестратору:
```json
{
  "task_id": 123,
  "step": "scout",
  "result": {
    "summary": "Краткое содержание найденной информации...",
    "raw_data": "Полный текст результатов поиска...",
    "sources": ["https://example1.com", "https://example2.com"]
  },
  "success": true
}
```

## Конфигурация

### Переменные окружения

#### Основной файл `.env.docker`:
```bash
# База данных PostgreSQL
DB_USER=user
DB_PASSWORD=password
DB_NAME=agency_db

# RabbitMQ
RABBITMQ_USER=guest
RABBITMQ_PASSWORD=guest

# Telegram Bot
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here

# LLM провайдеры
LLM_PROVIDER=ollama  # или openrouter
OPENROUTER_API_KEY=your_openrouter_api_key_here
OPENROUTER_URL=https://openrouter.ai/api/v1
OPENROUTER_MODEL=meta-llama/llama-3.2-3b-instruct:free
FALLBACK_TO_OLLAMA=true

# pgAdmin
PGADMIN_EMAIL=admin@admin.com
PGADMIN_PASSWORD=admin
```

#### Агент-специфичные переменные:
- `AGENT_ROLE`: scout, analyst, writer, critic
- `MODEL_NAME`: qwen2.5:1.5b (по умолчанию)
- `TEMPERATURE`: 0.7 (scout), 0.3 (analyst), 0.5 (writer), 0.2 (critic)
- `OLLAMA_HOST`: ollama:11434
- `SEARXNG_URL`: http://searxng:8080
- `RABBITMQ_URL`: amqp://guest:guest@rabbitmq:5672/

### Конфигурация SearXNG

Файл `data/searxng/settings.yml`:
```yaml
server:
  port: 8080
  bind_address: "0.0.0.0"
  
search:
  safe_search: 0
  autocomplete: ""
  
engines:
  - name: duckduckgo
    engine: duckduckgo
    shortcut: ddg
  
ui:
  theme: simple
  results_on_new_tab: false
```

### Модель данных PostgreSQL

**Таблица `tasks`:**
```sql
CREATE TABLE tasks (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    step VARCHAR(50) NOT NULL DEFAULT 'scout',
    iteration INTEGER NOT NULL DEFAULT 1,
    context JSONB NOT NULL DEFAULT '{}'::jsonb,
    telegram_chat_id BIGINT,
    telegram_message_id INTEGER
);

CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_step ON tasks(step);
```

**JSON-структура контекста:**
```json
{
  "query": "исходный запрос пользователя",
  "scout_result": {"summary": "...", "raw_data": "...", "sources": []},
  "analyst_result": {"analysis": "...", "key_points": []},
  "writer_result": {"draft": "...", "title": "...", "structure": {}},
  "critic_result": {"score": 8, "feedback": "...", "suggestions": []},
  "final_post": "готовый пост для отправки",
  "metadata": {"total_iterations": 3, "current_iteration": 2}
}
```

## Развертывание

### Требования к системе
- **Docker**: 20.10+
- **Docker Compose**: 2.20+
- **Память**: минимум 8GB RAM (рекомендуется 16GB для Ollama)
- **Диск**: 10GB свободного места
- **Сеть**: доступ к интернету для загрузки образов

### Пошаговое развертывание

1. **Клонирование репозитория:**
```bash
git clone <repository-url>
cd agents_app
```

2. **Настройка переменных окружения:**
```bash
cp .env.docker .env
# Редактируем .env, устанавливаем TELEGRAM_BOT_TOKEN и другие параметры
```

3. **Запуск системы:**
```bash
docker-compose up -d
```

4. **Инициализация Ollama модели:**
```bash
docker exec ai_agency_ollama ollama pull qwen2.5:1.5b
```

5. **Проверка работоспособности:**
```bash
# Проверяем логи оркестратора
docker logs ai_agency_orchestrator

# Проверяем доступность RabbitMQ
curl http://localhost:15672

# Проверяем SearXNG
curl http://localhost:8081
```

6. **Взаимодействие через Telegram:**
- Найти бота по имени
- Отправить команду `/start`
- Отправить запрос для обработки

### Масштабирование

**Горизонтальное масштабирование агентов:**
```yaml
# В docker-compose.yml можно добавить
agent-scout-2:
  extends: agent-scout
  container_name: ai_agency_agent_scout_2
  
agent-analyst-2:
  extends: agent-analyst
  container_name: ai_agency_agent_analyst_2
```

**Настройки RabbitMQ для кластера:**
```yaml
rabbitmq:
  image: rabbitmq:3.12-management-alpine
  environment:
    RABBITMQ_ERLANG_COOKIE: "secret_cookie"
    RABBITMQ_NODE_NAME: "rabbit@node1"
    RABBITMQ_CLUSTER_NODE_NAME: "rabbit@node2"
```

## Мониторинг и отладка

### Логирование

**Уровни логирования:**
- **Оркестратор (Go)**: INFO, ERROR, DEBUG (через стандартный логгер)
- **Агенты (Python)**: INFO через `logging` модуль
- **RabbitMQ**: управление через веб-интерфейс (15672)

**Просмотр логов:**
```bash
# Все логи системы
docker-compose logs -f

# Логи конкретного сервиса
docker logs ai_agency_orchestrator -f
docker logs ai_agency_agent_scout -f
```

### Метрики и healthchecks

**Встроенные healthchecks:**
- **PostgreSQL**: `pg_isready` каждые 10 секунд
- **RabbitMQ**: `rabbitmq-diagnostics ping` каждые 10 секунд
- **Оркестратор**: HTTP endpoint `/health` на порту 8080

**Мониторинг через веб-интерфейсы:**
- **RabbitMQ**: http://localhost:15672 (guest/guest)
- **pgAdmin**: http://localhost:5050 (admin@admin.com/admin)
- **SearXNG**: http://localhost:8081

### Отладка проблем

#### 1. Проблемы с RabbitMQ
```bash
# Проверка подключения
docker exec ai_agency_rabbitmq rabbitmqctl list_queues

# Просмотр сообщений
docker exec ai_agency_rabbitmq rabbitmqctl list_queues name messages_ready messages_unacknowledged
```

#### 2. Проблемы с LLM
```bash
# Проверка Ollama
curl http://localhost:11434/api/tags

# Тестовый запрос к модели
curl http://localhost:11434/api/generate -d '{
  "model": "qwen2.5:1.5b",
  "prompt": "Тестовое сообщение",
  "stream": false
}'
```

#### 3. Проблемы с поиском
```bash
# Проверка SearXNG
curl "http://localhost:8081/search?q=test&format=json"

# Прямой запрос к DuckDuckGo
curl "https://api.duckduckgo.com/?q=test&format=json&pretty=1"
```

#### 4. Проблемы с базой данных
```bash
# Подключение к PostgreSQL
docker exec -it ai_agency_db psql -U user -d agency_db

# Проверка задач
SELECT id, status, step, iteration FROM tasks ORDER BY created_at DESC LIMIT 10;
```

### Восстановление после сбоев

**Автоматическое восстановление:**
- Контейнеры настроены с `restart: always` (кроме агентов с `restart: no`)
- Volume'ы сохраняют данные между перезапусками
- Механизм retry в LLM клиенте

**Ручное восстановление:**
```bash
# Полная перезагрузка системы
docker-compose down
docker-compose up -d

# Перезагрузка конкретного сервиса
docker-compose restart orchestrator
docker-compose restart agent-scout
```

## Планы развития

### Краткосрочные улучшения (1-2 месяца)

1. **Кэширование результатов поиска**
   - Redis для хранения результатов поиска
   - TTL 24 часа для актуальности данных
   - Снижение нагрузки на поисковые системы

2. **Расширение ролей агентов**
   - Агент-редактор для финальной полировки
   - Агент-иллюстратор для генерации изображений
   - Агент-проверки фактов

3. **Улучшение мониторинга**
   - Prometheus + Grafana для метрик
   - Sentry для отслеживания ошибок
   - Логирование в ELK стек

### Среднесрочные улучшения (3-6 месяцев)

1. **Поддержка multiple языков**
   - Автоматическое определение языка запроса
   - Перевод между языками
   - Культурная адаптация контента

2. **Распределенная архитектура**
   - Kubernetes вместо Docker Compose
   - Service mesh для межсервисной коммуникации
   - Геораспределение для снижения задержек

3. **Расширение источников данных**
   - Интеграция с научными базами данных
   - Подписки на RSS/Atom потоки
   - API социальных сетей для трендов

### Долгосрочное видение (6+ месяцев)

1. **Самообучающаяся система**
   - Обратная связь от пользователей для улучшения промптов
   - A/B тестирование разных подходов
   - Автоматическая оптимизация гиперпараметров

2. **Мультимодальность**
   - Генерация видео-контента
   - Обработка аудио-запросов
   - Создание интерактивных презентаций

3. **Экосистема плагинов**
   - Модульная архитектура для легкого добавления функций
   - Marketplace для сторонних агентов
   - API для интеграции с внешними системами

### Технический долг и оптимизации

1. **Производительность**
   - Оптимизация запросов к PostgreSQL
   - Пулинг соединений к RabbitMQ
   - Балансировка нагрузки между агентами

2. **Безопасность**
   - HTTPS для всех внешних endpoint'ов
   - Аутентификация между сервисами
   - Шифрование чувствительных данных

3. **Надежность**
   - Резервное копирование базы данных
   - Disaster recovery процедуры
   - Тестирование отказоустойчивости

## Заключение

Система "Ансамбль AI-агентов с Docker-оркестрацией" представляет собой современное решение для автоматизированного создания контента. Архитектура, основанная на микросервисах и асинхронной коммуникации, обеспечивает масштабируемость и отказоустойчивость.

**Ключевые преимущества:**
- Полная контейнеризация для простого развертывания
- Гибкая архитектура с возможностью добавления новых агентов
- Поддержка multiple LLM провайдеров с автоматическим fallback
- Приватный поиск через SearXNG для конфиденциальности
- Итеративный процесс создания качественного контента

Система готова к промышленному использованию и имеет четкий план развития для будущих улучшений.