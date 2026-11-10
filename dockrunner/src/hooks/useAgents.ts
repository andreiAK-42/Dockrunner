import { useEffect } from 'react'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { fetchAgentNodes } from '../store/slices/agentsSlice'

export function useAgents() {
  const dispatch = useAppDispatch()
  const { nodes, status, error } = useAppSelector((state) => state.agents)

  useEffect(() => {
    if (status === 'idle') {
      void dispatch(fetchAgentNodes())
    }
  }, [status, dispatch])

  return { nodes, loading: status === 'idle' || status === 'loading', error }
}
