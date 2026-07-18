package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"javboss/internal/common"
	"javboss/internal/models"

	"gorm.io/gorm"
)

const authAccountID = 1

// GetAuthAccount returns the single local administrator account.
func GetAuthAccount(ctx context.Context) (models.AuthAccount, error) {
	var account models.AuthAccount
	if err := common.DB.WithContext(ctx).First(&account, authAccountID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.AuthAccount{}, fmt.Errorf("auth account not found: %w", err)
		}
		return models.AuthAccount{}, fmt.Errorf("get auth account: %w", err)
	}
	return account, nil
}

// ListActiveAuthSessions removes stale sessions and returns sessions for the current credential version.
func ListActiveAuthSessions(ctx context.Context, sessionVersion uint64, now time.Time) ([]models.AuthSession, error) {
	var sessions []models.AuthSession
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at <= ? OR session_version <> ?", now, sessionVersion).Delete(&models.AuthSession{}).Error; err != nil {
			return err
		}
		return tx.Where("session_version = ? AND expires_at > ?", sessionVersion, now).Find(&sessions).Error
	})
	if err != nil {
		return nil, fmt.Errorf("list active auth sessions: %w", err)
	}
	return sessions, nil
}

// CreateAuthSession persists a new session after pruning expired records.
func CreateAuthSession(ctx context.Context, session models.AuthSession, now time.Time) error {
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at <= ?", now).Delete(&models.AuthSession{}).Error; err != nil {
			return err
		}
		return tx.Create(&session).Error
	})
	if err != nil {
		return fmt.Errorf("create auth session: %w", err)
	}
	return nil
}

// RenewAuthSession extends an active session without changing its token.
func RenewAuthSession(ctx context.Context, tokenHash string, sessionVersion uint64, expiresAt, now time.Time) error {
	result := common.DB.WithContext(ctx).
		Model(&models.AuthSession{}).
		Where("token_hash = ? AND session_version = ? AND expires_at > ?", tokenHash, sessionVersion, now).
		Update("expires_at", expiresAt)
	if result.Error != nil {
		return fmt.Errorf("renew auth session: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("renew auth session: active session not found")
	}
	return nil
}

// DeleteAuthSession permanently revokes a session.
func DeleteAuthSession(ctx context.Context, tokenHash string) error {
	if err := common.DB.WithContext(ctx).Where("token_hash = ?", tokenHash).Delete(&models.AuthSession{}).Error; err != nil {
		return fmt.Errorf("delete auth session: %w", err)
	}
	return nil
}

// ResetAuthPassword replaces the password and revokes every session.
func ResetAuthPassword(ctx context.Context, passwordHash string) error {
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var account models.AuthAccount
		if err := tx.First(&account, authAccountID).Error; err != nil {
			return err
		}
		account.PasswordHash = passwordHash
		account.SessionVersion++
		if err := tx.Save(&account).Error; err != nil {
			return err
		}
		return tx.Where("1 = 1").Delete(&models.AuthSession{}).Error
	})
	if err != nil {
		return fmt.Errorf("reset auth password: %w", err)
	}
	return nil
}

// UpdateAuthPasswordAndReplaceSessions changes the password, revokes every old
// session, and persists the replacement session in one transaction.
func UpdateAuthPasswordAndReplaceSessions(ctx context.Context, passwordHash string, session models.AuthSession) (models.AuthAccount, error) {
	var account models.AuthAccount
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&account, authAccountID).Error; err != nil {
			return err
		}
		account.PasswordHash = passwordHash
		account.SessionVersion++
		if err := tx.Save(&account).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.AuthSession{}).Error; err != nil {
			return err
		}
		session.SessionVersion = account.SessionVersion
		return tx.Create(&session).Error
	})
	if err != nil {
		return models.AuthAccount{}, fmt.Errorf("update auth password and sessions: %w", err)
	}
	return account, nil
}
