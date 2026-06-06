import { get, writable } from 'svelte/store'

const defaultToastDurationMs = 5000

export type ToastInput = {
  type: string
  title?: string
  message: string
  duration?: number
}

type ToastItem = {
  id: string
  type: string
  title: string
  message: string
  count: number
}

const { subscribe, update } = writable<ToastItem[]>([])

const timers = new Map<string, ReturnType<typeof setTimeout>>()

function contentKey(t: { type: string; title: string; message: string }): string {
  return `${t.type}\u0000${t.title}\u0000${t.message}`
}

function resolvedDurationMs(input: ToastInput): number {
  const d = input.duration
  if (d !== undefined && d > 0) {
    return d
  }
  return defaultToastDurationMs
}

function scheduleRemoval(id: string, ms: number): void {
  const prev = timers.get(id)
  if (prev !== undefined) {
    clearTimeout(prev)
  }
  timers.set(
    id,
    setTimeout(() => {
      timers.delete(id)
      popOneOccurrence(id, ms)
    }, ms)
  )
}

/** Drops one merged occurrence (count) or removes the toast row when count is 1. */
function popOneOccurrence(id: string, nextDurationMs: number): void {
  update((list) => {
    const idx = list.findIndex((t) => t.id === id)
    if (idx < 0) return list
    const cur = list[idx]
    if (cur.count > 1) {
      const next = list.slice()
      next[idx] = { ...cur, count: cur.count - 1 }
      scheduleRemoval(id, nextDurationMs)
      return next
    }
    return list.filter((t) => t.id !== id)
  })
}

/** Show or merge a toast (same type + title + message refreshes the timer and increments count). */
export function pushToast(input: ToastInput): void {
  const durationMs = resolvedDurationMs(input)
  const title = (input.title ?? '').trim()
  const message = (input.message ?? '').trim()
  const key = contentKey({ type: input.type, title, message })

  update((list) => {
    const idx = list.findIndex((t) => contentKey(t) === key)
    if (idx >= 0) {
      const cur = list[idx]
      scheduleRemoval(cur.id, durationMs)
      const next = list.slice()
      next[idx] = {
        ...cur,
        count: cur.count + 1,
      }
      return next
    }
    const id = crypto.randomUUID()
    scheduleRemoval(id, durationMs)
    return [...list, { id, type: input.type, title, message, count: 1 }]
  })
}

/**
 * Dismiss one occurrence: merged toasts (count > 1) decrement; otherwise the row is removed.
 * Uses the default duration for the auto-dismiss timer if further occurrences remain.
 */
export function dismissToastById(id: string): void {
  const prev = timers.get(id)
  if (prev !== undefined) {
    clearTimeout(prev)
    timers.delete(id)
  }
  popOneOccurrence(id, defaultToastDurationMs)
}

export const toastStore = { subscribe }

/** Read current list (e.g. tests). */
function getToasts(): ToastItem[] {
  return get({ subscribe })
}
