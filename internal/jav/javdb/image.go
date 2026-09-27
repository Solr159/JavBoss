package javdb

import (
	"bufio"
	"io"
	"net/http"
	"strings"
)

// JavDB API image responses can contain a one-byte XOR key followed by the
// encoded JPEG/PNG. Detect the decoded image signature instead of relying on
// the CDN hostname or Content-Type, which may be absent or octet-stream.
func DecodeImageBody(body io.Reader) (io.Reader, bool) {
	reader := bufio.NewReader(body)
	header, _ := reader.Peek(16)
	if len(header) < 4 || strings.HasPrefix(http.DetectContentType(header), "image/") {
		return reader, false
	}
	key := header[0]
	decoded := make([]byte, len(header)-1)
	for i := range decoded {
		decoded[i] = header[i+1] ^ key
	}
	kind := http.DetectContentType(decoded)
	if kind != "image/jpeg" && kind != "image/png" {
		return reader, false
	}
	_, _ = reader.Discard(1)
	return &xorImageReader{reader: reader, key: key}, true
}

type xorImageReader struct {
	reader io.Reader
	key    byte
}

func (r *xorImageReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	for i := range p[:n] {
		p[i] ^= r.key
	}
	return n, err
}
