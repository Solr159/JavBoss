package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStoresKeepIndependentConnections(t *testing.T) {
	stores := make([]*Store, 2)
	for i := range stores {
		database, err := Open(filepath.Join(t.TempDir(), "library.db"))
		if err != nil {
			t.Fatal(err)
		}
		connection, err := database.DB()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = connection.Close() })
		stores[i] = NewStore(database)
	}
	for i, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			store := stores[i]
			if err := store.UpsertConfig(ctx, map[string]string{"instance": name}); err != nil {
				t.Fatal(err)
			}
			path := t.TempDir()
			directory, err := store.CreateDirectory(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			if directory.ID != 1 {
				t.Fatalf("directory ID = %d, want independent ID 1", directory.ID)
			}
			for range 10 {
				cfg, err := store.ListConfig(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if cfg["instance"] != name {
					t.Fatalf("read another store's config: %v", cfg)
				}
				dirs, err := store.ListDirectories(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if len(dirs) != 1 || dirs[0].Path != path {
					t.Fatalf("read another store's directories: %+v", dirs)
				}
			}
		})
	}
}

func TestDefaultRequiresInitialization(t *testing.T) {
	previous := SetDefault(nil)
	t.Cleanup(func() { SetDefault(previous) })
	defer func() {
		if got := recover(); got != "db: default store is not initialized" {
			t.Fatalf("panic = %v, want an initialization error", got)
		}
	}()
	Default()
}
