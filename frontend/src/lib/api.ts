// API Client for Flux backend

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
  type: 'CODING' | 'RESEARCH' | 'DOCUMENT' | 'MAINTENANCE' | 'DEPLOY' | 'BUGFIX' | 'PLANNING'
  status: 'PENDING' | 'READY' | 'RUNNING' | 'COMPLETED' | 'FAILED' | 'RETRY' | 'ARCHIVED'
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
      const error = await response.text()
      throw new Error(error || `HTTP ${response.status}`)
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
    type: Task['type']
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

  async retryTask(id: string): Promise<Task> {
    return this.fetch(`/api/tasks/${id}/retry`, { method: 'POST' })
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

  // Logs
  async getRecentLogs(): Promise<
    { time: string; level: string; msg: string; attrs: Record<string, unknown> }[]
  > {
    const data = await this.fetch<{
      logs: { time: string; level: string; msg: string; attrs: Record<string, unknown> }[]
    }>('/api/logs/recent')
    return data.logs || []
  }
}

export const api = new APIClient()
