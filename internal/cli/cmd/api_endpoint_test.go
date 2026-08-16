package cmd

import (
	"strings"
	"testing"
)

func TestParseMethodNormalizesCase(t *testing.T) {
	method, err := parseMethod("post")
	if err != nil {
		t.Fatalf("parse method: %v", err)
	}
	if method != "POST" {
		t.Fatalf("method = %q, want %q", method, "POST")
	}
}

func TestParseMethodRejectsUnsupportedMethod(t *testing.T) {
	_, err := parseMethod("TRACE")
	if err == nil {
		t.Fatal("expected unsupported method to fail")
	}
	if !strings.Contains(err.Error(), "GET, POST, PUT, DELETE, PATCH") {
		t.Fatalf("error = %q, want it to list the supported methods", err)
	}
}

func TestNormalizeEndpointStripsLeadingSlashAndAPIPrefix(t *testing.T) {
	tests := map[string]struct {
		input          string
		wantPath       string
		wantPrefixFlag bool
	}{
		"plain":             {input: "apps/", wantPath: "apps/"},
		"leading slash":     {input: "/apps/", wantPath: "apps/"},
		"api prefix":        {input: "api/apps/", wantPath: "apps/", wantPrefixFlag: true},
		"absolute api":      {input: "/api/app/1/publish", wantPath: "app/1/publish", wantPrefixFlag: true},
		"bare api":          {input: "api", wantPath: "", wantPrefixFlag: true},
		"no trailing slash": {input: "app/1/publish", wantPath: "app/1/publish"},
		"query string":      {input: "apps/?limit=10", wantPath: "apps/?limit=10"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			endpoint, err := normalizeEndpoint(test.input)
			if err != nil {
				t.Fatalf("normalize endpoint: %v", err)
			}
			if endpoint.Path != test.wantPath {
				t.Fatalf("path = %q, want %q", endpoint.Path, test.wantPath)
			}
			if endpoint.StrippedPrefix != test.wantPrefixFlag {
				t.Fatalf("stripped prefix = %v, want %v", endpoint.StrippedPrefix, test.wantPrefixFlag)
			}
		})
	}
}

func TestNormalizeEndpointStripsCopyPasteLineBreaks(t *testing.T) {
	endpoint, err := normalizeEndpoint("  app/1/\n    publish  ")
	if err != nil {
		t.Fatalf("normalize endpoint: %v", err)
	}
	if endpoint.Path != "app/1/publish" {
		t.Fatalf("path = %q, want %q", endpoint.Path, "app/1/publish")
	}
	if !endpoint.StrippedLineBreaks {
		t.Fatal("expected line break stripping to be reported")
	}
}

func TestNormalizeEndpointRejectsPathTraversal(t *testing.T) {
	if _, err := normalizeEndpoint("apps/../../etc/passwd"); err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
}

func TestNormalizeEndpointRejectsControlCharacters(t *testing.T) {
	if _, err := normalizeEndpoint("apps/\x00"); err == nil {
		t.Fatal("expected control character to be rejected")
	}
}
