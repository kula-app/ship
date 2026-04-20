package cmd_auth

import (
	"net/url"
	"testing"

	"github.com/kula-app/ship/internal/cli/auth"
)

func TestBuildAuthURLIncludesOAuthParameters(t *testing.T) {
	authURL, err := buildAuthURL("https://example.com/auth?existing=1", "test-challenge", "test-state")
	if err != nil {
		t.Fatalf("build auth URL: %v", err)
	}

	parsedURL, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}

	query := parsedURL.Query()
	wantQuery := map[string]string{
		"client_id":             auth.ClientID,
		"response_type":         "code",
		"redirect_uri":          auth.RedirectURI,
		"code_challenge":        "test-challenge",
		"code_challenge_method": "S256",
		"scope":                 auth.DefaultScope,
		"state":                 "test-state",
		"existing":              "1",
	}

	for key, want := range wantQuery {
		if got := query.Get(key); got != want {
			t.Fatalf("query %q = %q, want %q", key, got, want)
		}
	}
}

func TestBuildAuthURLRejectsInvalidEndpoint(t *testing.T) {
	if _, err := buildAuthURL("%", "test-challenge", "test-state"); err == nil {
		t.Fatal("expected invalid auth endpoint to fail")
	}
}
