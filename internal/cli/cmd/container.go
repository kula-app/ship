package cmd

import (
	"log/slog"

	"github.com/kula-app/ship/internal/cli/config"
	"github.com/kula-app/ship/internal/cli/service"
)

// DependencyContainer wires the concrete dependencies used by CLI commands.
// It satisfies the per-command deps interfaces (logger + service factories).
type DependencyContainer struct {
	clientFactory service.ClientFactory
	logger        *slog.Logger
}

// NewDependencyContainer creates a container using the default logger.
func NewDependencyContainer() *DependencyContainer {
	return NewDependencyContainerWithLogger(slog.Default())
}

// NewDependencyContainerWithLogger creates a container with a custom logger.
// This allows commands to configure logging based on output format (e.g. silent for JSON mode).
func NewDependencyContainerWithLogger(logger *slog.Logger) *DependencyContainer {
	return &DependencyContainer{clientFactory: &service.DefaultClientFactory{}, logger: logger}
}

// GetLogger returns the container's logger.
func (d *DependencyContainer) GetLogger() *slog.Logger { return d.logger }

// GetShipService builds a client-bound ShipService for the given credentials.
func (d *DependencyContainer) GetShipService(logger *slog.Logger, credentials *config.Credentials) *service.ShipService {
	return service.NewShipService(d.newClient(credentials), logger)
}

// newClient builds the API client matching the resolved authentication method.
func (d *DependencyContainer) newClient(credentials *config.Credentials) service.Client {
	if credentials.IsAPIKey() {
		return d.clientFactory.NewAPIKeyClient(credentials.APIURL, credentials.APIKey)
	}
	return d.clientFactory.NewClient(credentials.APIURL, credentials.AccessToken)
}
