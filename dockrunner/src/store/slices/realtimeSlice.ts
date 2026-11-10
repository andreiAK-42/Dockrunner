import { createSlice } from '@reduxjs/toolkit'
import type { PayloadAction } from '@reduxjs/toolkit'

interface RealtimeState {
  connected: boolean
}

const initialState: RealtimeState = {
  connected: false,
}

const realtimeSlice = createSlice({
  name: 'realtime',
  initialState,
  reducers: {
    setRealtimeConnected(state, action: PayloadAction<boolean>) {
      state.connected = action.payload
    },
  },
})

export const { setRealtimeConnected } = realtimeSlice.actions

export default realtimeSlice.reducer
