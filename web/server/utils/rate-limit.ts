/**
 * In-memory fixed-window rate limiter for the few endpoints Nitro answers on
 * its own (everything proxied to the Go API is limited there instead).
 *
 * The counters live in this process only: on Vercel each function instance
 * keeps its own, so the effective limit is per instance, not global. That is
 * enough for what it's used for — making abuse expensive and keeping one
 * instance from hammering a third-party API — not for exact quotas.
 */
export interface FixedWindowLimiterOptions {
  /** Requests allowed per key and window. */
  limit: number
  windowMs: number
  /** Upper bound on tracked keys, so a flood of distinct keys can't grow memory without bound. */
  maxKeys?: number
}

/** Returns a function that records a hit for `key` and says whether it is still within the limit. */
export function createFixedWindowLimiter({ limit, windowMs, maxKeys = 10_000 }: FixedWindowLimiterOptions) {
  const windows = new Map<string, { start: number, count: number }>()

  return (key: string): boolean => {
    const now = Date.now()
    const current = windows.get(key)

    if (!current || now - current.start >= windowMs) {
      // Map iterates in insertion order: the first key is the oldest window.
      if (!current && windows.size >= maxKeys) {
        const oldest = windows.keys().next().value
        if (oldest !== undefined) windows.delete(oldest)
      }
      windows.set(key, { start: now, count: 1 })
      return true
    }

    current.count++
    return current.count <= limit
  }
}
