import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import sharp from 'sharp'
import {
  characterIdentityByHash,
  characterIdentityByPLID,
  characterRoster,
  characterSharePortraitProfile,
} from './characterRoster.js'

test('the canonical character roster covers every playable character exactly once', () => {
  assert.equal(characterRoster.length, 29)
  for (const field of ['hash', 'plId', 'slug', 'nameZh', 'nameEn']) {
    assert.equal(new Set(characterRoster.map(character => character[field])).size, 29, `${field} must be unique`)
  }
  assert.equal(characterIdentityByPLID('pl2700')?.nameZh, '尤斯提斯')
  assert.equal(characterIdentityByHash('0x9b15cfb1')?.nameEn, 'Eustace')
})

test('every share portrait has verified dimensions and bounded crop metadata', async () => {
  for (const character of characterRoster) {
    const profile = characterSharePortraitProfile(character.hash)
    const file = fileURLToPath(new URL(`../public${profile.path}`, import.meta.url))
    assert.ok(existsSync(file), `${character.slug} portrait must exist`)
    const metadata = await sharp(file).metadata()
    assert.deepEqual([metadata.width, metadata.height], profile.intrinsicSize, `${character.slug} dimensions must match the file`)
    assert.equal(profile.weaponSafeFrame.length, 4)
    assert.ok(profile.weaponSafeFrame.every(value => Number.isFinite(value) && value >= 0 && value <= 1))
    assert.ok(profile.faceFocus.every(value => Number.isFinite(value) && value >= 0 && value <= 1))
    for (const template of ['landscape', 'portrait', 'square']) {
      assert.equal(profile.anchors[template].fit, 'cover')
      assert.ok(profile.anchors[template].focus.every(value => Number.isFinite(value) && value >= 0 && value <= 1))
    }
  }
})
