// API Client for Flux backend

export const TaskStatus = {
  PENDING: 'PENDING',
  READY: 'READY',
  RUNNING: 'RUNNING',
  COMPLETED: 'COMPLETED',
  FAILED: 'FAILED',
  CANCELLED: 'CANCELLED',
  RETRY: 'RETRY',
  ARCHIVED: 'ARCHIVED',
  DECOMPOSED: 'DECOMPOSED',
} as const

export type TaskStatusType = (typeof TaskStatus)[keyof typeof TaskStatus]

export interface Goal {
  id: string
  title: string
  description: string
  priorities: string[]
  metrics: string[]
  status: 'PROPOSED' | 'ACTIVE' | 'COMPLETED' | 'SUPERSEDED'
  source: 'OPERATOR' | 'ORCHESTRATOR'
  created_at: string
  active_since?: string
}

export interface Task {
  id: string
  title: string
  description: string
  status: TaskStatusType
  priority: number
  project_id: string
  goal_id?: string
  source: 'OPERATOR' | 'RESEARCHER' | 'SELF' | 'SYSTEM'
  pr_url?: string
  pr_status?: string
  error_log?: string
  result?: string
  prompt?: string
  triage_analysis?: string
  triage_description?: string
  triage_title?: string
  plan?: string
  executor_id?: string
  model?: string
  branch_name?: string
  test_passed?: boolean | null
  retry_count?: number
  crash_recovery?: boolean
  tokens_used?: number
  cost_usd?: number
  created_at: string
  updated_at?: string
  started_at?: string
  completed_at?: string
  parent_id?: string
  depth?: number
  depends_on: string[]
  tags: string[]
  diff_lines?: number
  files_changed?: number
}

export interface Project {
  id: string
  name: string
  type: 'REPO' | 'SERVICE' | 'LIBRARY' | 'TOOL'
  repo_url?: string
  description: string
  vault_path: string
  status: 'PROPOSED' | 'ACTIVE' | 'ARCHIVED' | 'REJECTED'
  tech_stack: string[]
  inspiration?: string
  goal_id?: string
  created_at: string
  updated_at?: string
}

export interface Pod {
  id: string
  pod_type: 'executor' | 'triager' // researcher will be added in future
  status: 'idle' | 'busy'
  current_task: string
  task_title: string
  started_at: string
  last_seen: string
  task_count: number
}

export interface UpdaterStatus {
  enabled: boolean
  running: boolean
  state: string
  branch: string
  check_interval: string
  last_check_at: string | null
  last_update_at: string | null
  last_error?: string
  local_commit?: string
  remote_commit?: string
  update_count: number
  next_check_at: string | null
}

export interface DeployStatusResponse {
  version: string
  updater: UpdaterStatus
}

export interface ProjectActivity {
  project_id: string
  project_name: string
  task_count: number
}

export interface Insights {
  total_tokens: number
  total_cost: number
  project_activities: ProjectActivity[]
}

export interface TaskStatusCounts {
  completed: number
  failed: number
  running: number
  ready: number
  pending: number
}

export interface TaskStats {
  today: TaskStatusCounts
  yesterday: TaskStatusCounts
}

// Detailed insights types
export interface InsightsSummary {
  total_tasks: number
  completed_tasks: number
  failed_tasks: number
  success_rate: number
  total_tokens: number
  total_cost: number
  triage_tokens: number
  triage_cost: number
  execution_tokens: number
  execution_cost: number
  avg_latency_min: number
  active_projects: number
}

export interface DailyMetric {
  date: string
  tasks_completed: number
  tasks_failed: number
  tasks_created: number
  total_tokens: number
  total_cost: number
  avg_latency_minutes: number
}

export interface AgentPerformance {
  executor_id: string
  tasks_completed: number
  tasks_failed: number
  success_rate: number
  avg_duration_min: number
}

export interface PipelineHealth {
  status: string
  count: number
}

export interface FailureAnalysis {
  category: string
  count: number
  examples: string[]
}

export interface SubComponentStatus {
  name: string
  healthy: boolean
  last_tick: string
  last_error: string
}

export interface ScaleStatus {
  executor_pods: number
  triager_pods: number
  researcher_pods: number
  max_executor_pods: number
  max_triager_pods: number
  max_researcher_pods: number
  queue_state: string
  last_scale_time: string
}

export interface DiskStatus {
  available_bytes: number
  total_bytes: number
  used_bytes: number
  level: string
}

export interface OrchestratorStatus {
  running: boolean
  uptime: string
  started_at?: string
  tick_count: number
  rate_limited: boolean
  rate_limit_until?: string
  sub_components: SubComponentStatus[]
  scale_status?: ScaleStatus
  disk_status?: DiskStatus
}

export interface DiskUsageResponse {
  available_bytes: number
  total_bytes: number
  used_bytes: number
  level: string
  available_gb: number
  total_gb: number
  used_pct: number
}

export interface TaskAttempt {
  id: number
  task_id: string
  attempt: number
  status: string
  result: string
  error_log: string
  executor_id: string
  model: string
  branch_name: string
  pr_url: string
  pr_status: string
  diff_lines: number
  files_changed: number
  test_passed: boolean | null
  tokens_used: number
  cost_usd: number
  triage_analysis: string
  triage_description: string
  triage_title: string
  started_at: string
  completed_at: string
  created_at: string
}

export interface TaskAttemptsResponse {
  attempts: TaskAttempt[]
  total_tokens_used: number
  total_cost_usd: number
}

// Task usage event types
export interface TaskUsageEvent {
  id: number
  task_id: string
  source: string
  tokens: number
  cost_usd: number
  meta: Record<string, string>
  recorded_at: string
}

export interface TaskUsageResponse {
  events: TaskUsageEvent[]
  total_tokens: number
  total_cost: number
}

export interface UsageTimePoint {
  time: string
  tokens: number
  cost_usd: number
  task_count: number
}

export interface CCUsageModelBreakdown {
  modelName: string
  inputTokens: number
  outputTokens: number
  cacheCreationTokens: number
  cacheReadTokens: number
  cost: number
}

export interface CCUsageDaily {
  date: string
  inputTokens: number
  outputTokens: number
  cacheCreationTokens: number
  cacheReadTokens: number
  totalTokens: number
  totalCost: number
  modelsUsed?: string[]
  modelBreakdowns?: CCUsageModelBreakdown[]
}

export interface CCUsageBlock {
  blockStart: string
  blockEnd: string
  inputTokens: number
  outputTokens: number
  cacheCreationTokens: number
  cacheReadTokens: number
  totalTokens: number
  totalCost: number
  sessionCount: number
  isActive: boolean
  projectedTokens?: number
  projectedCost?: number
}

export interface BillingInfo {
  plan: string
  configured_plan: string
  is_api: boolean
  show_cost: boolean
  // ccusage data (always available, calculates cost regardless of plan)
  ccusage_daily?: CCUsageDaily
  ccusage_block?: CCUsageBlock
  // API billing fields
  daily_cost_budget?: number
  daily_cost_used?: number
  // Plan billing fields
  window_token_budget?: number
  window_hours?: number
  window_tokens_used?: number
  window_start?: string
  window_end?: string
}

class APIClient {
  private baseURL = ''

  private async fetch<T>(
    url: string,
    options: RequestInit = {}
  ): Promise<T> {
    const response = await fetch(this.baseURL + url, {
      ...options,
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    })

    if (!response.ok) {
      const text = await response.text()
      let message = text || `HTTP ${response.status}`
      try {
        const parsed = JSON.parse(text)
        if (parsed.error) message = parsed.error
      } catch {
        // Use raw text if not valid JSON
      }
      throw new Error(message)
    }

    return response.json()
  }

  // Auth
  async login(password: string): Promise<{ status: string }> {
    return this.fetch('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ password }),
    })
  }

  async logout(): Promise<void> {
    await this.fetch('/api/auth/logout', { method: 'POST' })
  }

  // Goals
  async listGoals(): Promise<Goal[]> {
    const data = await this.fetch<{ goals: Goal[] }>('/api/goals')
    return data.goals || []
  }

  async getCurrentGoal(): Promise<Goal | null> {
    try {
      const data = await this.fetch<{ goal: Goal }>('/api/goals/current')
      return data.goal
    } catch {
      return null
    }
  }

  async createGoal(goal: {
    title: string
    description: string
    priorities: string[]
    metrics: string[]
  }): Promise<Goal> {
    return this.fetch('/api/goals', {
      method: 'POST',
      body: JSON.stringify(goal),
    })
  }

  async updateGoal(id: string, updates: Partial<Goal>): Promise<Goal> {
    return this.fetch(`/api/goals/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(updates),
    })
  }

  async activateGoal(id: string): Promise<Goal> {
    return this.fetch(`/api/goals/${id}/activate`, {
      method: 'POST',
    })
  }

  // Tasks
  async listTasks(filters?: {
    status?: string
    project_id?: string
    page?: number
    limit?: number
  }): Promise<Task[]> {
    const params = new URLSearchParams()
    if (filters?.status) params.set('status', filters.status)
    if (filters?.project_id) params.set('project_id', filters.project_id)
    if (filters?.page) params.set('page', filters.page.toString())
    if (filters?.limit) params.set('limit', filters.limit.toString())

    const query = params.toString() ? `?${params.toString()}` : ''
    const data = await this.fetch<{ tasks: Task[] }>(`/api/tasks${query}`)
    return data.tasks || []
  }

  async getTask(id: string): Promise<Task> {
    return this.fetch(`/api/tasks/${id}`)
  }

  async createTask(task: {
    title: string
    description: string
    priority: number
    project_id: string
    goal_id?: string
    depends_on?: string[]
    tags?: string[]
  }): Promise<Task> {
    return this.fetch('/api/tasks', {
      method: 'POST',
      body: JSON.stringify({
        ...task,
        depends_on: task.depends_on || [],
        tags: task.tags || [],
      }),
    })
  }

  async updateTask(id: string, updates: Partial<Task>): Promise<Task> {
    return this.fetch(`/api/tasks/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(updates),
    })
  }

  async deleteTask(id: string): Promise<void> {
    await this.fetch(`/api/tasks/${id}`, { method: 'DELETE' })
  }

  async cancelTask(id: string): Promise<void> {
    await this.fetch(`/api/tasks/${id}/cancel`, { method: 'POST' })
  }

  async archiveTask(id: string): Promise<Task> {
    return this.fetch(`/api/tasks/${id}/archive`, { method: 'POST' })
  }

  async retryTask(id: string): Promise<Task> {
    return this.fetch(`/api/tasks/${id}/retry`, { method: 'POST' })
  }

  async getTaskAttempts(id: string): Promise<TaskAttemptsResponse> {
    return this.fetch(`/api/tasks/${id}/attempts`)
  }

  async listSubtasks(parentId: string): Promise<Task[]> {
    const data = await this.fetch<{ tasks: Task[] }>(`/api/tasks/${parentId}/subtasks`)
    return data.tasks || []
  }

  async getSubtaskDependencies(parentId: string): Promise<{
    nodes: Task[]
    edges: { dependent_id: string; dependency_id: string }[]
  }> {
    return this.fetch(`/api/tasks/${parentId}/subtasks/dependencies`)
  }

  // Task Stats
  async getTaskStats(): Promise<TaskStats> {
    return this.fetch('/api/tasks/stats')
  }

  // Projects
  async listProjects(): Promise<Project[]> {
    const data = await this.fetch<{ projects: Project[] }>('/api/projects')
    return data.projects || []
  }

  async getProject(id: string): Promise<Project> {
    return this.fetch(`/api/projects/${id}`)
  }

  async createProject(project: {
    name: string
    type: Project['type']
    description: string
    tech_stack: string[]
    inspiration?: string
    repo_url?: string
    goal_id?: string
  }): Promise<Project> {
    return this.fetch('/api/projects', {
      method: 'POST',
      body: JSON.stringify(project),
    })
  }

  async approveProject(id: string): Promise<Project> {
    return this.fetch(`/api/projects/${id}/approve`, {
      method: 'POST',
    })
  }

  async rejectProject(id: string): Promise<Project> {
    return this.fetch(`/api/projects/${id}/reject`, {
      method: 'POST',
    })
  }

  // PRs
  async listPRs(statusFilter?: string): Promise<Task[]> {
    const params = new URLSearchParams()
    if (statusFilter) params.set('status', statusFilter)

    const query = params.toString() ? `?${params.toString()}` : ''
    const data = await this.fetch<{ tasks: Task[] }>(`/api/prs/pending${query}`)
    return data.tasks || []
  }

  async approvePR(taskId: string): Promise<{ status: string; task: Task }> {
    return this.fetch(`/api/prs/${taskId}/approve`, {
      method: 'POST',
    })
  }

  async requestPRChanges(taskId: string): Promise<{ status: string; fix_task_id: string; fix_task: Task }> {
    return this.fetch(`/api/prs/${taskId}/request-changes`, {
      method: 'POST',
    })
  }

  async closePR(taskId: string): Promise<{ status: string; task: Task }> {
    return this.fetch(`/api/prs/${taskId}/close`, {
      method: 'POST',
    })
  }

  // System
  async restart(): Promise<{ status: string; message: string }> {
    return this.fetch('/api/system/restart', {
      method: 'POST',
    })
  }

  // Deploy
  async getDeployStatus(): Promise<DeployStatusResponse> {
    return this.fetch('/api/system/deploy/status')
  }

  async triggerDeploy(): Promise<{ status: string; message: string }> {
    return this.fetch('/api/system/deploy', {
      method: 'POST',
    })
  }

  async checkRemoteCommit(): Promise<{ status: string; message: string }> {
    return this.fetch('/api/system/deploy/check-remote', {
      method: 'POST',
    })
  }

  // Logs
  async getRecentLogs(): Promise<
    { time: string; level: string; msg: string; attrs: Record<string, unknown> }[]
  > {
    const data = await this.fetch<{
      logs: { time: string; level: string; msg: string; attrs: Record<string, unknown> }[]
    }>('/api/logs/recent')
    return data.logs || []
  }

  // Pods
  async listPods(): Promise<Pod[]> {
    const data = await this.fetch<{ pods: Pod[] }>('/api/pods')
    return data.pods || []
  }

  // Config
  async getHealth(): Promise<{ status: string; version: string; auth_enabled: boolean }> {
    return this.fetch('/health')
  }

  // Insights
  async getInsights(): Promise<Insights> {
    return this.fetch('/api/insights')
  }

  // Config
  async getConfig(): Promise<Record<string, unknown>> {
    return this.fetch('/api/config')
  }

  // Detailed Insights
  async getInsightsSummary(period: string): Promise<InsightsSummary> {
    return this.fetch(`/api/insights/summary?period=${period}`)
  }

  async getInsightsTimeseries(period: string): Promise<DailyMetric[]> {
    return this.fetch(`/api/insights/timeseries?period=${period}`)
  }

  async getInsightsEfficiency(): Promise<AgentPerformance[]> {
    return this.fetch('/api/insights/efficiency')
  }

  async getInsightsPipeline(): Promise<PipelineHealth[]> {
    return this.fetch('/api/insights/pipeline')
  }

  async getInsightsFailures(period: string): Promise<FailureAnalysis[]> {
    return this.fetch(`/api/insights/failures?period=${period}`)
  }

  // Knowledge
  async listNotes(folder?: string): Promise<string[]> {
    const params = folder ? `?folder=${encodeURIComponent(folder)}` : ''
    const data = await this.fetch<{ notes: string[] }>(`/api/knowledge/notes${params}`)
    return data.notes || []
  }

  async getNote(path: string): Promise<{ path: string; content: string }> {
    return this.fetch(`/api/knowledge/notes/${path}`)
  }

  async createNote(path: string, content: string): Promise<{ path: string; content: string }> {
    return this.fetch('/api/knowledge/notes', {
      method: 'POST',
      body: JSON.stringify({ path, content }),
    })
  }

  async updateNote(path: string, content: string): Promise<{ path: string; content: string }> {
    return this.fetch(`/api/knowledge/notes/${path}`, {
      method: 'PUT',
      body: JSON.stringify({ content }),
    })
  }

  async deleteNote(path: string): Promise<void> {
    await this.fetch(`/api/knowledge/notes/${path}`, { method: 'DELETE' })
  }

  async searchKnowledge(query: string): Promise<{ query: string; results: string[] }> {
    return this.fetch(`/api/knowledge/search?q=${encodeURIComponent(query)}`)
  }

  async getDaily(): Promise<{ date: string; path: string; content: string }> {
    return this.fetch('/api/knowledge/daily')
  }

  async appendDaily(content: string): Promise<{ status: string }> {
    return this.fetch('/api/knowledge/daily/append', {
      method: 'POST',
      body: JSON.stringify({ content }),
    })
  }

  async getKnowledgeStats(): Promise<{
    note_count: number
    folder_count: number
    total_size: number
    mode: string
    healthy: boolean
  }> {
    return this.fetch('/api/knowledge/stats')
  }

  async getKnowledgeHealth(): Promise<{ mode: string; healthy: boolean }> {
    return this.fetch('/api/knowledge/health')
  }

  async listFolders(): Promise<string[]> {
    const data = await this.fetch<{ folders: string[] }>('/api/knowledge/folders')
    return data.folders || []
  }

  async getRecentNotes(): Promise<{ path: string; mod_time: string }[]> {
    const data = await this.fetch<{ notes: { path: string; mod_time: string }[] }>('/api/knowledge/recent')
    return data.notes || []
  }

  async getOrphans(): Promise<string[]> {
    const data = await this.fetch<{ orphans: string[] }>('/api/knowledge/orphans')
    return data.orphans || []
  }

  // Task Usage
  async getTaskUsage(id: string): Promise<TaskUsageResponse> {
    return this.fetch(`/api/tasks/${id}/usage`)
  }

  // Realtime Usage
  async getInsightsUsageRealtime(minutes?: number): Promise<UsageTimePoint[]> {
    const params = minutes ? `?minutes=${minutes}` : ''
    return this.fetch(`/api/insights/usage-realtime${params}`)
  }

  // Orchestrator
  async getOrchestratorStatus(): Promise<OrchestratorStatus> {
    return this.fetch('/api/orchestrator/status')
  }

  // Disk Usage
  async getDiskUsage(): Promise<DiskUsageResponse> {
    return this.fetch('/api/system/disk')
  }

  // Billing Info
  async getBillingInfo(): Promise<BillingInfo> {
    return this.fetch('/api/billing')
  }
}

export const api = new APIClient()
