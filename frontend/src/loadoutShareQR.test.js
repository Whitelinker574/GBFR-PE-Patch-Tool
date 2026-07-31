import assert from 'node:assert/strict'
import test from 'node:test'
import QRCode from 'qrcode'
import sharp from 'sharp'
import {
  decodeLoadoutShareQRImageData,
  normalizeLoadoutShareQRPayload,
} from './loadoutShareQR.js'

test('share-image QR pixels decode into the same online loadout URL', async () => {
  const expected = 'https://example.com/s/0123-4567-89AB-CDEF'
  const png = await QRCode.toBuffer(expected, { errorCorrectionLevel: 'M', margin: 4, width: 512 })
  const { data, info } = await sharp(png).ensureAlpha().raw().toBuffer({ resolveWithObject: true })
  assert.equal(await decodeLoadoutShareQRImageData(new Uint8ClampedArray(data), info.width, info.height), expected)
})

test('QR payload validation accepts supported codes and rejects unrelated QR content', () => {
  assert.equal(normalizeLoadoutShareQRPayload('0123-4567-89AB-CDEF'), '0123456789ABCDEF')
  assert.equal(normalizeLoadoutShareQRPayload('https://example.com/download/0123-4567-89AB-CDEF.gbfr-loadout'), 'https://example.com/download/0123-4567-89AB-CDEF.gbfr-loadout')
  assert.throws(() => normalizeLoadoutShareQRPayload('https://example.com/unrelated'), error => error?.code === 'qr_not_loadout')
})
