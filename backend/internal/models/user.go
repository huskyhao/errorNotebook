package models

import "time"

// User is deliberately small for the anonymous-first MVP. A future account
// upgrade can attach an email/provider identity without changing ownership
// columns on the business tables.
type User struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Kind      string    `gorm:"column:kind;type:varchar(32);not null;default:'anonymous'" json:"kind"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (User) TableName() string { return "users" }

type UserSession struct {
	ID         int64     `gorm:"primaryKey" json:"-"`
	UserID     int64     `gorm:"column:user_id;not null;index" json:"userId"`
	TokenHash  string    `gorm:"column:token_hash;type:char(64);uniqueIndex;not null" json:"-"`
	ExpiresAt  time.Time `gorm:"column:expires_at;index;not null" json:"-"`
	LastSeenAt time.Time `gorm:"column:last_seen_at;not null" json:"-"`
	CreatedAt  time.Time `json:"-"`
}

func (UserSession) TableName() string { return "user_sessions" }
