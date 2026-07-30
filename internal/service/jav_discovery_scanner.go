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
		items, err := jav.FetchJavBusActressWorks(ctx, subscription.ProviderKey, subscription.Name)
		if err != nil {
			return fmt.Errorf("fetch javbus idol works: %w", err)
		}
		if err := db.UpsertJavDiscoveryItems(ctx, subscription.ID, items); err != nil {
			return err
		}
		logging.Info("synced jav discovery subscription id=%d name=%s items=%d", subscription.ID, subscription.Name, len(items))
		return nil
	default:
		return fmt.Errorf("unsupported jav discovery subscription kind %q", subscription.Kind)
	}
}
