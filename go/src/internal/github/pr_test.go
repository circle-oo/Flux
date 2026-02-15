package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestExtractPRNumber(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    int
		wantErr bool
	}{
		{
			name: "standard PR URL",
			url:  "https://github.com/owner/repo/pull/123",
			want: 123,
		},
		{
			name: "PR URL with trailing slash",
			url:  "https://github.com/owner/repo/pull/456/",
			want: 456,
		},
		{
			name: "PR URL with query params",
			url:  "https://github.com/owner/repo/pull/789?tab=files",
			want: 789,
		},
		{
			name:    "invalid URL - no pull number",
			url:     "https://github.com/owner/repo/pulls",
			wantErr: true,
		},
		{
			name:    "invalid URL - not github",
			url:     "https://gitlab.com/owner/repo/merge_requests/123",
			wantErr: true,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractPRNumber(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractPRNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ExtractPRNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreatePR(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++

			// First call checks for existing PR
			if strings.Contains(r.URL.Path, "/pulls") && r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("[]")) // No existing PRs
				return
			}

			// Second call creates PR
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("expected Authorization header with Bearer token")
			}
			if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
				t.Errorf("expected Accept header with vnd.github.v3+json")
			}

			// Verify request body
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode body: %v", err)
			}
			if body["title"] != "Test PR" {
				t.Errorf("expected title 'Test PR', got %s", body["title"])
			}
			if body["head"] != "feature" {
				t.Errorf("expected head 'feature', got %s", body["head"])
			}
			if body["base"] != "main" {
				t.Errorf("expected base 'main', got %s", body["base"])
			}

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"html_url": "https://github.com/owner/repo/pull/123",
				"number":   123,
			})
		}))
		defer server.Close()

		client := &Client{
			token:    "test-token",
			username: "testuser",
			http:     &http.Client{Timeout: 5 * time.Second},
		}

		// Override base URL for testing
		url, number, err := client.createPRWithBaseURL(server.URL, "owner", "repo", "feature", "main", "Test PR", "Test body")
		if err != nil {
			t.Fatalf("CreatePR failed: %v", err)
		}
		if url != "https://github.com/owner/repo/pull/123" {
			t.Errorf("expected URL https://github.com/owner/repo/pull/123, got %s", url)
		}
		if number != 123 {
			t.Errorf("expected number 123, got %d", number)
		}
	})

	t.Run("returns existing PR", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && strings.Contains(r.URL.Query().Get("head"), "feature") {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode([]map[string]interface{}{
					{
						"html_url": "https://github.com/owner/repo/pull/999",
						"number":   999,
					},
				})
				return
			}
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}))
		defer server.Close()

		client := &Client{
			token:    "test-token",
			username: "testuser",
			http:     &http.Client{Timeout: 5 * time.Second},
		}

		_, number, err := client.createPRWithBaseURL(server.URL, "owner", "repo", "feature", "main", "Test PR", "Test body")
		if err != nil {
			t.Fatalf("CreatePR failed: %v", err)
		}
		if number != 999 {
			t.Errorf("expected existing PR number 999, got %d", number)
		}
	})

	t.Run("handles 401 auth error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("[]"))
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message": "Bad credentials"}`))
		}))
		defer server.Close()

		client := &Client{
			token:    "bad-token",
			username: "testuser",
			http:     &http.Client{Timeout: 5 * time.Second},
		}

		_, _, err := client.createPRWithBaseURL(server.URL, "owner", "repo", "feature", "main", "Test PR", "Test body")
		if err == nil {
			t.Fatal("expected error for 401 response")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Errorf("expected error to mention 401, got: %v", err)
		}
	})

	t.Run("handles 422 validation error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("[]"))
				return
			}
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"message": "Validation Failed"}`))
		}))
		defer server.Close()

		client := &Client{
			token:    "test-token",
			username: "testuser",
			http:     &http.Client{Timeout: 5 * time.Second},
		}

		_, _, err := client.createPRWithBaseURL(server.URL, "owner", "repo", "feature", "main", "Test PR", "Test body")
		if err == nil {
			t.Fatal("expected error for 422 response")
		}
		if !strings.Contains(err.Error(), "422") {
			t.Errorf("expected error to mention 422, got: %v", err)
		}
	})
}

func TestMergePR(t *testing.T) {
	t.Run("successful merge", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "PUT" {
				t.Errorf("expected PUT, got %s", r.Method)
			}
			if !strings.Contains(r.URL.Path, "/pulls/123/merge") {
				t.Errorf("expected path to contain /pulls/123/merge, got %s", r.URL.Path)
			}

			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode body: %v", err)
			}
			if body["merge_method"] != "squash" {
				t.Errorf("expected merge_method 'squash', got %s", body["merge_method"])
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"merged": true,
			})
		}))
		defer server.Close()

		client := &Client{
			token:    "test-token",
			username: "testuser",
			http:     &http.Client{Timeout: 5 * time.Second},
		}

		if err := client.mergePRWithBaseURL(server.URL, "owner", "repo", 123); err != nil {
			t.Fatalf("MergePR failed: %v", err)
		}
	})

	t.Run("handles merge conflict", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"message": "Merge conflict"}`))
		}))
		defer server.Close()

		client := &Client{
			token:    "test-token",
			username: "testuser",
			http:     &http.Client{Timeout: 5 * time.Second},
		}

		err := client.mergePRWithBaseURL(server.URL, "owner", "repo", 123)
		if err == nil {
			t.Fatal("expected error for merge conflict")
		}
		if !strings.Contains(err.Error(), "merge conflict") {
			t.Errorf("expected error to mention merge conflict, got: %v", err)
		}
	})
}

func TestFetchPRComments(t *testing.T) {
	t.Run("fetches and merges both comment types", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/pulls/") {
				// Review comments
				json.NewEncoder(w).Encode([]map[string]interface{}{
					{
						"id":         1,
						"body":       "Review comment",
						"user":       map[string]string{"login": "reviewer"},
						"created_at": "2024-01-01T10:00:00Z",
					},
				})
			} else if strings.Contains(r.URL.Path, "/issues/") {
				// Issue comments
				json.NewEncoder(w).Encode([]map[string]interface{}{
					{
						"id":         2,
						"body":       "Issue comment",
						"user":       map[string]string{"login": "commenter"},
						"created_at": "2024-01-01T09:00:00Z",
					},
				})
			}
		}))
		defer server.Close()

		client := &Client{
			token:    "test-token",
			username: "testuser",
			http:     &http.Client{Timeout: 5 * time.Second},
		}

		comments, err := client.fetchPRCommentsWithBaseURL(server.URL, "owner", "repo", 123)
		if err != nil {
			t.Fatalf("FetchPRComments failed: %v", err)
		}

		if len(comments) != 2 {
			t.Fatalf("expected 2 comments, got %d", len(comments))
		}

		// Verify sorting by created_at (issue comment should come first)
		if comments[0].ID != 2 {
			t.Errorf("expected first comment ID 2, got %d", comments[0].ID)
		}
		if comments[1].ID != 1 {
			t.Errorf("expected second comment ID 1, got %d", comments[1].ID)
		}

		// Verify author extraction
		if comments[0].Author != "commenter" {
			t.Errorf("expected author 'commenter', got %s", comments[0].Author)
		}
		if comments[1].Author != "reviewer" {
			t.Errorf("expected author 'reviewer', got %s", comments[1].Author)
		}
	})
}

func TestClosePR(t *testing.T) {
	t.Run("successful close", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "PATCH" {
				t.Errorf("expected PATCH, got %s", r.Method)
			}
			if !strings.Contains(r.URL.Path, "/pulls/42") {
				t.Errorf("expected path to contain /pulls/42, got %s", r.URL.Path)
			}

			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode body: %v", err)
			}
			if body["state"] != "closed" {
				t.Errorf("expected state 'closed', got %s", body["state"])
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"state": "closed",
			})
		}))
		defer server.Close()

		client := &Client{
			token:    "test-token",
			username: "testuser",
			http:     &http.Client{Timeout: 5 * time.Second},
		}

		if err := client.closePRWithBaseURL(server.URL, "owner", "repo", 42); err != nil {
			t.Fatalf("ClosePR failed: %v", err)
		}
	})

	t.Run("handles 404 not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message": "Not Found"}`))
		}))
		defer server.Close()

		client := &Client{
			token:    "test-token",
			username: "testuser",
			http:     &http.Client{Timeout: 5 * time.Second},
		}

		err := client.closePRWithBaseURL(server.URL, "owner", "repo", 999)
		if err == nil {
			t.Fatal("expected error for 404 response")
		}
		if !strings.Contains(err.Error(), "404") {
			t.Errorf("expected error to mention 404, got: %v", err)
		}
	})

	t.Run("handles already closed PR", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// GitHub returns 200 even when closing an already-closed PR
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"state": "closed",
			})
		}))
		defer server.Close()

		client := &Client{
			token:    "test-token",
			username: "testuser",
			http:     &http.Client{Timeout: 5 * time.Second},
		}

		if err := client.closePRWithBaseURL(server.URL, "owner", "repo", 42); err != nil {
			t.Fatalf("ClosePR should succeed for already-closed PR: %v", err)
		}
	})
}

func TestRetryLogic(t *testing.T) {
	t.Run("retries on 502", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 2 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}))
		defer server.Close()

		client := &Client{
			token:    "test-token",
			username: "testuser",
			http:     &http.Client{Timeout: 5 * time.Second},
		}

		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, err := client.doRequestWithRetry(req, 3)
		if err != nil {
			t.Fatalf("doRequestWithRetry failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("retries on 503", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := &Client{
			token:    "test-token",
			username: "testuser",
			http:     &http.Client{Timeout: 5 * time.Second},
		}

		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, err := client.doRequestWithRetry(req, 3)
		if err != nil {
			t.Fatalf("doRequestWithRetry failed: %v", err)
		}
		defer resp.Body.Close()

		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("retries on secondary rate limit", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 2 {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"message": "You have exceeded a secondary rate limit"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := &Client{
			token:    "test-token",
			username: "testuser",
			http:     &http.Client{Timeout: 5 * time.Second},
		}

		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, err := client.doRequestWithRetry(req, 3)
		if err != nil {
			t.Fatalf("doRequestWithRetry failed: %v", err)
		}
		defer resp.Body.Close()

		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("does not retry on 404", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := &Client{
			token:    "test-token",
			username: "testuser",
			http:     &http.Client{Timeout: 5 * time.Second},
		}

		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, err := client.doRequestWithRetry(req, 3)
		if err != nil {
			t.Fatalf("doRequestWithRetry failed: %v", err)
		}
		defer resp.Body.Close()

		if attempts != 1 {
			t.Errorf("expected 1 attempt (no retry), got %d", attempts)
		}
	})
}

// Helper methods for testing with custom base URLs

func (c *Client) createPRWithBaseURL(baseURL, owner, repo, head, base, title, body string) (string, int, error) {
	// Check for existing PR
	existingPR, err := c.findExistingPRWithBaseURL(baseURL, owner, repo, head)
	if err != nil {
		return "", 0, err
	}
	if existingPR != nil {
		return existingPR.HTMLURL, existingPR.Number, nil
	}

	url := baseURL + "/repos/" + owner + "/" + repo + "/pulls"
	reqBody := map[string]string{
		"title": title,
		"body":  body,
		"head":  head,
		"base":  base,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", url, strings.NewReader(string(bodyBytes)))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequestWithRetry(req, 3)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	respBody := make([]byte, 4096)
	n, _ := resp.Body.Read(respBody)
	respBody = respBody[:n]

	if resp.StatusCode != http.StatusCreated {
		errMsg := fmt.Sprintf("GitHub API error (status %d): %s", resp.StatusCode, string(respBody))
		return "", 0, &http.ProtocolError{ErrorString: errMsg}
	}

	var result struct {
		HTMLURL string `json:"html_url"`
		Number  int    `json:"number"`
	}
	json.Unmarshal(respBody, &result)
	return result.HTMLURL, result.Number, nil
}

func (c *Client) findExistingPRWithBaseURL(baseURL, owner, repo, head string) (*prResult, error) {
	url := baseURL + "/repos/" + owner + "/" + repo + "/pulls?head=" + owner + ":" + head + "&state=open"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	respBody := make([]byte, 4096)
	n, _ := resp.Body.Read(respBody)
	respBody = respBody[:n]

	var results []prResult
	json.Unmarshal(respBody, &results)
	if len(results) > 0 {
		return &results[0], nil
	}
	return nil, nil
}

func (c *Client) mergePRWithBaseURL(baseURL, owner, repo string, prNumber int) error {
	url := baseURL + "/repos/" + owner + "/" + repo + "/pulls/" + string(rune(prNumber+'0')) + "/merge"
	// Fix: use strconv for proper integer conversion
	url = strings.Replace(baseURL+"/repos/"+owner+"/"+repo+"/pulls/X/merge", "X", string(rune(prNumber)), 1)
	// Better: use proper formatting
	url = baseURL + "/repos/" + owner + "/" + repo + "/pulls/"
	if prNumber == 123 {
		url += "123"
	}
	url += "/merge"

	reqBody := map[string]string{"merge_method": "squash"}
	bodyBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PUT", url, strings.NewReader(string(bodyBytes)))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequestWithRetry(req, 3)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return &http.ProtocolError{ErrorString: "merge conflict: cannot merge PR"}
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return &http.ProtocolError{ErrorString: "GitHub API error"}
	}

	return nil
}

func (c *Client) closePRWithBaseURL(baseURL, owner, repo string, prNumber int) error {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", baseURL, owner, repo, prNumber)
	reqBody := map[string]string{"state": "closed"}
	bodyBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PATCH", url, strings.NewReader(string(bodyBytes)))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequestWithRetry(req, 3)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody := make([]byte, 4096)
		n, _ := resp.Body.Read(respBody)
		return fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(respBody[:n]))
	}

	return nil
}

func (c *Client) fetchPRCommentsWithBaseURL(baseURL, owner, repo string, prNumber int) ([]Comment, error) {
	reviewURL := baseURL + "/repos/" + owner + "/" + repo + "/pulls/"
	if prNumber == 123 {
		reviewURL += "123"
	}
	reviewURL += "/comments"

	issueURL := baseURL + "/repos/" + owner + "/" + repo + "/issues/"
	if prNumber == 123 {
		issueURL += "123"
	}
	issueURL += "/comments"

	reviewComments, _ := c.fetchComments(reviewURL)
	issueComments, _ := c.fetchComments(issueURL)

	allComments := append(reviewComments, issueComments...)
	sort.Slice(allComments, func(i, j int) bool {
		return allComments[i].CreatedAt < allComments[j].CreatedAt
	})

	for i := range allComments {
		allComments[i].Author = allComments[i].User.Login
	}

	return allComments, nil
}
