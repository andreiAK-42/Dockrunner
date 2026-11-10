import { createSlice } from '@reduxjs/toolkit'
import type { PayloadAction } from '@reduxjs/toolkit'
import type { CheckStatus } from '../../api/containers'

interface ChecksState {
  byId: Record<string, CheckStatus>
}

const initialState: ChecksState = {
  byId: {},
}

const checksSlice = createSlice({
  name: 'checks',
  initialState,
  reducers: {
    checkReceived(state, action: PayloadAction<CheckStatus>) {
      if (!action.payload?.id) return
      state.byId[action.payload.id] = action.payload
    },
    resetChecks() {
      return initialState
    },
  },
})

export const { checkReceived, resetChecks } = checksSlice.actions

export default checksSlice.reducer
