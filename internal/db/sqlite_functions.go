package db

import (
	"database/sql"
	"sync"

	"github.com/mattn/go-sqlite3"
)

const sqliteDriverName = "sqlite3_pornboss"

var registerOnce sync.Once

func registerSQLiteFunctions() string {
	registerOnce.Do(func() {
		sql.Register(sqliteDriverName, &sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				if err := conn.RegisterFunc("splitmix64", splitmix64SQL, true); err != nil {
					return err
				}
				return conn.RegisterFunc("stable_random_rank", stableRandomRankSQL, true)
			},
		})
	})
	return sqliteDriverName
}

func splitmix64SQL(id int64, seed int64) int64 {
	x := uint64(id) + uint64(seed)
	x += 0x9e3779b97f4a7c15
	z := x
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z = z ^ (z >> 31)
	return int64(z & 0x7fffffffffffffff)
}

func stableRandomRankSQL(id int64, seed int64) int64 {
	x := uint64(seed) ^ 0x9e3779b97f4a7c15
	y := uint64(id) + 0xbf58476d1ce4e5b9
	x ^= y + 0x9e3779b97f4a7c15 + (x << 6) + (x >> 2)
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return int64(x & 0x7fffffffffffffff)
}
