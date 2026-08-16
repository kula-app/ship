package cmd_auth

import (
	"errors"
	"net/url"
	"strings"
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

func TestValidateCallbackResultReportsProviderErrorFirst(t *testing.T) {
	// A provider error must survive even when the state is missing or altered,
	// otherwise the actionable message is masked by a generic state error.
	tests := map[string]auth.CallbackResult{
		"missing state":    {Error: "invalid_client", State: ""},
		"mismatched state": {Error: "invalid_client", State: "other-state"},
		"matching state":   {Error: "invalid_client", State: "test-state"},
	}

	for name, result := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateCallbackResult(result, "test-state")
			if err == nil {
				t.Fatal("expected a provider error to fail the callback")
			}
			if !errors.Is(err, errProviderRejected) {
				t.Fatalf("error = %v, want it to wrap errProviderRejected", err)
			}
			if !strings.Contains(err.Error(), "invalid_client") {
				t.Fatalf("error = %v, want it to carry the provider message", err)
			}
		})
	}
}

func TestValidateCallbackResultRejectsBadState(t *testing.T) {
	tests := map[string]struct {
		result   auth.CallbackResult
		contains string
	}{
		"missing state":    {result: auth.CallbackResult{Code: "code"}, contains: "missing OAuth state"},
		"mismatched state": {result: auth.CallbackResult{Code: "code", State: "attacker"}, contains: "invalid OAuth state"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateCallbackResult(test.result, "test-state")
			if err == nil {
				t.Fatal("expected a bad state to fail the callback")
			}
			if errors.Is(err, errProviderRejected) {
				t.Fatalf("error = %v, want a state error rather than a provider error", err)
			}
			if !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("error = %v, want it to contain %q", err, test.contains)
			}
		})
	}
}

func TestValidateCallbackResultAcceptsMatchingState(t *testing.T) {
	result := auth.CallbackResult{Code: "code", State: "test-state"}

	if err := validateCallbackResult(result, "test-state"); err != nil {
		t.Fatalf("validate callback result: %v", err)
	}
}
