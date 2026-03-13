package model

import (
	"time"

	"github.com/google/uuid"
)

// Notification represents a scheduled push notification for an event.
type Notification struct {
	ID        uuid.UUID `gorm:"type:text;primaryKey" json:"id"`
	EventID   uuid.UUID `gorm:"type:text;not null;index" json:"eventId"`
	UserID    uuid.UUID `gorm:"type:text;not null;index" json:"userId"`
	NotifyAt  time.Time `gorm:"not null;index" json:"notifyAt"`
	OffsetMin int       `gorm:"not null" json:"offsetMin"`
	Sent      bool      `gorm:"not null;default:false" json:"sent"`
	Retries   int       `gorm:"not null;default:0" json:"-"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
	Event     Event     `gorm:"foreignKey:EventID" json:"-"`
}

// DeviceToken represents a registered FCM device token for a user.
type DeviceToken struct {
	ID        uuid.UUID `gorm:"type:text;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:text;not null;index" json:"userId"`
	Token     string    `gorm:"not null;uniqueIndex" json:"token"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}
