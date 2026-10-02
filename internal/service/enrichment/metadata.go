package enrichment

import (
	"math/rand/v2"

	"javboss/internal/db"
)

func shuffleCandidates(items []db.JavEnrichmentItem) {
	rand.Shuffle(len(items), func(i, j int) {
		items[i], items[j] = items[j], items[i]
	})
}
