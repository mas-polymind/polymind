# Python-агент на основе uv для быстрой установки зависимостей
FROM python:3.12-slim

WORKDIR /app

# Устанавливаем uv (современный менеджер пакетов Python)
RUN pip install --no-cache-dir uv

# Копируем файлы зависимостей
COPY pyproject.toml uv.lock ./

# Устанавливаем все зависимости из pyproject.toml в систему (глобально)
RUN uv pip install --system -r pyproject.toml

# Копируем исходный код
COPY brain.py .
COPY tools/ ./tools/
COPY roles/ ./roles/
COPY prompts/ ./prompts/

# Экспортируем порт для FastAPI (если нужен HTTP, иначе можно убрать)
EXPOSE 5000

# Запускаем приложение
CMD ["python", "brain.py"]