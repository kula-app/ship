package cmd_api

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
	"github.com/kula-app/ship/internal/cli/bootstrap"
	"github.com/kula-app/ship/internal/cli/config"
	"github.com/kula-app/ship/internal/cli/helpers"
	"github.com/kula-app/ship/internal/cli/service"
)

// APICommandDeps declares the dependencies required by the api command.
type APICommandDeps interface {
	bootstrap.LoggerFactory
	service.ShipServiceFactory
}

// NewAPICmd creates and returns the api command.
func NewAPICmd(cliName string, deps APICommandDeps) *cobra.Command {
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

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAPI(cmd, deps, cliName, args[0])
	}

	return cmd
}

func runAPI(cmd *cobra.Command, deps APICommandDeps, cliName, rawEndpoint string) error {
	// Start root Sentry transaction for CLI command
	transaction := helpers.StartCommandTransaction(cmd, fmt.Sprintf("%s api", cliName))
	transaction.SetData("command", "api")
	transaction.SetData("cli_name", cliName)
	defer transaction.Finish()

	ctx := cmd.Context()
	logger := deps.GetLogger()

	endpoint, err := normalizeEndpoint(rawEndpoint)
	if err != nil {
		return helpers.FailInvalidArgument(transaction, err)
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

	flags, err := readAPIFlags(cmd)
	if err != nil {
		return helpers.FailInvalidArgument(transaction, err)
	}

	method, err := parseMethod(flags.Method)
	if err != nil {
		return helpers.FailInvalidArgument(transaction, err)
	}
	transaction.SetData("method", method)

	payload, err := resolveBody(bodyFlags{
		Method:    method,
		Data:      flags.Data,
		HasData:   cmd.Flags().Changed("data"),
		Input:     flags.Input,
		HasInput:  cmd.Flags().Changed("input"),
		Fields:    flags.Fields,
		RawFields: flags.RawFields,
	}, cmd.InOrStdin())
	if err != nil {
		return helpers.FailInvalidArgument(transaction, err)
	}
	for _, warning := range payload.Warnings {
		logger.WarnContext(ctx, warning)
	}
	for _, notice := range payload.Notices {
		logger.InfoContext(ctx, notice)
	}

	headers, err := parseHeaders(flags.Headers)
	if err != nil {
		return helpers.FailInvalidArgument(transaction, err)
	}
	effectiveHeaders := resolveEffectiveHeaders(headers, payload)

	requestPath := buildRequestPath(endpoint.Path, payload)

	// A dry run reports the resolved request without authenticating, so it stays
	// usable before login.
	if flags.DryRun {
		transaction.SetData("dry_run", true)
		if err := printDryRun(cmd, dryRunRequest{
			Method:  method,
			URL:     config.ResolveAPIURL(cmd) + requestPath,
			Headers: effectiveHeaders,
			Body:    payload.Body,
		}); err != nil {
			return helpers.FailInternal(transaction, err)
		}

		transaction.Status = sentry.SpanStatusOK
		return nil
	}

	// Get API credentials
	logger.InfoContext(ctx, "retrieving API credentials")
	svc, credentials, err := helpers.ResolveShipService(cmd, deps)
	if err != nil {
		return helpers.FailUnauthenticated(transaction, err)
	}
	transaction.SetData("api_url", credentials.APIURL)

	body, err := encodeBody(payload)
	if err != nil {
		return helpers.FailInvalidArgument(transaction, err)
	}

	// --silent suppresses the response body, so the transcript would be the only
	// output left; keeping them in step matches the reference implementation.
	transcript := flags.Verbose && !flags.Silent
	if transcript {
		logRequest(cmd, method, requestPath, effectiveHeaders)
	}

	response, err := svc.SendRawRequest(ctx, method, requestPath, body, effectiveHeaders)
	if err != nil {
		return helpers.FailInternal(transaction, err)
	}
	if transcript {
		logResponse(cmd, response)
	}
	transaction.SetData("status_code", response.StatusCode)

	// The endpoint's own status drives the exit code, so a 4xx is reported as a
	// failed precondition rather than a CLI bug.
	if err := renderResponse(cmd, response, flags.Silent); err != nil {
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
	outputJSON, err := helpers.ResolveLogFormat(c)
	if err == nil && outputJSON {
		return string(body)
	}

	return formatAPIResponse(decodeResponseBody(body))
}

// encodeForOutput encodes a value for stdout, compact in JSON log format and
// indented otherwise.
func encodeForOutput(c *cobra.Command, value any) ([]byte, error) {
	outputJSON, err := helpers.ResolveLogFormat(c)
	if err != nil {
		return nil, err
	}
	if outputJSON {
		return marshalJSON(value)
	}

	return marshalJSONIndent(value)
}

// apiFlags holds the raw flag values of the api command.
type apiFlags struct {
	Method    string
	Data      string
	Input     string
	Fields    []string
	RawFields []string
	Headers   []string
	Silent    bool
	Verbose   bool
	DryRun    bool
}

// readAPIFlags reads every api command flag, reporting the first failure rather
// than silently falling back to a zero value.
func readAPIFlags(cmd *cobra.Command) (apiFlags, error) {
	var flags apiFlags
	var err error

	if flags.Method, err = cmd.Flags().GetString("method"); err != nil {
		return flags, fmt.Errorf("failed to get method flag: %w", err)
	}
	if flags.Data, err = cmd.Flags().GetString("data"); err != nil {
		return flags, fmt.Errorf("failed to get data flag: %w", err)
	}
	if flags.Input, err = cmd.Flags().GetString("input"); err != nil {
		return flags, fmt.Errorf("failed to get input flag: %w", err)
	}
	if flags.Fields, err = cmd.Flags().GetStringArray("field"); err != nil {
		return flags, fmt.Errorf("failed to get field flag: %w", err)
	}
	if flags.RawFields, err = cmd.Flags().GetStringArray("raw-field"); err != nil {
		return flags, fmt.Errorf("failed to get raw-field flag: %w", err)
	}
	if flags.Headers, err = cmd.Flags().GetStringArray("header"); err != nil {
		return flags, fmt.Errorf("failed to get header flag: %w", err)
	}
	if flags.Silent, err = cmd.Flags().GetBool("silent"); err != nil {
		return flags, fmt.Errorf("failed to get silent flag: %w", err)
	}
	if flags.Verbose, err = cmd.Flags().GetBool("verbose"); err != nil {
		return flags, fmt.Errorf("failed to get verbose flag: %w", err)
	}
	if flags.DryRun, err = cmd.Flags().GetBool("dry-run"); err != nil {
		return flags, fmt.Errorf("failed to get dry-run flag: %w", err)
	}

	return flags, nil
}
