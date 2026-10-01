package jav

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/util"
)

// ProviderOrigin identifies the actual origin used by a provider.
type ProviderOrigin interface{ OriginURL() string }

type AvailabilityProvider struct {
	ID         Provider            `json:"id"`
	Name       string              `json:"name"`
	Domain     string              `json:"domain"`
	Sample     string              `json:"sample"`
	LastResult *AvailabilityResult `json:"last_result,omitempty"`
}

type AvailabilityResult struct {
	Provider   Provider  `json:"provider"`
	Status     string    `json:"status"`
	HTTPStatus int       `json:"http_status,omitempty"`
	ElapsedMS  int64     `json:"elapsed_ms"`
	CheckedAt  time.Time `json:"checked_at"`
}

type availabilityCacheEntry struct {
	sequence uint64
	result   AvailabilityResult
}

// InvalidateAvailabilityCache also prevents in-flight checks from restoring old results.
func InvalidateAvailabilityCache() { defaultMetadataClient.invalidateAvailabilityCache() }

func (c *MetadataClient) invalidateAvailabilityCache() {
	c.availabilityMu.Lock()
	defer c.availabilityMu.Unlock()
	c.availabilityResults = nil
}

func (c *MetadataClient) cachedAvailabilityResult(provider Provider) *AvailabilityResult {
	c.availabilityMu.Lock()
	defer c.availabilityMu.Unlock()
	entry := c.availabilityResults[provider]
	if entry.result.CheckedAt.IsZero() {
		return nil
	}
	result := entry.result
	return &result
}

func (c *MetadataClient) beginAvailabilityCheck(provider Provider) uint64 {
	c.availabilityMu.Lock()
	defer c.availabilityMu.Unlock()
	if c.availabilityResults == nil {
		c.availabilityResults = make(map[Provider]availabilityCacheEntry)
	}
	c.availabilitySequence++
	entry := c.availabilityResults[provider]
	entry.sequence = c.availabilitySequence
	c.availabilityResults[provider] = entry
	return entry.sequence
}

func (c *MetadataClient) cacheAvailabilityResult(result AvailabilityResult, sequence uint64) {
	c.availabilityMu.Lock()
	defer c.availabilityMu.Unlock()
	entry, exists := c.availabilityResults[result.Provider]
	if !exists || entry.sequence != sequence {
		return
	}
	entry.result = result
	c.availabilityResults[result.Provider] = entry
}

func AvailabilityProviders() []AvailabilityProvider {
	return defaultMetadataClient.AvailabilityProviders()
}

func (c *MetadataClient) AvailabilityProviders() []AvailabilityProvider {
	result := make([]AvailabilityProvider, 0, len(c.providers))
	for id, implementation := range c.providers {
		if checker, ok := implementation.(ProviderOrigin); ok {
			target, err := url.Parse(checker.OriginURL())
			if err != nil || target.Hostname() == "" {
				continue
			}
			result = append(result, AvailabilityProvider{
				ID: id, Name: id.String(), Domain: target.Hostname(), Sample: availabilitySample(id), LastResult: c.cachedAvailabilityResult(id),
			})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func CheckAvailability(ctx context.Context, provider Provider) (AvailabilityResult, error) {
	return defaultMetadataClient.CheckAvailability(ctx, provider)
}

// CheckAvailability runs one complete lookup on a fresh provider and HTTP clients.
// It bypasses both lookup and URL caches, retaining only the check's summary.
func (c *MetadataClient) CheckAvailability(ctx context.Context, provider Provider) (result AvailabilityResult, err error) {
	result = AvailabilityResult{Provider: provider}
	if _, err = c.providerFor(provider); err != nil {
		return result, err
	}
	sequence := c.beginAvailabilityCheck(provider)
	callerCtx := ctx
	started := time.Now()
	var lookupErr error
	defer func() {
		result.ElapsedMS = time.Since(started).Milliseconds()
		result.CheckedAt = time.Now().UTC()
		if err != nil {
			lookupErr = err
		}
		if result.Status != "ok" {
			logAvailabilityFailure(result, lookupErr)
		}
		if result.Status != "canceled" && callerCtx.Err() == nil && err == nil {
			c.cacheAvailabilityResult(result, sequence)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if ctx.Err() != nil {
		lookupErr = ctx.Err()
		result.Status = availabilityErrorStatus(lookupErr)
		return result, nil
	}
	probe := util.NewHTTPProbe(newProviderHTTPClient(provider))
	defer probe.Close()
	implementation, err := c.availabilityFactory(provider, probe.Client)
	if err != nil {
		return result, err
	}
	// Query the fresh provider directly, bypassing the metadata lookup cache.
	sample := availabilitySample(provider)
	valid := false
	switch lookup := implementation.(type) {
	case MovieLookup:
		var info *JavInfo
		info, lookupErr = lookup.LookupJavByCode(ctx, sample)
		valid = info != nil && strings.EqualFold(strings.TrimSpace(info.Code), sample) && strings.TrimSpace(info.Title) != "" && !strings.EqualFold(strings.TrimSpace(info.Title), sample)
	case ActressNameLookup:
		var info *ActressInfo
		info, lookupErr = lookup.LookupActressByName(ctx, sample)
		valid = info != nil && strings.Join(strings.Fields(info.JapaneseName), "") == sample &&
			(strings.TrimSpace(info.RomanName) != "" || info.HeightCM > 0 || info.BirthDate != 0 || info.Bust > 0 || info.Waist > 0 || info.Hips > 0 || info.Cup > 0)
	default:
		return result, ErrUnsupportedOperation
	}
	result.HTTPStatus = probe.HTTPStatus()
	switch {
	case ctx.Err() != nil:
		lookupErr = ctx.Err()
		result.Status = availabilityErrorStatus(lookupErr)
	case lookupErr == nil && valid:
		result.Status = "ok"
	case lookupErr != nil:
		result.Status = availabilityErrorStatus(lookupErr)
		if (result.Status == "invalid_response" || result.Status == "not_found") && result.HTTPStatus >= 400 {
			result.Status = "http_error"
		}
	default:
		result.Status = "invalid_response"
	}
	return result, nil
}

func logAvailabilityFailure(result AvailabilityResult, err error) {
	if result.Status == "canceled" {
		logging.Info("jav availability check canceled: provider=%s elapsed_ms=%d", result.Provider, result.ElapsedMS)
		return
	}
	status := result.Status
	if status == "" {
		status = "error"
	}
	// Request URLs may contain proxy credentials, tokens or device identities.
	// Preserve the underlying cause without logging the URL, including nested
	// url.Errors produced by proxy requests.
	for {
		var requestErr *url.Error
		if !errors.As(err, &requestErr) {
			break
		}
		err = requestErr.Err
	}
	reason := "lookup returned no matching or sufficiently complete metadata"
	if err != nil {
		reason = err.Error()
	}
	logging.Error("jav availability check failed: provider=%s status=%s http_status=%d elapsed_ms=%d err=%q",
		result.Provider, status, result.HTTPStatus, result.ElapsedMS, reason)
}

// These stable examples exercise the capability supported by each provider.
// A missing sample is reported separately from network or parsing failures.
func availabilitySample(provider Provider) string {
	switch provider {
	case ProviderJavModel:
		return "波多野結衣"
	case ProviderMinnanoAV, ProviderAVWiki:
		return "三上悠亜"
	case ProviderAvsox:
		return "030919_047"
	default:
		return "SSIS-001"
	}
}

func availabilityErrorStatus(err error) string {
	var networkError net.Error
	var dnsError *net.DNSError
	var tlsError *tls.CertificateVerificationError
	var requestError *url.Error
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &networkError) && networkError.Timeout():
		return "timeout"
	case errors.As(err, &dnsError):
		return "dns_error"
	case errors.As(err, &tlsError):
		return "tls_error"
	case errors.As(err, &requestError):
		return "network_error"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	default:
		return "invalid_response"
	}
}
