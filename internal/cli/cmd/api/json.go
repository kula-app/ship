package cmd_api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// decodeJSON decodes exactly one JSON value. Number literals are preserved as
// json.Number so large integers such as job IDs survive the round trip, and
// trailing data is rejected so partially-valid input falls back to a string.
func decodeJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}
	if decoder.More() {
		return nil, errors.New("decoding JSON: unexpected trailing data")
	}

	return value, nil
}

// marshalJSON encodes a value without Go's default HTML escaping, so values
// containing <, > or & are transmitted the way they were provided.
func marshalJSON(value any) ([]byte, error) {
	return encodeJSON(value, "")
}

// marshalJSONIndent is marshalJSON with the two-space indentation used for
// human-readable output.
func marshalJSONIndent(value any) ([]byte, error) {
	return encodeJSON(value, "  ")
}

func encodeJSON(value any, indent string) ([]byte, error) {
	var buffer bytes.Buffer

	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", indent)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("encoding JSON: %w", err)
	}

	// Encode always appends a newline; callers decide their own line endings.
	return bytes.TrimRight(buffer.Bytes(), "\n"), nil
}
