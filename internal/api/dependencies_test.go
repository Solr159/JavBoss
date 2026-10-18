package api

import (
	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/directory"
	"testing"

	"gorm.io/gorm"
)

type testDependencies struct {
	database *gorm.DB
	config   *common.Config
}

func newTestDependencies() *testDependencies {
	setLANAccessUpdater(nil)
	return &testDependencies{}
}
func (d *testDependencies) bindDatabase(t *testing.T) {
	t.Helper()
	previous := db.SetDefault(db.NewStore(d.database))
	t.Cleanup(func() { db.SetDefault(previous) })
	previousConfig := common.SetConfig(d.config)
	t.Cleanup(func() { common.SetConfig(previousConfig) })
	coverDir := ""
	if d.config != nil {
		coverDir = d.config.JavCoverDir
	}
	directory.Init(coverDir)
}
