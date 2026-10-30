// Package autosub wraps the AutoSub subtitle gateway for the host app.
package autosub

import (
	"context"
)

// Service adds a thin layer over Client so the rest of the app can trigger a
// subtitle download or translation without knowing the gateway details.
type Service struct {
	client *Client
}

// NewService constructs a Service backed by a fresh Client.
func NewService() *Service {
	return &Service{client: NewClient()}
}

// Download launches a subtitle download for one video and returns the result.
func (s *Service) Download(ctx context.Context, video Video, language string) (DownloadResult, error) {
	return s.client.Download(ctx, DownloadRequest{Video: video, Language: language})
}

// Translate launches a subtitle translation for one video and returns the result.
func (s *Service) Translate(ctx context.Context, video Video, params map[string]string) (TranslateResult, error) {
	return s.client.Translate(ctx, TranslateRequest{Video: video, Params: params})
}

// Ping reports whether the gateway is reachable.
func (s *Service) Ping(ctx context.Context) error {
	return s.client.Ping(ctx)
}
