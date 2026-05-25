import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
// v0.8.10.2: SupKube design tokens (colour + typography). Imported BEFORE
// any component CSS so var(--sk-*) is available globally. See UI_GUIDELINES.md.
import './styles/tokens.css'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'

// v0.7.3 dark mode boot — read persisted preference before app mount so we
// don't get a flash of light theme. Toggle managed in App.vue.
const theme = localStorage.getItem('supkube.theme')
if (theme === 'dark') {
  document.documentElement.classList.add('dark')
}
// v0.7.4: set <html lang> to match i18n locale for a11y + browser hints.
document.documentElement.lang = i18n.global.locale.value

const app = createApp(App)
app.use(ElementPlus)
app.use(router)
app.use(i18n)
app.mount('#app')
