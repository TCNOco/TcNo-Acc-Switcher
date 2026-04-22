import App from './App.svelte'
import './styles/context_menu.scss'
import './styles/normalize.scss'
import './styles/style.scss'
import './styles/theme.scss'
import './styles/UI.scss'
import './styles/acclist.scss'
import './styles/rtl.scss'
import { initI18n } from './stores/i18n'
import { resolveInitialRoute, installHashSync } from './stores/nav'

const app = void (async () => {
  await initI18n()
  await resolveInitialRoute()
  installHashSync()
  new App({ target: document.getElementById('app')! })
})()

export default app
