package cmd_api

import (
	"strings"
	"testing"
)

// bodyJSON renders a parsed body as compact JSON for comparison in tests.
func bodyJSON(t *testing.T, value any) string {
	t.Helper()

	encoded, err := marshalJSON(value)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	return string(encoded)
}

func TestParseFieldsBuildsNestedStructures(t *testing.T) {
	tests := map[string]struct {
		fields []string
		want   string
	}{
		"simple":       {fields: []string{"name=ship"}, want: `{"name":"ship"}`},
		"json value":   {fields: []string{"count=10", "enabled=true"}, want: `{"count":10,"enabled":true}`},
		"nested":       {fields: []string{"options[sampleRate]=0.5"}, want: `{"options":{"sampleRate":0.5}}`},
		"deep nested":  {fields: []string{"a[b][c]=1"}, want: `{"a":{"b":{"c":1}}}`},
		"array append": {fields: []string{"tags[]=ios", "tags[]=android"}, want: `{"tags":["ios","android"]}`},
		"empty array":  {fields: []string{"tags[]"}, want: `{"tags":[]}`},
		"json object":  {fields: []string{`meta={"a":1}`}, want: `{"meta":{"a":1}}`},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			body, err := parseFields(test.fields, false)
			if err != nil {
				t.Fatalf("parse fields: %v", err)
			}
			if got := bodyJSON(t, body); got != test.want {
				t.Fatalf("body = %s, want %s", got, test.want)
			}
		})
	}
}

func TestParseFieldsKeepsRawValuesAsStrings(t *testing.T) {
	body, err := parseFields([]string{"count=10", "enabled=true"}, true)
	if err != nil {
		t.Fatalf("parse fields: %v", err)
	}
	if got := bodyJSON(t, body); got != `{"count":"10","enabled":"true"}` {
		t.Fatalf("body = %s, want raw string values", got)
	}
}

func TestParseFieldsPreservesLargeIntegers(t *testing.T) {
	body, err := parseFields([]string{"id=9007199254740993"}, false)
	if err != nil {
		t.Fatalf("parse fields: %v", err)
	}
	if got := bodyJSON(t, body); got != `{"id":9007199254740993}` {
		t.Fatalf("body = %s, want the integer to survive unchanged", got)
	}
}

func TestParseFieldsRejectsInvalidInput(t *testing.T) {
	tests := map[string]struct {
		fields   []string
		contains string
	}{
		"missing separator":      {fields: []string{"status"}, contains: "Expected key=value"},
		"empty brackets midway":  {fields: []string{"a[][b]=1"}, contains: "empty brackets"},
		"array over map":         {fields: []string{"a[b]=1", "a[]=2"}, contains: `expected array type under "a"`},
		"map over primitive":     {fields: []string{"a=1", "a[b]=2"}, contains: `expected map type under "a"`},
		"unbalanced bracket key": {fields: []string{"a[b=1"}, contains: "invalid field key format"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parseFields(test.fields, false)
			if err == nil {
				t.Fatalf("expected %v to fail", test.fields)
			}
			if !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("error = %q, want it to contain %q", err, test.contains)
			}
		})
	}
}

func TestBuildBodyFromFieldsLetsRawFieldsOverwriteTypedFields(t *testing.T) {
	body, err := buildBodyFromFields([]string{"count=10"}, []string{"count=10"})
	if err != nil {
		t.Fatalf("build body: %v", err)
	}
	if got := bodyJSON(t, body); got != `{"count":"10"}` {
		t.Fatalf("body = %s, want the raw field to win", got)
	}
}

func TestNormalizeFieldsCorrectsColonSeparator(t *testing.T) {
	fields, warnings := normalizeFields([]string{"status:resolved", "since:2026-02-25T11:20:00"})
	want := []string{"status=resolved", "since=2026-02-25T11:20:00"}

	for i, field := range fields {
		if field != want[i] {
			t.Fatalf("field %d = %q, want %q", i, field, want[i])
		}
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %d, want 2", len(warnings))
	}
}

func TestNormalizeFieldsLeavesValidAndJSONFieldsAlone(t *testing.T) {
	input := []string{"status=resolved", "tags[]", `{"status":"resolved"}`, ":leading"}
	fields, warnings := normalizeFields(input)

	for i, field := range fields {
		if field != input[i] {
			t.Fatalf("field %d = %q, want it unchanged", i, field)
		}
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
}

func TestParseHeadersTrimsWhitespace(t *testing.T) {
	headers, err := parseHeaders([]string{"X-Trace-Id: abc", "Content-Type:application/xml"})
	if err != nil {
		t.Fatalf("parse headers: %v", err)
	}
	if headers["X-Trace-Id"] != "abc" {
		t.Fatalf("X-Trace-Id = %q, want %q", headers["X-Trace-Id"], "abc")
	}
	if headers["Content-Type"] != "application/xml" {
		t.Fatalf("Content-Type = %q, want %q", headers["Content-Type"], "application/xml")
	}
}

func TestParseHeadersRejectsMissingSeparator(t *testing.T) {
	if _, err := parseHeaders([]string{"X-Trace-Id"}); err == nil {
		t.Fatal("expected header without ':' to fail")
	}
}

func TestBuildQueryParamsFromFieldsExpandsArraysAndRawValues(t *testing.T) {
	params, err := buildQueryParamsFromFields(
		[]string{`platforms=["ios","android"]`, "limit=10"},
		[]string{"cursor=abc"},
	)
	if err != nil {
		t.Fatalf("build query params: %v", err)
	}
	if got, want := params.Encode(), "cursor=abc&limit=10&platforms=ios&platforms=android"; got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}

func TestPrepareRequestOptionsRoutesFieldsByMethod(t *testing.T) {
	body, params, err := prepareRequestOptions("GET", []string{"limit=10"}, nil)
	if err != nil {
		t.Fatalf("prepare request options: %v", err)
	}
	if body != nil {
		t.Fatalf("body = %v, want none for GET", body)
	}
	if got := params.Encode(); got != "limit=10" {
		t.Fatalf("query = %q, want %q", got, "limit=10")
	}

	body, params, err = prepareRequestOptions("POST", []string{"limit=10"}, nil)
	if err != nil {
		t.Fatalf("prepare request options: %v", err)
	}
	if params != nil {
		t.Fatalf("params = %v, want none for POST", params)
	}
	if got := bodyJSON(t, body); got != `{"limit":10}` {
		t.Fatalf("body = %s, want %s", got, `{"limit":10}`)
	}
}
