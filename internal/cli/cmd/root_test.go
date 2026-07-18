package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRootPositionalSlugTriggersFullPublish(t *testing.T) {
	requests := executeCommandWithTestAPI(t, "my-app")

	assertRequests(t, requests, []apiRequest{
		{path: "/api/app/my-app/publish"},
	})
}

func TestRootPositionalSlugKeepsPlatformSelection(t *testing.T) {
	requests := executeCommandWithTestAPI(t, "my-app", "--platform", "ios")

	assertRequests(t, requests, []apiRequest{
		{path: "/api/app/my-app/publish", platforms: []string{"ios"}},
	})
}

func TestPublishPositionalSlugTriggersFullPublish(t *testing.T) {
	requests := executeCommandWithTestAPI(t, "publish", "my-app")

	assertRequests(t, requests, []apiRequest{
		{path: "/api/app/my-app/publish"},
	})
}

func TestPublishPositionalSlugKeepsPlatformSelection(t *testing.T) {
	requests := executeCommandWithTestAPI(t, "publish", "my-app", "--platform", "ios")

	assertRequests(t, requests, []apiRequest{
		{path: "/api/app/my-app/publish", platforms: []string{"ios"}},
	})
}

func TestPublishPartialPositionalSlug(t *testing.T) {
	requests := executeCommandWithTestAPI(t, "publish", "metadata", "my-app")

	assertRequests(t, requests, []apiRequest{
		{path: "/api/app/my-app/publish/metadata"},
	})
}

func TestPublishStatusPositionalSlug(t *testing.T) {
	requests := executeCommandWithTestAPI(t, "publish", "status", "my-app")

	assertRequests(t, requests, []apiRequest{
		{path: "/api/app/my-app/publish/status"},
	})
}

func TestPublishValidatePositionalSlug(t *testing.T) {
	requests := executeCommandWithTestAPI(t, "publish", "validate", "my-app")

	assertRequests(t, requests, []apiRequest{
		{path: "/api/app/my-app/pre-publish/generate"},
	})
}

func TestPublishPositionalSlugRejectsExplicitIdentifier(t *testing.T) {
	cmd := NewRootCommand("ship", BuildMetadata{})
	cmd.SetArgs([]string{"publish", "my-app", "--app-slug", "other-app"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected conflict error")
	}
}

type apiRequest struct {
	path      string
	platforms []string
}

func executeCommandWithTestAPI(t *testing.T, args ...string) []apiRequest {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SHIP_API_KEY", "test-key")

	var requests []apiRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-Key"); got != "test-key" {
			t.Errorf("X-API-Key = %q, want test-key", got)
		}
		request := apiRequest{path: r.URL.Path}
		if r.Method == http.MethodPost {
			var payload struct {
				Platforms []string `json:"platforms"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode request body: %v", err)
			}
			request.platforms = payload.Platforms
		}
		requests = append(requests, request)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "running",
			})
		case http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"job_id": "job-123",
				"is_new": true,
			})
		default:
			http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("SHIP_API_URL", server.URL)

	cmd := NewRootCommand("ship", BuildMetadata{})
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(%v): %v", args, err)
	}

	return requests
}

func assertRequests(t *testing.T, got, want []apiRequest) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("requests = %v, want %v", got, want)
	}
	for i := range want {
		if got[i].path != want[i].path {
			t.Fatalf("requests = %v, want %v", got, want)
		}
		if len(got[i].platforms) != len(want[i].platforms) {
			t.Fatalf("requests = %v, want %v", got, want)
		}
		for j := range want[i].platforms {
			if got[i].platforms[j] != want[i].platforms[j] {
				t.Fatalf("requests = %v, want %v", got, want)
			}
		}
	}
}
