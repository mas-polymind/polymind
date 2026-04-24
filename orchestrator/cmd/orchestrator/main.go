package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"orchestrator/internal/broker"
	"orchestrator/internal/database"
	"orchestrator/internal/handlers"
	"orchestrator/internal/telegram"
	"orchestrator/internal/web"
)

func main() {
	// 1. Подключение к БД
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
			getEnv("DB_HOST", "db"),
			getEnv("DB_USER", "user"),
			getEnv("DB_PASSWORD", "password"),
			getEnv("DB_NAME", "agency_db"),
			getEnv("DB_PORT", "5432"),
		)
		log.Printf("DB_DSN not set, constructed from env vars: host=%s, port=%s, user=%s, dbname=%s",
			getEnv("DB_HOST", "db"), getEnv("DB_PORT", "5432"),
			getEnv("DB_USER", "user"), getEnv("DB_NAME", "agency_db"))
	}

	if err := database.Init(dsn); err != nil {
		log.Fatal("DB init:", err)
	}
	log.Println("DB connected")

	// 2. Подключение к RabbitMQ с retry
	rabbitMQURL := os.Getenv("RABBITMQ_URL")
	if rabbitMQURL == "" {
		rabbitMQURL = "amqp://guest:guest@rabbitmq:5672/"
	}

	// Подключаемся с ретраями
	b, err := connectRabbitMQ(rabbitMQURL, 10, 5*time.Second)
	if err != nil {
		log.Fatal("RabbitMQ init:", err)
	}
	defer b.Close()
	log.Println("RabbitMQ connected")

	// 3. Объявляем очереди
	queues := []string{
		"task.scout",
		"task.analyst",
		"task.writer",
		"task.critic",
		"task.result",
		"agent.scout",
		"agent.analyst",
		"agent.writer",
		"agent.critic",
	}
	for _, q := range queues {
		if _, err := b.DeclareQueue(q); err != nil {
			log.Fatal("Declare queue:", err)
		}
	}
	log.Println("Queues declared")

	// 4. Telegram бот
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN not set")
	}

	bot, err := telegram.NewBot(token, b)
	if err != nil {
		log.Fatal("Telegram bot init:", err)
	}
	bot.Start()
	log.Println("Telegram bot started")

	// 5. Веб-сервер
	webServer := web.NewServer(b)
	go func() {
		if err := webServer.Start(":8080"); err != nil {
			log.Printf("Web server error: %v", err)
		}
	}()
	log.Println("Web server started on :8080")

	// 6. Создаём обработчики
	scoutHandler := handlers.NewScoutHandler(b)
	analystHandler := handlers.NewAnalystHandler(b)
	writerHandler := handlers.NewWriterHandler(b)
	criticHandler := handlers.NewCriticHandler(b)
	resultHandler := handlers.NewResultHandler(b, bot, webServer) // ← ДОБАВИТЬ ЭТУ СТРОКУ

	// 6. Запускаем consumer'ов (каждый в своей горутине)
	go b.Consume("task.scout", scoutHandler.Handle)
	go b.Consume("task.analyst", analystHandler.Handle)
	go b.Consume("task.writer", writerHandler.Handle)
	go b.Consume("task.critic", criticHandler.Handle)
	go b.Consume("task.result", resultHandler.Handle) // ← ДОБАВИТЬ ЭТУ СТРОКУ

	log.Println("Orchestrator is running...")

	// Бесконечное ожидание
	select {}
}

// connectRabbitMQ подключается к RabbitMQ с повторными попытками
func connectRabbitMQ(url string, maxRetries int, retryDelay time.Duration) (*broker.Broker, error) {
	var b *broker.Broker
	var err error

	for i := 0; i < maxRetries; i++ {
		b, err = broker.NewBroker(url)
		if err == nil {
			log.Printf("Successfully connected to RabbitMQ on attempt %d", i+1)
			return b, nil
		}

		log.Printf("RabbitMQ connection attempt %d/%d failed: %v", i+1, maxRetries, err)
		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return nil, fmt.Errorf("failed to connect to RabbitMQ after %d attempts: %w", maxRetries, err)
}

// getEnv получает переменную окружения или возвращает значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
