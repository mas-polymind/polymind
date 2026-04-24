package database

import (
	"time"
)

// Task — расширенная модель задачи
type Task struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Query      string    `gorm:"type:text" json:"query"`
	Response   string    `gorm:"type:text" json:"response"`
	Status     string    `gorm:"default:pending" json:"status"`
	TelegramID int64     `json:"telegram_id"`
	Step       string    `gorm:"default:scout" json:"step"`
	Context    JSONMap   `gorm:"type:jsonb" json:"context"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
