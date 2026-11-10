import { useCallback, useEffect } from 'react'
import type { EnvConfig, EnvVar } from '../types/envConfig'
import { ENV_CONFIG_LIMIT } from '../types/envConfig'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import {
  createEnvConfigThunk,
  deleteEnvConfigThunk,
  fetchEnvConfigsThunk,
  updateEnvConfigThunk,
} from '../store/slices/envConfigsSlice'

function keepOnlyFilledVars(vars: EnvVar[]): EnvVar[] {
  return vars.filter((envVar) => envVar.key.trim())
}

export function useEnvConfigs() {
  const dispatch = useAppDispatch()
  const { configs, status } = useAppSelector((state) => state.envConfigs)

  const reload = useCallback(() => {
    return dispatch(fetchEnvConfigsThunk()).unwrap()
  }, [dispatch])

  useEffect(() => {
    if (status === 'idle') {
      void dispatch(fetchEnvConfigsThunk())
    }
  }, [status, dispatch])

  const canCreate = configs.length < ENV_CONFIG_LIMIT
  const remaining = ENV_CONFIG_LIMIT - configs.length

  const create = useCallback(
    async (name: string, description: string, vars: EnvVar[]) => {
      const filledVars = keepOnlyFilledVars(vars)
      return dispatch(
        createEnvConfigThunk({ name: name.trim(), description: description.trim(), vars: filledVars }),
      ).unwrap()
    },
    [dispatch],
  )

  const update = useCallback(
    async (id: string, patch: Partial<Pick<EnvConfig, 'name' | 'description' | 'vars'>>) => {
      const currentConfig = configs.find((config) => config.id === id)
      if (!currentConfig) {
        return
      }

      await dispatch(
        updateEnvConfigThunk({
          id,
          name: patch.name ?? currentConfig.name,
          description: patch.description ?? currentConfig.description ?? '',
          vars: patch.vars ?? currentConfig.vars,
        }),
      ).unwrap()
    },
    [configs, dispatch],
  )

  const remove = useCallback(
    async (id: string) => {
      await dispatch(deleteEnvConfigThunk(id)).unwrap()
    },
    [dispatch],
  )

  const getById = useCallback((id: string) => configs.find((config) => config.id === id), [configs])

  return {
    configs,
    loading: status === 'idle' || status === 'loading',
    canCreate,
    remaining,
    create,
    update,
    remove,
    getById,
    reload,
  }
}
