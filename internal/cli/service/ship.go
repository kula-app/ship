package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/getsentry/sentry-go"

	"github.com/kula-app/ship/internal/cli/api"
)

// ShipService provides business logic for interacting with the Shipable API.
// It abstracts away the API interactions and provides a clean interface for
// the CLI commands.
type ShipService struct {
	client Client
	logger *slog.Logger
}

// App describes an app as returned by the apps listing endpoint.
type App struct {
	AppID   string  `json:"app_id"`
	Slug    *string `json:"slug"`
	AppName *string `json:"app_name"`
}

// PublishJobResult contains the result of triggering a publish job.
type PublishJobResult struct {
	JobID string `json:"job_id"`
	IsNew bool   `json:"is_new"`
}

// TaskStatus is the status of a single task within a publish job.
type TaskStatus struct {
	Status string `json:"status"`
}

// PublishStatus is the status of a publish job and its individual tasks.
type PublishStatus struct {
	Status string                `json:"status"`
	Tasks  map[string]TaskStatus `json:"tasks,omitempty"`
}

// publishJobRequest is the payload sent when triggering a publish job.
type publishJobRequest struct {
	Platforms []string `json:"platforms,omitempty"`
}

// NewShipService creates a new ShipService with the given dependencies.
func NewShipService(client Client, logger *slog.Logger) *ShipService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ShipService{
		client: client,
		logger: logger,
	}
}

// GetLogger returns the logger used by the service.
func (s *ShipService) GetLogger() *slog.Logger {
	return s.logger
}

// ListApps fetches all apps visible to the authenticated user.
// The raw response body is returned alongside the parsed apps so callers can
// emit the untouched payload in JSON mode.
func (s *ShipService) ListApps(ctx context.Context) ([]App, []byte, error) {
	span := sentry.StartSpan(ctx, "function", sentry.WithDescription("List apps via API"))
	defer span.Finish()

	s.logger.InfoContext(ctx, "fetching apps")
	body, err := s.client.Get("/api/apps/")
	if err != nil {
		span.Status = sentry.SpanStatusInternalError
		return nil, nil, fmt.Errorf("failed to fetch apps: %w", err)
	}

	var apps []App
	if err := json.Unmarshal(body, &apps); err != nil {
		span.Status = sentry.SpanStatusInternalError
		return nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	span.SetData("app_count", len(apps))
	span.Status = sentry.SpanStatusOK
	s.logger.InfoContext(ctx, "apps retrieved", "app_count", len(apps))

	return apps, body, nil
}

// TriggerPublish starts a full publish for the given app and platforms.
func (s *ShipService) TriggerPublish(ctx context.Context, appID string, platforms []string) (*PublishJobResult, []byte, error) {
	return s.triggerPublish(ctx, fmt.Sprintf("/api/app/%s/publish", appID), "publish", appID, platforms)
}

// TriggerPartialPublish starts a publish limited to a single variant, such as
// metadata, screenshots or the app binary.
func (s *ShipService) TriggerPartialPublish(ctx context.Context, appID, variant string, platforms []string) (*PublishJobResult, []byte, error) {
	return s.triggerPublish(
		ctx,
		fmt.Sprintf("/api/app/%s/publish/%s", appID, variant),
		fmt.Sprintf("%s publish", variant),
		appID,
		platforms,
	)
}

// TriggerValidation generates the Xcode project to verify the app configuration
// before publishing.
func (s *ShipService) TriggerValidation(ctx context.Context, appID string, platforms []string) (*PublishJobResult, []byte, error) {
	return s.triggerPublish(
		ctx,
		fmt.Sprintf("/api/app/%s/pre-publish/generate", appID),
		"validation",
		appID,
		platforms,
	)
}

// triggerPublish posts a publish request and parses the resulting job.
func (s *ShipService) triggerPublish(ctx context.Context, path, action, appID string, platforms []string) (*PublishJobResult, []byte, error) {
	span := sentry.StartSpan(ctx, "function", sentry.WithDescription(fmt.Sprintf("Trigger %s via API", action)))
	span.SetData("app_id", appID)
	span.SetData("platforms", platforms)
	defer span.Finish()

	s.logger.InfoContext(ctx, "triggering "+action, "app_id", appID, "platforms", platforms)
	body, err := s.client.Post(path, publishJobRequest{Platforms: platforms})
	if err != nil {
		span.Status = sentry.SpanStatusInternalError
		return nil, nil, fmt.Errorf("failed to trigger %s: %w", action, err)
	}

	var result PublishJobResult
	if err := json.Unmarshal(body, &result); err != nil {
		span.Status = sentry.SpanStatusInternalError
		return nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	span.SetData("job_id", result.JobID)
	span.SetData("is_new", result.IsNew)
	span.Status = sentry.SpanStatusOK
	s.logger.InfoContext(ctx, action+" triggered", "app_id", appID, "job_id", result.JobID, "is_new", result.IsNew)

	return &result, body, nil
}

// GetPublishStatus fetches the status of the app's current publish job.
func (s *ShipService) GetPublishStatus(ctx context.Context, appID string) (*PublishStatus, []byte, error) {
	span := sentry.StartSpan(ctx, "function", sentry.WithDescription("Get publish status via API"))
	span.SetData("app_id", appID)
	defer span.Finish()

	s.logger.InfoContext(ctx, "checking publish status", "app_id", appID)
	body, err := s.client.Get(fmt.Sprintf("/api/app/%s/publish/status", appID))
	if err != nil {
		span.Status = sentry.SpanStatusInternalError
		return nil, nil, fmt.Errorf("failed to fetch publish status: %w", err)
	}

	var status PublishStatus
	if err := json.Unmarshal(body, &status); err != nil {
		span.Status = sentry.SpanStatusInternalError
		return nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	span.SetData("publish_status", status.Status)
	span.Status = sentry.SpanStatusOK
	s.logger.InfoContext(ctx, "publish status retrieved", "app_id", appID, "status", status.Status)

	return &status, body, nil
}

// SendRawRequest performs an arbitrary authenticated API request on behalf of
// the api command. Non-2xx responses are returned rather than treated as errors.
func (s *ShipService) SendRawRequest(ctx context.Context, method, path string, body []byte, headers map[string]string) (*api.RawResponse, error) {
	span := sentry.StartSpan(ctx, "function", sentry.WithDescription("Send raw API request"))
	span.SetData("method", method)
	span.SetData("path", path)
	defer span.Finish()

	s.logger.InfoContext(ctx, "sending raw API request", "method", method, "path", path)
	response, err := s.client.RawRequest(ctx, method, path, body, headers)
	if err != nil {
		span.Status = sentry.SpanStatusInternalError
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	span.SetData("status_code", response.StatusCode)
	span.Status = sentry.SpanStatusOK
	s.logger.InfoContext(ctx, "raw API request completed", "method", method, "path", path, "status_code", response.StatusCode)

	return response, nil
}

// ResolveURL returns the absolute URL the client would use for the given path.
func (s *ShipService) ResolveURL(path string) string {
	return s.client.URL(path)
}
