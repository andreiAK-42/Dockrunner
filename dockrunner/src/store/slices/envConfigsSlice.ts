import { createAsyncThunk, createSlice } from '@reduxjs/toolkit'
import { createEnvConfig, deleteEnvConfig, fetchEnvConfigs, updateEnvConfig } from '../../api/envConfigs'
import type { ApiEnvConfig, ApiEnvVar } from '../../api/envConfigs'
import type { EnvConfig } from '../../types/envConfig'

export function toConfig(apiConfig: ApiEnvConfig): EnvConfig {
  return {
    id: apiConfig.id,
    name: apiConfig.name,
    description: apiConfig.description || '',
    vars: apiConfig.vars || [],
    createdAt: apiConfig.created_at,
    updatedAt: apiConfig.updated_at,
  }
}

interface EnvConfigsState {
  configs: EnvConfig[]
  status: 'idle' | 'loading' | 'succeeded' | 'failed'
}

const initialState: EnvConfigsState = {
  configs: [],
  status: 'idle',
}

export const fetchEnvConfigsThunk = createAsyncThunk<EnvConfig[]>('envConfigs/fetch', async () => {
  const res = await fetchEnvConfigs()
  return res.data.map(toConfig)
})

export const createEnvConfigThunk = createAsyncThunk<
  EnvConfig,
  { name: string; description: string; vars: ApiEnvVar[] }
>('envConfigs/create', async (payload) => {
  const res = await createEnvConfig(payload)
  return toConfig(res.data)
})

export const updateEnvConfigThunk = createAsyncThunk<
  EnvConfig,
  { id: string; name: string; description: string; vars: ApiEnvVar[] }
>('envConfigs/update', async ({ id, ...payload }) => {
  const res = await updateEnvConfig(id, payload)
  return toConfig(res.data)
})

export const deleteEnvConfigThunk = createAsyncThunk<string, string>('envConfigs/delete', async (id) => {
  await deleteEnvConfig(id)
  return id
})

const envConfigsSlice = createSlice({
  name: 'envConfigs',
  initialState,
  reducers: {
    resetEnvConfigs() {
      return initialState
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchEnvConfigsThunk.pending, (state) => {
        state.status = 'loading'
      })
      .addCase(fetchEnvConfigsThunk.fulfilled, (state, action) => {
        state.status = 'succeeded'
        state.configs = action.payload
      })
      .addCase(fetchEnvConfigsThunk.rejected, (state) => {
        state.status = 'failed'
      })
      .addCase(createEnvConfigThunk.fulfilled, (state, action) => {
        state.configs = [...state.configs, action.payload]
      })
      .addCase(updateEnvConfigThunk.fulfilled, (state, action) => {
        state.configs = state.configs.map((config) => (config.id === action.payload.id ? action.payload : config))
      })
      .addCase(deleteEnvConfigThunk.fulfilled, (state, action) => {
        state.configs = state.configs.filter((config) => config.id !== action.payload)
      })
  },
})

export const { resetEnvConfigs } = envConfigsSlice.actions

export default envConfigsSlice.reducer
