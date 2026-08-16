package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	// fieldKeyPattern matches a base key followed by zero or more [bracket] segments.
	fieldKeyPattern = regexp.MustCompile(`^([^\[\]]+)((?:\[[^\[\]]*\])*)$`)

	// bracketContentsPattern extracts the contents of each [bracket] segment.
	bracketContentsPattern = regexp.MustCompile(`\[([^\[\]]*)\]`)
)

// parseFieldValue parses a --field value as JSON, falling back to the raw string
// when it is not valid JSON.
func parseFieldValue(value string) any {
	parsed, err := decodeJSON([]byte(value))
	if err != nil {
		return value
	}

	return parsed
}

// parseFieldKey splits a bracket-notation field key into path segments:
//
//	name       -> ["name"]
//	user[name] -> ["user", "name"]
//	a[b][c]    -> ["a", "b", "c"]
//	tags[]     -> ["tags", ""]
func parseFieldKey(key string) ([]string, error) {
	match := fieldKeyPattern.FindStringSubmatch(key)
	if match == nil {
		return nil, fmt.Errorf("invalid field key format: %s", key)
	}

	path := []string{match[1]}
	for _, bracket := range bracketContentsPattern.FindAllStringSubmatch(match[2], -1) {
		path = append(path, bracket[1])
	}

	return path, nil
}

// validatePathSegments rejects empty brackets anywhere but at the end of a key.
// A key like a[][b] would otherwise silently lose its value.
func validatePathSegments(path []string) error {
	for _, segment := range path[:len(path)-1] {
		if segment == "" {
			return errors.New("invalid field key: empty brackets [] can only appear at the end of a key")
		}
	}

	return nil
}

// typeName returns a human-readable type name for error messages.
func typeName(value any) string {
	switch value.(type) {
	case []any:
		return "array"
	case map[string]any:
		return "map"
	case string:
		return "string"
	case bool:
		return "boolean"
	case json.Number:
		return "number"
	case nil:
		return "null"
	}

	return fmt.Sprintf("%T", value)
}

// formatPathForError renders path segments up to endIndex in bracket notation,
// e.g. ["user", "name"] at index 1 becomes user[name].
func formatPathForError(path []string, endIndex int) string {
	segments := path[:endIndex+1]
	if len(segments) == 1 {
		return segments[0]
	}

	return fmt.Sprintf("%s[%s]", segments[0], strings.Join(segments[1:], "]["))
}

// setNestedValue stores value in obj at the path described by a bracket-notation
// key, creating intermediate maps and arrays along the way. Pass hasValue=false
// for the bare "key[]" form, which only initializes an empty array.
func setNestedValue(obj map[string]any, key string, value any, hasValue bool) error {
	path, err := parseFieldKey(key)
	if err != nil {
		return err
	}
	if err := validatePathSegments(path); err != nil {
		return err
	}

	return assignPath(obj, path, 0, value, hasValue)
}

// assignPath walks path from index onwards, creating the missing levels, and
// stores value at the leaf. A trailing empty segment appends to an array.
func assignPath(container map[string]any, path []string, index int, value any, hasValue bool) error {
	key := path[index]
	remaining := len(path) - index - 1

	if remaining == 0 {
		container[key] = value
		return nil
	}

	if remaining == 1 && path[index+1] == "" {
		existing, ok := container[key]
		if !ok {
			existing = []any{}
		}
		list, isList := existing.([]any)
		if !isList {
			return fmt.Errorf("expected array type under %q, got %s", formatPathForError(path, index), typeName(existing))
		}
		if hasValue {
			list = append(list, value)
		}
		container[key] = list

		return nil
	}

	existing, ok := container[key]
	if !ok {
		existing = map[string]any{}
	}
	child, isMap := existing.(map[string]any)
	if !isMap {
		return fmt.Errorf("expected map type under %q, got %s", formatPathForError(path, index), typeName(existing))
	}
	container[key] = child

	return assignPath(child, path, index+1, value, hasValue)
}

// processField parses a single "key=value" field and stores it in result. When
// raw is true the value is kept as a string instead of being parsed as JSON.
func processField(result map[string]any, field string, raw bool) error {
	eqIndex := strings.Index(field, "=")
	if eqIndex == -1 {
		if strings.HasSuffix(field, "[]") {
			return setNestedValue(result, field, nil, false)
		}

		return fmt.Errorf("invalid field format: %s. Expected key=value", field)
	}

	key := field[:eqIndex]
	rawValue := field[eqIndex+1:]
	if raw {
		return setNestedValue(result, key, rawValue, true)
	}

	return setNestedValue(result, key, parseFieldValue(rawValue), true)
}

// parseFields parses field arguments into a nested request body object.
func parseFields(fields []string, raw bool) (map[string]any, error) {
	result := map[string]any{}
	for _, field := range fields {
		if err := processField(result, field, raw); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// normalizeFields rewrites fields that use ":" instead of "=" as the separator,
// a common slip when reaching for search-query syntax (-F status:resolved). It
// returns the corrected fields plus a warning for every correction made.
//
// Splitting on the first ":" keeps colons inside values — ISO timestamps, URLs —
// intact. Fields that cannot be corrected are returned unchanged so the parser
// reports its own error.
func normalizeFields(fields []string) ([]string, []string) {
	if len(fields) == 0 {
		return fields, nil
	}

	corrected := make([]string, 0, len(fields))
	var warnings []string

	for _, field := range fields {
		// Already valid: contains "=", or is the empty-array form "key[]".
		if strings.Contains(field, "=") || strings.HasSuffix(field, "[]") {
			corrected = append(corrected, field)
			continue
		}

		// JSON-shaped values must not be corrected — a colon inside them is JSON
		// syntax, not a key/value separator.
		if strings.HasPrefix(field, "{") || strings.HasPrefix(field, "[") {
			corrected = append(corrected, field)
			continue
		}

		// A leading ":" would produce an empty key, which the parser rejects
		// regardless, so it is left uncorrected.
		colonIndex := strings.Index(field, ":")
		if colonIndex <= 0 {
			corrected = append(corrected, field)
			continue
		}

		replacement := field[:colonIndex] + "=" + field[colonIndex+1:]
		warnings = append(warnings, fmt.Sprintf(
			"field '%s' looks like it uses ':' instead of '=' — interpreting as '%s'",
			field, replacement,
		))
		corrected = append(corrected, replacement)
	}

	return corrected, warnings
}

// parseHeaders parses "Key: Value" header arguments.
func parseHeaders(headers []string) (map[string]string, error) {
	result := map[string]string{}
	for _, header := range headers {
		colonIndex := strings.Index(header, ":")
		if colonIndex == -1 {
			return nil, fmt.Errorf("invalid header format: %s. Expected Key: Value", header)
		}
		key := strings.TrimSpace(header[:colonIndex])
		result[key] = strings.TrimSpace(header[colonIndex+1:])
	}

	return result, nil
}

// stringifyValue renders a parsed field value for a query string, JSON-encoding
// objects and arrays instead of printing their Go representation.
func stringifyValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case bool:
		return strconv.FormatBool(typed)
	case nil:
		return "null"
	}

	encoded, err := marshalJSON(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}

	return string(encoded)
}

// buildQueryParams converts typed field arguments into query parameters for
// methods that carry no request body. Array values become repeated keys.
func buildQueryParams(fields []string) (url.Values, error) {
	params := url.Values{}
	for _, field := range fields {
		eqIndex := strings.Index(field, "=")
		if eqIndex == -1 {
			return nil, fmt.Errorf("invalid field format: %s. Expected key=value", field)
		}

		key := field[:eqIndex]
		if !fieldKeyPattern.MatchString(key) {
			return nil, fmt.Errorf("invalid field key format: %s", key)
		}

		value := parseFieldValue(field[eqIndex+1:])
		list, isList := value.([]any)
		if !isList {
			params.Set(key, stringifyValue(value))
			continue
		}

		params.Del(key)
		for _, item := range list {
			params.Add(key, stringifyValue(item))
		}
	}

	return params, nil
}

// buildRawQueryParams converts raw field arguments into query parameters. Values
// are used exactly as provided — no JSON parsing, no bracket notation — and
// repeated keys are collected instead of overwritten.
func buildRawQueryParams(fields []string) (url.Values, error) {
	params := url.Values{}
	for _, field := range fields {
		eqIndex := strings.Index(field, "=")
		if eqIndex == -1 {
			return nil, fmt.Errorf("invalid field format: %s. Expected key=value", field)
		}

		key := field[:eqIndex]
		if key == "" {
			return nil, errors.New("invalid field key format: key cannot be empty")
		}
		params.Add(key, field[eqIndex+1:])
	}

	return params, nil
}

// buildQueryParamsFromFields merges typed and raw field arguments into a single
// query parameter set. Raw fields are applied last and win on conflicting keys.
func buildQueryParamsFromFields(typedFields, rawFields []string) (url.Values, error) {
	params, err := buildQueryParams(typedFields)
	if err != nil {
		return nil, err
	}

	rawParams, err := buildRawQueryParams(rawFields)
	if err != nil {
		return nil, err
	}
	for key, values := range rawParams {
		params[key] = values
	}

	return params, nil
}

// buildBodyFromFields merges typed and raw field arguments into a request body.
// Raw fields are applied last so they can overwrite typed ones at the same path.
func buildBodyFromFields(typedFields, rawFields []string) (map[string]any, error) {
	if len(typedFields) == 0 && len(rawFields) == 0 {
		return nil, nil
	}

	result, err := parseFields(typedFields, false)
	if err != nil {
		return nil, err
	}
	for _, field := range rawFields {
		if err := processField(result, field, true); err != nil {
			return nil, err
		}
	}
	if len(result) == 0 {
		return nil, nil
	}

	return result, nil
}

// prepareRequestOptions routes field arguments to the request body or to the
// query string, depending on whether the method carries a body.
func prepareRequestOptions(method string, typedFields, rawFields []string) (map[string]any, url.Values, error) {
	if len(typedFields) == 0 && len(rawFields) == 0 {
		return nil, nil, nil
	}

	if method != http.MethodGet {
		body, err := buildBodyFromFields(typedFields, rawFields)
		return body, nil, err
	}

	params, err := buildQueryParamsFromFields(typedFields, rawFields)

	return nil, params, err
}
