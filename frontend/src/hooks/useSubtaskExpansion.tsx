import { useState, useCallback } from 'react'
import { Task, api } from '../lib/api'

interface SubtaskExpansionState {
  subtaskCounts: Record<string, number>
  expandedTasks: Set<string>
  loadedSubtasks: Record<string, Task[]>
  loadingSubtasks: Set<string>
  fetchSubtaskCounts: (tasks: Task[]) => Promise<void>
  toggleSubtasks: (taskId: string, e: React.MouseEvent) => Promise<void>
}

export function useSubtaskExpansion(): SubtaskExpansionState {
  const [subtaskCounts, setSubtaskCounts] = useState<Record<string, number>>({})
  const [expandedTasks, setExpandedTasks] = useState<Set<string>>(new Set())
  const [loadedSubtasks, setLoadedSubtasks] = useState<Record<string, Task[]>>({})
  const [loadingSubtasks, setLoadingSubtasks] = useState<Set<string>>(new Set())

  const fetchSubtaskCounts = useCallback(async (tasks: Task[]) => {
    const parents = tasks.filter((t) => t.status === 'DECOMPOSED' || t.tags?.includes('build-failure'))
    if (parents.length === 0) return

    // Fetch all subtask counts in parallel
    const results = await Promise.all(
      parents.map(async (parent) => {
        try {
          const subs = await api.listSubtasks(parent.id)
          return { id: parent.id, count: subs.length }
        } catch {
          return { id: parent.id, count: 0 }
        }
      })
    )

    const counts: Record<string, number> = {}
    for (const { id, count } of results) {
      if (count > 0) counts[id] = count
    }
    setSubtaskCounts(counts)
  }, [])

  const toggleSubtasks = useCallback(async (taskId: string, e: React.MouseEvent) => {
    e.stopPropagation()

    if (expandedTasks.has(taskId)) {
      setExpandedTasks((prev) => {
        const next = new Set(prev)
        next.delete(taskId)
        return next
      })
    } else {
      setExpandedTasks((prev) => {
        const next = new Set(prev)
        next.add(taskId)
        return next
      })

      if (!loadedSubtasks[taskId]) {
        setLoadingSubtasks((prev) => {
          const next = new Set(prev)
          next.add(taskId)
          return next
        })

        try {
          const subtasks = await api.listSubtasks(taskId)
          setLoadedSubtasks((prev) => ({ ...prev, [taskId]: subtasks }))
        } catch (error) {
          console.error('Failed to load subtasks:', error)
        } finally {
          setLoadingSubtasks((prev) => {
            const next = new Set(prev)
            next.delete(taskId)
            return next
          })
        }
      }
    }
  }, [expandedTasks, loadedSubtasks])

  return {
    subtaskCounts,
    expandedTasks,
    loadedSubtasks,
    loadingSubtasks,
    fetchSubtaskCounts,
    toggleSubtasks,
  }
}
