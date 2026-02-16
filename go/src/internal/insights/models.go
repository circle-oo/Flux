package insights

// DailyMetric holds date-bucketed task statistics.
type DailyMetric struct {
	Date              string  `json:"date"`
	TasksCompleted    int     `json:"tasks_completed"`
	TasksFailed       int     `json:"tasks_failed"`
	TasksCreated      int     `json:"tasks_created"`
	TotalTokens       int     `json:"total_tokens"`
	TotalCost         float64 `json:"total_cost"`
	AvgLatencyMinutes float64 `json:"avg_latency_minutes"`
}

// AgentPerformance holds per-executor statistics.
type AgentPerformance struct {
	ExecutorID     string  `json:"executor_id"`
	TasksCompleted int     `json:"tasks_completed"`
	TasksFailed    int     `json:"tasks_failed"`
	SuccessRate    float64 `json:"success_rate"`
	AvgDurationMin float64 `json:"avg_duration_min"`
}

// ToolUsageStat holds model usage distribution.
type ToolUsageStat struct {
	Model      string `json:"model"`
	TaskCount  int    `json:"task_count"`
	TokensUsed int    `json:"tokens_used"`
}

// PipelineHealth holds status distribution counts.
type PipelineHealth struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// FailureAnalysis holds error categorization with examples.
type FailureAnalysis struct {
	Category string   `json:"category"`
	Count    int      `json:"count"`
	Examples []string `json:"examples"`
}

// InsightsSummary holds overview metrics for a given period.
type InsightsSummary struct {
	TotalTasks     int     `json:"total_tasks"`
	CompletedTasks int     `json:"completed_tasks"`
	FailedTasks    int     `json:"failed_tasks"`
	SuccessRate    float64 `json:"success_rate"`
	TotalTokens    int     `json:"total_tokens"`
	TotalCost      float64 `json:"total_cost"`
	AvgLatencyMin  float64 `json:"avg_latency_min"`
	ActiveProjects int     `json:"active_projects"`
}
