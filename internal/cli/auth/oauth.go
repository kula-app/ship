package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	// ClientID is the registered OAuth client ID for the ship CLI.
	// This is a public client (no secret) using PKCE.
	ClientID = "02c0cda0-835b-4c94-b090-0c64157b0ea7"

	// DefaultScope is the set of user scopes requested by the CLI.
	DefaultScope = "email profile"
)

// AuthEndpoints holds the discovered OAuth authorization and token endpoints.
type AuthEndpoints struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

// TokenResponse represents the response from the OAuth token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// DiscoverAuthEndpoints fetches OAuth endpoints from the well-known configuration URL.
//
// It expects the server to expose a JSON document at:
//
//	{apiURL}/.well-known/oauth-authorization-server
//
// The document must contain at minimum:
//   - authorization_endpoint: URL for user authorization
//   - token_endpoint: URL for token exchange
func DiscoverAuthEndpoints(apiURL string) (*AuthEndpoints, error) {
	wellKnownURL := strings.TrimRight(apiURL, "/") + "/.well-known/oauth-authorization-server"

	resp, err := http.Get(wellKnownURL)
	if err != nil {
		return nil, fmt.Errorf("failed to discover auth endpoints: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to discover auth endpoints: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read auth discovery response: %w", err)
	}

	var endpoints AuthEndpoints
	if err := json.Unmarshal(body, &endpoints); err != nil {
		return nil, fmt.Errorf("failed to parse auth discovery response: %w", err)
	}

	if endpoints.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("auth discovery response missing authorization_endpoint")
	}
	if endpoints.TokenEndpoint == "" {
		return nil, fmt.Errorf("auth discovery response missing token_endpoint")
	}

	// Supabase's OAuth 2.1 Server endpoints are under /oauth/ (e.g. /auth/v1/oauth/authorize),
	// but the well-known document may return the base auth endpoints (/auth/v1/authorize).
	// Rewrite to the correct OAuth 2.1 paths if needed.
	endpoints.AuthorizationEndpoint = rewriteToOAuthPath(endpoints.AuthorizationEndpoint)
	endpoints.TokenEndpoint = rewriteToOAuthPath(endpoints.TokenEndpoint)

	return &endpoints, nil
}

// rewriteToOAuthPath ensures that Supabase auth endpoints use the OAuth 2.1 Server
// path (/auth/v1/oauth/) instead of the base auth path (/auth/v1/).
// This is idempotent — endpoints already containing /oauth/ are left unchanged.
func rewriteToOAuthPath(endpoint string) string {
	if strings.Contains(endpoint, "/auth/v1/oauth/") {
		return endpoint
	}
	return strings.Replace(endpoint, "/auth/v1/", "/auth/v1/oauth/", 1)
}

// ExchangeCode exchanges an authorization code for tokens using the PKCE flow.
//
// It sends a POST request to the token endpoint with:
//   - grant_type: authorization_code
//   - code: the authorization code from the callback
//   - code_verifier: the PKCE code verifier
//   - redirect_uri: the local callback URL
func ExchangeCode(tokenEndpoint, code, codeVerifier, redirectURI string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {codeVerifier},
		"redirect_uri":  {redirectURI},
		"client_id":     {ClientID},
	}

	return postTokenRequest(tokenEndpoint, data)
}

// RefreshToken exchanges a refresh token for a new access token.
func RefreshToken(tokenEndpoint, refreshToken string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {ClientID},
	}

	return postTokenRequest(tokenEndpoint, data)
}

func postTokenRequest(tokenEndpoint string, data url.Values) (*TokenResponse, error) {
	resp, err := http.PostForm(tokenEndpoint, data)
	if err != nil {
		return nil, fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token")
	}
	if tokenResp.ExpiresIn <= 0 {
		return nil, fmt.Errorf("token response missing expires_in")
	}

	return &tokenResp, nil
}
