package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// CreateRepo creates a new GitHub repository using the REST API v3.
// Returns the clone URL on success.
func (c *Client) CreateRepo(name string, private bool) (string, error) {
	payload := map[string]interface{}{
		"name":    name,
		"private": private,
		"auto_init": true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal repo request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.github.com/user/repos", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("github API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("github API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		CloneURL string `json:"clone_url"`
		HTMLURL  string `json:"html_url"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse github response: %w", err)
	}

	return result.HTMLURL, nil
}
