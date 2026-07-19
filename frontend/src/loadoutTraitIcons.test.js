import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const traitIcons = JSON.parse(readFileSync(new URL('./loadoutTraitIcons.json', import.meta.url), 'utf8'))
const gameAssets = JSON.parse(readFileSync(new URL('./gameAssetIcons.json', import.meta.url), 'utf8'))

const expectedOfficialMappings = {
  'War Elemental': 'cmn_icskill_02_08.png',
  'Fearless Drive': 'cmn_icskill_05_pl0000.png',
  'Fearless Heart': 'cmn_icskill_05_pl0000.png',
  'Fearless Spirit': 'cmn_icskill_05_pl0000.png',
  '属性克制转换': 'cmn_icskill_02_08.png',
  '勇士的毅力': 'cmn_icskill_05_pl0900.png',
  '涯之二王': 'cmn_icskill_05_pl2300.png',
  '狼王的激昂': 'cmn_icskill_05_pl2400.png',
  '刃姬的轮舞曲': 'cmn_icskill_05_pl2500.png',
  '群青的剑光': 'cmn_icskill_05_pl2600.png',
  '雷狼的慧眼': 'cmn_icskill_05_pl2700.png',
  '转世的恩宠': 'cmn_icskill_05_pl2800.png',
  "The Black's Impulse": 'cmn_icskill_05_pl2900.png',
}

function assertPng(file, label) {
  const bytes = readFileSync(new URL(`../public/loadout-icons/traits/${file}`, import.meta.url))
  assert.deepEqual([...bytes.subarray(0, 8)], [137, 80, 78, 71, 13, 10, 26, 10], `${label}: PNG signature`)
  assert.equal(bytes.subarray(12, 16).toString('ascii'), 'IHDR', `${label}: IHDR chunk`)
  assert.ok(bytes.readUInt32BE(16) > 0 && bytes.readUInt32BE(20) > 0, `${label}: image dimensions`)
  assert.equal(bytes.subarray(-8).toString('hex'), '49454e44ae426082', `${label}: IEND chunk`)
}

test('trait-name compatibility map is generated from the official semantic catalog', () => {
  assert.deepEqual(traitIcons, gameAssets.traits.byName)

  for (const [name, file] of Object.entries(expectedOfficialMappings)) {
    assert.equal(traitIcons[name], file, `${name} should resolve to ${file}`)
  }

  assert.equal(traitIcons['刃姬的圆舞曲'], undefined, 'do not retain the non-canonical 圆舞曲 spelling')
})

test('all trait-name mappings are bundled official game PNGs, never community avatars', () => {
  const uniqueFiles = new Set(Object.values(traitIcons))
  assert.ok(uniqueFiles.size > 0)

  for (const file of uniqueFiles) {
    assert.match(file, /^cmn_icskill_[a-z0-9_]+\.png$/i, `${file}: internal game trait asset name`)
    assert.doesNotMatch(file, /avatar\.png/i, `${file}: community avatar must not be used as a trait icon`)
    assert.doesNotMatch(file, /[\\/]/, `${file}: mapping must stay inside the traits asset folder`)
    assertPng(file, file)
  }
})
