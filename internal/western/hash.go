package western

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const openSubtitlesHashChunk = 64 * 1024

// OpenSubtitlesHash calculates the 64-bit hash used by Emby-compatible
// metadata providers: file size plus the first and last 64 KiB.
func OpenSubtitlesHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open video for hash: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat video for hash: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("video for hash is not a regular file")
	}

	buffer := make([]byte, openSubtitlesHashChunk)
	var hash uint64 = uint64(info.Size())
	for _, offset := range []int64{0, maxInt64(0, info.Size()-openSubtitlesHashChunk)} {
		count, readErr := file.ReadAt(buffer, offset)
		if readErr != nil && readErr != io.EOF {
			return "", fmt.Errorf("read video for hash: %w", readErr)
		}
		for index := 0; index+8 <= count; index += 8 {
			hash += binary.LittleEndian.Uint64(buffer[index : index+8])
		}
	}
	return fmt.Sprintf("%016x", hash), nil
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
