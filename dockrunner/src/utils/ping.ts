const DEFAULT_TIMEOUT_MS = 3000

export async function measurePing(baseUrl: string, timeoutMs = DEFAULT_TIMEOUT_MS): Promise<number | null> {
  if (!baseUrl) {
    return null
  }

  const start = performance.now()
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeoutMs)

  const baseWithoutSlash = baseUrl.replace(/\/$/, '')
  const healthUrl = `${baseWithoutSlash}/health`

  try {
    await fetch(healthUrl, { signal: controller.signal, cache: 'no-store' })
    clearTimeout(timer)
    return Math.round(performance.now() - start)
  } catch {
    clearTimeout(timer)
    return null
  }
}
