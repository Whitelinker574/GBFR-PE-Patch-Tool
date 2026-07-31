import { isOfflineLoadoutShareCode, normalizeLoadoutShareText } from './loadoutShareCode.js'

export const LOADOUT_SHARE_QR_MAX_FILE_BYTES = 32 * 1024 * 1024
const MAX_CANVAS_EDGE = 2560
const MAX_CANVAS_PIXELS = 8_000_000
const SHORT_CODE_PATTERN = /^[0-9A-HJKMNP-TV-Z]{12}$|^[0-9A-HJKMNP-TV-Z]{16,24}$/u
let decoderPromise

function qrError(code) {
  const error = new Error(code)
  error.code = code
  return error
}

export function normalizeLoadoutShareQRPayload(value) {
  const raw = String(value || '').trim()
  if (!raw) throw qrError('qr_empty')
  if (isOfflineLoadoutShareCode(raw)) return normalizeLoadoutShareText(raw)

  try {
    const parsed = new URL(raw)
    if (!['http:', 'https:'].includes(parsed.protocol)) throw qrError('qr_not_loadout')
    const path = decodeURIComponent(parsed.pathname).replace(/^\/+|\/+$/gu, '')
    if (/^(?:s|api\/v1\/loadouts)\/[0-9A-HJKMNP-TV-Z-]{12,31}$/iu.test(path)
      || /^download\/[0-9A-HJKMNP-TV-Z-]{12,31}\.gbfr-loadout$/iu.test(path)) {
      return raw
    }
    throw qrError('qr_not_loadout')
  } catch (error) {
    if (error?.code === 'qr_not_loadout') throw error
  }

  const code = raw.toUpperCase().replace(/[-\s]+/gu, '')
  if (!SHORT_CODE_PATTERN.test(code)) throw qrError('qr_not_loadout')
  return code
}

async function qrDecoder() {
  decoderPromise ||= import('jsqr').then(module => module.default)
  return decoderPromise
}

export async function decodeLoadoutShareQRImageData(data, width, height) {
  const jsQR = await qrDecoder()
  const decoded = jsQR(data, width, height, { inversionAttempts: 'attemptBoth' })
  if (!decoded?.data) throw qrError('qr_not_found')
  return normalizeLoadoutShareQRPayload(decoded.data)
}

function supportedImage(file) {
  if (String(file?.type || '').startsWith('image/')) return true
  return /\.(?:jpe?g|png|webp|bmp)$/iu.test(String(file?.name || ''))
}

async function loadImageSource(file) {
  if (typeof createImageBitmap === 'function') {
    return createImageBitmap(file, { imageOrientation: 'from-image' })
  }
  const url = URL.createObjectURL(file)
  try {
    return await new Promise((resolve, reject) => {
      const image = new Image()
      image.onload = () => resolve(image)
      image.onerror = () => reject(qrError('qr_image_failed'))
      image.src = url
    })
  } finally {
    URL.revokeObjectURL(url)
  }
}

export async function decodeLoadoutShareQRFile(file) {
  if (!file || !supportedImage(file)) throw qrError('qr_image_type')
  if (!Number.isFinite(file.size) || file.size <= 0) throw qrError('qr_image_empty')
  if (file.size > LOADOUT_SHARE_QR_MAX_FILE_BYTES) throw qrError('qr_image_too_large')

  let image
  try {
    image = await loadImageSource(file)
    const sourceWidth = Number(image.width)
    const sourceHeight = Number(image.height)
    if (!(sourceWidth > 0) || !(sourceHeight > 0)) throw qrError('qr_image_failed')
    const scale = Math.min(
      1,
      MAX_CANVAS_EDGE / Math.max(sourceWidth, sourceHeight),
      Math.sqrt(MAX_CANVAS_PIXELS / (sourceWidth * sourceHeight)),
    )
    const width = Math.max(1, Math.round(sourceWidth * scale))
    const height = Math.max(1, Math.round(sourceHeight * scale))
    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const context = canvas.getContext('2d', { willReadFrequently: true })
    if (!context) throw qrError('qr_image_failed')
    context.drawImage(image, 0, 0, width, height)
    return await decodeLoadoutShareQRImageData(context.getImageData(0, 0, width, height).data, width, height)
  } catch (error) {
    if (error?.code) throw error
    throw qrError('qr_image_failed')
  } finally {
    image?.close?.()
  }
}
