package model

import (
	"time"

	"github.com/google/uuid"
)

// User represents a registered user authenticated via OAuth provider.
type User struct {
	ID                 uuid.UUID  `gorm:"type:text;primaryKey" json:"id"`
	Provider           string     `gorm:"not null;uniqueIndex:idx_provider_sub" json:"provider"`
	ProviderSub        string     `gorm:"not null;uniqueIndex:idx_provider_sub" json:"providerSub"`
	Email              *string    `json:"email,omitempty"`
	Nickname           *string    `json:"nickname,omitempty"`
	ProfileImage       *string    `json:"profileImage,omitempty"`
	SubscriptionStatus        string     `gorm:"not null;default:'free'" json:"subscriptionStatus"`
	SubscriptionExpiry        *time.Time `json:"subscriptionExpiry,omitempty"`
	OriginalTransactionID     *string    `gorm:"uniqueIndex" json:"-"`
	VoiceParseCount    int        `gorm:"not null;default:0" json:"-"`
	VoiceParseDate     *time.Time `json:"-"`
	CreatedAt          time.Time  `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt          time.Time  `gorm:"autoUpdateTime" json:"updatedAt"`
	LastLoginAt        *time.Time `json:"lastLoginAt,omitempty"`
}

// IsPremium returns true if the user has an active premium subscription.
// Subscription status is managed by Apple Server Notifications webhook,
// not by checking expiry time locally.
func (u *User) IsPremium() bool {
	return u.SubscriptionStatus == "premium"
}

// RefreshToken represents a stored refresh token for JWT rotation.
type RefreshToken struct {
	ID        uuid.UUID `gorm:"type:text;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:text;not null;index" json:"userId"`
	Token     string    `gorm:"not null;uniqueIndex" json:"-"`
	ExpiresAt time.Time `gorm:"not null" json:"expiresAt"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
	User      User      `gorm:"foreignKey:UserID" json:"-"`
}
