package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newRecordingServer returns a server that records the last request it received
// and replies with the given status and body.
func newRecordingServer(t *testing.T, status int, body string) (*httptest.Server, *http.Request, *string) {
	t.Helper()

	received := &http.Request{}
	requestBody := new(string)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		read, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		*requestBody = string(read)
		*received = *r.Clone(r.Context())

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if _, err := io.WriteString(w, body); err != nil {
			t.Errorf("write response body: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server, received, requestBody
}

func TestRawRequestReturnsErrorStatusWithoutFailing(t *testing.T) {
	server, _, _ := newRecordingServer(t, http.StatusNotFound, `{"detail":"not found"}`)

	client := NewClient(server.URL, "token")
	response, err := client.RawRequest(context.Background(), http.MethodGet, "/api/apps/", nil, nil)
	if err != nil {
		t.Fatalf("raw request: %v", err)
	}

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
	if string(response.Body) != `{"detail":"not found"}` {
		t.Fatalf("body = %q, want the error payload", response.Body)
	}
	if response.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q, want the response header to be exposed", response.Header.Get("Content-Type"))
	}
}

func TestRawRequestSendsBearerToken(t *testing.T) {
	server, received, _ := newRecordingServer(t, http.StatusOK, `{}`)

	client := NewClient(server.URL, "token")
	if _, err := client.RawRequest(context.Background(), http.MethodGet, "/api/apps/", nil, nil); err != nil {
		t.Fatalf("raw request: %v", err)
	}

	if received.Header.Get("Authorization") != "Bearer token" {
		t.Fatalf("Authorization = %q, want %q", received.Header.Get("Authorization"), "Bearer token")
	}
}

func TestRawRequestSendsAPIKey(t *testing.T) {
	server, received, _ := newRecordingServer(t, http.StatusOK, `{}`)

	client := NewAPIKeyClient(server.URL, "test-key")
	if _, err := client.RawRequest(context.Background(), http.MethodGet, "/api/apps/", nil, nil); err != nil {
		t.Fatalf("raw request: %v", err)
	}

	if received.Header.Get("X-API-Key") != "test-key" {
		t.Fatalf("X-API-Key = %q, want %q", received.Header.Get("X-API-Key"), "test-key")
	}
}

func TestRawRequestLetsCallerHeadersOverrideAuthentication(t *testing.T) {
	server, received, _ := newRecordingServer(t, http.StatusOK, `{}`)

	client := NewClient(server.URL, "token")
	headers := map[string]string{"Authorization": "Bearer override"}
	if _, err := client.RawRequest(context.Background(), http.MethodGet, "/api/apps/", nil, headers); err != nil {
		t.Fatalf("raw request: %v", err)
	}

	if received.Header.Get("Authorization") != "Bearer override" {
		t.Fatalf("Authorization = %q, want the caller header to win", received.Header.Get("Authorization"))
	}
}

func TestRawRequestSendsBodyAndHeaders(t *testing.T) {
	server, received, body := newRecordingServer(t, http.StatusOK, `{}`)

	client := NewClient(server.URL, "token")
	headers := map[string]string{"Content-Type": "application/json"}
	_, err := client.RawRequest(
		context.Background(),
		http.MethodPost,
		"/api/app/1/publish",
		[]byte(`{"platforms":["ios"]}`),
		headers,
	)
	if err != nil {
		t.Fatalf("raw request: %v", err)
	}

	if received.Method != http.MethodPost {
		t.Fatalf("method = %q, want %q", received.Method, http.MethodPost)
	}
	if *body != `{"platforms":["ios"]}` {
		t.Fatalf("body = %q, want the payload", *body)
	}
	if received.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", received.Header.Get("Content-Type"), "application/json")
	}
}

func TestRawRequestReportsTransportFailure(t *testing.T) {
	server, _, _ := newRecordingServer(t, http.StatusOK, `{}`)
	serverURL := server.URL
	server.Close()

	client := NewClient(serverURL, "token")
	if _, err := client.RawRequest(context.Background(), http.MethodGet, "/api/apps/", nil, nil); err == nil {
		t.Fatal("expected a transport failure to be reported")
	}
}

func TestRawRequestHonorsContextCancellation(t *testing.T) {
	server, _, _ := newRecordingServer(t, http.StatusOK, `{}`)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewClient(server.URL, "token")
	if _, err := client.RawRequest(ctx, http.MethodGet, "/api/apps/", nil, nil); err == nil {
		t.Fatal("expected a cancelled context to abort the request")
	}
}

func TestURLJoinsBaseAndPath(t *testing.T) {
	client := NewClient("https://api.shipable.dev", "token")

	if got := client.URL("/api/apps/"); got != "https://api.shipable.dev/api/apps/" {
		t.Fatalf("URL = %q, want %q", got, "https://api.shipable.dev/api/apps/")
	}
}
