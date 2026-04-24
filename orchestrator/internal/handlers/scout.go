package handlers

import (
	"encoding/json"
	"log"
	"orchestrator/internal/broker"
	"orchestrator/internal/database"
)

type ScoutHandler struct {
	broker *broker.Broker
}

func NewScoutHandler(b *broker.Broker) *ScoutHandler {
	return &ScoutHandler{broker: b}
}

func (h *ScoutHandler) Handle(body []byte) error {
	var msg struct {
		TaskID uint `json:"task_id"`
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		log.Printf("ScoutHandler: invalid message: %v", err)
		return err
	}

	task, err := database.GetTask(msg.TaskID)
	if err != nil {
		log.Printf("ScoutHandler: task %d not found", msg.TaskID)
		return nil
	}
	if task.Step != "scout" {
		log.Printf("ScoutHandler: task %d step is %s, skipping", task.ID, task.Step)
		return nil
	}

	// Публикуем задачу в очередь агента-scout (не вызываем HTTP!)
	agentPayload := map[string]interface{}{
		"task_id": task.ID,
		"query":   task.Query,
	}

	payloadBytes, err := json.Marshal(agentPayload)
	if err != nil {
		log.Printf("ScoutHandler: failed to marshal payload for task %d: %v", task.ID, err)
		return err
	}

	err = h.broker.Publish("agent.scout", payloadBytes)
	if err != nil {
		log.Printf("ScoutHandler: failed to publish to agent.scout for task %d: %v", task.ID, err)
		return err
	}

	log.Printf("ScoutHandler: task %d published to agent.scout queue", task.ID)
	return nil
}
