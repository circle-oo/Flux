package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Comment represents a GitHub PR comment.
type Comment struct {
	ID        int    `json:"id"`
	Body      string `json:"body"`
	User      User   `json:"user"`
	CreatedAt string `json:"created_at"`
	Author    string `json:"-"` // extracted from user.login
}

// User represents a GitHub user.
type User struct {
	Login string `json:"login"`
}

// CreatePR creates a pull request and returns the PR URL and PR number.
func (c *Client) CreatePR(owner, repo, head, base, title, body string) (string, int, error) {
	// Check for existing PR on the same branch
	existingPR, err := c.findExistingPR(owner, repo, head)
	if err != nil {
		return "", 0, fmt.Errorf("checking for existing PR: %w", err)
	}
	if existingPR != nil {
		return existingPR.HTMLURL, existingPR.Number, nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, repo)
	reqBody := map[string]string{
		"title": title,
		"body":  body,
		"head":  head,
		"base":  base,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("marshaling request body: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequestWithRetry(req, 3)
	if err != nil {
		return "", 0, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", 0, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		HTMLURL string `json:"html_url"`
		Number  int    `json:"number"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", 0, fmt.Errorf("unmarshaling response: %w", err)
	}

	return result.HTMLURL, result.Number, nil
}

// prResult represents a PR from the GitHub API.
type prResult struct {
	HTMLURL string `json:"html_url"`
	Number  int    `json:"number"`
}

// findExistingPR checks if a PR already exists for the given head branch.
func (c *Client) findExistingPR(owner, repo, head string) (*prResult, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?head=%s:%s&state=open", owner, repo, owner, head)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil // No existing PR
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var results []prResult
	if err := json.Unmarshal(respBody, &results); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	if len(results) > 0 {
		return &results[0], nil
	}
	return nil, nil
}

// MergePR merges a pull request using squash merge.
func (c *Client) MergePR(owner, repo string, prNumber int) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/merge", owner, repo, prNumber)
	reqBody := map[string]string{
		"merge_method": "squash",
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request body: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequestWithRetry(req, 3)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("merge conflict: cannot merge PR #%d", prNumber)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// FetchPRComments fetches both review comments and issue comments from a PR.
func (c *Client) FetchPRComments(owner, repo string, prNumber int) ([]Comment, error) {
	// Fetch review comments
	reviewComments, err := c.fetchComments(fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/comments", owner, repo, prNumber))
	if err != nil {
		return nil, fmt.Errorf("fetching review comments: %w", err)
	}

	// Fetch issue comments
	issueComments, err := c.fetchComments(fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", owner, repo, prNumber))
	if err != nil {
		return nil, fmt.Errorf("fetching issue comments: %w", err)
	}

	// Merge and sort by created_at
	allComments := append(reviewComments, issueComments...)
	sort.Slice(allComments, func(i, j int) bool {
		return allComments[i].CreatedAt < allComments[j].CreatedAt
	})

	// Extract author from user.login
	for i := range allComments {
		allComments[i].Author = allComments[i].User.Login
	}

	return allComments, nil
}

// fetchComments is a helper that fetches comments from a specific endpoint.
func (c *Client) fetchComments(url string) ([]Comment, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var comments []Comment
	if err := json.Unmarshal(respBody, &comments); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	return comments, nil
}

// CreateComment posts a comment on a pull request.
func (c *Client) CreateComment(owner, repo string, prNumber int, body string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", owner, repo, prNumber)
	reqBody := map[string]string{
		"body": body,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request body: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequestWithRetry(req, 3)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// ExtractPRNumber parses a PR number from a GitHub PR URL.
func ExtractPRNumber(prURL string) (int, error) {
	// Match URLs like https://github.com/owner/repo/pull/123
	re := regexp.MustCompile(`github\.com/[^/]+/[^/]+/pull/(\d+)`)
	matches := re.FindStringSubmatch(prURL)
	if len(matches) < 2 {
		return 0, fmt.Errorf("invalid PR URL format: %s", prURL)
	}
	return strconv.Atoi(matches[1])
}

// doRequestWithRetry executes an HTTP request with exponential backoff retry.
func (c *Client) doRequestWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Clone the request body for retries
		var bodyBytes []byte
		if req.Body != nil {
			bodyBytes, err = io.ReadAll(req.Body)
			if err != nil {
				return nil, fmt.Errorf("reading request body: %w", err)
			}
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err = c.http.Do(req)
		if err != nil {
			// Network error - retry
			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(1<<attempt) * time.Second)
				continue
			}
			return nil, err
		}

		// Check if we should retry
		shouldRetry := false
		if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable {
			shouldRetry = true
		} else if resp.StatusCode == http.StatusForbidden {
			// Check for secondary rate limit
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if strings.Contains(strings.ToLower(string(body)), "secondary rate limit") {
				shouldRetry = true
			}
			// Restore body for return
			resp.Body = io.NopCloser(bytes.NewReader(body))
		}

		if !shouldRetry || attempt == maxRetries-1 {
			return resp, nil
		}

		resp.Body.Close()
		time.Sleep(time.Duration(1<<attempt) * time.Second)

		// Reset request body for next attempt
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	}

	return resp, err
}
