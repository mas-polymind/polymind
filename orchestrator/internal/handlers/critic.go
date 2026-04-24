package handlers

import (
	"encoding/json"
	"log"
	"orchestrator/internal/broker"
	"orchestrator/internal/database"
)

type CriticHandler struct {
	broker *broker.Broker
}

func NewCriticHandler(b *broker.Broker) *CriticHandler {
	return &CriticHandler{broker: b}
}

func (h *CriticHandler) Handle(body []byte) error {
	var msg struct {
		TaskID uint `json:"task_id"`
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		log.Printf("CriticHandler: invalid message: %v", err)
		return err
	}

	task, err := database.GetTask(msg.TaskID)
	if err != nil {
		log.Printf("CriticHandler: task %d not found", msg.TaskID)
		return nil
	}
	if task.Step != "critic" {
		log.Printf("CriticHandler: task %d step is %s, skipping", task.ID, task.Step)
		return nil
	}

	// Получаем черновик из контекста
	draft, ok := task.Context["draft"].(string)
	if !ok || draft == "" {
		log.Printf("CriticHandler: task %d has no draft", task.ID)
		return nil
	}

	// Публикуем задачу в очередь агента-critic
	agentPayload := map[string]interface{}{
		"task_id": task.ID,
		"topic":   task.Query,
		"draft":   draft,
	}

	payloadBytes, err := json.Marshal(agentPayload)
	if err != nil {
		log.Printf("CriticHandler: failed to marshal payload for task %d: %v", task.ID, err)
		return err
	}

	err = h.broker.Publish("agent.critic", payloadBytes)
	if err != nil {
		log.Printf("CriticHandler: failed to publish to agent.critic for task %d: %v", task.ID, err)
		return err
	}

	log.Printf("CriticHandler: task %d published to agent.critic queue", task.ID)
	return nil
}
