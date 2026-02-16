import { useState, useCallback } from 'react'

interface AsyncActionState<T> {
  execute: (...args: unknown[]) => Promise<T | undefined>
  isLoading: boolean
  error: string | null
}

export function useAsyncAction<T>(
  action: (...args: unknown[]) => Promise<T>
): AsyncActionState<T> {
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const execute = useCallback(
    async (...args: unknown[]): Promise<T | undefined> => {
      setIsLoading(true)
      setError(null)
      try {
        const result = await action(...args)
        return result
      } catch (err) {
        const message = err instanceof Error ? err.message : 'An error occurred'
        setError(message)
        return undefined
      } finally {
        setIsLoading(false)
      }
    },
    [action]
  )

  return { execute, isLoading, error }
}
