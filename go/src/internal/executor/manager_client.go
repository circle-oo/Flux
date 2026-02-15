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

// ReportTaskDone reports task completion to the Manager.
// POST /internal/tasks/{id}/done
func (c *ManagerClient) ReportTaskDone(taskID, status, result, errorLog string, tokensUsed int, costUSD float64) error {
	req := map[string]interface{}{
		"status":      status,
		"result":      result,
		"error_log":   errorLog,
		"tokens_used": tokensUsed,
		"cost_usd":    costUSD,
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

	return nil
}

// CreateTask registers a new task via the Manager internal API.
// POST /internal/tasks
func (c *ManagerClient) CreateTask(task *models.Task) error {
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
