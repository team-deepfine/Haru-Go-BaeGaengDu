package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Int64Array is a JSON-serialized []int64 that works with both PostgreSQL and SQLite.
type Int64Array []int64

// Value implements the driver.Valuer interface.
func (a Int64Array) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("marshal Int64Array: %w", err)
	}
	return string(b), nil
}

// Scan implements the sql.Scanner interface.
func (a *Int64Array) Scan(value interface{}) error {
	if value == nil {
		*a = Int64Array{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("unsupported type for Int64Array: %T", value)
	}
	return json.Unmarshal(bytes, a)
}

// Event represents a calendar event.
type Event struct {
	ID              uuid.UUID  `gorm:"type:text;primaryKey" json:"id"`
	Title           string     `gorm:"not null" json:"title"`
	StartAt         time.Time  `gorm:"not null;index" json:"startAt"`
	EndAt           time.Time  `gorm:"not null;index" json:"endAt"`
	AllDay          bool       `gorm:"not null;default:false" json:"allDay"`
	Timezone        string     `gorm:"not null;default:'UTC'" json:"timezone"`
	LocationName    *string    `json:"locationName,omitempty"`
	LocationAddress *string    `json:"locationAddress,omitempty"`
	LocationLat     *float64   `json:"locationLat,omitempty"`
	LocationLng     *float64   `json:"locationLng,omitempty"`
	ReminderOffsets Int64Array `gorm:"type:text;not null;default:'[]'" json:"reminderOffsets"`
	Notes           *string    `json:"notes,omitempty"`
	CreatedAt       time.Time  `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime" json:"updatedAt"`
}

// BeforeCreate generates a UUID v7 before inserting a new event.
func (e *Event) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.Must(uuid.NewV7())
	}
	return nil
}
