package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/models"
)

var javDiscoverySyncRequests = make(chan struct{}, 1)

const (
	javDiscoveryHistoryBatchSize = 10
	javDiscoveryLatestScanLimit  = 100
)

var fetchJavBusActressWorks = jav.FetchJavBusActressWorks

// TriggerJavDiscoverySync asks the background scanner to run soon. Requests are
// coalesced so repeated UI actions cannot create concurrent JavBus crawls.
func TriggerJavDiscoverySync() {
	select {
	case javDiscoverySyncRequests <- struct{}{}:
	default:
	}
}

// StartJavDiscoveryScanner runs discovery sync immediately, on each interval,
// and whenever a subscription change explicitly triggers it.
func StartJavDiscoveryScanner(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	go func() {
		timer := time.NewTimer(0)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			case <-javDiscoverySyncRequests:
			}
			select {
			case <-javDiscoverySyncRequests:
			default:
			}
			if err := ScanJavDiscovery(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logging.Error("jav discovery scan failed: %v", err)
			}
			timer.Reset(interval)
		}
	}()
}

// ScanJavDiscovery refreshes every enabled discovery subscription serially.
func ScanJavDiscovery(ctx context.Context) error {
	subscriptions, err := db.ListJavDiscoverySubscriptions(ctx)
	if err != nil {
		return err
	}
	logging.Info("starting jav discovery scan for %d subscriptions", len(subscriptions))
	for _, subscription := range subscriptions {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := syncJavDiscoverySubscription(ctx, subscription); err != nil {
			logging.Error("sync jav discovery subscription failed id=%d name=%s err=%v", subscription.ID, subscription.Name, err)
			if markErr := db.MarkJavDiscoverySubscriptionSync(ctx, subscription.ID, err); markErr != nil {
				logging.Error("mark jav discovery subscription failure failed id=%d err=%v", subscription.ID, markErr)
			}
			continue
		}
		if err := db.MarkJavDiscoverySubscriptionSync(ctx, subscription.ID, nil); err != nil {
			return err
		}
	}
	return nil
}

func syncJavDiscoverySubscription(ctx context.Context, subscription models.JavDiscoverySubscription) error {
	switch subscription.Kind {
	case "idol":
		createdDateUnix := javDiscoverySubscriptionDateUnix(subscription.CreatedAt)
		if createdDateUnix <= 0 {
			return errors.New("jav discovery subscription has no creation date")
		}
		items, err := fetchJavBusActressWorks(
			ctx,
			subscription.ProviderLocator,
			subscription.Name,
			jav.JavBusActressWorksOptions{
				Limit:             javDiscoveryLatestScanLimit,
				ReleasedNotBefore: createdDateUnix,
			},
		)
		if err != nil {
			return fmt.Errorf("fetch javbus idol works: %w", err)
		}
		recentItems := discoveryItemsReleasedSince(items, createdDateUnix)
		if err := db.UpsertJavDiscoveryItems(ctx, subscription.ID, recentItems); err != nil {
			return err
		}
		logging.Info("synced jav discovery subscription id=%d name=%s items=%d", subscription.ID, subscription.Name, len(recentItems))
		return nil
	default:
		return fmt.Errorf("unsupported jav discovery subscription kind %q", subscription.Kind)
	}
}

func javDiscoverySubscriptionDateUnix(createdAt time.Time) int64 {
	if createdAt.IsZero() {
		return 0
	}
	createdAt = createdAt.In(time.Local)
	return time.Date(createdAt.Year(), createdAt.Month(), createdAt.Day(), 0, 0, 0, 0, time.UTC).Unix()
}

func discoveryItemsReleasedSince(items []jav.JavBusDiscoveryItem, createdDateUnix int64) []jav.JavBusDiscoveryItem {
	if createdDateUnix <= 0 {
		return nil
	}
	recentItems := make([]jav.JavBusDiscoveryItem, 0, len(items))
	for _, item := range items {
		if item.ReleaseUnix < createdDateUnix {
			continue
		}
		recentItems = append(recentItems, item)
	}
	return recentItems
}

// LoadMoreJavDiscoveryHistory loads the next bounded window of works released
// before the subscription date. Existing historical associations form the
// offset within that date-filtered listing.
func LoadMoreJavDiscoveryHistory(ctx context.Context, subscriptionID int64) (int, error) {
	subscription, err := db.GetJavDiscoverySubscription(ctx, subscriptionID)
	if err != nil {
		return 0, err
	}
	if subscription.Kind != "idol" {
		return 0, fmt.Errorf("unsupported jav discovery subscription kind %q", subscription.Kind)
	}
	createdDateUnix := javDiscoverySubscriptionDateUnix(subscription.CreatedAt)
	if createdDateUnix <= 0 {
		return 0, errors.New("jav discovery subscription has no creation date")
	}
	before, err := db.CountJavDiscoverySubscriptionItemsReleasedBefore(ctx, subscription.ID, createdDateUnix)
	if err != nil {
		return 0, err
	}
	items, err := fetchJavBusActressWorks(
		ctx,
		subscription.ProviderLocator,
		subscription.Name,
		jav.JavBusActressWorksOptions{
			Offset:         int(before),
			Limit:          javDiscoveryHistoryBatchSize,
			ReleasedBefore: createdDateUnix,
		},
	)
	if err != nil {
		return 0, fmt.Errorf("fetch javbus idol history: %w", err)
	}
	if err := db.UpsertJavDiscoveryItems(ctx, subscription.ID, items); err != nil {
		return 0, err
	}
	after, err := db.CountJavDiscoverySubscriptionItemsReleasedBefore(ctx, subscription.ID, createdDateUnix)
	if err != nil {
		return 0, err
	}
	loaded := max(0, int(after-before))
	logging.Info(
		"loaded jav discovery history id=%d name=%s offset=%d requested=%d loaded=%d",
		subscription.ID,
		subscription.Name,
		before,
		javDiscoveryHistoryBatchSize,
		loaded,
	)
	return loaded, nil
}
