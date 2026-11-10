import { createAsyncThunk, createSlice } from '@reduxjs/toolkit'
import type { PayloadAction } from '@reduxjs/toolkit'
import { fetchMyDeploymentRequests } from '../../api/deploymentRequests'
import type { DeploymentRequest } from '../../api/deploymentRequests'
import { fetchPendingRequests } from '../../api/admin'

export interface RequestEventPayload {
  eventType: string
  request: DeploymentRequest
  currentLogin?: string
  isAdmin: boolean
}

interface DeploymentRequestsState {
  mine: DeploymentRequest[]
  mineStatus: 'idle' | 'loading' | 'succeeded' | 'failed'
  mineError: string
  pending: DeploymentRequest[]
  pendingStatus: 'idle' | 'loading' | 'succeeded' | 'failed'
  pendingError: string
}

const initialState: DeploymentRequestsState = {
  mine: [],
  mineStatus: 'idle',
  mineError: '',
  pending: [],
  pendingStatus: 'idle',
  pendingError: '',
}

export const fetchMineRequests = createAsyncThunk<DeploymentRequest[]>('deploymentRequests/fetchMine', async () => {
  return fetchMyDeploymentRequests()
})

export const fetchPendingRequestsThunk = createAsyncThunk<DeploymentRequest[]>('deploymentRequests/fetchPending', async () => {
  return fetchPendingRequests()
})

const deploymentRequestsSlice = createSlice({
  name: 'deploymentRequests',
  initialState,
  reducers: {
    requestEventReceived(state, action: PayloadAction<RequestEventPayload>) {
      const { request, currentLogin, isAdmin } = action.payload
      if (!request?.id) return

      const existsInMine = state.mine.some((item) => item.id === request.id)
      state.mine = state.mine.map((item) => (item.id === request.id ? request : item))
      if (!existsInMine && request.ownerLogin === currentLogin) {
        state.mine = [request, ...state.mine]
      }

      const existsInPending = state.pending.some((item) => item.id === request.id)
      state.pending = state.pending.map((item) => (item.id === request.id ? request : item))
      state.pending = state.pending.filter((item) => item.status === 'pending')
      if (isAdmin && !existsInPending && request.status === 'pending') {
        state.pending = [request, ...state.pending]
      }
    },
    resetDeploymentRequests() {
      return initialState
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchMineRequests.pending, (state) => {
        state.mineStatus = 'loading'
        state.mineError = ''
      })
      .addCase(fetchMineRequests.fulfilled, (state, action) => {
        state.mineStatus = 'succeeded'
        state.mine = action.payload
      })
      .addCase(fetchMineRequests.rejected, (state) => {
        state.mineStatus = 'failed'
        state.mineError = 'Не удалось загрузить заявки'
      })
      .addCase(fetchPendingRequestsThunk.pending, (state) => {
        state.pendingStatus = 'loading'
        state.pendingError = ''
      })
      .addCase(fetchPendingRequestsThunk.fulfilled, (state, action) => {
        state.pendingStatus = 'succeeded'
        state.pending = action.payload
      })
      .addCase(fetchPendingRequestsThunk.rejected, (state) => {
        state.pendingStatus = 'failed'
        state.pendingError = 'Не удалось загрузить заявки'
      })
  },
})

export const { requestEventReceived, resetDeploymentRequests } = deploymentRequestsSlice.actions

export default deploymentRequestsSlice.reducer
