package service

import (
	"context"
	"errors"
	"strings"

	"javboss/internal/db"
	"javboss/internal/western"
)

func importWesternNFO(ctx context.Context, videoID int64, videoPath string) error {
	metadata, err := western.ReadNFO(videoPath)
	if errors.Is(err, western.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	existing, err := db.GetWesternMetadata(ctx, videoID)
	if err != nil {
		return err
	}
	if existing != nil && !strings.HasPrefix(strings.ToLower(existing.Source), "nfo") {
		return nil
	}
	_, err = db.SaveWesternMetadata(ctx, videoID, *metadata)
	return err
}
