// Package api provides a reusable authenticated HTTP client for the Shipable API.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is an authenticated HTTP client for the Shipable API.
type Client struct {
	apiURL     string
	token      string
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

// Get sends an authenticated GET request to the given path and returns the response body.
func (c *Client) Get(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.apiURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

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

	req.Header.Set("Authorization", "Bearer "+c.token)
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
