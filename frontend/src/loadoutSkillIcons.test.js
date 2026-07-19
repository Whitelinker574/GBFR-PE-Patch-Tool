import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { readdirSync, readFileSync } from 'node:fs'
import test from 'node:test'

const skillIcons = JSON.parse(readFileSync(new URL('./loadoutSkillIcons.json', import.meta.url), 'utf8'))
const skillCatalog = JSON.parse(readFileSync(new URL('../../data/skill_names.json', import.meta.url), 'utf8')).skills
const editor = readFileSync(new URL('./components/LoadoutEditor.vue', import.meta.url), 'utf8')
const viewer = readFileSync(new URL('./components/LoadoutViewer.vue', import.meta.url), 'utf8')
const exactSkillFiles = new Set(readdirSync(new URL('../public/loadout-icons/skills/', import.meta.url)))

function assertGamePng(key, file) {
  assert.doesNotMatch(file, /[\\/]/, `${key}: mapping must stay inside the ability asset folder`)
  assert.ok(exactSkillFiles.has(file), `${key}: filename case must exactly match the embedded path`)
  const bytes = readFileSync(new URL(`../public/loadout-icons/skills/${file}`, import.meta.url))
  assert.deepEqual([...bytes.subarray(0, 8)], [137, 80, 78, 71, 13, 10, 26, 10], `${key}: PNG signature`)
  assert.equal(bytes.subarray(12, 16).toString('ascii'), 'IHDR', `${key}: IHDR chunk`)
  assert.ok(bytes.readUInt32BE(16) > 0 && bytes.readUInt32BE(20) > 0, `${key}: image dimensions`)
  assert.equal(bytes.subarray(-8).toString('hex'), '49454e44ae426082', `${key}: IEND chunk`)
}

const endlessRagnarokSkills = Object.entries(skillIcons).filter(([key]) => {
  const match = key.match(/^AB_PL(\d{4})_(\d{2})$/)
  if (!match) return false
  const owner = Number(match[1])
  return owner >= 2400 && owner <= 2900
})

test('audited 2.0.2 ability rows keep their exact official icon semantics', () => {
  const expectedMappings = {
    AB_PL2000_01: 'cmn_icablt_pl2000_01.png',
    AB_PL2000_02: 'cmn_icablt_pl2000_05.png',
    AB_PL2000_03: 'cmn_icablt_pl2000_07.png',
    AB_PL2000_04: 'cmn_icablt_pl2000_08.png',
    AB_PL2000_05: 'cmn_icablt_pl1900_06.png',
    AB_PL2200_03: 'cmn_icablt_pl2200_03.png',
    AB_PL2200_06: 'cmn_icablt_pl2200_06.png',
  }
  const officialHashes = {
    'cmn_icablt_pl2000_01.png': 'aad9022fab8736292119cf358c5b7f5d376721cd6fdfe383d060bffe9aabfee6',
    'cmn_icablt_pl2000_05.png': 'ae372166c2ad781b62da9eb08e90312b2bfc363fc3f53522e10566f427eb153d',
    'cmn_icablt_pl2000_07.png': 'd086a645e0052b716254ecd9b4288f9d0947cb8dbfb54aafb17e8989a92fb7d4',
    'cmn_icablt_pl2000_08.png': '1e2cd80a2b6248249d74e12ef8a4d78baf9425838148a884422d26a72366615c',
    'cmn_icablt_pl1900_06.png': '2985cb1f7c3fddab811bb1f891886857fd89270e8128b1aa94f5517a0aadba3c',
    'cmn_icablt_pl2200_03.png': '70a928bdf3372508c5851251ea47609ed94b0bdcf0ae6278ba7eb1bdf9ae7147',
    'cmn_icablt_pl2200_06.png': '122b29c547e72fc781a83f321c07322e09318b58d070ec55b23729e967e5b492',
  }

  for (const [key, file] of Object.entries(expectedMappings)) {
    assert.equal(skillIcons[key], file, `${key}: ability.tbl icon semantics`)
    assertGamePng(key, file)
  }
  for (const [file, expectedHash] of Object.entries(officialHashes)) {
    const bytes = readFileSync(new URL(`../public/loadout-icons/skills/${file}`, import.meta.url))
    assert.equal(createHash('sha256').update(bytes).digest('hex'), expectedHash, `${file}: official 2.0.2 asset`)
  }
})

test('all six Endless Ragnarok characters use the exact 2.0.2 ability icons', () => {
  assert.equal(endlessRagnarokSkills.length, 48)
  for (const [key, file] of endlessRagnarokSkills) {
    assertGamePng(key, file)
  }
  assert.equal(skillIcons.AB_PL2400_09, undefined, '狼牙斩 has no matching cmn_icablt asset and must keep the neutral fallback')
})

test('every mapped playable ability uses an exact bundled game PNG path', () => {
  const files = new Set()
  for (const [key, file] of Object.entries(skillIcons)) {
    assertGamePng(key, file)
    files.add(file)
  }
  assert.ok(files.size > 200, `expected broad ability icon coverage, got ${files.size} unique files`)
})

test('every table-backed playable ability uses its official 2.0.2 sprite filename', () => {
  const playableKeys = Object.values(skillCatalog)
    .filter(skill => /^PL\d{4}$/.test(skill.char))
    .map(skill => skill.key)
  const nonOfficial = playableKeys
    .filter(key => key !== 'AB_PL2400_09')
    .filter(key => !/^cmn_icablt_pl\d{4}_\d{2}\.png$/.test(skillIcons[key] ?? ''))

  assert.deepEqual(nonOfficial, [], `${nonOfficial.length} playable skills still use community/translated filenames`)
})

test('all mapped playable ability bytes match the canonical 2.0.2 table and reference ZIP', () => {
  // This digest locks each sorted `skill key | ability.tbl filename | PNG SHA-256`
  // tuple. It was derived from the unpacked 2.0.2 ability.tbl and the untouched
  // PNG streams in GBFR-UI-Reference-Library-2.0.2.zip.
  const canonicalLines = Object.values(skillCatalog)
    .filter(skill => /^PL\d{4}$/.test(skill.char) && skill.key !== 'AB_PL2400_09')
    .map(skill => {
      const file = skillIcons[skill.key]
      assertGamePng(skill.key, file)
      const bytes = readFileSync(new URL(`../public/loadout-icons/skills/${file}`, import.meta.url))
      const pngHash = createHash('sha256').update(bytes).digest('hex')
      return `${skill.key}|${file}|${pngHash}`
    })
    .sort()

  assert.equal(canonicalLines.length, 261)
  assert.equal(
    createHash('sha256').update(canonicalLines.join('\n')).digest('hex'),
    'ba5248fbfc9491550bccb9360db4f13df32fc199ad3aaf1b8ecd9c721a7e859c',
  )
})

test('playable active-skill coverage is exact and excludes NPC-only rows', () => {
  const playable = Object.values(skillCatalog).filter(skill => /^PL\d{4}$/.test(skill.char))
  const npc = Object.values(skillCatalog).filter(skill => /^NP\d{4}$/.test(skill.char))
  const missing = playable.filter(skill => !skillIcons[skill.key]).map(skill => skill.key)

  assert.equal(playable.length, 262)
  assert.equal(npc.length, 16)
  assert.equal(playable.length - missing.length, 261)
  assert.deepEqual(missing, ['AB_PL2400_09'])
})

test('editor and viewer do not discard verified DLC icon mappings', () => {
  for (const source of [editor, viewer]) {
    assert.doesNotMatch(source, /Number\(owner\[1\]\)\s*<\s*2400/)
  }
})
