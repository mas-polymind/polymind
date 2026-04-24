# Многостадийная сборка для Go-оркестратора
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Копируем файлы зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код (только необходимые директории)
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Собираем бинарник, указывая путь к main.go
RUN go build -o orchestrator ./cmd/orchestrator

# Финальный образ на основе Alpine
FROM alpine:3.19

# Устанавливаем CA-сертификаты для HTTPS-запросов
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Копируем бинарник из стадии сборки
COPY --from=builder /app/orchestrator .

# Порт для HTTP-сервера (колбэки от агентов)
EXPOSE 8080

# Запускаем приложение
CMD ["./orchestrator"]