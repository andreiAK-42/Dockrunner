import { useEffect, useState, useCallback } from 'react'
import type { EnvConfig, EnvVar } from '../types/envConfig'
import { ENV_CONFIG_LIMIT } from '../types/envConfig'
import { fetchEnvConfigs, createEnvConfig, updateEnvConfig, deleteEnvConfig } from '../api/envConfigs'
import type { ApiEnvConfig } from '../api/envConfigs'

function toConfig(apiConfig: ApiEnvConfig): EnvConfig {
  return {
    id: apiConfig.id,
    name: apiConfig.name,
    description: apiConfig.description || '',
    vars: apiConfig.vars || [],
    createdAt: apiConfig.created_at,
    updatedAt: apiConfig.updated_at,
  }
}

function keepOnlyFilledVars(vars: EnvVar[]): EnvVar[] {
  return vars.filter((envVar) => envVar.key.trim())
}

export function useEnvConfigs() {
  const [configs, setConfigs] = useState<EnvConfig[]>([])
  const [loading, setLoading] = useState(true)

  const reload = useCallback(() => {
    return fetchEnvConfigs()
      .then((res) => {
        const parsedConfigs = res.data.map(toConfig)
        setConfigs(parsedConfigs)
        setLoading(false)
      })
      .catch(() => setLoading(false))
  }, [])

  useEffect(() => {
    reload()
  }, [reload])

  const canCreate = configs.length < ENV_CONFIG_LIMIT
  const remaining = ENV_CONFIG_LIMIT - configs.length

  const create = useCallback(async (name: string, description: string, vars: EnvVar[]) => {
    const filledVars = keepOnlyFilledVars(vars)
    const trimmedName = name.trim()
    const trimmedDescription = description.trim()
    const res = await createEnvConfig({ name: trimmedName, description: trimmedDescription, vars: filledVars })
    await reload()
    return toConfig(res.data)
  }, [reload])

  const update = useCallback(async (id: string, patch: Partial<Pick<EnvConfig, 'name' | 'description' | 'vars'>>) => {
    const currentConfig = configs.find((config) => config.id === id)
    if (!currentConfig) {
      return
    }

    const nextName = patch.name ?? currentConfig.name
    const nextDescription = patch.description ?? currentConfig.description
    const nextVars = patch.vars ?? currentConfig.vars

    await updateEnvConfig(id, {
      name: nextName,
      description: nextDescription,
      vars: nextVars,
    })
    await reload()
  }, [configs, reload])

  const remove = useCallback(async (id: string) => {
    await deleteEnvConfig(id)
    await reload()
  }, [reload])

  const getById = useCallback((id: string) => configs.find((config) => config.id === id), [configs])

  return { configs, loading, canCreate, remaining, create, update, remove, getById, reload }
}
