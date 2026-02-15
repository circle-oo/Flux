package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/circle-oo/flux/internal/models"
)

// ManagerClient provides an HTTP client for Executor pods to communicate with Manager.
type ManagerClient struct {
	baseURL string
	http    *http.Client
}

// NewManagerClient creates a new ManagerClient.
func NewManagerClient(baseURL string) *ManagerClient {
	return &ManagerClient{
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

// NextTask requests the next task from the Manager.
// POST /internal/tasks/next
func (c *ManagerClient) NextTask(podID, podType string) (*models.Task, error) {
	req := map[string]string{
		"pod_id":   podID,
		"pod_type": podType,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.http.Post(
		c.baseURL+"/internal/tasks/next",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("post request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result struct {
		Task *models.Task `json:"task"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Task, nil
}

// TaskDoneRequest contains all fields reported when a task completes execution.
type TaskDoneRequest struct {
	Status       string  `json:"status"`
	Result       string  `json:"result"`
	ErrorLog     string  `json:"error_log"`
	TokensUsed   int     `json:"tokens_used"`
	CostUSD      float64 `json:"cost_usd"`
	Model        string  `json:"model,omitempty"`
	BranchName   string  `json:"branch_name,omitempty"`
	PRUrl        string  `json:"pr_url,omitempty"`
	PRStatus     string  `json:"pr_status,omitempty"`
	DiffLines    int     `json:"diff_lines,omitempty"`
	FilesChanged int     `json:"files_changed,omitempty"`
	TestPassed   *bool   `json:"test_passed,omitempty"`
}

// ReportTaskDone reports task completion to the Manager.
// POST /internal/tasks/{id}/done
func (c *ManagerClient) ReportTaskDone(taskID string, done TaskDoneRequest) error {
	body, err := json.Marshal(done)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/internal/tasks/%s/done", c.baseURL, taskID)
	resp, err := c.http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// SubtaskRequest represents a subtask creation request.
type SubtaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// CreateSubtasks creates subtasks for a parent task.
// POST /internal/subtasks
func (c *ManagerClient) CreateSubtasks(parentID string, subtasks []SubtaskRequest) error {
	req := map[string]interface{}{
		"parent_id": parentID,
		"subtasks":  subtasks,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.http.Post(
		c.baseURL+"/internal/subtasks",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("post request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetModel queries which model to use for a task.
// GET /internal/model/{task_id}
func (c *ManagerClient) GetModel(taskID string) (string, error) {
	url := fmt.Sprintf("%s/internal/model/%s", c.baseURL, taskID)
	resp, err := c.http.Get(url)
	if err != nil {
		return "", fmt.Errorf("get request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return result.Model, nil
}

// GetProject retrieves project information.
// GET /api/projects/{id}
func (c *ManagerClient) GetProject(projectID string) (*models.Project, error) {
	url := fmt.Sprintf("%s/api/projects/%s", c.baseURL, projectID)
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var project models.Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &project, nil
}
