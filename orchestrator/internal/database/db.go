package database

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init(dsn string) error {
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}
	return DB.AutoMigrate(&Task{})
}

// GetTask возвращает задачу по ID
func GetTask(id uint) (*Task, error) {
	var task Task
	err := DB.First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetTaskForUpdate блокирует строку задачи для обновления
func GetTaskForUpdate(id uint) (*Task, *gorm.DB, error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, nil, tx.Error
	}
	var task Task
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&task, id).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	return &task, tx, nil
}

// UpdateTaskWithTx обновляет задачу в рамках транзакции
func UpdateTaskWithTx(tx *gorm.DB, task *Task) error {
	return tx.Save(task).Error
}

// UpdateTask сохраняет задачу (без транзакции)
func UpdateTask(task *Task) error {
	return DB.Save(task).Error
}

// CreateTask создает новую задачу
func CreateTask(telegramID int64, query string) (*Task, error) {
	task := &Task{
		TelegramID: telegramID,
		Query:      query,
		Status:     "pending",
		Step:       "scout",
		Context:    make(JSONMap),
	}
	err := DB.Create(task).Error
	return task, err
}

// AdvanceTask пытается перевести задачу из ожидаемого шага в следующий
func AdvanceTask(taskID uint, expectedStep, nextStep string, updateContext func(JSONMap) JSONMap) (bool, error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return false, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var task Task
	err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ? AND step = ?", taskID, expectedStep).First(&task).Error
	if err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}

	if task.Context == nil {
		task.Context = make(JSONMap)
	}
	task.Context = updateContext(task.Context)
	task.Step = nextStep
	if nextStep == "done" {
		task.Status = "completed"
	}

	if err := tx.Save(&task).Error; err != nil {
		tx.Rollback()
		return false, err
	}
	tx.Commit()
	return true, nil
}
