import { useState, useCallback, useRef, useEffect } from 'react'
import { checkRepository, fetchCheck } from '../../../api/containers'
import type { CheckStatus } from '../../../api/containers'
import { ApiError } from '../../../api/client'
import { useAppDispatch, useAppSelector } from '../../../store/hooks'
import { checkReceived } from '../../../store/slices/checksSlice'
import { waitForCheck, CANCELLED_MESSAGE } from '../../../utils/check'

export interface UseRepoCheckOptions {
  appName: string
  repoUrl: string
  branch: string
  effectiveNodeId: string
}

export interface UseRepoCheckReturn {
  check: CheckStatus | null
  checking: boolean
  elapsed: number
  formError: string
  setFormError: React.Dispatch<React.SetStateAction<string>>
  handleCheck: () => Promise<void>
  handleCancelCheck: () => void
  isReady: boolean
}

/**
 * Hook for handling repository check logic.
 *
 * Check snapshots live in the `checks` slice: realtime events and the polling
 * fallback both write there, and this hook only reads the current check.
 */
export function useRepoCheck(options: UseRepoCheckOptions): UseRepoCheckReturn {
  const { appName, repoUrl, branch, effectiveNodeId } = options
  const dispatch = useAppDispatch()

  const [checkId, setCheckId] = useState<string | null>(null)
  const check = useAppSelector((state) => (checkId ? state.checks.byId[checkId] ?? null : null))
  const [checking, setChecking] = useState(false)
  const [elapsed, setElapsed] = useState(0)
  const [formError, setFormError] = useState('')
  const cancelRef = useRef(false)
  const pendingRef = useRef<{ id: string; resolve: (check: CheckStatus) => void } | null>(null)

  const isReady = check?.status === 'ready'

  // The realtime bridge stores push updates; resolve the waiting promise here.
  useEffect(() => {
    if (check && pendingRef.current && check.id === pendingRef.current.id) {
      pendingRef.current.resolve(check)
    }
  }, [check])

  const handleCheck = useCallback(async () => {
    const appNameFilled = appName.trim()
    const repoUrlFilled = repoUrl.trim()
    const branchFilled = branch.trim()

    if (!appNameFilled || !repoUrlFilled) {
      setFormError('Укажите название приложения и ссылку на репозиторий')
      return
    }
    if (!branchFilled) {
      setFormError('Укажите ветку Git для деплоя')
      return
    }
    if (!effectiveNodeId) {
      setFormError('Выберите узел для развёртывания')
      return
    }

    setFormError('')
    setChecking(true)
    setElapsed(0)
    cancelRef.current = false

    const timer = setInterval(() => setElapsed((seconds) => seconds + 1), 1000)

    try {
      const checkRes = await checkRepository({ app_name: appNameFilled, repo_url: repoUrlFilled, node_id: effectiveNodeId, branch: branchFilled })
      dispatch(checkReceived(checkRes.data))
      setCheckId(checkRes.data.id)

      let settled = false
      const pushed = new Promise<CheckStatus>((resolve) => {
        pendingRef.current = { id: checkRes.data.id, resolve }
      })
      const polling = waitForCheck(checkRes.data.id, () => cancelRef.current || settled, fetchCheck)
      // Проигравшая гонку ветка не должна давать unhandled rejection.
      polling.then((result) => dispatch(checkReceived(result))).catch(() => {})
      const finishedCheck = await Promise.race([polling, pushed])
      settled = true
      dispatch(checkReceived(finishedCheck))
      if (finishedCheck.status === 'ready') {
        setFormError('')
        // Toast will be handled by parent
      } else {
        setFormError(finishedCheck.error || 'Проверка не пройдена')
      }
    } catch (checkError) {
      const isCancelled = checkError instanceof Error && checkError.message === CANCELLED_MESSAGE
      if (isCancelled) {
        return
      }

      const isLostCheck = checkError instanceof ApiError && checkError.status === 404
      if (isLostCheck) {
        setFormError('Проверка потеряна — сервер перезапускался? Запустите снова')
        return
      }

      if (checkError instanceof Error) {
        setFormError(checkError.message)
      } else {
        setFormError('Проверка не удалась')
      }
    } finally {
      pendingRef.current = null
      clearInterval(timer)
      setChecking(false)
    }
  }, [appName, repoUrl, branch, effectiveNodeId, dispatch])

  const handleCancelCheck = useCallback(() => {
    cancelRef.current = true
  }, [])

  return {
    check,
    checking,
    elapsed,
    formError,
    setFormError,
    handleCheck,
    handleCancelCheck,
    isReady,
  }
}
