package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRootPositionalSlugTriggersFullPublish(t *testing.T) {
	paths := executeCommandWithTestAPI(t, "my-app")

	assertPaths(t, paths, []string{"/api/app/my-app/publish"})
}

func TestPublishPositionalSlugTriggersFullPublish(t *testing.T) {
	paths := executeCommandWithTestAPI(t, "publish", "my-app")

	assertPaths(t, paths, []string{"/api/app/my-app/publish"})
}

func TestPublishPositionalSlugKeepsPlatformSelection(t *testing.T) {
	paths := executeCommandWithTestAPI(t, "publish", "my-app", "--platform", "ios")

	assertPaths(t, paths, []string{"/api/app/my-app/publish"})
}

func TestPublishPartialPositionalSlug(t *testing.T) {
	paths := executeCommandWithTestAPI(t, "publish", "metadata", "my-app")

	assertPaths(t, paths, []string{"/api/app/my-app/publish/metadata"})
}

func TestPublishStatusPositionalSlug(t *testing.T) {
	paths := executeCommandWithTestAPI(t, "publish", "status", "my-app")

	assertPaths(t, paths, []string{"/api/app/my-app/publish/status"})
}

func TestPublishValidatePositionalSlug(t *testing.T) {
	paths := executeCommandWithTestAPI(t, "publish", "validate", "my-app")

	assertPaths(t, paths, []string{"/api/app/my-app/pre-publish/generate"})
}

func TestPublishPositionalSlugRejectsExplicitIdentifier(t *testing.T) {
	cmd := NewRootCommand("ship", BuildMetadata{})
	cmd.SetArgs([]string{"publish", "my-app", "--app-slug", "other-app"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected conflict error")
	}
}

func executeCommandWithTestAPI(t *testing.T, args ...string) []string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SHIP_API_KEY", "test-key")

	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-Key"); got != "test-key" {
			t.Errorf("X-API-Key = %q, want test-key", got)
		}
		paths = append(paths, r.URL.Path)
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

	return paths
}

func assertPaths(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paths = %v, want %v", got, want)
		}
	}
}
