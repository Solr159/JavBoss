package jav

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"sort"
	"time"
)

// ConnectivityChecker makes a fresh HTTP request using the provider's transport.
// Implementations must honor ctx and leave closing the response body to the caller.
type ConnectivityChecker interface {
	CheckConnectivity(context.Context) (*http.Response, error)
}

type ConnectivityProvider struct {
	ID   Provider `json:"id"`
	Name string   `json:"name"`
}

type ConnectivityResult struct {
	Provider   Provider `json:"provider"`
	Status     string   `json:"status"`
	HTTPStatus int      `json:"http_status,omitempty"`
	ElapsedMS  int64    `json:"elapsed_ms"`
}

func ConnectivityProviders() []ConnectivityProvider { return defaultClient.ConnectivityProviders() }

func (c *Client) ConnectivityProviders() []ConnectivityProvider {
	result := make([]ConnectivityProvider, 0, len(c.providers))
	for id, implementation := range c.providers {
		if _, ok := implementation.(ConnectivityChecker); ok {
			result = append(result, ConnectivityProvider{ID: id, Name: id.String()})
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
func (c *Client) CheckConnectivity(ctx context.Context, provider Provider) (ConnectivityResult, error) {
	result := ConnectivityResult{Provider: provider}
	implementation, err := c.providerFor(provider)
	if err != nil {
		return result, err
	}
	checker, ok := implementation.(ConnectivityChecker)
	if !ok {
		return result, ErrUnsupportedOperation
	}
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
