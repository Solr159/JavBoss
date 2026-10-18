package translation

import (
	"context"
	"net/http"
	"sync/atomic"
)

var providerClient atomic.Pointer[Client]

func init() { providerClient.Store(NewClient()) }

// SetHTTPClient configures the transport for future requests and returns the previous one.
// Install it before consumers start; ongoing requests retain their existing client.
func SetHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = NewClient().HTTP
	}
	return providerClient.Swap(&Client{HTTP: client}).HTTP
}

func Translate(ctx context.Context, provider string, profile Profile, prompt, title string, override *bool) (string, error) {
	return providerClient.Load().Translate(ctx, provider, profile, prompt, title, override)
}

func Models(ctx context.Context, provider string, profile Profile) ([]Model, error) {
	return providerClient.Load().Models(ctx, provider, profile)
}
