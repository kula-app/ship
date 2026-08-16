package service

import (
	"context"
	"log/slog"

	"github.com/kula-app/ship/internal/cli/api"
	"github.com/kula-app/ship/internal/cli/config"
)

// Client is an interface representing the API client operations needed by the service.
// This allows for easier testing and dependency injection.
type Client interface {
	Get(path string) ([]byte, error)
	Post(path string, payload any) ([]byte, error)
	RawRequest(ctx context.Context, method, path string, body []byte, headers map[string]string) (*api.RawResponse, error)
	URL(path string) string
}

// ClientFactory creates new API clients bound to a set of credentials.
type ClientFactory interface {
	NewClient(apiURL, token string) Client
	NewAPIKeyClient(apiURL, apiKey string) Client
}

// ShipServiceFactory creates client-bound ShipService instances for CLI commands.
type ShipServiceFactory interface {
	GetShipService(logger *slog.Logger, credentials *config.Credentials) *ShipService
}

// DefaultClientFactory is the default implementation of ClientFactory.
// It creates real API clients using the api package.
type DefaultClientFactory struct{}

// NewClient creates a new API client authenticated with a bearer token.
func (f *DefaultClientFactory) NewClient(apiURL, token string) Client {
	return api.NewClient(apiURL, token)
}

// NewAPIKeyClient creates a new API client authenticated with an API key.
func (f *DefaultClientFactory) NewAPIKeyClient(apiURL, apiKey string) Client {
	return api.NewAPIKeyClient(apiURL, apiKey)
}
