import { decode as decodeBase32768, encode as encodeBase32768 } from 'base32768'

export const LOADOUT_SHARE_ASCII_PREFIX = 'GBFRC1A.'
export const LOADOUT_SHARE_UNICODE_PREFIX = 'GBFRC1U.'

export function normalizeLoadoutShareText(value) {
  return String(value || '').replace(/[\s\u00AD\u200B-\u200D\u2060\uFEFF]+/gu, '')
}

function bytesToBase64Url(bytes) {
  let binary = ''
  const chunkSize = 0x8000
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + chunkSize))
  }
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

function base64UrlToBytes(value) {
  if (!/^[A-Za-z0-9_-]+$/u.test(value)) throw new Error('兼容码包含无效字符')
  const padded = value.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - value.length % 4) % 4)
  const binary = atob(padded)
  return Uint8Array.from(binary, character => character.charCodeAt(0))
}

export function compactLoadoutShareCode(compatibilityCode) {
  const normalized = normalizeLoadoutShareText(compatibilityCode)
  if (!normalized.startsWith(LOADOUT_SHARE_ASCII_PREFIX)) {
    throw new Error('后端返回的配装兼容码前缀无效')
  }
  const frame = base64UrlToBytes(normalized.slice(LOADOUT_SHARE_ASCII_PREFIX.length))
  return LOADOUT_SHARE_UNICODE_PREFIX + encodeBase32768(frame)
}

export function compatibleLoadoutShareCode(value) {
  const normalized = normalizeLoadoutShareText(value)
  if (normalized.startsWith(LOADOUT_SHARE_ASCII_PREFIX)) return normalized
  if (!normalized.startsWith(LOADOUT_SHARE_UNICODE_PREFIX)) {
    throw new Error('无法识别分享码；应以 GBFRC1U. 或 GBFRC1A. 开头')
  }
  const body = normalized.slice(LOADOUT_SHARE_UNICODE_PREFIX.length)
  if (!body) throw new Error('紧凑码内容为空')
  let frame
  try {
    frame = decodeBase32768(body)
  } catch (error) {
    throw new Error(`紧凑码字符损坏：${String(error?.message || error)}`)
  }
  return LOADOUT_SHARE_ASCII_PREFIX + bytesToBase64Url(frame)
}

export function isOfflineLoadoutShareCode(value) {
  const normalized = normalizeLoadoutShareText(value)
  return normalized.startsWith(LOADOUT_SHARE_ASCII_PREFIX)
    || normalized.startsWith(LOADOUT_SHARE_UNICODE_PREFIX)
}

export function loadoutShareCodeCharacters(value) {
  return Array.from(normalizeLoadoutShareText(value)).length
}
