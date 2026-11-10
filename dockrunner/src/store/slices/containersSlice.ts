import { createAsyncThunk, createSlice } from '@reduxjs/toolkit'
import type { PayloadAction } from '@reduxjs/toolkit'
import { fetchMine, type MineContainer } from '../../api/containers'
import { fetchAdminContainers, type AdminContainer } from '../../api/admin'

interface ContainerPort {
  host_port: number
  container_port: number
}

export interface ContainerEventPayload {
  id?: string
  app_id?: string
  app_name?: string
  owner_login?: string
  image?: string
  status?: string
  node_id?: string
  ports?: ContainerPort[]
  removed?: boolean
  error?: string
}

interface ContainersState {
  mine: MineContainer[]
  mineStatus: 'idle' | 'loading' | 'succeeded' | 'failed'
  mineError: string
  admin: AdminContainer[]
  adminStatus: 'idle' | 'loading' | 'succeeded' | 'failed'
  adminError: string
}

const initialState: ContainersState = {
  mine: [],
  mineStatus: 'idle',
  mineError: '',
  admin: [],
  adminStatus: 'idle',
  adminError: '',
}

export const fetchMineContainers = createAsyncThunk<MineContainer[]>('containers/fetchMine', async () => {
  const res = await fetchMine()
  return res.data
})

export const fetchAdminContainersThunk = createAsyncThunk<AdminContainer[]>('containers/fetchAdmin', async () => {
  return fetchAdminContainers()
})

const containersSlice = createSlice({
  name: 'containers',
  initialState,
  reducers: {
    containerEventReceived(state, action: PayloadAction<ContainerEventPayload>) {
      const data = action.payload
      if (!data.id) return

      if (data.removed) {
        state.mine = state.mine.filter((container) => container.id !== data.id)
        state.admin = state.admin.filter((container) => container.id !== data.id)
        return
      }

      state.mine = state.mine.map((container) =>
        container.id === data.id
          ? {
              ...container,
              status: data.status || container.status,
              image: data.image || container.image,
              app_name: data.app_name || container.app_name,
              node_id: data.node_id || container.node_id,
              ports: data.ports && data.ports.length > 0 ? data.ports : container.ports,
            }
          : container,
      )

      state.admin = state.admin.map((container) =>
        container.id === data.id
          ? {
              ...container,
              status: data.status || container.status,
              image: data.image || container.image,
              app_name: data.app_name || container.app_name,
              owner_login: data.owner_login || container.owner_login,
              node_id: data.node_id || container.node_id,
              ports: data.ports && data.ports.length > 0 ? data.ports : container.ports,
            }
          : container,
      )
    },
    containerStatusPatched(state, action: PayloadAction<{ id: string; status: string }>) {
      const { id, status } = action.payload
      state.mine = state.mine.map((container) => (container.id === id ? { ...container, status } : container))
      state.admin = state.admin.map((container) => (container.id === id ? { ...container, status } : container))
    },
    resetContainers() {
      return initialState
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchMineContainers.pending, (state) => {
        state.mineStatus = 'loading'
        state.mineError = ''
      })
      .addCase(fetchMineContainers.fulfilled, (state, action) => {
        state.mineStatus = 'succeeded'
        state.mine = action.payload
      })
      .addCase(fetchMineContainers.rejected, (state) => {
        state.mineStatus = 'failed'
        state.mineError = 'Не удалось загрузить контейнеры'
      })
      .addCase(fetchAdminContainersThunk.pending, (state) => {
        state.adminStatus = 'loading'
        state.adminError = ''
      })
      .addCase(fetchAdminContainersThunk.fulfilled, (state, action) => {
        state.adminStatus = 'succeeded'
        state.admin = action.payload
      })
      .addCase(fetchAdminContainersThunk.rejected, (state) => {
        state.adminStatus = 'failed'
        state.adminError = 'Не удалось загрузить контейнеры'
      })
  },
})

export const { containerEventReceived, containerStatusPatched, resetContainers } = containersSlice.actions

export default containersSlice.reducer
