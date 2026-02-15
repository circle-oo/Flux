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

// postJSON marshals reqBody as JSON, POSTs to the given path, checks for the
// expected status code, and optionally decodes the response into respBody.
// Pass nil for respBody if no response decoding is needed.
func (c *Client) postJSON(path string, expectedStatus int, reqBody, respBody interface{}) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.http.Post(c.baseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if respBody != nil {
		if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// getJSON sends a GET to the given path, checks for 200 OK, and decodes the
// response into respBody.
func (c *Client) getJSON(path string, respBody interface{}) error {
	resp, err := c.http.Get(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("get request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

// NextTask requests the next task from the Manager.
// POST /internal/tasks/next
func (c *Client) NextTask(podID, podType string) (*models.Task, error) {
	slog.Debug("requesting next task from manager", "pod_id", podID, "pod_type", podType)

	var result struct {
		Task *models.Task `json:"task"`
	}
	err := c.postJSON("/internal/tasks/next", http.StatusOK,
		map[string]string{"pod_id": podID, "pod_type": podType},
		&result,
	)
	if err != nil {
		return nil, err
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

	path := fmt.Sprintf("/internal/tasks/%s/done", taskID)
	if err := c.postJSON(path, http.StatusOK, req, nil); err != nil {
		return err
	}

	slog.Debug("task completion reported successfully", "task_id", taskID)
	return nil
}

// CreateTask registers a new task via the Manager internal API.
// POST /internal/tasks
func (c *Client) CreateTask(task *models.Task) error {
	return c.postJSON("/internal/tasks", http.StatusCreated, task, task)
}

// GetTaskStatus fetches the current status of a task.
// GET /internal/tasks/{id}/status
func (c *Client) GetTaskStatus(taskID string) (string, error) {
	var result struct {
		Status string `json:"status"`
	}
	path := fmt.Sprintf("/internal/tasks/%s/status", taskID)
	if err := c.getJSON(path, &result); err != nil {
		return "", err
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
	if err := c.postJSON("/internal/subtasks", http.StatusCreated, req, nil); err != nil {
		return err
	}

	slog.Debug("subtasks created successfully", "parent_id", parentID)
	return nil
}

// ReportTaskStarted reports execution start details to the server immediately.
// POST /internal/tasks/{id}/started
func (c *Client) ReportTaskStarted(taskID, executorID, model, branchName string) error {
	slog.Debug("reporting task execution start", "task_id", taskID, "executor_id", executorID, "model", model)

	path := fmt.Sprintf("/internal/tasks/%s/started", taskID)
	return c.postJSON(path, http.StatusOK,
		map[string]string{"executor_id": executorID, "model": model, "branch_name": branchName},
		nil,
	)
}

// NextPending requests the next PENDING task for the triager.
// POST /internal/tasks/next-pending
func (c *Client) NextPending(triagerID string) (*models.Task, error) {
	slog.Debug("requesting next pending task", "triager_id", triagerID)

	var result struct {
		Task *models.Task `json:"task"`
	}
	err := c.postJSON("/internal/tasks/next-pending", http.StatusOK,
		map[string]string{"triager_id": triagerID},
		&result,
	)
	if err != nil {
		return nil, err
	}

	return result.Task, nil
}

// ReportTriaged reports triage completion for a task, promoting it to READY.
// POST /internal/tasks/{id}/triaged
func (c *Client) ReportTriaged(taskID, analysis, description string, priority int) error {
	slog.Info("reporting triage completion", "task_id", taskID)

	path := fmt.Sprintf("/internal/tasks/%s/triaged", taskID)
	return c.postJSON(path, http.StatusOK,
		map[string]interface{}{"analysis": analysis, "description": description, "priority": priority},
		nil,
	)
}

// GetModel queries which model to use for a task.
// GET /internal/model/{task_id}
func (c *Client) GetModel(taskID string) (string, error) {
	slog.Debug("requesting model assignment", "task_id", taskID)

	var result struct {
		Model string `json:"model"`
	}
	path := fmt.Sprintf("/internal/model/%s", taskID)
	if err := c.getJSON(path, &result); err != nil {
		return "", err
	}

	slog.Debug("model assigned", "task_id", taskID, "model", result.Model)
	return result.Model, nil
}

// GetProject retrieves project information via the internal API.
// GET /internal/projects/{id}
func (c *Client) GetProject(projectID string) (*models.Project, error) {
	slog.Debug("requesting project info", "project_id", projectID)

	var project models.Project
	path := fmt.Sprintf("/internal/projects/%s", projectID)
	if err := c.getJSON(path, &project); err != nil {
		return nil, err
	}

	return &project, nil
}
