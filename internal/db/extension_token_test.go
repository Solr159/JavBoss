package db

import (
	"path/filepath"
	"testing"
	"time"

	"javboss/internal/models"
)

func TestExtensionTokenPersistence(t *testing.T) {
	deps := newTestDependencies()
	path := filepath.Join(t.TempDir(), "extension.db")
	database, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	deps.database = database
	deps.bindDatabase()

	t.Cleanup(func() {
		if connection, err := deps.database.DB(); err == nil {
			_ = connection.Close()
		}

	})
	now := time.Now().UTC()
	token := models.ExtensionToken{Name: "browser", Token: "stored-token", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := deps.store.CreateExtensionToken(t.Context(), &token); err != nil {
		t.Fatal(err)
	}
	connection, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	deps.database = reopened
	deps.bindDatabase()

	authenticated, err := deps.store.UseExtensionToken(t.Context(), token.Token, now)
	if err != nil || !authenticated {
		t.Fatalf("persisted authorization: %t %v", authenticated, err)
	}
	if err := deps.store.DeleteExtensionToken(t.Context(), token.ID); err != nil {
		t.Fatal(err)
	}
	authenticated, err = deps.store.UseExtensionToken(t.Context(), token.Token, now)
	if err != nil || authenticated {
		t.Fatalf("deleted persisted token: %t %v", authenticated, err)
	}
}

func TestExtensionTokenExpiry(t *testing.T) {
	deps := newTestDependencies()
	database, err := Open(filepath.Join(t.TempDir(), "expiry.db"))
	if err != nil {
		t.Fatal(err)
	}

	deps.database = database
	deps.bindDatabase()

	t.Cleanup(func() {
		connection, _ := database.DB()
		_ = connection.Close()

	})
	now := time.Now().UTC()
	for _, tc := range []struct {
		name    string
		expiry  time.Time
		deleted bool
		want    bool
	}{
		{"never expires", time.Time{}, false, true},
		{"future expiry", now.Add(time.Hour), false, true},
		{"expired", now.Add(-time.Second), false, false},
		{"expiry boundary", now, false, false},
		{"deleted never expires", time.Time{}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := models.ExtensionToken{Name: tc.name, Token: tc.name, CreatedAt: now, ExpiresAt: tc.expiry}
			if err := deps.store.CreateExtensionToken(t.Context(), &token); err != nil {
				t.Fatal(err)
			}
			if tc.deleted {
				if err := deps.store.DeleteExtensionToken(t.Context(), token.ID); err != nil {
					t.Fatal(err)
				}
			}
			valid, err := deps.store.UseExtensionToken(t.Context(), token.Token, now)
			if err != nil || valid != tc.want {
				t.Fatalf("valid=%v want=%v err=%v", valid, tc.want, err)
			}
			if tc.expiry.IsZero() && !tc.deleted {
				valid, err = deps.store.UseExtensionToken(t.Context(), token.Token, now.AddDate(100, 0, 0))
				if err != nil || !valid {
					t.Fatalf("non-expiring token expired: %v", err)
				}
			}
		})
	}
}
