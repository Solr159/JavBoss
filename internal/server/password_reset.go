package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"unicode/utf8"

	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"

	"golang.org/x/crypto/bcrypt"
)

const (
	PasswordResetFilename = "password_reset.txt"
	maxPasswordResetSize  = 256
)

// ApplyPasswordResetFile consumes a one-time password reset file when present.
func ApplyPasswordResetFile(ctx context.Context, path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("inspect password reset file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("password reset path must be a regular file")
	}
	if info.Size() <= 0 || info.Size() > maxPasswordResetSize {
		return false, fmt.Errorf("password reset file must contain 1-%d bytes", maxPasswordResetSize)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		logging.Info("warning: password reset file permissions are broader than 0600: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read password reset file: %w", err)
	}
	if len(data) > maxPasswordResetSize {
		return false, fmt.Errorf("password reset file exceeds %d bytes", maxPasswordResetSize)
	}
	password, err := parsePasswordResetFile(data)
	if err != nil {
		return false, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, fmt.Errorf("hash reset password: %w", err)
	}
	if err := dbpkg.ResetAuthPassword(ctx, string(hash)); err != nil {
		return false, err
	}
	if err := os.Remove(path); err != nil {
		return false, fmt.Errorf("password was reset but reset file could not be removed: %w", err)
	}
	return true, nil
}

func parsePasswordResetFile(data []byte) (string, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if bytes.HasSuffix(data, []byte("\r\n")) {
		data = data[:len(data)-2]
	} else if bytes.HasSuffix(data, []byte("\n")) {
		data = data[:len(data)-1]
	}
	if !utf8.Valid(data) || bytes.ContainsAny(data, "\r\n") {
		return "", fmt.Errorf("password reset file must contain one UTF-8 line")
	}
	password := string(data)
	if !validNewPassword(password) {
		return "", fmt.Errorf("password reset value must be 6-20 characters without surrounding spaces")
	}
	if strings.ContainsRune(password, '\x00') {
		return "", fmt.Errorf("password reset value contains an invalid character")
	}
	return password, nil
}
