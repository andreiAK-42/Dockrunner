import { createAsyncThunk, createSlice } from '@reduxjs/toolkit'
import { fetchQuotas } from '../../api/quotas'
import type { Quotas } from '../../api/quotas'

interface QuotasState {
  data: Quotas | null
  status: 'idle' | 'loading' | 'succeeded' | 'failed'
}

const initialState: QuotasState = {
  data: null,
  status: 'idle',
}

export const fetchQuotasThunk = createAsyncThunk<Quotas>('quotas/fetch', async () => {
  const res = await fetchQuotas()
  return res.data
})

const quotasSlice = createSlice({
  name: 'quotas',
  initialState,
  reducers: {
    resetQuotas() {
      return initialState
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchQuotasThunk.pending, (state) => {
        state.status = 'loading'
      })
      .addCase(fetchQuotasThunk.fulfilled, (state, action) => {
        state.status = 'succeeded'
        state.data = action.payload
      })
      .addCase(fetchQuotasThunk.rejected, (state) => {
        state.status = 'failed'
      })
  },
})

export const { resetQuotas } = quotasSlice.actions

export default quotasSlice.reducer
