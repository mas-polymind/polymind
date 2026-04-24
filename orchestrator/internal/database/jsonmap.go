package database

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSONMap для хранения контекста в PostgreSQL JSONB
type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan JSONMap: invalid type")
	}
	return json.Unmarshal(bytes, j)
}
