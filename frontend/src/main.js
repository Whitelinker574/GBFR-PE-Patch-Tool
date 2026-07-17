import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import './backendLanguage'
import { installI18nObserver } from './i18n'
import { installUiScale } from './utils/uiScale'

function bootstrap() {
  installUiScale()
  createApp(App).mount('#app')
  installI18nObserver()
}

bootstrap()
