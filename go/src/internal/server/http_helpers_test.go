package server

import (
	"net/url"
	"testing"
)

func TestQueryInt(t *testing.T) {
	tests := []struct {
		name string
		q    url.Values
		key  string
		want int
	}{
		{
			name: "valid integer",
			q:    url.Values{"page": []string{"3"}},
			key:  "page",
			want: 3,
		},
		{
			name: "missing key returns zero",
			q:    url.Values{},
			key:  "page",
			want: 0,
		},
		{
			name: "invalid integer returns zero",
			q:    url.Values{"page": []string{"abc"}},
			key:  "page",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := queryInt(tt.q, tt.key)
			if got != tt.want {
				t.Fatalf("queryInt() = %d, want %d", got, tt.want)
			}
		})
	}
}
