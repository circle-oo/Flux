package server

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
)

// queryInt parses an integer query parameter.
// Invalid or empty values fall back to zero for backward compatibility.
func queryInt(q url.Values, key string) int {
	raw := q.Get(key)
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		slog.Debug("invalid query parameter", "key", key, "value", raw)
		return 0
	}
	return value
}

// sliceOrEmpty normalizes nil slices to empty slices for JSON responses.
func sliceOrEmpty[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// readOptionalJSON decodes request JSON only when a body is provided.
func readOptionalJSON(w http.ResponseWriter, r *http.Request, v interface{}) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}
	return readJSON(w, r, v)
}
