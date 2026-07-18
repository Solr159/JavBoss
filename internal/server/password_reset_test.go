package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"javboss/internal/common"
	dbpkg "javboss/internal/db"
)

func TestApplyPasswordResetFileResetsPasswordAndRevokesSessions(t *testing.T) {
	dbPath := openPasswordResetTestDB(t)
	ctx := context.Background()
	auth, err := NewAuthService(ctx)
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	oldToken, _, err := auth.Login(ctx, "test", "admin")
	if err != nil {
		t.Fatalf("login before reset: %v", err)
	}

	resetPath := filepath.Join(filepath.Dir(dbPath), PasswordResetFilename)
	if err := os.WriteFile(resetPath, []byte("recover1\r\n"), 0o600); err != nil {
		t.Fatalf("write password reset file: %v", err)
	}
	applied, err := ApplyPasswordResetFile(ctx, resetPath)
	if err != nil {
		t.Fatalf("apply password reset file: %v", err)
	}
	if !applied {
		t.Fatal("password reset file was not applied")
	}
	if _, err := os.Stat(resetPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("password reset file was not removed: %v", err)
	}

	restarted, err := NewAuthService(ctx)
	if err != nil {
		t.Fatalf("restart auth service: %v", err)
	}
	if restarted.Authenticated(oldToken) {
		t.Fatal("old session remained valid after password reset")
	}
	if _, _, err := restarted.Login(ctx, "old-password", "admin"); err == nil {
		t.Fatal("old password still works after reset")
	}
	if _, _, err := restarted.Login(ctx, "new-password", "recover1"); err != nil {
		t.Fatalf("reset password login: %v", err)
	}
}

func TestApplyPasswordResetFileRejectsInvalidContent(t *testing.T) {
	dbPath := openPasswordResetTestDB(t)
	resetPath := filepath.Join(filepath.Dir(dbPath), PasswordResetFilename)
	if err := os.WriteFile(resetPath, []byte("short\n"), 0o600); err != nil {
		t.Fatalf("write password reset file: %v", err)
	}
	applied, err := ApplyPasswordResetFile(context.Background(), resetPath)
	if err == nil {
		t.Fatal("invalid reset password was accepted")
	}
	if applied {
		t.Fatal("invalid reset password was reported as applied")
	}
	if _, statErr := os.Stat(resetPath); statErr != nil {
		t.Fatalf("invalid reset file should remain for correction: %v", statErr)
	}

	auth, authErr := NewAuthService(context.Background())
	if authErr != nil {
		t.Fatalf("new auth service: %v", authErr)
	}
	if _, _, loginErr := auth.Login(context.Background(), "test", "admin"); loginErr != nil {
		t.Fatalf("invalid reset changed the existing password: %v", loginErr)
	}
}

func TestApplyPasswordResetFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("recover1"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	resetPath := filepath.Join(dir, PasswordResetFilename)
	if err := os.Symlink(target, resetPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	applied, err := ApplyPasswordResetFile(context.Background(), resetPath)
	if err == nil {
		t.Fatal("password reset symlink was accepted")
	}
	if applied {
		t.Fatal("password reset symlink was reported as applied")
	}
}

func TestParsePasswordResetFile(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
		ok   bool
	}{
		{name: "plain", data: []byte("abcdef"), want: "abcdef", ok: true},
		{name: "utf8 bom and newline", data: append([]byte{0xef, 0xbb, 0xbf}, []byte("密码安全测试\n")...), want: "密码安全测试", ok: true},
		{name: "multiple lines", data: []byte("abcdef\nsecond\n"), ok: false},
		{name: "too short", data: []byte("abcde"), ok: false},
		{name: "too long", data: []byte("123456789012345678901"), ok: false},
		{name: "surrounding space", data: []byte(" abcdef"), ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePasswordResetFile(tt.data)
			if (err == nil) != tt.ok {
				t.Fatalf("parse error = %v, want valid=%v", err, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("parsed password = %q, want %q", got, tt.want)
			}
		})
	}
}

func openPasswordResetTestDB(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data", "javboss.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("create test data directory: %v", err)
	}
	database, err := dbpkg.Open(dbPath)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	previousDB := common.DB
	common.DB = database
	t.Cleanup(func() {
		common.DB = previousDB
		if sqlDB, dbErr := database.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return dbPath
}
