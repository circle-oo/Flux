package github

import "fmt"

// Comment represents a GitHub PR comment.
type Comment struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
	User string `json:"user"`
}

// CreatePR creates a pull request. Stub for Phase 2A.
func (c *Client) CreatePR(owner, repo, head, base, title, body string) (string, error) {
	return "", fmt.Errorf("CreatePR not implemented (Phase 2A)")
}

// MergePR merges a pull request. Stub for Phase 2A.
func (c *Client) MergePR(owner, repo string, prNumber int) error {
	return fmt.Errorf("MergePR not implemented (Phase 2A)")
}

// FetchPRComments fetches comments from a PR. Stub for Phase 2A.
func (c *Client) FetchPRComments(owner, repo string, prNumber int) ([]Comment, error) {
	return nil, fmt.Errorf("FetchPRComments not implemented (Phase 2A)")
}
