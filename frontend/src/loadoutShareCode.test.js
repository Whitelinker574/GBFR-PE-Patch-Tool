import test from 'node:test'
import assert from 'node:assert/strict'
import {
  compactLoadoutShareCode,
  compatibleLoadoutShareCode,
  isOfflineLoadoutShareCode,
  loadoutShareCodeCharacters,
  normalizeLoadoutShareText,
} from './loadoutShareCode.js'

test('loadout share code converts the exact frame between compact and compatible text', () => {
  const frame = Uint8Array.from({ length: 1728 }, (_, index) => (index * 31 + 7) & 0xff)
  const binary = String.fromCharCode(...frame)
  const compatibility = `GBFRC1A.${btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')}`
  const compact = compactLoadoutShareCode(compatibility)

  assert.match(compact, /^GBFRC1U\./u)
  assert.equal(compatibleLoadoutShareCode(compact), compatibility)
  assert.ok(loadoutShareCodeCharacters(compact) < loadoutShareCodeCharacters(compatibility) / 2)
})

test('loadout share code tolerates line wrapping and rejects unknown prefixes', () => {
  const compatibility = 'GBFRC1A.R0JMQwEBAQAAAAAAAAB4'
  const wrapped = ` GBFRC1A.\n${compatibility.slice('GBFRC1A.'.length, -3)}\u200B${compatibility.slice(-3)} `
  assert.equal(normalizeLoadoutShareText(wrapped), compatibility)
  assert.equal(compatibleLoadoutShareCode(wrapped), compatibility)
  assert.throws(() => compatibleLoadoutShareCode('GBFR-old.invalid'), /无法识别分享码/u)
  assert.equal(isOfflineLoadoutShareCode(wrapped), true)
  assert.equal(isOfflineLoadoutShareCode('0123-4567-89AB-CDEF'), false)
})
