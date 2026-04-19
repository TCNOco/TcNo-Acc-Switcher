/// <reference types="svelte" />
/// <reference types="vite/client" />

interface WindowToastOpts {
  type: string
  title?: string
  message: string
  renderTo?: string
  duration?: number
}

interface Window {
  /** Global toast API (mirrors legacy host bridge); set while Toast.svelte is mounted. */
  notification?: {
    new: (opts: WindowToastOpts) => void
  }
}
