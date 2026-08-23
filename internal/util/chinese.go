package util

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/liuzl/gocc"
)

var (
	goccOnce             sync.Once
	goccConverter        *gocc.OpenCC
	goccErr              error
	traditionalOnce      sync.Once
	traditionalConverter *gocc.OpenCC
	traditionalErr       error
)

// SimplifyChineseName best-effort converts traditional Chinese to simplified.
// If conversion fails, it returns the input.
func SimplifyChineseName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	cc, err := openccConverter()
	if err != nil || cc == nil {
		return value
	}
	simplified, err := cc.Convert(value)
	if err != nil {
		return value
	}
	simplified = strings.TrimSpace(simplified)
	if simplified == "" {
		return value
	}
	return simplified
}

func openccConverter() (*gocc.OpenCC, error) {
	goccOnce.Do(func() {
		goccConverter, goccErr = gocc.New("t2s")
	})
	return goccConverter, goccErr
}

// TraditionalizeChineseName converts simplified Chinese to traditional.
// Unlike SimplifyChineseName, conversion errors are returned so callers do not
// persist an unnormalized value when canonical identity depends on the result.
func TraditionalizeChineseName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return value, nil
	}
	cc, err := openTraditionalChineseConverter()
	if err != nil {
		return "", fmt.Errorf("create traditional Chinese converter: %w", err)
	}
	if cc == nil {
		return "", errors.New("traditional Chinese converter is unavailable")
	}
	traditional, err := cc.Convert(value)
	if err != nil {
		return "", fmt.Errorf("convert to traditional Chinese: %w", err)
	}
	traditional = strings.TrimSpace(traditional)
	if traditional == "" {
		return "", errors.New("traditional Chinese conversion returned an empty value")
	}
	return traditional, nil
}

func openTraditionalChineseConverter() (*gocc.OpenCC, error) {
	traditionalOnce.Do(func() {
		traditionalConverter, traditionalErr = gocc.New("s2t")
	})
	return traditionalConverter, traditionalErr
}
