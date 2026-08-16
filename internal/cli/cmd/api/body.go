package cmd_api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
)

// bodyFlags is the subset of command flags that determine the request payload.
type bodyFlags struct {
	Method    string
	Data      string
	HasData   bool
	Input     string
	HasInput  bool
	Fields    []string
	RawFields []string
}

// requestPayload is the resolved body and query string of a request, together
// with the advisories collected while resolving them.
type requestPayload struct {
	Body     any
	HasBody  bool
	Params   url.Values
	Warnings []string
	Notices  []string
}

// parseDataBody parses inline --data as JSON, falling back to the raw string so
// non-JSON payloads still work.
func parseDataBody(data string) any {
	parsed, err := decodeJSON([]byte(data))
	if err != nil {
		return data
	}

	return parsed
}

// dataToQueryParams converts --data into query parameters for methods that carry
// no body. URL-encoded strings ("stat=received&resolution=1d") and JSON objects
// are both supported; arrays and primitives cannot become query parameters.
func dataToQueryParams(data any) (url.Values, error) {
	if text, isText := data.(string); isText {
		params, err := url.ParseQuery(text)
		if err != nil {
			return nil, fmt.Errorf("parsing --data as query parameters: %w", err)
		}

		return params, nil
	}

	object, isObject := data.(map[string]any)
	if !isObject {
		return nil, errors.New(
			"cannot use --data with a JSON primitive or array for GET requests. " +
				"Only JSON objects and URL-encoded strings can be converted to query parameters. " +
				"Use --method POST to send this data as a request body",
		)
	}

	params := url.Values{}
	for key, value := range object {
		if text, isText := value.(string); isText {
			params.Set(key, text)
			continue
		}

		encoded, err := marshalJSON(value)
		if err != nil {
			return nil, fmt.Errorf("encoding --data value for %q: %w", key, err)
		}
		params.Set(key, string(encoded))
	}

	return params, nil
}

// jsonFieldBody is the result of scanning field arguments for a bare JSON body.
type jsonFieldBody struct {
	Body      any
	HasBody   bool
	Remaining []string
	Notices   []string
}

// tryParseJSONField reports whether a field is a bare JSON object or array.
//
// Restricting detection to "{" and "[" is deliberate rather than an
// optimization: it excludes JSON primitives such as 42 or true, which would
// otherwise be mistaken for a body that field flags cannot be merged into.
func tryParseJSONField(field string) (any, bool) {
	if strings.Contains(field, "=") {
		return nil, false
	}
	if !strings.HasPrefix(field, "{") && !strings.HasPrefix(field, "[") {
		return nil, false
	}

	parsed, err := decodeJSON([]byte(field))
	if err != nil {
		return nil, false
	}

	return parsed, true
}

// extractJSONBody pulls a bare JSON object or array out of the field arguments
// and uses it as the request body, covering the common mistake of passing
// -f '{"platforms":["ios"]}' instead of -d. Only one such body is allowed;
// several are ambiguous.
func extractJSONBody(fields []string) (jsonFieldBody, error) {
	if len(fields) == 0 {
		return jsonFieldBody{}, nil
	}

	result := jsonFieldBody{}
	for _, field := range fields {
		parsed, isJSON := tryParseJSONField(field)
		if !isJSON {
			result.Remaining = append(result.Remaining, field)
			continue
		}

		if result.HasBody {
			return jsonFieldBody{}, errors.New(
				"multiple JSON bodies detected in field arguments. " +
					"Use --data/-d to pass an inline JSON body explicitly",
			)
		}

		result.Body = parsed
		result.HasBody = true

		preview := field
		if len(preview) > 60 {
			preview = preview[:57] + "..."
		}
		result.Notices = append(result.Notices, fmt.Sprintf(
			"'%s' was used as the request body. Use --data/-d to pass inline JSON next time.",
			preview,
		))
	}

	return result, nil
}

// buildFromFields builds the body and query parameters from the field flags,
// auto-detecting a bare JSON body among them.
func buildFromFields(method string, typedFields, rawFields []string) (requestPayload, error) {
	fields, warnings := normalizeFields(typedFields)
	raw, rawWarnings := normalizeFields(rawFields)

	payload := requestPayload{Warnings: append(warnings, rawWarnings...)}

	// GET requests have no body, so JSON-shaped values are left alone and fall
	// through to query parameter routing, which reports a clearer error.
	if method != http.MethodGet {
		extracted, err := extractJSONBody(raw)
		if err != nil {
			return requestPayload{}, err
		}
		payload.Body = extracted.Body
		payload.HasBody = extracted.HasBody
		payload.Notices = extracted.Notices
		raw = extracted.Remaining
	}

	body, params, err := prepareRequestOptions(method, fields, raw)
	if err != nil {
		return requestPayload{}, err
	}
	payload.Params = params

	if body == nil {
		return payload, nil
	}
	if !payload.HasBody {
		payload.Body = body
		payload.HasBody = true

		return payload, nil
	}

	if _, isList := payload.Body.([]any); isList {
		return requestPayload{}, errors.New(
			"cannot combine a JSON array body with field flags (-F/-f). " +
				"Use --data/-d to pass the array as the full body without extra fields",
		)
	}

	existing, isMap := payload.Body.(map[string]any)
	if !isMap {
		payload.Body = body

		return payload, nil
	}

	// Detect top-level key conflicts before merging — a shallow merge would
	// silently drop nested keys of the detected JSON body.
	var conflicts []string
	for key := range body {
		if _, exists := existing[key]; exists {
			conflicts = append(conflicts, key)
		}
	}
	if len(conflicts) > 0 {
		slices.Sort(conflicts)

		return requestPayload{}, fmt.Errorf(
			"field flag(s) conflict with detected JSON body at key(s): %s. "+
				"Use --data/-d to pass the full JSON body, or use only field flags (-F/-f)",
			strings.Join(conflicts, ", "),
		)
	}

	for key, value := range body {
		existing[key] = value
	}
	payload.Body = existing

	return payload, nil
}

// buildBodyFromInput reads the request body from a file, or from stdin when the
// path is "-". JSON content is parsed; anything else is sent as-is.
func buildBodyFromInput(inputPath string, stdin io.Reader) (any, error) {
	if inputPath == "-" {
		content, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("reading body from stdin: %w", err)
		}

		return parseInputContent(content), nil
	}

	content, err := os.ReadFile(inputPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("file not found: %s", inputPath)
	}
	if err != nil {
		return nil, fmt.Errorf("reading body from %s: %w", inputPath, err)
	}

	return parseInputContent(content), nil
}

func parseInputContent(content []byte) any {
	parsed, err := decodeJSON(content)
	if err != nil {
		return string(content)
	}

	return parsed
}

// resolveBody determines the request body and query parameters from the flags,
// in priority order: --data, then --input, then the field flags. Combinations
// that would silently drop one of them are rejected.
func resolveBody(flags bodyFlags, stdin io.Reader) (requestPayload, error) {
	if flags.HasData && flags.HasInput {
		return requestPayload{}, errors.New(
			"cannot use --data and --input together. " +
				"Use --data/-d for inline JSON, or --input/-i for file/stdin",
		)
	}
	if flags.HasData && (len(flags.Fields) > 0 || len(flags.RawFields) > 0) {
		return requestPayload{}, errors.New(
			"cannot use --data with --field or --raw-field. " +
				"Use --data/-d for a full JSON body, or -F/-f for individual fields",
		)
	}

	if flags.HasData {
		parsed := parseDataBody(flags.Data)

		// GET carries no body — convert the data to query parameters instead.
		if flags.Method == http.MethodGet {
			params, err := dataToQueryParams(parsed)
			if err != nil {
				return requestPayload{}, err
			}

			return requestPayload{Params: params}, nil
		}

		return requestPayload{Body: parsed, HasBody: true}, nil
	}

	if flags.HasInput {
		body, err := buildBodyFromInput(flags.Input, stdin)
		if err != nil {
			return requestPayload{}, err
		}

		return requestPayload{Body: body, HasBody: true}, nil
	}

	return buildFromFields(flags.Method, flags.Fields, flags.RawFields)
}

// encodeBody serializes the resolved body for transport. String bodies are sent
// verbatim; everything else is JSON-encoded.
func encodeBody(payload requestPayload) ([]byte, error) {
	if !payload.HasBody {
		return nil, nil
	}
	if text, isText := payload.Body.(string); isText {
		return []byte(text), nil
	}

	encoded, err := marshalJSON(payload.Body)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}

	return encoded, nil
}

// resolveEffectiveHeaders returns the headers the request will be sent with,
// adding a JSON content type for structured bodies unless one was set by hand.
// Authentication headers are added by the client and are not included here.
func resolveEffectiveHeaders(customHeaders map[string]string, payload requestPayload) map[string]string {
	headers := make(map[string]string, len(customHeaders)+1)
	for key, value := range customHeaders {
		headers[key] = value
	}

	if !payload.HasBody {
		return headers
	}
	if _, isText := payload.Body.(string); isText {
		return headers
	}
	for key := range headers {
		if strings.EqualFold(key, "Content-Type") {
			return headers
		}
	}
	headers["Content-Type"] = "application/json"

	return headers
}
