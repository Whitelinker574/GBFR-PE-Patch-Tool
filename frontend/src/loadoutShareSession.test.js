import assert from 'node:assert/strict'
import test from 'node:test'
import {
  forgetPublishedLoadoutShare,
  loadoutShareSessionKey,
  publishedLoadoutShare,
  rememberPublishedLoadoutShare,
} from './loadoutShareSession.js'

test('published loadouts use stable source-specific session keys', () => {
  const saveKey = loadoutShareSessionKey({ savePath: 'C:/SaveData1.dat', charaHash: '0d21b430', unitId: 42 })
  const codeKey = loadoutShareSessionKey({ compatibilityCode: 'gblc-test-code' })
  const runtimeKey = loadoutShareSessionKey({ source: 'runtime', recordId: 'battle-7', role: 'party1' })
  assert.equal(saveKey, 'save:C:/SAVEDATA1.DAT:0D21B430:42')
  assert.equal(codeKey, 'code:GBLC-TEST-CODE')
  assert.equal(runtimeKey, 'runtime:BATTLE-7:PARTY1')
})

test('published loadout snapshots are reused without exposing mutable caller state', () => {
  const key = loadoutShareSessionKey({ compatibilityCode: 'frame-1' })
  const result = { code: 'ABC123', url: 'https://example.invalid/s/ABC123' }
  rememberPublishedLoadoutShare(key, result)
  result.url = 'https://example.invalid/changed'
  assert.equal(publishedLoadoutShare(key)?.url, 'https://example.invalid/s/ABC123')
  forgetPublishedLoadoutShare(key)
  assert.equal(publishedLoadoutShare(key), null)
})
