package cmd_api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveBodyRejectsConflictingFlags(t *testing.T) {
	tests := map[string]bodyFlags{
		"data and input":  {Method: "POST", Data: "{}", HasData: true, Input: "body.json", HasInput: true},
		"data and fields": {Method: "POST", Data: "{}", HasData: true, Fields: []string{"a=1"}},
		"data and raw":    {Method: "POST", Data: "{}", HasData: true, RawFields: []string{"a=1"}},
	}

	for name, flags := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := resolveBody(flags, strings.NewReader("")); err == nil {
				t.Fatal("expected conflicting flags to fail")
			}
		})
	}
}

func TestResolveBodyParsesInlineData(t *testing.T) {
	payload, err := resolveBody(bodyFlags{
		Method:  "POST",
		Data:    `{"platforms":["ios"]}`,
		HasData: true,
	}, strings.NewReader(""))
	if err != nil {
		t.Fatalf("resolve body: %v", err)
	}
	if got := bodyJSON(t, payload.Body); got != `{"platforms":["ios"]}` {
		t.Fatalf("body = %s, want the parsed JSON object", got)
	}
}

func TestResolveBodySendsNonJSONDataVerbatim(t *testing.T) {
	payload, err := resolveBody(bodyFlags{
		Method:  "POST",
		Data:    "plain text",
		HasData: true,
	}, strings.NewReader(""))
	if err != nil {
		t.Fatalf("resolve body: %v", err)
	}

	encoded, err := encodeBody(payload)
	if err != nil {
		t.Fatalf("encode body: %v", err)
	}
	if string(encoded) != "plain text" {
		t.Fatalf("body = %q, want it sent verbatim", encoded)
	}

	headers := resolveEffectiveHeaders(nil, payload)
	if _, exists := headers["Content-Type"]; exists {
		t.Fatal("expected no Content-Type to be added for a string body")
	}
}

func TestResolveBodyConvertsDataToQueryParamsForGET(t *testing.T) {
	tests := map[string]struct {
		data string
		want string
	}{
		"url encoded": {data: "stat=received&resolution=1d", want: "resolution=1d&stat=received"},
		"json object": {data: `{"limit":10,"query":"ios"}`, want: "limit=10&query=ios"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			payload, err := resolveBody(bodyFlags{Method: "GET", Data: test.data, HasData: true}, strings.NewReader(""))
			if err != nil {
				t.Fatalf("resolve body: %v", err)
			}
			if payload.HasBody {
				t.Fatal("expected GET to carry no body")
			}
			if got := payload.Params.Encode(); got != test.want {
				t.Fatalf("query = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveBodyRejectsJSONArrayDataForGET(t *testing.T) {
	_, err := resolveBody(bodyFlags{Method: "GET", Data: `["a","b"]`, HasData: true}, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected a JSON array to be rejected as query parameters")
	}
	if !strings.Contains(err.Error(), "--method POST") {
		t.Fatalf("error = %q, want it to suggest --method POST", err)
	}
}

func TestResolveBodyReadsInputFromStdin(t *testing.T) {
	payload, err := resolveBody(bodyFlags{
		Method:   "POST",
		Input:    "-",
		HasInput: true,
	}, strings.NewReader(`{"platforms":["ios"]}`))
	if err != nil {
		t.Fatalf("resolve body: %v", err)
	}
	if got := bodyJSON(t, payload.Body); got != `{"platforms":["ios"]}` {
		t.Fatalf("body = %s, want the stdin payload", got)
	}
}

func TestResolveBodyReadsInputFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, []byte(`{"platforms":["android"]}`), 0o600); err != nil {
		t.Fatalf("write body file: %v", err)
	}

	payload, err := resolveBody(bodyFlags{Method: "POST", Input: path, HasInput: true}, strings.NewReader(""))
	if err != nil {
		t.Fatalf("resolve body: %v", err)
	}
	if got := bodyJSON(t, payload.Body); got != `{"platforms":["android"]}` {
		t.Fatalf("body = %s, want the file payload", got)
	}
}

func TestResolveBodyReportsMissingInputFile(t *testing.T) {
	_, err := resolveBody(bodyFlags{
		Method:   "POST",
		Input:    filepath.Join(t.TempDir(), "missing.json"),
		HasInput: true,
	}, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected a missing input file to fail")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("error = %q, want it to report a missing file", err)
	}
}

func TestResolveBodyDetectsBareJSONInRawField(t *testing.T) {
	payload, err := resolveBody(bodyFlags{
		Method:    "PUT",
		RawFields: []string{`{"status":"resolved"}`},
	}, strings.NewReader(""))
	if err != nil {
		t.Fatalf("resolve body: %v", err)
	}
	if got := bodyJSON(t, payload.Body); got != `{"status":"resolved"}` {
		t.Fatalf("body = %s, want the detected JSON body", got)
	}
	if len(payload.Notices) != 1 {
		t.Fatalf("notices = %v, want one hint about --data", payload.Notices)
	}
}

func TestResolveBodyMergesFieldsIntoDetectedJSONBody(t *testing.T) {
	payload, err := resolveBody(bodyFlags{
		Method:    "PUT",
		Fields:    []string{"count=2"},
		RawFields: []string{`{"status":"resolved"}`},
	}, strings.NewReader(""))
	if err != nil {
		t.Fatalf("resolve body: %v", err)
	}
	if got := bodyJSON(t, payload.Body); got != `{"count":2,"status":"resolved"}` {
		t.Fatalf("body = %s, want the merged body", got)
	}
}

func TestResolveBodyRejectsConflictWithDetectedJSONBody(t *testing.T) {
	_, err := resolveBody(bodyFlags{
		Method:    "PUT",
		Fields:    []string{"status=ignored"},
		RawFields: []string{`{"status":"resolved"}`},
	}, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected conflicting keys to be rejected")
	}
	if !strings.Contains(err.Error(), "status") {
		t.Fatalf("error = %q, want it to name the conflicting key", err)
	}
}

func TestResolveBodyRejectsMultipleDetectedJSONBodies(t *testing.T) {
	_, err := resolveBody(bodyFlags{
		Method:    "PUT",
		RawFields: []string{`{"a":1}`, `{"b":2}`},
	}, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected multiple JSON bodies to be rejected")
	}
}

func TestResolveBodyRejectsFieldsWithJSONArrayBody(t *testing.T) {
	_, err := resolveBody(bodyFlags{
		Method:    "PUT",
		Fields:    []string{"count=2"},
		RawFields: []string{`[1,2]`},
	}, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected fields combined with an array body to be rejected")
	}
}

func TestResolveEffectiveHeadersKeepsExplicitContentType(t *testing.T) {
	payload := requestPayload{Body: map[string]any{"a": 1}, HasBody: true}

	headers := resolveEffectiveHeaders(map[string]string{"content-type": "application/xml"}, payload)
	if _, exists := headers["Content-Type"]; exists {
		t.Fatal("expected the explicit content type to be kept")
	}
	if headers["content-type"] != "application/xml" {
		t.Fatalf("content-type = %q, want %q", headers["content-type"], "application/xml")
	}
}

func TestEncodeBodyDoesNotEscapeHTML(t *testing.T) {
	payload := requestPayload{Body: map[string]any{"query": "a<b&c"}, HasBody: true}

	encoded, err := encodeBody(payload)
	if err != nil {
		t.Fatalf("encode body: %v", err)
	}
	if string(encoded) != `{"query":"a<b&c"}` {
		t.Fatalf("body = %s, want the value unescaped", encoded)
	}
}
