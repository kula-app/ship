package cmd_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kula-app/ship/internal/cli/cmd"
)

// capturedRequest records what the test server received.
type capturedRequest struct {
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   string
}

// runCommand executes the api command against server, with stdout and stderr
// captured. The --api-key flag keeps authentication out of the local database.
func runCommand(t *testing.T, server *httptest.Server, stdin string, args ...string) (string, string, error) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	root := cmd.NewRootCommand("ship", cmd.BuildMetadata{})
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(append([]string{
		"--api-url", server.URL,
		"--api-key", "test-key",
		"api",
	}, args...))

	err := root.Execute()

	return stdout.String(), stderr.String(), err
}

// newTestServer returns a server that records the last request it handled and
// replies with the given status, content type and body.
func newTestServer(t *testing.T, status int, contentType string, body []byte) (*httptest.Server, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}

		*captured = capturedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.RawQuery,
			Header: r.Header.Clone(),
			Body:   string(requestBody),
		}

		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(status)
		if _, err := w.Write(body); err != nil {
			t.Errorf("write response body: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server, captured
}

func TestAPICommandPrependsAPIPrefixAndSendsQueryParameters(t *testing.T) {
	server, captured := newTestServer(t, http.StatusOK, "application/json", []byte(`[{"app_id":"1"}]`))

	stdout, _, err := runCommand(t, server, "", "apps/", "-F", "limit=10", "-f", "cursor=abc")
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}

	if captured.Method != http.MethodGet {
		t.Fatalf("method = %q, want %q", captured.Method, http.MethodGet)
	}
	if captured.Path != "/api/apps/" {
		t.Fatalf("path = %q, want %q", captured.Path, "/api/apps/")
	}
	if captured.Query != "cursor=abc&limit=10" {
		t.Fatalf("query = %q, want %q", captured.Query, "cursor=abc&limit=10")
	}
	if captured.Body != "" {
		t.Fatalf("body = %q, want none for GET", captured.Body)
	}
	if captured.Header.Get("X-API-Key") != "test-key" {
		t.Fatal("expected the API key to be sent")
	}
	if stdout != "[\n  {\n    \"app_id\": \"1\"\n  }\n]\n" {
		t.Fatalf("stdout = %q, want indented JSON", stdout)
	}
}

func TestAPICommandSendsJSONBodyAndCustomHeader(t *testing.T) {
	server, captured := newTestServer(t, http.StatusOK, "application/json", []byte(`{"job_id":"1"}`))

	_, _, err := runCommand(t, server, "",
		"app/1/publish", "-X", "POST", "-F", "platforms[]=ios", "-H", "X-Trace-Id: t1")
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}

	if captured.Method != http.MethodPost {
		t.Fatalf("method = %q, want %q", captured.Method, http.MethodPost)
	}
	if captured.Body != `{"platforms":["ios"]}` {
		t.Fatalf("body = %q, want the field to be encoded as JSON", captured.Body)
	}
	if captured.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", captured.Header.Get("Content-Type"), "application/json")
	}
	if captured.Header.Get("X-Trace-Id") != "t1" {
		t.Fatalf("X-Trace-Id = %q, want %q", captured.Header.Get("X-Trace-Id"), "t1")
	}
}

func TestAPICommandReadsBodyFromStdin(t *testing.T) {
	server, captured := newTestServer(t, http.StatusOK, "application/json", []byte(`{}`))

	_, _, err := runCommand(t, server, `{"platforms":["android"]}`, "app/1/publish", "-X", "PUT", "-i", "-")
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}

	if captured.Body != `{"platforms":["android"]}` {
		t.Fatalf("body = %q, want the stdin payload", captured.Body)
	}
}

func TestAPICommandPrintsBodyAndFailsOnErrorStatus(t *testing.T) {
	server, _ := newTestServer(t, http.StatusNotFound, "application/json", []byte(`{"detail":"not found"}`))

	stdout, _, err := runCommand(t, server, "", "apps/")
	if err == nil {
		t.Fatal("expected an error status to fail the command")
	}
	if err.Error() != "HTTP 404" {
		t.Fatalf("error = %q, want %q", err, "HTTP 404")
	}
	if !strings.Contains(stdout, `"detail"`) {
		t.Fatalf("stdout = %q, want the response body", stdout)
	}
}

func TestAPICommandSilentSuppressesBody(t *testing.T) {
	server, _ := newTestServer(t, http.StatusNotFound, "application/json", []byte(`{"detail":"not found"}`))

	stdout, _, err := runCommand(t, server, "", "apps/", "--silent")
	if err == nil {
		t.Fatal("expected an error status to fail the command")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want no output", stdout)
	}
}

func TestAPICommandSummarizesBinaryErrorBody(t *testing.T) {
	server, _ := newTestServer(t, http.StatusInternalServerError, "image/png", []byte{0x89, 0x50, 0x4e, 0x47})

	stdout, _, err := runCommand(t, server, "", "apps/")
	if err == nil {
		t.Fatal("expected an error status to fail the command")
	}
	if err.Error() != "HTTP 500 — binary error body (image/png, 4 bytes)" {
		t.Fatalf("error = %q, want a binary body summary", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want no raw bytes", stdout)
	}
}

func TestAPICommandWritesBinaryResponseVerbatim(t *testing.T) {
	payload := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a}
	server, _ := newTestServer(t, http.StatusOK, "image/png", payload)

	stdout, _, err := runCommand(t, server, "", "apps/1/icon")
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if stdout != string(payload) {
		t.Fatalf("stdout = %q, want the bytes untouched", stdout)
	}
}

func TestAPICommandVerboseWritesTranscriptToStderr(t *testing.T) {
	server, _ := newTestServer(t, http.StatusOK, "application/json", []byte(`{}`))

	_, stderr, err := runCommand(t, server, "", "apps/", "-H", "X-Trace-Id: t1", "--verbose")
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}

	for _, want := range []string{"> GET /api/apps/", "> X-Trace-Id: t1", "< HTTP 200", "< content-type: application/json"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want it to contain %q", stderr, want)
		}
	}
}

func TestAPICommandVerboseTranscriptIncludesAutoAddedHeaders(t *testing.T) {
	server, captured := newTestServer(t, http.StatusOK, "application/json", []byte(`{}`))

	_, stderr, err := runCommand(t, server, "", "app/1/publish", "-X", "POST", "-F", "platforms[]=ios", "--verbose")
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}

	// The transcript must report what is actually sent, not just what was typed.
	if captured.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q, want it to be sent", captured.Header.Get("Content-Type"))
	}
	if !strings.Contains(stderr, "> Content-Type: application/json") {
		t.Fatalf("stderr = %q, want the auto-added Content-Type in the transcript", stderr)
	}
}

func TestAPICommandSilentSuppressesVerboseTranscript(t *testing.T) {
	server, _ := newTestServer(t, http.StatusOK, "application/json", []byte(`{}`))

	stdout, stderr, err := runCommand(t, server, "", "apps/", "--verbose", "--silent")
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want no output", stdout)
	}
	if strings.Contains(stderr, "> GET") || strings.Contains(stderr, "< HTTP") {
		t.Fatalf("stderr = %q, want no transcript when --silent is set", stderr)
	}
}

func TestAPICommandDryRunDoesNotSendRequest(t *testing.T) {
	server, captured := newTestServer(t, http.StatusOK, "application/json", []byte(`{}`))

	stdout, _, err := runCommand(t, server, "", "app/1/publish", "-X", "POST", "-F", "platforms[]=ios", "-n")
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if captured.Method != "" {
		t.Fatal("expected --dry-run to send no request")
	}

	for _, want := range []string{
		`"method": "POST"`,
		`"url": "` + server.URL + `/api/app/1/publish"`,
		`"Content-Type": "application/json"`,
		`"platforms"`,
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want it to contain %q", stdout, want)
		}
	}
}

func TestAPICommandJSONLogFormatPrintsRawBody(t *testing.T) {
	server, _ := newTestServer(t, http.StatusOK, "application/json", []byte(`{"app_id":"1"}`))

	stdout, _, err := runCommand(t, server, "", "apps/", "--log-format", "json")
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if stdout != "{\"app_id\":\"1\"}\n" {
		t.Fatalf("stdout = %q, want the untouched server response", stdout)
	}
}

func TestAPICommandDryRunIsCompactInJSONLogFormat(t *testing.T) {
	server, captured := newTestServer(t, http.StatusOK, "application/json", []byte(`{}`))

	stdout, _, err := runCommand(t, server, "", "apps/", "-n", "--log-format", "json")
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if captured.Method != "" {
		t.Fatal("expected --dry-run to send no request")
	}
	if strings.Contains(stdout, "\n  ") {
		t.Fatalf("stdout = %q, want a single compact JSON line", stdout)
	}

	var preview map[string]any
	if err := json.Unmarshal([]byte(stdout), &preview); err != nil {
		t.Fatalf("parse dry-run preview: %v", err)
	}
	if preview["url"] != server.URL+"/api/apps/" {
		t.Fatalf("url = %v, want %q", preview["url"], server.URL+"/api/apps/")
	}
}

func TestAPICommandRejectsInvalidMethodBeforeSending(t *testing.T) {
	server, captured := newTestServer(t, http.StatusOK, "application/json", []byte(`{}`))

	_, _, err := runCommand(t, server, "", "apps/", "-X", "TRACE")
	if err == nil {
		t.Fatal("expected an unsupported method to fail")
	}
	if captured.Method != "" {
		t.Fatal("expected no request to be sent")
	}
}

func TestAPICommandRejectsMalformedDataQueryParameters(t *testing.T) {
	server, captured := newTestServer(t, http.StatusOK, "application/json", []byte(`{}`))

	_, _, err := runCommand(t, server, "", "apps/", "-d", "cursor=%zz")
	if err == nil {
		t.Fatal("expected malformed percent-encoding to fail")
	}
	if captured.Method != "" {
		t.Fatal("expected no request to be sent")
	}
}

func TestAPICommandReportsUnreadableInputFile(t *testing.T) {
	server, captured := newTestServer(t, http.StatusOK, "application/json", []byte(`{}`))

	// A directory exists but cannot be read as a request body.
	_, _, err := runCommand(t, server, "", "apps/", "-X", "POST", "-i", t.TempDir())
	if err == nil {
		t.Fatal("expected an unreadable input path to fail")
	}
	if captured.Method != "" {
		t.Fatal("expected no request to be sent")
	}
}

func TestAPICommandRequiresExactlyOneEndpoint(t *testing.T) {
	server, captured := newTestServer(t, http.StatusOK, "application/json", []byte(`{}`))

	for _, args := range [][]string{{}, {"apps/", "orgs/"}} {
		_, _, err := runCommand(t, server, "", args...)
		if err == nil {
			t.Fatalf("expected %v to fail", args)
		}
		if captured.Method != "" {
			t.Fatal("expected no request to be sent")
		}
	}
}

func TestAPICommandSendsNestedObjectAsQueryParameter(t *testing.T) {
	server, captured := newTestServer(t, http.StatusOK, "application/json", []byte(`{}`))

	_, _, err := runCommand(t, server, "", "apps/", "-F", `filter={"platform":"ios"}`)
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if captured.Query != `filter=%7B%22platform%22%3A%22ios%22%7D` {
		t.Fatalf("query = %q, want the object JSON-encoded", captured.Query)
	}
}

func TestAPICommandIsRegistered(t *testing.T) {
	root := cmd.NewRootCommand("ship", cmd.BuildMetadata{})

	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "api" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected the root command to expose 'api'")
	}
}
