package manager

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"slices"
)

const coverSampleSize int64 = 256

// Fingerprints hash three 256-byte windows: start, center, and end, in that
// order. Only files with a known size need content reads (768 bytes total).
// These identify known placeholder files, not resized or recompressed variants.
var coverBlacklist = map[int64][]string{
	// "NOW PRINTING" JPEG found in MUM-064, MUM-176, and MUM-184 covers.
	19378: {"afc620b085e1833a33b16a37e783938cc5ea9eae8992ecc30f615bc12be2be69"},
}

func isBlacklistedCoverFile(path string, size int64) (bool, error) {
	fingerprints, ok := coverBlacklist[size]
	if !ok {
		return false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	fingerprint, err := sampledCoverFingerprint(f, size)
	if err != nil {
		return false, err
	}
	return slices.Contains(fingerprints, fingerprint), nil
}

func sampledCoverFingerprint(r io.ReaderAt, size int64) (string, error) {
	if size < 3*coverSampleSize {
		return "", fmt.Errorf("cover too small for sampling: %d", size)
	}
	hash := sha256.New()
	var sample [coverSampleSize]byte
	for _, offset := range []int64{0, (size - coverSampleSize) / 2, size - coverSampleSize} {
		if _, err := r.ReadAt(sample[:], offset); err != nil {
			return "", err
		}
		_, _ = hash.Write(sample[:])
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
