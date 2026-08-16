package cmd_api

import (
	"net/http"
	"testing"
)

func TestIsTextualContentType(t *testing.T) {
	tests := map[string]bool{
		"":                                true,
		"application/json":                true,
		"application/json; charset=utf-8": true,
		"text/plain":                      true,
		"application/problem+json":        true,
		"application/atom+xml":            true,
		"image/png":                       false,
		"application/octet-stream":        false,
	}

	for contentType, want := range tests {
		t.Run(contentType, func(t *testing.T) {
			if got := isTextualContentType(contentType); got != want {
				t.Fatalf("isTextualContentType(%q) = %v, want %v", contentType, got, want)
			}
		})
	}
}

func TestFormatAPIResponseIndentsJSON(t *testing.T) {
	body := decodeResponseBody([]byte(`{"app_id":"1"}`))

	want := "{\n  \"app_id\": \"1\"\n}"
	if got := formatAPIResponse(body); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFormatAPIResponsePassesThroughNonJSON(t *testing.T) {
	body := decodeResponseBody([]byte("not json"))

	if got := formatAPIResponse(body); got != "not json" {
		t.Fatalf("output = %q, want the raw text", got)
	}
}

func TestFormatAPIResponseHandlesEmptyBody(t *testing.T) {
	if got := formatAPIResponse(decodeResponseBody(nil)); got != "" {
		t.Fatalf("output = %q, want an empty string", got)
	}
}

func TestFormatBinaryErrorBodySummarizesInsteadOfDumping(t *testing.T) {
	header := http.Header{"Content-Type": []string{"image/png"}}

	want := "HTTP 500 — binary error body (image/png, 3 bytes)"
	if got := formatBinaryErrorBody(http.StatusInternalServerError, header, []byte{1, 2, 3}); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestBuildRequestPathAppendsQueryString(t *testing.T) {
	payload := requestPayload{}
	if got := buildRequestPath("apps/", payload); got != "/api/apps/" {
		t.Fatalf("path = %q, want %q", got, "/api/apps/")
	}

	payload.Params = map[string][]string{"limit": {"10"}}
	if got := buildRequestPath("apps/", payload); got != "/api/apps/?limit=10" {
		t.Fatalf("path = %q, want %q", got, "/api/apps/?limit=10")
	}

	if got := buildRequestPath("apps/?cursor=abc", payload); got != "/api/apps/?cursor=abc&limit=10" {
		t.Fatalf("path = %q, want the query parameters merged", got)
	}
}
