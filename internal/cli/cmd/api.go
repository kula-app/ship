package cmd

import (
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/kula-app/ship/internal/cli/api"
	"github.com/kula-app/ship/internal/cli/config"
	"github.com/kula-app/ship/internal/cli/helpers"
)

// newAPICmd creates and returns the api command.
func newAPICmd(cliName string, deps ShipCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api <endpoint>",
		Short: "Make an authenticated API request",
		Long: `Make a raw API request to the Shipable API. Similar to 'gh api' for GitHub. ` +
			`The endpoint is relative to /api/ (do not include the prefix). ` +
			`Authentication is handled automatically using your stored credentials.

Body options:
  --data/-d '{"key":"value"}'   Inline JSON body (like curl -d)
  --input/-i file.json          Read body from file (or "-" for stdin)

Field syntax (--field/-F):
  key=value          Simple field (values parsed as JSON if valid)
  key[sub]=value     Nested object: {key: {sub: value}}
  key[]=value        Array append: {key: [value]}
  key[]              Empty array: {key: []}

Use --raw-field/-f to send values as strings without JSON parsing.

For GET requests the field flags become query parameters instead of a body.`,
		Example: fmt.Sprintf(`  # List apps
  %[1]s api apps/

  # Trigger a publish with a field flag
  %[1]s api app/<uuid>/publish -X POST -F platforms[]=ios

  # Trigger a publish with an inline JSON body
  %[1]s api app/<uuid>/publish -X POST -d '{"platforms":["ios","android"]}'

  # Query parameters for GET requests
  %[1]s api apps/ -F limit=10`, cliName),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAPI(cmd, deps, cliName, args[0])
		},
	}

	cmd.Flags().StringP("method", "X", http.MethodGet, "The HTTP method for the request")
	cmd.Flags().StringP("data", "d", "", "Inline JSON body for the request (like curl -d)")
	cmd.Flags().StringArrayP("field", "F", nil, "Add a typed parameter (key=value, key[sub]=value, key[]=value)")
	cmd.Flags().StringArrayP("raw-field", "f", nil, "Add a string parameter without JSON parsing")
	cmd.Flags().StringArrayP("header", "H", nil, "Add a HTTP request header in key:value format")
	cmd.Flags().StringP("input", "i", "", `The file to use as body for the HTTP request (use "-" to read from standard input)`)
	cmd.Flags().Bool("silent", false, "Do not print the response body")
	cmd.Flags().Bool("verbose", false, "Include full HTTP request and response in the output")
	cmd.Flags().BoolP("dry-run", "n", false, "Show the resolved request without sending it")

	return cmd
}

func runAPI(cmd *cobra.Command, deps ShipCommandDeps, cliName, rawEndpoint string) error {
	// Start root Sentry transaction for CLI command
	transaction := helpers.StartCommandTransaction(cmd, fmt.Sprintf("%s api", cliName))
	transaction.SetData("command", "api")
	transaction.SetData("cli_name", cliName)
	defer transaction.Finish()

	ctx := cmd.Context()
	logger := deps.GetLogger()

	endpoint, err := normalizeEndpoint(rawEndpoint)
	if err != nil {
		return failInvalidArgument(transaction, err)
	}
	if endpoint.StrippedLineBreaks {
		logger.WarnContext(ctx, "stripped line breaks from endpoint (copy-paste artifact)")
	}
	if endpoint.StrippedPrefix {
		// Silent auto-fix, not a warning: pasted URLs commonly carry the /api
		// prefix that the client adds itself.
		logger.DebugContext(ctx, "stripped /api prefix from endpoint (added automatically)")
	}
	transaction.SetData("endpoint", endpoint.Path)

	methodFlag, _ := cmd.Flags().GetString("method")
	method, err := parseMethod(methodFlag)
	if err != nil {
		return failInvalidArgument(transaction, err)
	}
	transaction.SetData("method", method)

	dataFlag, _ := cmd.Flags().GetString("data")
	inputFlag, _ := cmd.Flags().GetString("input")
	fieldFlags, _ := cmd.Flags().GetStringArray("field")
	rawFieldFlags, _ := cmd.Flags().GetStringArray("raw-field")
	headerFlags, _ := cmd.Flags().GetStringArray("header")
	silent, _ := cmd.Flags().GetBool("silent")
	verbose, _ := cmd.Flags().GetBool("verbose")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	payload, err := resolveBody(bodyFlags{
		Method:    method,
		Data:      dataFlag,
		HasData:   cmd.Flags().Changed("data"),
		Input:     inputFlag,
		HasInput:  cmd.Flags().Changed("input"),
		Fields:    fieldFlags,
		RawFields: rawFieldFlags,
	}, cmd.InOrStdin())
	if err != nil {
		return failInvalidArgument(transaction, err)
	}
	for _, warning := range payload.Warnings {
		logger.WarnContext(ctx, warning)
	}
	for _, notice := range payload.Notices {
		logger.InfoContext(ctx, notice)
	}

	headers, err := parseHeaders(headerFlags)
	if err != nil {
		return failInvalidArgument(transaction, err)
	}
	effectiveHeaders := resolveEffectiveHeaders(headers, payload)

	requestPath := buildRequestPath(endpoint.Path, payload)

	// A dry run reports the resolved request without authenticating, so it stays
	// usable before login.
	if dryRun {
		transaction.SetData("dry_run", true)
		if err := printDryRun(cmd, dryRunRequest{
			Method:  method,
			URL:     config.ResolveAPIURL(cmd) + requestPath,
			Headers: effectiveHeaders,
			Body:    payload.Body,
		}); err != nil {
			return failInternal(transaction, err)
		}

		transaction.Status = sentry.SpanStatusOK
		return nil
	}

	// Get API credentials
	logger.InfoContext(ctx, "retrieving API credentials")
	svc, credentials, err := resolveShipService(cmd, deps)
	if err != nil {
		return failUnauthenticated(transaction, err)
	}
	transaction.SetData("api_url", credentials.APIURL)

	body, err := encodeBody(payload)
	if err != nil {
		return failInvalidArgument(transaction, err)
	}

	// --silent suppresses the response body, so the transcript would be the only
	// output left; keeping them in step matches the reference implementation.
	transcript := verbose && !silent
	if transcript {
		logRequest(cmd, method, requestPath, effectiveHeaders)
	}

	response, err := svc.SendRawRequest(ctx, method, requestPath, body, effectiveHeaders)
	if err != nil {
		return failInternal(transaction, err)
	}
	if transcript {
		logResponse(cmd, response)
	}
	transaction.SetData("status_code", response.StatusCode)

	// The endpoint's own status drives the exit code, so a 4xx is reported as a
	// failed precondition rather than a CLI bug.
	if err := renderResponse(cmd, response, silent); err != nil {
		transaction.Status = sentry.SpanStatusFailedPrecondition
		return err
	}

	transaction.Status = sentry.SpanStatusOK
	return nil
}

// buildRequestPath assembles the API path and query string of the request.
func buildRequestPath(endpointPath string, payload requestPayload) string {
	path := apiPathPrefix + endpointPath

	query := payload.Params.Encode()
	if query == "" {
		return path
	}

	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}

	return path + separator + query
}

// dryRunRequest is the request preview printed by --dry-run.
type dryRunRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    any               `json:"body"`
}

func printDryRun(c *cobra.Command, request dryRunRequest) error {
	encoded, err := encodeForOutput(c, request)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.OutOrStdout(), string(encoded))

	return nil
}

// logRequest writes the outgoing request to stderr in curl's verbose "> " style.
// It receives the effective headers, so the transcript shows what is actually
// sent — including an auto-added Content-Type. Authentication headers are added
// further down in the client and stay out of the transcript.
func logRequest(c *cobra.Command, method, path string, headers map[string]string) {
	out := c.ErrOrStderr()

	fmt.Fprintf(out, "> %s %s\n", method, path)
	for _, key := range slices.Sorted(maps.Keys(headers)) {
		fmt.Fprintf(out, "> %s: %s\n", key, headers[key])
	}
	fmt.Fprintln(out, ">")
}

// logResponse writes the incoming response to stderr in curl's verbose "< " style.
func logResponse(c *cobra.Command, response *api.RawResponse) {
	out := c.ErrOrStderr()

	fmt.Fprintf(out, "< HTTP %d\n", response.StatusCode)
	for _, key := range slices.Sorted(maps.Keys(response.Header)) {
		for _, value := range response.Header[key] {
			fmt.Fprintf(out, "< %s: %s\n", strings.ToLower(key), value)
		}
	}
	fmt.Fprintln(out, "<")
}

// renderResponse writes the response body to stdout and turns an error status
// into a non-zero exit code.
func renderResponse(c *cobra.Command, response *api.RawResponse, silent bool) error {
	isError := response.StatusCode >= 400
	isBinary := !isTextualContentType(response.Header.Get("Content-Type"))

	if silent {
		if isError {
			return fmt.Errorf("HTTP %d", response.StatusCode)
		}

		return nil
	}

	// Never dump raw bytes for a failed request — a short summary is more useful
	// than a screenful of binary.
	if isError && isBinary {
		return fmt.Errorf("%s", formatBinaryErrorBody(response.StatusCode, response.Header, response.Body))
	}

	if isBinary {
		if isatty.IsTerminal(os.Stdout.Fd()) {
			slog.Default().WarnContext(c.Context(),
				"Binary response written to a TTY — redirect stdout to a file (e.g. `> file.bin`) to capture raw bytes cleanly")
		}
		if _, err := c.OutOrStdout().Write(response.Body); err != nil {
			return fmt.Errorf("writing response body: %w", err)
		}

		return nil
	}

	fmt.Fprintln(c.OutOrStdout(), renderBody(c, response.Body))

	if isError {
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}

	return nil
}

// renderBody formats a textual response body: pretty-printed JSON for humans,
// the untouched server response in JSON log format.
func renderBody(c *cobra.Command, body []byte) string {
	logFormat, _ := c.Flags().GetString("log-format")
	if logFormat == "json" {
		return string(body)
	}

	return formatAPIResponse(decodeResponseBody(body))
}

// encodeForOutput encodes a value for stdout, compact in JSON log format and
// indented otherwise.
func encodeForOutput(c *cobra.Command, value any) ([]byte, error) {
	logFormat, _ := c.Flags().GetString("log-format")
	if logFormat == "json" {
		return marshalJSON(value)
	}

	return marshalJSONIndent(value)
}
