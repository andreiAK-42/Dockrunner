import { configureStore } from '@reduxjs/toolkit'
import auth from './slices/authSlice'
import containers from './slices/containersSlice'
import agents from './slices/agentsSlice'
import deploymentRequests from './slices/deploymentRequestsSlice'
import envConfigs from './slices/envConfigsSlice'
import quotas from './slices/quotasSlice'
import checks from './slices/checksSlice'
import realtime from './slices/realtimeSlice'

export const store = configureStore({
  reducer: {
    auth,
    containers,
    agents,
    deploymentRequests,
    envConfigs,
    quotas,
    checks,
    realtime,
  },
})

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch
