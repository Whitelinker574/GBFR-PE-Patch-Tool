import assert from 'node:assert/strict'
import test from 'node:test'

import jsQR from 'jsqr'
import QRCode from 'qrcode'
import sharp from 'sharp'

const shareUrl = 'https://example.invalid/s/74R5HQWYK0JDGYPB'

async function decode(buffer) {
  const { data, info } = await sharp(buffer)
    .ensureAlpha()
    .raw()
    .toBuffer({ resolveWithObject: true })
  return jsQR(new Uint8ClampedArray(data), info.width, info.height, {
    inversionAttempts: 'dontInvert',
  })?.data || ''
}

async function compressedVariant(source, variant) {
  let pipeline = sharp(source).resize(variant.width, variant.width, {
    fit: 'fill',
    kernel: sharp.kernel.lanczos3,
  })
  if (variant.format === 'jpeg') pipeline = pipeline.jpeg({ quality: variant.quality, chromaSubsampling: '4:2:0' })
  else if (variant.format === 'webp') pipeline = pipeline.webp({ quality: variant.quality, smartSubsample: true })
  else pipeline = pipeline.png({ compressionLevel: 9 })
  return pipeline.toBuffer()
}

test('share QR survives chat-style resize and recompression', async () => {
  const source = await QRCode.toBuffer(shareUrl, {
    type: 'png',
    errorCorrectionLevel: 'M',
    margin: 4,
    width: 240,
    color: { dark: '#2c241b', light: '#fffdf7' },
  })
  const variants = [
    { name: 'export raster', width: 108, format: 'png' },
    { name: 'high quality JPEG', width: 108, format: 'jpeg', quality: 85 },
    { name: 'group-chat JPEG', width: 96, format: 'jpeg', quality: 72 },
    { name: 'group-chat WebP', width: 96, format: 'webp', quality: 68 },
  ]
  for (const variant of variants) {
    const encoded = await compressedVariant(source, variant)
    assert.equal(await decode(encoded), shareUrl, `${variant.name} no longer decodes`)
  }
})
