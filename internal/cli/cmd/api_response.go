package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
)

// isTextualContentType reports whether a response body should be decoded as
// text. Anything not on the allowlist is kept as raw bytes so binary downloads
// are not corrupted by UTF-8 decoding. A missing Content-Type stays textual,
// because API endpoints commonly omit or under-specify it.
func isTextualContentType(contentType string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if mediaType == "" {
		return true
	}
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}

	switch mediaType {
	case "application/json",
		"application/yaml",
		"application/x-yaml",
		"application/javascript",
		"application/xml",
		"application/xhtml+xml":
		return true
	}

	// Structured suffixes: application/problem+json, application/atom+xml, …
	return strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml")
}

// decodeResponseBody parses a textual response body as JSON, falling back to the
// raw text when it is not JSON.
func decodeResponseBody(body []byte) any {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}

	parsed, err := decodeJSON(body)
	if err != nil {
		return string(body)
	}

	return parsed
}

// formatAPIResponse renders a decoded response body for human-readable output.
func formatAPIResponse(data any) string {
	if data == nil {
		return ""
	}
	if text, isText := data.(string); isText {
		return text
	}

	encoded, err := marshalJSONIndent(data)
	if err != nil {
		return fmt.Sprintf("%v", data)
	}

	return string(encoded)
}

// formatBinaryErrorBody summarizes a non-textual error body instead of dumping
// raw bytes into the terminal.
func formatBinaryErrorBody(status int, header http.Header, body []byte) string {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		contentType = "unknown"
	}

	return fmt.Sprintf("HTTP %d — binary error body (%s, %d bytes)", status, contentType, len(body))
}
