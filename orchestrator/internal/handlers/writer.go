package handlers

import (
	"encoding/json"
	"log"
	"orchestrator/internal/broker"
	"orchestrator/internal/database"
)

type WriterHandler struct {
	broker *broker.Broker
}

func NewWriterHandler(b *broker.Broker) *WriterHandler {
	return &WriterHandler{broker: b}
}

func (h *WriterHandler) Handle(body []byte) error {
	var msg struct {
		TaskID uint `json:"task_id"`
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		log.Printf("WriterHandler: invalid message: %v", err)
		return err
	}

	task, err := database.GetTask(msg.TaskID)
	if err != nil {
		log.Printf("WriterHandler: task %d not found", msg.TaskID)
		return nil
	}
	if task.Step != "writer" {
		log.Printf("WriterHandler: task %d step is %s, skipping", task.ID, task.Step)
		return nil
	}

	// Получаем анализ из контекста
	analysis, ok := task.Context["analysis"]
	if !ok {
		// Попробуем найти анализ в других местах контекста
		if scoutData, ok := task.Context["scout"]; ok {
			if scoutMap, ok := scoutData.(map[string]interface{}); ok {
				if summary, ok := scoutMap["summary"]; ok {
					analysis = summary
					log.Printf("WriterHandler: task %d using scout summary as analysis", task.ID)
				}
			}
		}
		
		if analysis == nil {
			log.Printf("WriterHandler: task %d has no analysis data, using query as fallback", task.ID)
			analysis = task.Query
		}
	}

	// Подготовка payload для агента
	agentPayload := map[string]interface{}{
		"task_id":  task.ID,
		"topic":    task.Query,
		"analysis": analysis,
	}

	// Если есть рецензия (доработка)
	if review, ok := task.Context["review"]; ok {
		agentPayload["review"] = review
	}

	// Если есть предыдущий черновик
	if draft, ok := task.Context["draft"]; ok {
		agentPayload["draft"] = draft
	}

	// Добавляем информацию об итерации
	if iteration, ok := task.Context["iteration"]; ok {
		agentPayload["iteration"] = iteration
		log.Printf("WriterHandler: task %d iteration %v", task.ID, iteration)
	} else {
		agentPayload["iteration"] = 0
		log.Printf("WriterHandler: task %d first iteration", task.ID)
	}

	// Добавляем предыдущую оценку, если есть
	if score, ok := task.Context["previous_score"]; ok {
		agentPayload["previous_score"] = score
	}

	payloadBytes, err := json.Marshal(agentPayload)
	if err != nil {
		log.Printf("WriterHandler: failed to marshal payload for task %d: %v", task.ID, err)
		return err
	}

	err = h.broker.Publish("agent.writer", payloadBytes)
	if err != nil {
		log.Printf("WriterHandler: failed to publish to agent.writer for task %d: %v", task.ID, err)
		return err
	}

	log.Printf("WriterHandler: task %d published to agent.writer queue (iteration: %v)",
		task.ID, agentPayload["iteration"])
	return nil
}
