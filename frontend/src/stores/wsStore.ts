import { create } from 'zustand'
import { useGoalStore } from './goalStore'
import { useTaskStore } from './taskStore'
import { useLogStore } from './logStore'
import { useDeployStore } from './deployStore'
import type { UpdaterStatus } from '../lib/api'

type EventType = 'TASK_UPDATED' | 'GOAL_CHANGED' | 'PR_STATUS' | 'POD_STATUS' | 'LOG_ENTRY' | 'DEPLOY_STATUS'

interface TaskUpdatedData { task_id: string; status: string }
interface GoalChangedData { goal_id: string; status: string }
interface PRStatusData { task_id: string; pr_status: string }
interface PodStatusData { pod_id: string; status: string }
interface DeployStatusData { updater: UpdaterStatus }

interface LogEntryData { time: string; level: string; msg: string; attrs: Record<string, unknown> }

type WSEventData = TaskUpdatedData | GoalChangedData | PRStatusData | PodStatusData | DeployStatusData | LogEntryData

interface WSEvent {
  type: EventType
  data: WSEventData
}

interface WSState {
  connected: boolean
  reconnecting: boolean
  socket: WebSocket | null
  taskUpdateCounter: number
  connect: () => void
  disconnect: () => void
  send: (event: WSEvent) => void
}

let reconnectTimeout: ReturnType<typeof setTimeout> | null = null
let reconnectAttempts = 0
const MAX_RECONNECT_ATTEMPTS = 10
const BASE_RECONNECT_DELAY = 1000 // 1 second

export const useWSStore = create<WSState>((set, get) => ({
  connected: false,
  reconnecting: false,
  socket: null,
  taskUpdateCounter: 0,

  connect: () => {
    const { socket, connected } = get()

    // Don't reconnect if already connected
    if (socket && connected) {
      return
    }

    // Clear any pending reconnect
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }

    try {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const wsURL = `${protocol}//${window.location.host}/ws/events`
      const ws = new WebSocket(wsURL)

      ws.onopen = () => {
        if (import.meta.env.DEV) {
          console.log('WebSocket connected')
        }
        reconnectAttempts = 0
        set({ connected: true, reconnecting: false, socket: ws })
      }

      ws.onmessage = (event) => {
        try {
          const wsEvent: WSEvent = JSON.parse(event.data)
          handleEvent(wsEvent)
        } catch (error) {
          console.error('Failed to parse WebSocket event:', error)
        }
      }

      ws.onerror = (error) => {
        console.error('WebSocket error:', error)
      }

      ws.onclose = () => {
        if (import.meta.env.DEV) {
          console.log('WebSocket disconnected')
        }
        set({ connected: false, socket: null })

        // Attempt to reconnect with exponential backoff
        if (reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
          const delay = Math.min(
            BASE_RECONNECT_DELAY * Math.pow(2, reconnectAttempts),
            30000 // Max 30 seconds
          )
          reconnectAttempts++

          set({ reconnecting: true })
          reconnectTimeout = setTimeout(() => {
            if (import.meta.env.DEV) {
              console.log(`Reconnecting... (attempt ${reconnectAttempts})`)
            }
            get().connect()
          }, delay)
        } else {
          console.error('Max reconnect attempts reached')
          set({ reconnecting: false })
        }
      }

      set({ socket: ws })
    } catch (error) {
      console.error('Failed to create WebSocket:', error)
    }
  },

  disconnect: () => {
    const { socket } = get()

    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }

    if (socket) {
      socket.close()
      set({ connected: false, socket: null, reconnecting: false })
    }

    reconnectAttempts = 0
  },

  send: (event: WSEvent) => {
    const { socket, connected } = get()

    if (socket && connected) {
      socket.send(JSON.stringify(event))
    } else {
      console.warn('Cannot send event: WebSocket not connected')
    }
  },
}))

function handleEvent(event: WSEvent) {
  if (import.meta.env.DEV) {
    console.log('WebSocket event received:', event.type, event.data)
  }

  switch (event.type) {
    case 'TASK_UPDATED': {
      // Incrementally update the specific task instead of full refetch
      const taskData = event.data as TaskUpdatedData
      if (taskData.task_id) {
        useTaskStore.getState().refreshTask(taskData.task_id)
      } else {
        useTaskStore.getState().fetchTasks()
      }
      useWSStore.setState((s) => ({ taskUpdateCounter: s.taskUpdateCounter + 1 }))
      break
    }

    case 'GOAL_CHANGED':
      // Refresh goals and current goal
      useGoalStore.getState().fetchGoals()
      useGoalStore.getState().fetchCurrentGoal()
      break

    case 'PR_STATUS': {
      // Incrementally update the specific task's PR status
      const prData = event.data as PRStatusData
      if (prData.task_id) {
        useTaskStore.getState().refreshTask(prData.task_id)
      } else {
        useTaskStore.getState().fetchTasks()
      }
      break
    }

    case 'POD_STATUS':
      // Pod status updates (Phase 2A)
      if (import.meta.env.DEV) {
        console.log('Pod status update:', event.data)
      }
      break

    case 'LOG_ENTRY':
      useLogStore.getState().addLog(event.data as LogEntryData)
      break

    case 'DEPLOY_STATUS':
      // Update deploy status when remote commit is fetched
      useDeployStore.getState().fetchStatus()
      break

    default:
      console.warn('Unknown event type:', event.type)
  }
}

// Auto-connect on store creation (after login)
// The auth store should call this after successful login
export function initializeWebSocket() {
  useWSStore.getState().connect()
}

// Cleanup on logout
export function cleanupWebSocket() {
  useWSStore.getState().disconnect()
}
