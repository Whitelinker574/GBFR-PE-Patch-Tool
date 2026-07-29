import { shallowReactive } from 'vue'

const publishedShares = shallowReactive(new Map())

function normalizedPart(value) {
  return String(value ?? '').trim().toUpperCase()
}

export function loadoutShareSessionKey({ source = 'save', savePath = '', charaHash = '', unitId = 0, compatibilityCode = '', recordId = '', role = '' } = {}) {
  if (compatibilityCode) return `code:${normalizedPart(compatibilityCode)}`
  if (source === 'runtime') return `runtime:${normalizedPart(recordId)}:${normalizedPart(role)}`
  return `save:${normalizedPart(savePath)}:${normalizedPart(charaHash)}:${Number(unitId) || 0}`
}

export function publishedLoadoutShare(key) {
  return publishedShares.get(String(key || '')) || null
}

export function rememberPublishedLoadoutShare(key, published) {
  const normalizedKey = String(key || '')
  if (!normalizedKey || !published?.url) return null
  const snapshot = { ...published }
  publishedShares.set(normalizedKey, snapshot)
  return snapshot
}

export function forgetPublishedLoadoutShare(key) {
  publishedShares.delete(String(key || ''))
}

function legacyCopy(value) {
  const field = document.createElement('textarea')
  field.value = value
  field.setAttribute('readonly', '')
  field.style.position = 'fixed'
  field.style.opacity = '0'
  document.body.appendChild(field)
  field.focus()
  field.select()
  const copied = document.execCommand?.('copy') === true
  field.remove()
  if (!copied) throw new Error('clipboard.copy_failed')
}

export async function copyShareText(value) {
  const text = String(value || '')
  if (!text) return false
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      legacyCopy(text)
      return true
    }
  }
  legacyCopy(text)
  return true
}
