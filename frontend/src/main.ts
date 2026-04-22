import App from './App.svelte'
import './styles/context_menu.scss'
import './styles/normalize.scss'
import './styles/style.scss'
import './styles/theme.scss'
import './styles/UI.scss'
import './styles/acclist.scss'
import { initI18n } from './stores/i18n'

const app = void (async () => {
  await initI18n()
  new App({ target: document.getElementById('app')! })
})()

export default app
