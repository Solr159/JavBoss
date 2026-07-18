package models

import "time"

// AuthSession stores a hashed browser session so it can survive server restarts.
type AuthSession struct {
	TokenHash      string    `json:"-" gorm:"primaryKey;size:64"`
	SessionVersion uint64    `json:"-" gorm:"not null"`
	ExpiresAt      time.Time `json:"-" gorm:"not null;index"`
	CreatedAt      time.Time `json:"-" gorm:"not null"`
}
