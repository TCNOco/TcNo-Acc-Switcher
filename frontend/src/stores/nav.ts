import { writable } from 'svelte/store'

export type Route =
  | { page: 'home' }
  | { page: 'settings' }
  | { page: 'manage-platforms' }
  | { page: 'platform'; platformName: string }
  | { page: 'platform-settings'; platformName: string }

export const route = writable<Route>({ page: 'home' })
export const previousPage = writable<Route | null>(null)
export const appBarTitle = writable('TcNo Account Switcher')
