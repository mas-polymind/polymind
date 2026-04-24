package handlers

import (
	"encoding/json"
	"log"
	"orchestrator/internal/broker"
	"orchestrator/internal/database"
)

type AnalystHandler struct {
	broker *broker.Broker
}

func NewAnalystHandler(b *broker.Broker) *AnalystHandler {
	return &AnalystHandler{broker: b}
}

func (h *AnalystHandler) Handle(body []byte) error {
	var msg struct {
		TaskID uint `json:"task_id"`
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		log.Printf("AnalystHandler: invalid message: %v", err)
		return err
	}

	task, err := database.GetTask(msg.TaskID)
	if err != nil {
		log.Printf("AnalystHandler: task %d not found", msg.TaskID)
		return nil
	}
	if task.Step != "analyst" {
		log.Printf("AnalystHandler: task %d step is %s, skipping", task.ID, task.Step)
		return nil
	}

	// Получаем данные от scout из контекста
	scoutData, ok := task.Context["scout"]
	if !ok {
		log.Printf("AnalystHandler: task %d has no scout data", task.ID)
		return nil
	}

	// Публикуем задачу в очередь агента-analyst
	agentPayload := map[string]interface{}{
		"task_id": task.ID,
		"query":   task.Query,
		"data":    scoutData,
	}

	payloadBytes, err := json.Marshal(agentPayload)
	if err != nil {
		log.Printf("AnalystHandler: failed to marshal payload for task %d: %v", task.ID, err)
		return err
	}

	err = h.broker.Publish("agent.analyst", payloadBytes)
	if err != nil {
		log.Printf("AnalystHandler: failed to publish to agent.analyst for task %d: %v", task.ID, err)
		return err
	}

	log.Printf("AnalystHandler: task %d published to agent.analyst queue", task.ID)
	return nil
}
