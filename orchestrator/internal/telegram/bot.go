package telegram

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
	"orchestrator/internal/broker"
	"orchestrator/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api    *tgbotapi.BotAPI
	broker *broker.Broker
}

func NewBot(token string, b *broker.Broker) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Bot{api: api, broker: b}, nil
}

// splitText разбивает текст на части максимальной длины maxLen, стараясь не разрывать слова.
func splitText(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	start := 0

	for start < len(text) {
		end := start + maxLen
		if end > len(text) {
			end = len(text)
		} else {
			// Попробуем найти границу предложения или слова
			// Ищем последний символ новой строки, точку, пробел или запятую
			searchEnd := end
			if searchEnd > len(text) {
				searchEnd = len(text)
			}
			// Ищем назад от end до start
			for i := searchEnd - 1; i > start && i > start+maxLen*3/4; i-- {
				if text[i] == '\n' || text[i] == '.' || text[i] == '!' || text[i] == '?' || text[i] == ' ' || text[i] == ',' {
					end = i + 1
					break
				}
			}
		}

		chunks = append(chunks, text[start:end])
		start = end
	}

	return chunks
}

// SendMessage отправляет текстовое сообщение пользователю.
// Если текст превышает 4096 символов, он автоматически разбивается на части.
func (b *Bot) SendMessage(chatID int64, text string) error {
	const telegramMaxLength = 4096

	if len(text) <= telegramMaxLength {
		msg := tgbotapi.NewMessage(chatID, text)
		_, err := b.api.Send(msg)
		return err
	}

	// Разбиваем текст на части
	chunks := splitText(text, telegramMaxLength)
	log.Printf("SendMessage: splitting %d chars into %d chunks", len(text), len(chunks))

	var firstErr error
	for i, chunk := range chunks {
		// Добавляем индикатор прогресса (часть X из Y)
		prefix := ""
		if len(chunks) > 1 {
			prefix = fmt.Sprintf("[%d/%d]\n", i+1, len(chunks))
		}
		fullChunk := prefix + chunk

		msg := tgbotapi.NewMessage(chatID, fullChunk)
		_, err := b.api.Send(msg)
		if err != nil {
			log.Printf("SendMessage: failed to send chunk %d/%d: %v", i+1, len(chunks), err)
			if firstErr == nil {
				firstErr = err
			}
		} else {
			log.Printf("SendMessage: chunk %d/%d sent successfully (%d chars)", i+1, len(chunks), len(fullChunk))
		}

		// Небольшая задержка между сообщениями, чтобы не превысить лимиты Telegram
		if i < len(chunks)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return firstErr
}

// Start запускает прослушивание обновлений Telegram.
func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	go func() {
		for update := range updates {
			if update.Message == nil {
				continue
			}
			chatID := update.Message.Chat.ID
			text := update.Message.Text

			// Создаём задачу в БД
			task, err := database.CreateTask(chatID, text)
			if err != nil {
				b.SendMessage(chatID, "❌ Ошибка создания задачи")
				log.Printf("CreateTask error: %v", err)
				continue
			}
			b.SendMessage(chatID, "🔍 Ищу информацию... (шаг 1/4)")

			// Публикуем в очередь task.scout
			msgBytes, _ := json.Marshal(map[string]uint{"task_id": task.ID})
			if err := b.broker.Publish("task.scout", msgBytes); err != nil {
				log.Printf("Failed to publish scout task %d: %v", task.ID, err)
				b.SendMessage(chatID, "❌ Ошибка отправки задачи агенту")
			}
		}
	}()
}
