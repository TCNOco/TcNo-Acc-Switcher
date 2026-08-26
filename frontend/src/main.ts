import { mount } from 'svelte'
import App from './App.svelte'
import './styles/context_menu.scss'
import './styles/normalize.scss'
import './styles/style.scss'
import './styles/theme.scss'
import './styles/overlayReceivers.scss'
import './styles/UI.scss'
import './styles/modal-primary.scss'
import './styles/acclist.scss'
import './styles/rtl.scss'
import { initI18n } from './stores/i18n'
import { initOfflineMode } from './stores/offlineMode'
import { resolveInitialRoute, installHashSync } from './stores/nav'
import { initTheme } from './lib/themes'
import { installNavigationGuard } from './lib/navigationGuard'

/**
 * Nothing before mount() may decide whether the app paints: the window is
 * frameless, so an app that never mounts is a bare rectangle with no title bar
 * and no way to report why. Each step degrades to its defaults instead.
 */
const STEP_TIMEOUT_MS = 8000

function guard(name: string, run: () => void): void {
  try {
    run()
  } catch (err) {
    console.error(`[boot] ${name} failed`, err)
  }
}

async function step(name: string, run: () => Promise<unknown>): Promise<void> {
  let timer: ReturnType<typeof setTimeout> | undefined
  try {
    await Promise.race([
      run(),
      new Promise((_, reject) => {
        timer = setTimeout(
          () => reject(new Error(`${name} timed out after ${STEP_TIMEOUT_MS}ms`)),
          STEP_TIMEOUT_MS,
        )
      }),
    ])
  } catch (err) {
    console.error(`[boot] ${name} failed`, err)
  } finally {
    clearTimeout(timer)
  }
}

guard('navigation guard', installNavigationGuard)

void (async () => {
  await step('i18n', initI18n)
  await step('offline mode', initOfflineMode)
  await step('theme', initTheme)
  await step('initial route', resolveInitialRoute)
  guard('hash sync', installHashSync)

  try {
    const target = document.getElementById('app')
    if (!target) {
      throw new Error('#app is missing from the document')
    }
    mount(App, { target })
    window.__tcnoBoot?.ready()
  } catch (err) {
    window.__tcnoBoot?.fail('mount', err)
    throw err
  }
})()
