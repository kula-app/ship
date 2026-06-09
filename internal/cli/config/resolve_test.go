package config

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/auth"
	"github.com/kula-app/ship/internal/cli/db"
)

func TestAuthenticatedClientRefreshesExpiredCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var serverURL string
	var sawRefresh bool
	var sawResource bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"authorization_endpoint": serverURL + "/oauth/authorize",
				"token_endpoint":         serverURL + "/oauth/token",
			})
		case "/oauth/token":
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse refresh form: %v", err)
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			if got := r.Form.Get("grant_type"); got != "refresh_token" {
				t.Errorf("grant_type = %q, want refresh_token", got)
			}
			if got := r.Form.Get("refresh_token"); got != "stored-refresh-token" {
				t.Errorf("refresh_token = %q, want stored-refresh-token", got)
			}
			if got := r.Form.Get("client_id"); got != auth.ClientID {
				t.Errorf("client_id = %q, want %q", got, auth.ClientID)
			}

			sawRefresh = true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "new-access-token",
				"refresh_token": "new-refresh-token",
				"expires_in":    3600,
				"token_type":    "bearer",
			})
		case "/resource":
			if got := r.Header.Get("Authorization"); got != "Bearer new-access-token" {
				t.Errorf("Authorization = %q, want refreshed bearer token", got)
			}
			sawResource = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	t.Setenv("SHIP_API_URL", server.URL)
	defer db.CloseDB()

	if err := db.SetAuthToken("expired-access-token", "stored-refresh-token", -60); err != nil {
		t.Fatalf("set expired auth token: %v", err)
	}

	client, err := AuthenticatedClient(testRootCommand(""))
	if err != nil {
		t.Fatalf("authenticated client: %v", err)
	}

	body, err := client.Get("/resource")
	if err != nil {
		t.Fatalf("get resource: %v", err)
	}
	if got := strings.TrimSpace(string(body)); got != `{"ok":true}` {
		t.Fatalf("resource body = %q, want ok response", got)
	}
	if !sawRefresh {
		t.Fatal("expected expired credentials to be refreshed")
	}
	if !sawResource {
		t.Fatal("expected resource request with refreshed token")
	}

	credentials, err := db.GetAuthCredentials()
	if err != nil {
		t.Fatalf("get refreshed credentials: %v", err)
	}
	if credentials == nil {
		t.Fatal("expected refreshed credentials to be stored")
	}
	if credentials.AccessToken != "new-access-token" {
		t.Fatalf("stored access token = %q, want new-access-token", credentials.AccessToken)
	}
	if credentials.RefreshToken != "new-refresh-token" {
		t.Fatalf("stored refresh token = %q, want new-refresh-token", credentials.RefreshToken)
	}
}

func TestAuthenticatedClientUsesEnvAPIKeyWhenNoSessionExists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SHIP_API_KEY", "env-api-key")
	defer db.CloseDB()

	var sawResource bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want empty", got)
		}
		if got := r.Header.Get("X-API-Key"); got != "env-api-key" {
			t.Errorf("X-API-Key = %q, want env-api-key", got)
		}
		sawResource = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	t.Setenv("SHIP_API_URL", server.URL)

	client, err := AuthenticatedClient(testRootCommand(""))
	if err != nil {
		t.Fatalf("authenticated client: %v", err)
	}

	if _, err := client.Get("/resource"); err != nil {
		t.Fatalf("get resource: %v", err)
	}
	if !sawResource {
		t.Fatal("expected resource request")
	}
}

func TestAuthenticatedClientAPIKeyFlagOverridesEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SHIP_API_KEY", "env-api-key")
	defer db.CloseDB()

	var sawResource bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-Key"); got != "flag-api-key" {
			t.Errorf("X-API-Key = %q, want flag-api-key", got)
		}
		sawResource = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	t.Setenv("SHIP_API_URL", server.URL)

	client, err := AuthenticatedClient(testRootCommand("flag-api-key"))
	if err != nil {
		t.Fatalf("authenticated client: %v", err)
	}

	if _, err := client.Get("/resource"); err != nil {
		t.Fatalf("get resource: %v", err)
	}
	if !sawResource {
		t.Fatal("expected resource request")
	}
}

func TestAuthenticatedClientUsesAPIKeyWhenStoredCredentialsExpiredWithoutRefreshToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SHIP_API_KEY", "env-api-key")
	defer db.CloseDB()

	if err := db.SetAuthToken("expired-access-token", "", -60); err != nil {
		t.Fatalf("set expired auth token: %v", err)
	}

	var sawResource bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want empty", got)
		}
		if got := r.Header.Get("X-API-Key"); got != "env-api-key" {
			t.Errorf("X-API-Key = %q, want env-api-key", got)
		}
		sawResource = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	t.Setenv("SHIP_API_URL", server.URL)

	client, err := AuthenticatedClient(testRootCommand(""))
	if err != nil {
		t.Fatalf("authenticated client: %v", err)
	}

	if _, err := client.Get("/resource"); err != nil {
		t.Fatalf("get resource: %v", err)
	}
	if !sawResource {
		t.Fatal("expected resource request")
	}
}

func TestAuthenticatedClientSessionTakesPrecedenceOverAPIKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SHIP_API_KEY", "env-api-key")
	defer db.CloseDB()

	if err := db.SetAuthToken("stored-access-token", "stored-refresh-token", 3600); err != nil {
		t.Fatalf("set auth token: %v", err)
	}

	var sawResource bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer stored-access-token" {
			t.Errorf("Authorization = %q, want stored bearer token", got)
		}
		if got := r.Header.Get("X-API-Key"); got != "" {
			t.Errorf("X-API-Key = %q, want empty", got)
		}
		sawResource = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	t.Setenv("SHIP_API_URL", server.URL)

	client, err := AuthenticatedClient(testRootCommand("flag-api-key"))
	if err != nil {
		t.Fatalf("authenticated client: %v", err)
	}

	if _, err := client.Get("/resource"); err != nil {
		t.Fatalf("get resource: %v", err)
	}
	if !sawResource {
		t.Fatal("expected resource request")
	}
}

func TestResolveAPIURLFlagOverridesEnv(t *testing.T) {
	t.Setenv("SHIP_API_URL", "https://env.example.com")

	cmd := testRootCommand("")
	if err := cmd.PersistentFlags().Set("api-url", "https://flag.example.com"); err != nil {
		t.Fatalf("set api-url flag: %v", err)
	}

	if got := ResolveAPIURL(cmd); got != "https://flag.example.com" {
		t.Fatalf("ResolveAPIURL = %q, want flag value", got)
	}
}

func TestResolveAPIURLFallsBackToEnv(t *testing.T) {
	t.Setenv("SHIP_API_URL", "https://env.example.com")

	if got := ResolveAPIURL(testRootCommand("")); got != "https://env.example.com" {
		t.Fatalf("ResolveAPIURL = %q, want env value", got)
	}
}

func testRootCommand(apiKey string) *cobra.Command {
	cmd := &cobra.Command{Use: "ship"}
	cmd.PersistentFlags().String("api-url", "", "API base URL")
	cmd.PersistentFlags().String("api-key", "", "API key")
	if apiKey != "" {
		_ = cmd.PersistentFlags().Set("api-key", apiKey)
	}
	return cmd
}
