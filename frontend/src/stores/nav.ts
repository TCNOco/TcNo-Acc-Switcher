import { writable } from 'svelte/store'

export type Route =
  | { page: 'home' }
  | { page: 'platform'; platformName: string }

export const route = writable<Route>({ page: 'home' })
export const appBarTitle = writable('TcNo Account Switcher')
