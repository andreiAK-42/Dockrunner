import { useState, useCallback } from 'react'

export interface UseFormArrayOptions<T> {
  initialValue: T[]
  validate?: (item: T) => boolean
}

export interface UseFormArrayReturn<T> {
  items: T[]
  addItem: (item: T) => void
  updateItem: (index: number, patch: Partial<T>) => void
  removeItem: (index: number) => void
  setItems: (items: T[]) => void
  clear: () => void
}

/**
 * Generic hook for managing arrays of form items
 * Used for ports, volumes, env vars, etc.
 */
export function useFormArray<T extends object>(
  options: UseFormArrayOptions<T>
): UseFormArrayReturn<T> {
  const [items, setItemsState] = useState<T[]>(options.initialValue)

  const addItem = useCallback((item: T) => {
    setItemsState((prev) => [...prev, item])
  }, [])

  const updateItem = useCallback((index: number, patch: Partial<T>) => {
    setItemsState((prev) => {
      const next = [...prev]
      if (index >= 0 && index < next.length) {
        next[index] = { ...next[index], ...patch }
      }
      return next
    })
  }, [])

  const removeItem = useCallback((index: number) => {
    setItemsState((prev) => prev.filter((_, i) => i !== index))
  }, [])

  const setItems = useCallback((newItems: T[]) => {
    setItemsState(newItems)
  }, [])

  const clear = useCallback(() => {
    setItemsState([])
  }, [])

  return {
    items,
    addItem,
    updateItem,
    removeItem,
    setItems,
    clear,
  }
}