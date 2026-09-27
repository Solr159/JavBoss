package jav

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sort"
	"time"
)

// ConnectivityChecker makes a fresh HTTP request using the provider's transport.
// Implementations must honor ctx and leave closing the response body to the caller.
type ConnectivityChecker interface {
	ConnectivityURL() string
	CheckConnectivity(context.Context) (*http.Response, error)
}

type ConnectivityProvider struct {
	ID         Provider            `json:"id"`
	Name       string              `json:"name"`
	Domain     string              `json:"domain"`
	LastResult *ConnectivityResult `json:"last_result,omitempty"`
}

type ConnectivityResult struct {
	Provider   Provider  `json:"provider"`
	Status     string    `json:"status"`
	HTTPStatus int       `json:"http_status,omitempty"`
	ElapsedMS  int64     `json:"elapsed_ms"`
	CheckedAt  time.Time `json:"checked_at"`
}

type connectivityCacheEntry struct {
	sequence uint64
	result   ConnectivityResult
}

// InvalidateConnectivityCache also prevents in-flight checks from restoring old results.
func InvalidateConnectivityCache() { defaultClient.invalidateConnectivityCache() }

func (c *Client) invalidateConnectivityCache() {
	c.connectivityMu.Lock()
	defer c.connectivityMu.Unlock()
	c.connectivityResults = nil
}

func (c *Client) cachedConnectivityResult(provider Provider) *ConnectivityResult {
	c.connectivityMu.Lock()
	defer c.connectivityMu.Unlock()
	entry := c.connectivityResults[provider]
	if entry.result.CheckedAt.IsZero() {
		return nil
	}
	result := entry.result
	return &result
}

func (c *Client) beginConnectivityCheck(provider Provider) uint64 {
	c.connectivityMu.Lock()
	defer c.connectivityMu.Unlock()
	if c.connectivityResults == nil {
		c.connectivityResults = make(map[Provider]connectivityCacheEntry)
	}
	c.connectivitySequence++
	entry := c.connectivityResults[provider]
	entry.sequence = c.connectivitySequence
	c.connectivityResults[provider] = entry
	return entry.sequence
}

func (c *Client) cacheConnectivityResult(result ConnectivityResult, sequence uint64) {
	c.connectivityMu.Lock()
	defer c.connectivityMu.Unlock()
	entry, exists := c.connectivityResults[result.Provider]
	if !exists || entry.sequence != sequence {
		return
	}
	entry.result = result
	c.connectivityResults[result.Provider] = entry
}

func ConnectivityProviders() []ConnectivityProvider { return defaultClient.ConnectivityProviders() }

func (c *Client) ConnectivityProviders() []ConnectivityProvider {
	result := make([]ConnectivityProvider, 0, len(c.providers))
	for id, implementation := range c.providers {
		if checker, ok := implementation.(ConnectivityChecker); ok {
			target, err := url.Parse(checker.ConnectivityURL())
			if err != nil || target.Hostname() == "" {
				continue
			}
			result = append(result, ConnectivityProvider{
				ID: id, Name: id.String(), Domain: target.Hostname(), LastResult: c.cachedConnectivityResult(id),
			})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func CheckConnectivity(ctx context.Context, provider Provider) (ConnectivityResult, error) {
	return defaultClient.CheckConnectivity(ctx, provider)
}

// CheckConnectivity reports HTTP reachability, not metadata lookup success. It never
// reads or populates lookup/404 caches, nor exposes URLs, proxy credentials or API identities.
func (c *Client) CheckConnectivity(ctx context.Context, provider Provider) (result ConnectivityResult, err error) {
	result = ConnectivityResult{Provider: provider}
	implementation, err := c.providerFor(provider)
	if err != nil {
		return result, err
	}
	checker, ok := implementation.(ConnectivityChecker)
	if !ok {
		return result, ErrUnsupportedOperation
	}
	sequence := c.beginConnectivityCheck(provider)
	callerCtx := ctx
	defer func() {
		result.CheckedAt = time.Now().UTC()
		if result.Status != "canceled" && callerCtx.Err() == nil {
			c.cacheConnectivityResult(result, sequence)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	started := time.Now()
	response, err := checker.CheckConnectivity(ctx)
	result.ElapsedMS = time.Since(started).Milliseconds()
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err != nil {
		result.Status = connectivityErrorStatus(err)
		return result, nil
	}
	if response == nil {
		result.Status = "network_error"
		return result, nil
	}
	result.HTTPStatus = response.StatusCode
	result.Status = "http_error"
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		result.Status = "ok"
	}
	return result, nil
}

func connectivityErrorStatus(err error) string {
	var networkError net.Error
	var dnsError *net.DNSError
	var tlsError *tls.CertificateVerificationError
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &networkError) && networkError.Timeout():
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.As(err, &dnsError):
		return "dns_error"
	case errors.As(err, &tlsError):
		return "tls_error"
	default:
		return "network_error"
	}
}
