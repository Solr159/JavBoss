package models

import "time"

// AuthAccount stores the single local administrator credential.
type AuthAccount struct {
	ID             uint      `json:"-" gorm:"primaryKey"`
	PasswordHash   string    `json:"-" gorm:"type:text;not null"`
	SessionVersion uint64    `json:"-" gorm:"not null;default:1"`
	CreatedAt      time.Time `json:"-" gorm:"not null"`
	UpdatedAt      time.Time `json:"-" gorm:"not null"`
}
