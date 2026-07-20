import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import test from 'node:test'

const catalog = JSON.parse(readFileSync(new URL('./gameAssetIcons.json', import.meta.url), 'utf8'))
const traits = JSON.parse(readFileSync(new URL('../../data/traits.json', import.meta.url), 'utf8')).traits
const weapons = JSON.parse(readFileSync(new URL('../../data/weapons.json', import.meta.url), 'utf8')).weapons
const summons = JSON.parse(readFileSync(new URL('../../data/summons.json', import.meta.url), 'utf8')).summons
const items = JSON.parse(readFileSync(new URL('../../data/items.json', import.meta.url), 'utf8')).items
const iconSyncScript = readFileSync(new URL('../../tools/sync_reference_icons.ps1', import.meta.url), 'utf8')

const expectedCharacterIcons = {
  '2A26B1B2': 'cmn_mini_s_pl0000.png',
  A4ACBA76: 'cmn_mini_s_pl0100.png',
  '18E2F9F9': 'cmn_mini_s_pl0200.png',
  '079DF0CC': 'cmn_mini_s_pl0300.png',
  '4D0A60C3': 'cmn_mini_s_pl0400.png',
  DD7A151E: 'cmn_mini_s_pl0500.png',
  C8616284: 'cmn_mini_s_pl0600.png',
  C3FFD418: 'cmn_mini_s_pl0700.png',
  '22E437E5': 'cmn_mini_s_pl0800.png',
  '2EBE91D5': 'cmn_mini_s_pl0900.png',
  BDEF7181: 'cmn_mini_s_pl1000.png',
  '627BCB0D': 'cmn_mini_s_pl1100.png',
  FD3BE362: 'cmn_mini_s_pl1200.png',
  FC6CDF7B: 'cmn_mini_s_pl1300.png',
  E7053919: 'cmn_mini_s_pl1400.png',
  '978E4B18': 'cmn_mini_s_pl1500.png',
  '0D21B430': 'cmn_mini_s_pl1600.png',
  F0EB77EF: 'cmn_mini_s_pl1700.png',
  AA66178A: 'cmn_mini_s_pl1800.png',
  A3A3CB2F: 'cmn_mini_s_pl1900.png',
  '718E1A14': 'cmn_mini_s_pl2100.png',
  '296471BE': 'cmn_mini_s_pl2200.png',
  BAD16E3B: 'cmn_mini_s_pl2300.png',
  '1BB37EF0': 'cmn_mini_s_pl2400.png',
  '25D46F4B': 'cmn_mini_s_pl2500.png',
  '9A8AF295': 'cmn_mini_s_pl2600.png',
  '9B15CFB1': 'cmn_mini_s_pl2700.png',
  '646C3168': 'cmn_mini_s_pl2800.png',
  '74DD4C79': 'cmn_mini_s_pl2900.png',
}

const expectedMissingItemHashes = [
  '131A4636',
  '29BBA035',
  '7F695B76',
  '9FC6585E',
  'CB39E0FC',
  'CD6AF550',
  'CE0B379E',
  'E600BE75',
  'E8C461CA',
  'EAC2D7AB',
  'F384E322',
]

const sectionFilePatterns = {
  traits: /^cmn_icskill_[a-z0-9_]+\.png$/i,
  weapons: /^cmn_imgequ_wp[a-z0-9_]+\.png$/i,
  summons: /^cmn_icitmsmn02_[a-z0-9_]+\.png$/i,
  items: /^cmn_icitm_[a-z0-9_]+\.png$/i,
  characters: /^cmn_mini_s_pl\d{4}\.png$/i,
}

function normalizeHash(value) {
  return String(value ?? '').trim().replace(/^0x/i, '').toUpperCase()
}

function mappedFiles(section) {
  return new Set(Object.values(catalog[section]).flatMap(mapping => Object.values(mapping)))
}

function assertPng(section, file) {
  const bytes = readFileSync(new URL(`../public/loadout-icons/${section}/${file}`, import.meta.url))
  assert.deepEqual([...bytes.subarray(0, 8)], [137, 80, 78, 71, 13, 10, 26, 10], `${section}/${file}: PNG signature`)
  assert.equal(bytes.subarray(12, 16).toString('ascii'), 'IHDR', `${section}/${file}: IHDR chunk`)
  assert.ok(bytes.readUInt32BE(16) > 0 && bytes.readUInt32BE(20) > 0, `${section}/${file}: image dimensions`)
  assert.equal(bytes.subarray(-8).toString('hex'), '49454e44ae426082', `${section}/${file}: IEND chunk`)
}

test('coverage is derived from the current application catalogs', () => {
  const expectedCoverage = {
    traits: `${traits.filter(row => catalog.traits.byId[row.internalId]).length}/${traits.length}`,
    weapons: `${weapons.filter(row => catalog.weapons.byHash[normalizeHash(row.hash)]).length}/${weapons.length}`,
    summons: `${summons.filter(row => catalog.summons.byHash[normalizeHash(row.hash)]).length}/${summons.length}`,
    items: `${items.filter(row => catalog.items.byHash[normalizeHash(row.hash)]).length}/${items.length}`,
    characters: `${Object.keys(catalog.characters.byHash).length}/${Object.keys(expectedCharacterIcons).length}`,
  }

  assert.equal(catalog.schemaVersion, 1)
  assert.equal(catalog.source, 'GBFR UI Reference Library 2.0.2 / semantic catalog + unpacked 2.0.2 game tables')
  assert.deepEqual(expectedCoverage, {
    traits: '183/184',
    weapons: '159/163',
    summons: '189/189',
    items: '301/312',
    characters: '29/29',
  })
  assert.deepEqual(catalog.coverage, expectedCoverage)
})

test('all mapped files exist locally and retain valid game PNG structure', () => {
  assert.doesNotMatch(JSON.stringify(catalog), /avatar\.png/i, 'semantic mappings must not use community avatars')

  for (const [section, pattern] of Object.entries(sectionFilePatterns)) {
    const files = mappedFiles(section)
    const exactDiskNames = new Set(readdirSync(new URL(`../public/loadout-icons/${section}/`, import.meta.url)))
    assert.ok(files.size > 0, `${section}: expected at least one mapped icon`)
    for (const file of files) {
      assert.match(file, pattern, `${section}/${file}: official internal game asset name`)
      assert.doesNotMatch(file, /[\\/]/, `${section}/${file}: mapping must stay inside its asset folder`)
      assert.ok(exactDiskNames.has(file), `${section}/${file}: filename case must exactly match the embedded path`)
      assertPng(section, file)
    }
  }
})

test('all 29 save character hashes resolve to their exact official compact icon', () => {
  assert.deepEqual(catalog.characters.byHash, expectedCharacterIcons)
  assert.equal(new Set(Object.values(catalog.characters.byHash)).size, 29, 'each roster entry should have its own player-code icon')
})

test('Endless Ragnarok trait IDs keep the exact per-character official emblem', () => {
  const dlcTraits = traits.filter(row => /^SKILL_17[3-8]_/.test(row.internalId))
  assert.equal(dlcTraits.length, 17)

  for (const trait of dlcTraits) {
    const skillNumber = Number(trait.internalId.slice(6, 9))
    const playerCode = String((skillNumber - 149) * 100).padStart(4, '0')
    const expectedFile = `cmn_icskill_05_pl${playerCode}.png`
    assert.equal(catalog.traits.byId[trait.internalId], expectedFile, `${trait.internalId}: ID mapping`)
    assert.equal(catalog.traits.byHash[normalizeHash(trait.hash)], expectedFile, `${trait.internalId}: hash mapping`)
  }

  assert.equal(catalog.traits.byId.SKILL_146_00, 'cmn_icskill_02_08.png', 'War Elemental exact semantic join')
  assert.equal(catalog.traits.byId.SKILL_143_00, 'cmn_icskill_02_00.png', 'Catastrophe exact hash reuse')
  assert.equal(catalog.traits.byHash['40223C28'], 'cmn_icskill_02_00.png', 'Catastrophe authoritative hash mapping')
})

test('records without an authoritative 2.0.2 join remain unmapped', () => {
  const missingTraitIDs = traits.filter(row => !catalog.traits.byId[row.internalId]).map(row => row.internalId)
  const missingWeaponHashes = weapons.filter(row => !catalog.weapons.byHash[normalizeHash(row.hash)]).map(row => normalizeHash(row.hash))
  const missingItemHashes = items.filter(row => !catalog.items.byHash[normalizeHash(row.hash)]).map(row => normalizeHash(row.hash)).sort()

  assert.deepEqual(missingTraitIDs, ['SKILL_112_00'])
  assert.deepEqual(missingWeaponHashes, ['2C4CAADD', 'DFBB5727', '73D34F1B', 'DA807CA2'])
  assert.deepEqual(missingItemHashes, expectedMissingItemHashes)
  assert.equal(catalog.traits.byHash.D0A1C6E5, undefined)
  assert.equal(catalog.traits.byName['Window of Opportunity'], undefined)
  assert.equal(catalog.traits.byId.SKILL_023_00, 'cmn_icskill_01_00.png', 'skill.tbl IconId1 is the exact ID proof')
  assert.equal(catalog.traits.byHash.CAC6AFF2, 'cmn_icskill_01_00.png', 'skill.tbl hash row is authoritative')
  assert.equal(catalog.traits.byName['Potent Greens'], 'cmn_icskill_04_04.png', 'legacy name-only compatibility fallback remains explicit')
  assert.equal(catalog.weapons.byId.WEP_PL2100_06, undefined)
  assert.equal(catalog.weapons.byHash.DFBB5727, undefined)
  assert.equal(catalog.weapons.byId.WEP_PL2100_03, undefined)
  assert.equal(catalog.weapons.byHash['2C4CAADD'], undefined)
  assert.equal(catalog.weapons.byId.WEP_PL2200_03, undefined)
  assert.equal(catalog.weapons.byHash['73D34F1B'], undefined)
  assert.equal(catalog.weapons.byId.WEP_PL2300_03, undefined)
  assert.equal(catalog.weapons.byHash.DA807CA2, undefined)
  assert.equal(catalog.weapons.byHash.AD915067, 'cmn_imgequ_wp2106.png', 'special runtime canonical hash')
  assert.equal(catalog.weapons.byHash.FA5F32D5, 'cmn_imgequ_wp2206.png', 'special runtime canonical hash')
  assert.equal(catalog.weapons.byId.WEP_PL2300_07, 'cmn_imgequ_wp2306.png', 'special runtime canonical ID')
  assert.equal(catalog.weapons.byHash['4CBA06D8'], 'cmn_imgequ_wp2306.png', 'special runtime canonical hash')
})

test('the generator rebuilds application trait icons from the exact skill table hash and IconId1', () => {
  assert.match(iconSyncScript, /\$skillTableBytes\s*=\s*\[byte\[\]\]\(Read-GameTableBytes 'skill\.tbl'\)/)
  assert.match(iconSyncScript, /\$skillTableRowSize\s*=\s*112/)
  assert.match(iconSyncScript, /ToUInt32\(\$skillTableBytes,\s*\$offset\s*\+\s*68\)/)
  assert.match(iconSyncScript, /Read-FixedASCII \$skillTableBytes \$offset 16/)
  assert.match(iconSyncScript, /Unexpected missing 2\.0\.2 trait sprites/)
})

test('the generator audits every application item through the exact item table hash and icon id', () => {
  assert.match(iconSyncScript, /\$itemRows\s*=.*data\\items\.json/s)
  assert.match(iconSyncScript, /\$itemTableRowSize\s*=\s*128/)
  assert.match(iconSyncScript, /ToUInt32\(\$itemTableBytes,\s*\$offset\s*\+\s*32\)/)
  assert.match(iconSyncScript, /Read-FixedASCII \$itemTableBytes \(\$offset \+ 16\) 16/)
  assert.match(iconSyncScript, /Unexpected missing 2\.0\.2 item sprites/)
})
