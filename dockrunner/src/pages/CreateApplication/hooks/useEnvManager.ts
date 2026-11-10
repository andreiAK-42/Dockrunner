import { useState, useCallback } from 'react'
import type { EnvConfig } from '../../../types/envConfig'
import { buildEnvMap, type EnvRow } from '../../../utils/docker'

export interface UseEnvManagerOptions {
  configs: EnvConfig[]
}

export interface UseEnvManagerReturn {
  envRows: EnvRow[]
  setEnvRows: React.Dispatch<React.SetStateAction<EnvRow[]>>
  appliedKeys: string[]
  setAppliedKeys: React.Dispatch<React.SetStateAction<string[]>>
  envConfigId: string
  setEnvConfigId: React.Dispatch<React.SetStateAction<string>>
  updateEnv: (index: number, patch: Partial<EnvRow>) => void
  removeEnv: (index: number) => void
  fillFromConfig: (id: string) => void
  buildEnvMap: () => Record<string, string>
}

/**
 * Hook for managing environment variables form state
 */
export function useEnvManager(options: UseEnvManagerOptions): UseEnvManagerReturn {
  const { configs } = options

  const [envRows, setEnvRows] = useState<EnvRow[]>([{ key: '', value: '' }])
  const [appliedKeys, setAppliedKeys] = useState<string[]>([])
  const [envConfigId, setEnvConfigId] = useState<string>('')

  const updateEnv = useCallback((index: number, patch: Partial<EnvRow>) => {
    setEnvRows((prev) => prev.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  }, [])

  const removeEnv = useCallback((index: number) => {
    setEnvRows((prev) => prev.filter((_, i) => i !== index))
  }, [])

  const fillFromConfig = useCallback((id: string) => {
    setEnvConfigId(id)
    const selectedConfig = configs.find((config) => config.id === id)

    // Убираем переменные прошлого конфига, свои строки не трогаем
    const keptRows = envRows.filter((row) => {
      const key = row.key.trim()
      const isOwnRow = key !== '' && !appliedKeys.includes(key)
      return isOwnRow
    })

    if (!selectedConfig) {
      setAppliedKeys([])
      if (keptRows.length > 0) {
        setEnvRows(keptRows)
      } else {
        setEnvRows([{ key: '', value: '' }])
      }
      return
    }

    const keptKeys = new Set(keptRows.map((row) => row.key.trim()))
    const addedVars = selectedConfig.vars.filter((configVar) => !keptKeys.has(configVar.key))
    const addedRows = addedVars.map((configVar) => ({ key: configVar.key, value: configVar.value }))
    const selectedKeys = selectedConfig.vars.map((configVar) => configVar.key)

    setAppliedKeys(selectedKeys)
    setEnvRows([...keptRows, ...addedRows])
  }, [configs, envRows, appliedKeys])

  const buildEnvMapResult = useCallback((): Record<string, string> => {
    return buildEnvMap(envRows)
  }, [envRows])

  return {
    envRows,
    setEnvRows,
    appliedKeys,
    setAppliedKeys,
    envConfigId,
    setEnvConfigId,
    updateEnv,
    removeEnv,
    fillFromConfig,
    buildEnvMap: buildEnvMapResult,
  }
}