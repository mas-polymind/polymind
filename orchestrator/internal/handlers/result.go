package handlers

import (
	"encoding/json"
	"log"
	"orchestrator/internal/broker"
	"orchestrator/internal/database"
	"orchestrator/internal/telegram"
	"orchestrator/internal/web"
	"strconv"
)

const (
	MaxIterations = 3
	ScoreAccept   = 9
)

type ResultHandler struct {
	broker    *broker.Broker
	bot       *telegram.Bot
	webServer *web.Server
}

func NewResultHandler(b *broker.Broker, bot *telegram.Bot, webServer *web.Server) *ResultHandler {
	return &ResultHandler{broker: b, bot: bot, webServer: webServer}
}

// sendNotification отправляет сообщение в соответствующий канал (Telegram или веб).
func (h *ResultHandler) sendNotification(task *database.Task, message string) {
	if task.TelegramID == 0 {
		// Веб-пользователь
		if h.webServer != nil {
			h.webServer.SendToTask(task.ID, message)
		}
	} else {
		// Telegram пользователь
		if h.bot != nil {
			if err := h.bot.SendMessage(task.TelegramID, message); err != nil {
				log.Printf("Failed to send Telegram message for task %d: %v", task.ID, err)
			}
		}
	}
}

func (h *ResultHandler) Handle(body []byte) error {
	var msg struct {
		TaskID  uint                   `json:"task_id"`
		Step    string                 `json:"step"`
		Result  map[string]interface{} `json:"result"`
		Success bool                   `json:"success"`
	}

	if err := json.Unmarshal(body, &msg); err != nil {
		log.Printf("ResultHandler: invalid message: %v", err)
		return err
	}

	log.Printf("ResultHandler: received result for task %d from %s (success=%v)", msg.TaskID, msg.Step, msg.Success)

	if !msg.Success {
		log.Printf("ResultHandler: task %d failed in step %s: %v", msg.TaskID, msg.Step, msg.Result)
		return nil
	}

	task, err := database.GetTask(msg.TaskID)
	if err != nil {
		log.Printf("ResultHandler: task %d not found", msg.TaskID)
		return nil
	}

	switch msg.Step {
	case "scout":
		updated, err := database.AdvanceTask(msg.TaskID, "scout", "analyst", func(ctx database.JSONMap) database.JSONMap {
			ctx["scout"] = msg.Result
			return ctx
		})
		if err != nil || !updated {
			log.Printf("ResultHandler: failed to advance task %d from scout", msg.TaskID)
			return err
		}
		h.sendNotification(task, "Нашел информацию, анализирую ...")
		nextMsg, _ := json.Marshal(map[string]uint{"task_id": msg.TaskID})
		return h.broker.Publish("task.analyst", nextMsg)

	case "analyst":
		analysis, ok := msg.Result["analysis"]
		if !ok {
			log.Printf("ResultHandler: task %d has no analysis in result", msg.TaskID)
			return nil
		}
		updated, err := database.AdvanceTask(msg.TaskID, "analyst", "writer", func(ctx database.JSONMap) database.JSONMap {
			ctx["analysis"] = analysis
			return ctx
		})
		if err != nil || !updated {
			log.Printf("ResultHandler: failed to advance task %d from analyst", msg.TaskID)
			return err
		}
		h.sendNotification(task, "Пишу пост ...")
		nextMsg, _ := json.Marshal(map[string]uint{"task_id": msg.TaskID})
		return h.broker.Publish("task.writer", nextMsg)

	case "writer":
		draft, ok := msg.Result["draft"].(string)
		if !ok || draft == "" {
			log.Printf("ResultHandler: task %d has invalid draft", msg.TaskID)
			return nil
		}

		updated, err := database.AdvanceTask(msg.TaskID, "writer", "critic", func(ctx database.JSONMap) database.JSONMap {
			ctx["draft"] = draft
			if _, ok := ctx["iteration"]; !ok {
				ctx["iteration"] = 0
			}
			return ctx
		})
		if err != nil || !updated {
			log.Printf("ResultHandler: failed to advance task %d from writer", msg.TaskID)
			return err
		}
		h.sendNotification(task, "Проверяю написанный пост...")
		nextMsg, _ := json.Marshal(map[string]uint{"task_id": msg.TaskID})
		return h.broker.Publish("task.critic", nextMsg)

	case "critic":
		var score float64 = 5.0
		switch v := msg.Result["score"].(type) {
		case float64:
			score = v
		case int:
			score = float64(v)
		case string:
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				score = parsed
			}
		}

		review, _ := msg.Result["review"].(string)

		// Calculate iteration based on score_history length
		iter := 0
		if scores, ok := task.Context["score_history"]; ok {
			if scoreList, ok := scores.([]interface{}); ok {
				iter = len(scoreList)
			}
		}
		newIter := iter + 1

		log.Printf("ResultHandler: critic score %.1f/10, iteration %d/%d for task %d (score_history length: %d)",
			score, newIter, MaxIterations, msg.TaskID, iter)

		if score >= ScoreAccept || newIter >= MaxIterations {
			// Finalize task
			updated, err := database.AdvanceTask(msg.TaskID, "critic", "done", func(ctx database.JSONMap) database.JSONMap {
				ctx["final_draft"] = task.Context["draft"]
				ctx["final_score"] = score
				ctx["iteration"] = newIter
				ctx["review"] = review
				// Store score history
				if scores, ok := ctx["score_history"]; ok {
					if scoreList, ok := scores.([]interface{}); ok {
						ctx["score_history"] = append(scoreList, score)
					} else {
						ctx["score_history"] = []interface{}{score}
					}
				} else {
					ctx["score_history"] = []interface{}{score}
				}
				return ctx
			})
			if err != nil || !updated {
				log.Printf("ResultHandler: failed to complete task %d", msg.TaskID)
				return err
			}
			draft, ok := task.Context["draft"].(string)
			if !ok || draft == "" {
				log.Printf("ResultHandler: task %d has no valid draft in context", msg.TaskID)
				draft = "Пост готов, но черновик не найден."
			}
			log.Printf("ResultHandler: task %d completed with score %.1f after %d iterations",
				msg.TaskID, score, newIter)
			h.sendNotification(task, "\n"+draft)
		} else {
			// Return to writer for another iteration
			// Validate that analysis exists before returning to writer
			if _, ok := task.Context["analysis"]; !ok {
				log.Printf("ResultHandler: task %d has no analysis, cannot return to writer", msg.TaskID)
				// Try to use scout data as fallback
				if scoutData, ok := task.Context["scout"]; ok {
					log.Printf("ResultHandler: using scout data as analysis fallback for task %d", msg.TaskID)
					task.Context["analysis"] = scoutData
				} else {
					log.Printf("ResultHandler: no analysis or scout data for task %d, marking as failed", msg.TaskID)
					h.sendNotification(task, "❌ Ошибка: потеряны исходные данные для доработки")
					return nil
				}
			}

			updated, err := database.AdvanceTask(msg.TaskID, "critic", "writer", func(ctx database.JSONMap) database.JSONMap {
				ctx["iteration"] = newIter
				ctx["review"] = review
				ctx["previous_score"] = score
				// Store score history
				if scores, ok := ctx["score_history"]; ok {
					if scoreList, ok := scores.([]interface{}); ok {
						ctx["score_history"] = append(scoreList, score)
					} else {
						ctx["score_history"] = []interface{}{score}
					}
				} else {
					ctx["score_history"] = []interface{}{score}
				}
				return ctx
			})
			if err != nil || !updated {
				log.Printf("ResultHandler: failed to return task %d to writer", msg.TaskID)
				return err
			}
			log.Printf("ResultHandler: task %d returning to writer for iteration %d (score: %.1f)",
				msg.TaskID, newIter, score)
			h.sendNotification(task, "✍️ Дорабатываю пост ...")
			nextMsg, _ := json.Marshal(map[string]uint{"task_id": msg.TaskID})
			return h.broker.Publish("task.writer", nextMsg)
		}
	}

	return nil
}
