package db

import "gorm.io/gorm"

type testDependencies struct {
	store    *Store
	database *gorm.DB
}

func newTestDependencies() *testDependencies { return &testDependencies{} }
func (d *testDependencies) bindDatabase()    { d.store = NewStore(d.database) }
