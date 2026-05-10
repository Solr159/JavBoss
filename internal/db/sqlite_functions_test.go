package db

import "testing"

func TestStableRandomRankSQL(t *testing.T) {
	const seed = int64(123456789)

	first := stableRandomRankSQL(42, seed)
	second := stableRandomRankSQL(42, seed)
	if first != second {
		t.Fatalf("stableRandomRankSQL should be deterministic: got %d and %d", first, second)
	}
	if first <= 0 {
		t.Fatalf("stableRandomRankSQL should return a positive signed rank: %d", first)
	}
	if got := stableRandomRankSQL(43, seed); got == first {
		t.Fatalf("different ids should rank differently: id 42 and 43 both got %d", first)
	}
	if got := stableRandomRankSQL(42, seed+1); got == first {
		t.Fatalf("different seeds should rank differently: seed %d and %d both got %d", seed, seed+1, first)
	}
}
