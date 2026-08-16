package cmd_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/kula-app/ship/internal/cli/api"
	"github.com/kula-app/ship/internal/cli/cmd"
	"github.com/kula-app/ship/internal/cli/config"
	"github.com/kula-app/ship/internal/cli/service"
)

type rootTestDependencies struct {
	logger      *slog.Logger
	service     *service.ShipService
	credentials *config.Credentials
}

func (d *rootTestDependencies) GetLogger() *slog.Logger {
	return d.logger
}

func (d *rootTestDependencies) GetShipService(logger *slog.Logger, credentials *config.Credentials) *service.ShipService {
	d.credentials = credentials
	return d.service
}

type rootTestClient struct {
	path string
}

func (c *rootTestClient) Get(path string) ([]byte, error) {
	c.path = path
	return []byte("[]"), nil
}

func (c *rootTestClient) Post(string, any) ([]byte, error) {
	return nil, errors.New("unexpected POST request")
}

func (c *rootTestClient) RawRequest(context.Context, string, string, []byte, map[string]string) (*api.RawResponse, error) {
	return nil, errors.New("unexpected raw request")
}

func (c *rootTestClient) URL(path string) string {
	return path
}

func TestNewRootCommandWithDependenciesUsesInjectedService(t *testing.T) {
	client := &rootTestClient{}
	deps := &rootTestDependencies{
		logger:  slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		service: service.NewShipService(client, slog.Default()),
	}
	root := cmd.NewRootCommandWithDependencies("ship", cmd.BuildMetadata{}, deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--api-url", "https://example.test", "--api-key", "test-key", "--log-format", "json", "apps", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute root command: %v", err)
	}
	if client.path != "/api/apps/" {
		t.Fatalf("request path = %q, want %q", client.path, "/api/apps/")
	}
	if deps.credentials == nil {
		t.Fatal("expected injected dependency to receive credentials")
	}
	if deps.credentials.APIURL != "https://example.test" {
		t.Fatalf("API URL = %q, want %q", deps.credentials.APIURL, "https://example.test")
	}
	if deps.credentials.APIKey != "test-key" {
		t.Fatalf("API key = %q, want %q", deps.credentials.APIKey, "test-key")
	}
	if stdout.String() != "[]\n" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "[]\n")
	}
}
