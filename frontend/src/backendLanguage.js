import { SetLanguage } from '../wailsjs/go/main/App'
import { getStoredLanguage } from './i18n'

const selectedLanguage = getStoredLanguage()

// Start the bridge call before Vue mounts so localized data requests cannot
// overtake it. The shell still paints immediately; only name-bearing catalog
// requests wait for the backend to acknowledge the selected language.
export const backendLanguageReady = SetLanguage(selectedLanguage).catch((error) => {
  console.warn('Unable to synchronise backend language:', error)
  return selectedLanguage
})
