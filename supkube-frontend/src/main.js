import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import App from './App.vue'
import router from './router'

// v0.7.3 dark mode boot — read persisted preference before app mount so we
// don't get a flash of light theme. Toggle managed in App.vue.
const theme = localStorage.getItem('supkube.theme')
if (theme === 'dark') {
  document.documentElement.classList.add('dark')
}

const app = createApp(App)
app.use(ElementPlus)
app.use(router)
app.mount('#app')
