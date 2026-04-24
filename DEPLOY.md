# Деплой AI Agents Ensemble на VPS

Это руководство описывает процесс развёртывания проекта на VPS с настройкой взаимодействия через веб-интерфейс и Telegram.

## 1. Требования к VPS

- **ОС**: Ubuntu 22.04 LTS (рекомендуется) или 24.04 LTS.
- **Ресурсы**: минимум 2 ядра CPU, 4 ГБ RAM, 30 ГБ SSD.
- **Доступ**: SSH с правами root или пользователя с sudo.

## 2. Подготовка VPS

### 2.1. Обновление системы

```bash
sudo apt update && sudo apt upgrade -y
```

### 2.2. Установка Docker

```bash
# Установка зависимостей
sudo apt install -y apt-transport-https ca-certificates curl software-properties-common

# Добавление официального GPG ключа Docker
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

# Добавление репозитория Docker
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Установка Docker Engine
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Проверка установки
sudo docker --version
```

### 2.3. Установка Docker Compose (если не установлен через плагин)

```bash
sudo apt install -y docker-compose-plugin
# или
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
```

### 2.4. Настройка firewall (опционально)

```bash
sudo ufw allow 22/tcp   # SSH
sudo ufw allow 80/tcp   # HTTP
sudo ufw allow 443/tcp  # HTTPS
sudo ufw allow 8080/tcp # Веб-интерфейс оркестратора (временно)
sudo ufw enable
```

## 3. Копирование проекта на сервер

### 3.1. Клонирование репозитория

```bash
cd /opt
sudo git clone <URL_вашего_репозитория> ai-agents
cd ai-agents
```

Если репозиторий приватный, используйте SSH-ключ или токен.

### 3.2. Настройка прав

```bash
sudo chown -R $USER:$USER .
```

## 4. Настройка переменных окружения

Создайте файл `.env` в корне проекта на основе примера:

```bash
cp .env.example .env
nano .env
```

Заполните следующие обязательные переменные:

```env
# База данных
DB_USER=user
DB_PASSWORD=strong_password
DB_NAME=agency_db

# RabbitMQ
RABBITMQ_USER=guest
RABBITMQ_PASSWORD=guest

# Telegram Bot
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here

# LLM провайдер (выберите один)
LLM_PROVIDER=ollama  # или openrouter
OPENROUTER_API_KEY=your_key_if_using_openrouter

# Дополнительные настройки
SERVER_PORT=8080
```

**Примечание**: Для получения токена Telegram бота обратитесь к [@BotFather](https://t.me/BotFather).

## 5. Запуск проекта через Docker Compose

### 5.1. Запуск всех сервисов

```bash
docker-compose up -d
```

Это запустит:
- PostgreSQL (порт 5432)
- RabbitMQ (порты 5672, 15672)
- SearXNG (порт 8081)
- Ollama (порт 11434, если используется)
- Агенты (scout, analyst, writer, critic)
- Оркестратор (порт 8080)

### 5.2. Проверка статуса

```bash
docker-compose ps
docker-compose logs -f orchestrator
```

### 5.3. Остановка и перезапуск

```bash
docker-compose stop
docker-compose start
docker-compose restart
```

## 6. Настройка обратного прокси (nginx) и HTTPS

### 6.1. Установка nginx

```bash
sudo apt install -y nginx
```

### 6.2. Создание конфигурации сайта

Создайте файл `/etc/nginx/sites-available/ai-agents`:

```nginx
server {
    listen 80;
    server_name your-domain.com; # Замените на ваш домен или IP

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Статические файлы (если нужно обслуживать chat_modified.html)
    location /static/ {
        alias /opt/ai-agents/static/;
    }
}
```

### 6.3. Активация сайта

```bash
sudo ln -s /etc/nginx/sites-available/ai-agents /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl restart nginx
```

## 7. Настройка домена и SSL (Let's Encrypt)

### 7.1. Установка Certbot

```bash
sudo apt install -y certbot python3-certbot-nginx
```

### 7.2. Получение SSL-сертификата

```bash
sudo certbot --nginx -d your-domain.com
```

Следуйте инструкциям. Certbot автоматически обновит конфигурацию nginx.

### 7.3. Автоматическое обновление сертификата

```bash
sudo certbot renew --dry-run
```

## 8. Настройка веб-интерфейса

### 8.1. Использование модифицированного чата

Скопируйте файл `static/chat_modified.html` в директорию, обслуживаемую nginx, или используйте прокси.

В файле `chat_modified.html` измените `API_BASE` на ваш домен:

```javascript
const API_BASE = 'https://your-domain.com'; // вместо localhost:8080
```

### 8.2. Альтернатива: встроенный веб-интерфейс

Оркестратор уже предоставляет API (`/api/message`) и WebSocket (`/ws`). Вы можете создать свой фронтенд или использовать существующий.

## 9. Настройка Telegram бота

### 9.1. Создание бота через @BotFather

1. Откройте Telegram, найдите @BotFather.
2. Отправьте `/newbot`, следуйте инструкциям.
3. Получите токен, добавьте его в `.env` как `TELEGRAM_BOT_TOKEN`.

### 9.2. Проверка работы бота

Отправьте боту сообщение в Telegram. Он должен ответить "Ищу информацию...". Если нет, проверьте логи:

```bash
docker-compose logs -f orchestrator
```

### 9.3. Настройка вебхука (опционально)

Для продакшена рекомендуется настроить вебхук вместо long polling. Однако текущая реализация использует long polling, что работает за NAT.

## 10. Тестирование взаимодействия

### 10.1. Тест веб-интерфейса

Откройте в браузере:
- `https://your-domain.com/static/chat_modified.html`
- Введите запрос, нажмите "Отправить".
- Должно появиться уведомление о принятии запроса, а затем результат.

### 10.2. Тест Telegram

Напишите боту в Telegram любой запрос. Должен прийти ответ через несколько секунд.

### 10.3. Проверка логов

```bash
docker-compose logs --tail=50 agent-scout
docker-compose logs --tail=50 orchestrator
```

## 11. Дополнительные настройки

### 11.1. Мониторинг

- **RabbitMQ management**: `http://your-domain.com:15672` (логин/пароль из .env)
- **PgAdmin**: `http://your-domain.com:5050` (если включён в docker-compose)
- **SearXNG**: `http://your-domain.com:8081`

### 11.2. Резервное копирование

Настройте регулярное резервное копирование томов Docker:
- `postgres_data`
- `ollama_data`
- `rabbitmq_data`

### 11.3. Обновление проекта

```bash
cd /opt/ai-agents
git pull
docker-compose build --pull
docker-compose up -d
```

## 12. Устранение неполадок

### 12.1. Сервисы не запускаются

Проверьте логи:
```bash
docker-compose logs
```

Убедитесь, что все переменные окружения заданы правильно.

### 12.2. Ошибки подключения к RabbitMQ

Убедитесь, что RabbitMQ здоров:
```bash
docker-compose exec rabbitmq rabbitmq-diagnostics ping
```

### 12.3. Медленная работа LLM

Если используется Ollama, убедитесь, что модель загружена:
```bash
docker-compose exec ollama ollama list
```

При необходимости используйте более лёгкую модель или внешний провайдер.

### 12.4. Проблемы с WebSocket

Проверьте, что nginx корректно проксирует WebSocket (конфиг выше включает необходимые заголовки).

## 13. Заключение

Проект успешно развёрнут. Теперь вы можете взаимодействовать с системой через:
- **Telegram бота**
- **Веб-интерфейс** по адресу `https://your-domain.com/static/chat_modified.html`
- **Прямые API-запросы** к `https://your-domain.com/api/message`

Для дальнейшей кастомизации изучите исходный код агентов и оркестратора.