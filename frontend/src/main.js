import { createApp } from 'vue'
import './style.css'
import './ui.css'
import App from './App.vue'
import './backendLanguage'
import { installI18nObserver } from './i18n'
import { installPerformanceMonitor } from './performanceMonitor.js'
import { installUiScale } from './utils/uiScale'

function bootstrap() {
  installPerformanceMonitor()
  installUiScale()
  createApp(App).mount('#app')
  installI18nObserver()
}

bootstrap()
