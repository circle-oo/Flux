package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/circle-oo/flux/internal/models"
)

// Client provides an HTTP client for pods to communicate with Manager.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient creates a new Client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

// NextTask requests the next task from the Manager.
// POST /internal/tasks/next
func (c *Client) NextTask(podID, podType string) (*models.Task, error) {
	slog.Debug("requesting next task from manager", "pod_id", podID, "pod_type", podType)
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

	slog.Debug("next task response", "has_task", result.Task != nil)
	return result.Task, nil
}

// TaskDoneRequest holds all execution detail fields reported on task completion.
type TaskDoneRequest struct {
	Status       string  `json:"status"`
	Result       string  `json:"result"`
	ErrorLog     string  `json:"error_log"`
	TokensUsed   int     `json:"tokens_used"`
	CostUSD      float64 `json:"cost_usd"`
	ExecutorID   string  `json:"executor_id,omitempty"`
	Model        string  `json:"model,omitempty"`
	BranchName   string  `json:"branch_name,omitempty"`
	DiffLines    int     `json:"diff_lines,omitempty"`
	FilesChanged int     `json:"files_changed,omitempty"`
	TestPassed   *bool   `json:"test_passed,omitempty"`
	PRUrl        string  `json:"pr_url,omitempty"`
	PRStatus     string  `json:"pr_status,omitempty"`
}

// ReportTaskDone reports task completion to the Manager.
// POST /internal/tasks/{id}/done
func (c *Client) ReportTaskDone(taskID string, task *models.Task, status, result, errorLog string, tokensUsed int, costUSD float64) error {
	slog.Info("reporting task completion", "task_id", taskID, "status", status, "tokens", tokensUsed, "cost_usd", costUSD)
	req := TaskDoneRequest{
		Status:     status,
		Result:     result,
		ErrorLog:   errorLog,
		TokensUsed: tokensUsed,
		CostUSD:    costUSD,
	}
	if task != nil {
		req.ExecutorID = task.ExecutorID
		req.Model = task.Model
		req.BranchName = task.BranchName
		req.DiffLines = task.DiffLines
		req.FilesChanged = task.FilesChanged
		req.TestPassed = task.TestPassed
		req.PRUrl = task.PRUrl
		req.PRStatus = task.PRStatus
	}

	body, err := json.Marshal(req)
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

	slog.Debug("task completion reported successfully", "task_id", taskID)
	return nil
}

// CreateTask registers a new task via the Manager internal API.
// POST /internal/tasks
func (c *Client) CreateTask(task *models.Task) error {
	body, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}

	resp, err := c.http.Post(
		c.baseURL+"/internal/tasks",
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

	// Decode response to populate task ID
	if err := json.NewDecoder(resp.Body).Decode(task); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

// GetTaskStatus fetches the current status of a task.
// GET /internal/tasks/{id}/status
func (c *Client) GetTaskStatus(taskID string) (string, error) {
	url := fmt.Sprintf("%s/internal/tasks/%s/status", c.baseURL, taskID)
	resp, err := c.http.Get(url)
	if err != nil {
		return "", fmt.Errorf("get request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return result.Status, nil
}

// SubtaskRequest represents a subtask creation request.
type SubtaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// CreateSubtasks creates subtasks for a parent task.
// POST /internal/subtasks
func (c *Client) CreateSubtasks(parentID string, subtasks []SubtaskRequest) error {
	slog.Info("creating subtasks", "parent_id", parentID, "count", len(subtasks))
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

	slog.Debug("subtasks created successfully", "parent_id", parentID)
	return nil
}

// ReportTaskStarted reports execution start details to the server immediately.
// POST /internal/tasks/{id}/started
func (c *Client) ReportTaskStarted(taskID, executorID, model, branchName string) error {
	slog.Debug("reporting task execution start", "task_id", taskID, "executor_id", executorID, "model", model)
	req := map[string]string{
		"executor_id": executorID,
		"model":       model,
		"branch_name": branchName,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/internal/tasks/%s/started", c.baseURL, taskID)
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

// NextPending requests the next PENDING task for the triager.
// POST /internal/tasks/next-pending
func (c *Client) NextPending(triagerID string) (*models.Task, error) {
	slog.Debug("requesting next pending task", "triager_id", triagerID)
	req := map[string]string{
		"triager_id": triagerID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.http.Post(
		c.baseURL+"/internal/tasks/next-pending",
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

// ReportTriaged reports triage completion for a task, promoting it to READY.
// POST /internal/tasks/{id}/triaged
func (c *Client) ReportTriaged(taskID, analysis, description string, priority int) error {
	slog.Info("reporting triage completion", "task_id", taskID)
	req := map[string]interface{}{
		"analysis":    analysis,
		"description": description,
		"priority":    priority,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/internal/tasks/%s/triaged", c.baseURL, taskID)
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

// GetModel queries which model to use for a task.
// GET /internal/model/{task_id}
func (c *Client) GetModel(taskID string) (string, error) {
	slog.Debug("requesting model assignment", "task_id", taskID)
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

	slog.Debug("model assigned", "task_id", taskID, "model", result.Model)
	return result.Model, nil
}

// GetProject retrieves project information via the internal API.
// GET /internal/projects/{id}
func (c *Client) GetProject(projectID string) (*models.Project, error) {
	slog.Debug("requesting project info", "project_id", projectID)
	url := fmt.Sprintf("%s/internal/projects/%s", c.baseURL, projectID)
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
