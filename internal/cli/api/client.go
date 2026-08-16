// Package api provides a reusable authenticated HTTP client for the Shipable API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is an authenticated HTTP client for the Shipable API.
type Client struct {
	apiURL     string
	token      string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new API client with the given base URL and bearer token.
func NewClient(apiURL, token string) *Client {
	return &Client{
		apiURL:     apiURL,
		token:      token,
		httpClient: &http.Client{},
	}
}

// NewAPIKeyClient creates a new API client authenticated with an API key.
func NewAPIKeyClient(apiURL, apiKey string) *Client {
	return &Client{
		apiURL:     apiURL,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// Get sends an authenticated GET request to the given path and returns the response body.
func (c *Client) Get(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.apiURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// Post sends an authenticated POST request with a JSON body and returns the response body.
func (c *Client) Post(path string, payload any) ([]byte, error) {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.apiURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.setAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// RawResponse is the complete result of a raw API request.
type RawResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// URL returns the absolute URL for the given API path.
func (c *Client) URL(path string) string {
	return c.apiURL + path
}

// RawRequest sends an authenticated request with an arbitrary method, body and
// extra headers, and returns the complete response. Unlike Get and Post, a
// non-2xx status code is not an error — callers inspect StatusCode themselves.
// Headers passed by the caller override the authentication headers.
func (c *Client) RawRequest(ctx context.Context, method, path string, body []byte, headers map[string]string) (*RawResponse, error) {
	var payload io.Reader
	if body != nil {
		payload = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.URL(path), payload)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.setAuthHeaders(req)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return &RawResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       respBody,
	}, nil
}

func (c *Client) setAuthHeaders(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
		return
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
}
