// Check/polling utilities for repository verification

import type { CheckStatus } from '../api/containers'

export const CHECK_POLL_INTERVAL_MS = 2000
export const CHECK_TIMEOUT_MS = 90000
export const CANCELLED_MESSAGE = 'cancelled'

/**
 * Simple delay utility
 */
export function delay(ms: number): Promise<void> {
  return new Promise<void>((resolve) => setTimeout(resolve, ms))
}

/**
 * Check if check status is finished (ready or failed)
 */
export function isCheckFinished(status: string): boolean {
  return status === 'ready' || status === 'failed'
}

/**
 * Poll for check completion
 */
export async function waitForCheck(
  id: string,
  isCancelled: () => boolean,
  fetchCheckFn: (id: string) => Promise<{ data: CheckStatus }>
): Promise<CheckStatus> {
  const started = Date.now()

  for (;;) {
    if (isCancelled()) {
      throw new Error(CANCELLED_MESSAGE)
    }

    await delay(CHECK_POLL_INTERVAL_MS)

    if (isCancelled()) {
      throw new Error(CANCELLED_MESSAGE)
    }

    const res = await fetchCheckFn(id)
    if (isCheckFinished(res.data.status)) {
      return res.data
    }

    const waitedTooLong = Date.now() - started > CHECK_TIMEOUT_MS
    if (waitedTooLong) {
      throw new Error('Превышено время ожидания проверки')
    }
  }
}