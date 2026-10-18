package enrichment

import (
	"testing"

	"javboss/internal/db"

	"gorm.io/gorm"
)

type testDependencies struct {
	database *gorm.DB
}

func newTestDependencies() *testDependencies { return &testDependencies{} }
func (d *testDependencies) bindDatabase(t *testing.T) {
	t.Helper()
	previous := db.SetDefault(db.NewStore(d.database))
	t.Cleanup(func() { db.SetDefault(previous) })
}
