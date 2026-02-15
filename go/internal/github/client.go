package github

import (
	"net/http"
	"time"
)

// Client is a GitHub API client.
type Client struct {
	token    string
	username string
	http     *http.Client
}

// NewClient creates a new GitHub API client.
func NewClient(token, username string) *Client {
	return &Client{
		token:    token,
		username: username,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}
