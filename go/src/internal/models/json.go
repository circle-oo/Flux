package models

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// marshalStringSlice marshals a string slice to a JSON string for DB storage.
// Returns "[]" for nil slices to ensure consistent DB values.
func marshalStringSlice(field string, v []string) (string, error) {
	if v == nil {
		return "[]", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal %s: %w", field, err)
	}
	return string(b), nil
}

// unmarshalStringSlice unmarshals a JSON string from DB into a string slice.
// Logs a warning (instead of failing) for corrupt JSON and always returns
// a non-nil slice.
func unmarshalStringSlice(field, raw string) []string {
	var v []string
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		slog.Warn("corrupt JSON in DB", "field", field, "error", err)
	}
	if v == nil {
		v = []string{}
	}
	return v
}
