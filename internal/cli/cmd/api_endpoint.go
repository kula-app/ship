package cmd

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
)

// apiPathPrefix is prepended to every endpoint before the request is sent.
const apiPathPrefix = "/api/"

var (
	// lineBreakPattern matches line breaks together with the indentation around
	// them, as produced by copy-pasting a multi-line endpoint from docs or a script.
	lineBreakPattern = regexp.MustCompile(`[ \t]*[\r\n]+[ \t]*`)

	// controlCharPattern matches ASCII control characters (0x00-0x1F). They are
	// invisible and have no valid use in an endpoint.
	controlCharPattern = regexp.MustCompile(`[\x00-\x1f]`)

	// pathTraversalPattern matches ".." segments anchored to segment boundaries.
	pathTraversalPattern = regexp.MustCompile(`(^|/)\.\.(/|$)`)
)

// validMethods are the HTTP methods accepted by --method.
var validMethods = []string{
	http.MethodGet,
	http.MethodPost,
	http.MethodPut,
	http.MethodDelete,
	http.MethodPatch,
}

// parseMethod normalizes an HTTP method to upper case and rejects methods the
// API does not serve.
func parseMethod(value string) (string, error) {
	upper := strings.ToUpper(value)
	if !slices.Contains(validMethods, upper) {
		return "", fmt.Errorf("invalid method: %s. Must be one of: %s", value, strings.Join(validMethods, ", "))
	}

	return upper, nil
}

// normalizedEndpoint is a user-supplied endpoint after cleaning and validation.
type normalizedEndpoint struct {
	// Path is the endpoint relative to the API root, without a leading slash.
	Path string
	// StrippedLineBreaks reports whether copy-paste line breaks were removed.
	StrippedLineBreaks bool
	// StrippedPrefix reports whether a redundant "api/" prefix was removed.
	StrippedPrefix bool
}

// normalizeEndpoint cleans and validates a user-supplied endpoint.
//
// Line breaks and their surrounding indentation are stripped because endpoints
// are commonly pasted out of multi-line snippets. An "api/" prefix is stripped
// as well — it is added back when the request is built, and keeping it would
// produce a doubled "/api/api/..." path. Other control characters are rejected
// rather than cleaned: they indicate corruption, not a copy-paste artifact.
//
// Unlike the Sentry API, the Shipable API does not require trailing slashes, so
// the remaining path is passed through verbatim.
func normalizeEndpoint(raw string) (normalizedEndpoint, error) {
	cleaned := strings.TrimSpace(lineBreakPattern.ReplaceAllString(raw, ""))

	if match := controlCharPattern.FindString(cleaned); match != "" {
		return normalizedEndpoint{}, fmt.Errorf(
			"invalid API endpoint: contains a control character (0x%02x).\n  Endpoints must not contain control characters",
			match[0],
		)
	}
	if pathTraversalPattern.MatchString(cleaned) {
		return normalizedEndpoint{}, fmt.Errorf(
			"invalid API endpoint: contains %q path traversal.\n  Use plain API paths (e.g., apps/)",
			"..",
		)
	}

	endpoint := normalizedEndpoint{StrippedLineBreaks: cleaned != raw}

	path := strings.TrimPrefix(cleaned, "/")
	switch {
	case path == "api":
		path = ""
		endpoint.StrippedPrefix = true
	case strings.HasPrefix(path, "api/"):
		path = strings.TrimPrefix(path, "api/")
		endpoint.StrippedPrefix = true
	}
	endpoint.Path = path

	return endpoint, nil
}
