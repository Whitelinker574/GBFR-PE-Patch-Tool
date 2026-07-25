import test from 'node:test'
import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { decode } from '@msgpack/msgpack'
import worker, {
  displayCode,
  frameIdentity,
  normalizeCode,
  validateFrame,
} from '../src/index.js'
import { loadoutJSONToFrame } from '../src/loadoutUpload.js'

function makeFrame(compressed = new Uint8Array([1, 2, 3, 4]), version = 1) {
  const frame = new Uint8Array(18 + compressed.length)
  frame.set(new TextEncoder().encode('GBLC'), 0)
  frame[4] = version
  frame[5] = 1
  const view = new DataView(frame.buffer)
  view.setUint32(6, 64, true)
  view.setUint32(10, 0x12345678, true)
  view.setUint32(14, compressed.length, true)
  frame.set(compressed, 18)
  return frame
}

function makeR2() {
  const objects = new Map()
  const wrap = entry => entry && {
    customMetadata: entry.customMetadata,
    httpEtag: `"${entry.customMetadata.sha256}"`,
    arrayBuffer: async () => entry.bytes.slice().buffer,
  }
  return {
    objects,
    async head(key) {
      return wrap(objects.get(key))
    },
    async get(key) {
      return wrap(objects.get(key))
    },
    async put(key, value, options) {
      objects.set(key, {
        bytes: typeof value === 'string' ? new TextEncoder().encode(value) : new Uint8Array(value),
        customMetadata: options.customMetadata,
      })
    },
    async list(options = {}) {
      const prefix = options.prefix || ''
      const items = [...objects.keys()]
        .filter(key => key.startsWith(prefix))
        .map(key => ({ key }))
      return { objects: items, truncated: false }
    },
  }
}

function makeV10JSON() {
  return {
    format: 'gbfr-loadout', version: 10, charaHash: '4D0A60C3', charaName: '伊欧', ownerCode: 'PL0400', name: '网页上传测试',
    weaponHash: 'BE1BA9E3', weaponName: '星晶武器',
    sigils: [{ index: 0, hash: '0053599E', name: '快速冷却 V+', level: 15, primaryTraitHash: '318D12E9', primaryTraitLevel: 15, secondaryTraitHash: '50079A1C', secondaryTraitLevel: 15 }],
    summons: Array.from({ length: 4 }, () => ({ typeHash: '0033943A', name: '巴哈姆特', mainTraitHash: '50079A1C', mainTraitLevel: 10, subParamHash: '00D171E0', subParamLevel: 3, rank: 5 })),
    skills: [{ hash: '74AB18E8', name: '专注', key: 'SKILL_PL0400_01' }],
    weaponSkillHashes: ['8D78A19B', 'C0979A17', 'AEFEB1BC', 'E69A4694', '020DB733'],
    masteryHashes: Array.from({ length: 50 }, (_, index) => index === 0 ? '1F52146F' : '00000000'),
    character: {
      characterLevel: 100, baseHp: 3156, baseAtk: 666, baseStunBits: 1090519040, baseCritRate: 5, characterBaseCaptured: true,
      masterTotalMsp: 100, legacyProgress: 55, enhancementPanel: [508, 427], enhancementNodeValues: [7, 31, 1],
      weapons: [], weaponWrightstonesCaptured: true,
    },
    weapon: { storedHash: 'BE1BA9E3', xp: 162540, uncap: 6, mirage: 99, awakening: 0, transcendence: 7, exactState: true, flags: 28, wrightstoneReference: '', state: 65, skillHashes: ['8D78A19B', 'C0979A17', 'AEFEB1BC', 'E69A4694', '020DB733'] },
    overLimit: [
      { index: 0, attributeHash: '6CB38EF3', level: 10 },
      { index: 1, attributeHash: '6CB38EF3', level: 10 },
      { index: 2, attributeHash: '43B7581D', level: 10 },
      { index: 3, attributeHash: '9C555433', level: 10 },
    ],
  }
}

test('v10 JSON conversion preserves dense enhancement nodes in the GBLC v2 wire schema', async () => {
  let packed
  const source = makeV10JSON()
  const result = await loadoutJSONToFrame(JSON.stringify(source), bytes => { packed = bytes; return new Uint8Array([1, 2, 3]) })
  const wire = decode(packed)
  assert.equal(result.frame[4], 2)
  assert.deepEqual(wire[12][9], [[0, 7], [1, 31], [2, 1]])
  assert.equal(wire[7][0][4], 0x318d12e9)
  assert.equal(result.preview.characterName, '伊欧')
})

test('JSON import route rejects old exports and publishes a valid v10 file', async () => {
  const env = { LOADOUTS: makeR2() }
  const old = makeV10JSON()
  old.version = 8
  const rejected = await worker.fetch(new Request('https://share.example/api/v1/loadouts/import', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(old),
  }), env)
  assert.equal(rejected.status, 400)
  assert.match((await rejected.json()).error, /v10/)

  const imported = await worker.fetch(new Request('https://share.example/api/v1/loadouts/import', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(makeV10JSON()),
  }), env)
  assert.equal(imported.status, 201)
  const published = await imported.json()
  assert.match(published.code, /^[0-9A-HJKMNP-TV-Z]{4}(?:-[0-9A-HJKMNP-TV-Z]{4}){3}$/)
  const metadata = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${published.compactCode}/meta`), env).then(response => response.json())
  assert.equal(metadata.preview.characterName, '伊欧')
  assert.equal(metadata.preview.sigils[0].primaryLevel, 15)
  assert.deepEqual(metadata.preview.overLimit.map(item => [item.name, item.level, item.value, item.unit]), [
    ['昏厥值', 10, 20, 'flat'], ['昏厥值', 10, 20, 'flat'],
    ['普通攻击伤害上限', 10, 20, 'pct'], ['能力伤害上限', 10, 20, 'pct'],
  ])
})

test('frame validation accepts bounded GBLC v1 and v2 frames', () => {
  const frame = makeFrame()
  assert.equal(validateFrame(frame), '')
  assert.equal(validateFrame(makeFrame(new Uint8Array([1, 2, 3, 4]), 2)), '')
  assert.match(validateFrame(new TextEncoder().encode('plain text')), /过短|标识/)
  assert.match(validateFrame(makeFrame(new Uint8Array([1, 2, 3, 4]), 3)), /版本/)
})

test('short codes are deterministic and use readable Crockford characters', async () => {
  const first = await frameIdentity(makeFrame())
  const second = await frameIdentity(makeFrame())
  assert.equal(first.code, second.code)
  assert.match(first.code, /^[0-9A-HJKMNP-TV-Z]{16}$/)
  assert.equal(normalizeCode(displayCode(first.code)), first.code)
})

test('publish, load, download and landing routes round-trip one immutable frame', async () => {
  const r2 = makeR2()
  const env = { LOADOUTS: r2 }
  const frame = makeFrame()
  const publish = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/octet-stream' },
    body: frame,
  }), env)
  assert.equal(publish.status, 201)
  const result = await publish.json()
  assert.match(result.code, /^[0-9A-HJKMNP-TV-Z]{4}(?:-[0-9A-HJKMNP-TV-Z]{4}){3}$/)

  const repeated = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/octet-stream' },
    body: frame,
  }), env)
  assert.equal(repeated.status, 200)
  assert.equal((await repeated.json()).reused, true)

  const loaded = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${result.compactCode}`), env)
  assert.equal(loaded.status, 200)
  assert.deepEqual(new Uint8Array(await loaded.arrayBuffer()), frame)

  const landing = await worker.fetch(new Request(`https://share.example/s/${result.compactCode}`), env)
  assert.equal(landing.status, 200)
  assert.match(await landing.text(), new RegExp(result.code))

  const download = await worker.fetch(new Request(`https://share.example/download/${result.compactCode}.gbfr-loadout`), env)
  assert.equal(download.status, 200)
  assert.match(download.headers.get('Content-Disposition'), /\.gbfr-loadout/)
})

test('the service rejects arbitrary paste content and unknown codes', async () => {
  const env = { LOADOUTS: makeR2() }
  const invalid = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST',
    headers: { 'Content-Type': 'text/plain' },
    body: 'hello',
  }), env)
  assert.equal(invalid.status, 415)

  const missing = await worker.fetch(new Request('https://share.example/api/v1/loadouts/0123-4567-89AB-CDEF'), env)
  assert.equal(missing.status, 404)
})

test('publishing stores a sanitized complete preview and refreshes it when the same code is republished', async () => {
  const env = { LOADOUTS: makeR2() }
  const frame = makeFrame()
  const preview = {
    characterHash: '4D0A60C3', characterName: '伊欧', weaponHash: '02352554', weaponName: '星晶武器',
    sigils: [
      { hash: '0053599E', name: '快速冷却', level: 15, primaryHash: '0053599E', primary: '快速冷却', primaryLevel: 14 },
      { name: '怒发冲冠 + 伤害上限', level: 15, primary: '怒发冲冠', primaryLevel: 15, secondary: '伤害上限', secondaryLevel: 7 },
      { name: '体力V+', level: 15, primary: '体力', secondary: '伤害上限' },
      { name: '躲避性能+', level: 15, primary: '躲避性能' },
    ],
    abilities: [{ hash: '0053599E', name: '专注' }],
    weaponSkills: [{ hash: '020DB733', name: '超凡技艺', level: 15, effect: '攻击力+25%' }],
    wrightstone: { name: '勇气之石', traits: [{ hash: '0053599E', name: '暴击率', level: 10 }] },
    summons: [{ typeHash: '0033943A', name: '巴哈姆特', rank: 5, mainTraitHash: '50079A1C', mainTrait: '攻击力', mainTraitLevel: 10, subParamHash: '00D171E0', subParam: '暴击率', subParamLevel: 3, subParamValue: 12, subParamUnit: 'pct' }],
    masteryCount: 50, masteryCat: 'SB_ATK', masteryLabel: '真谛：魔法连锁',
    masterySkills: [{ hash: '0053599E', rank: '1阶专精技能', name: '魔法连锁', effect: '攻击力+10%', count: 2 }],
    combinedSkills: [
      { hash: '0053599E', name: '攻击力', level: 45, rawLevel: 55, maxLevel: 45, effect: '攻击力+100%', sources: ['因子01 · Tyranny', '武器 · Stygian Ornament · Tyranny Lv5', '武炼结晶 · Sequestration Wrightstone · Tyranny Lv10'] },
    ],
  }
  const encoded = Buffer.from(JSON.stringify(preview)).toString('base64url')
  const publish = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Title-B64': Buffer.from('训练场测试').toString('base64'), 'X-Loadout-Character-B64': Buffer.from('伊欧').toString('base64'), 'X-Loadout-Character-Hash': '4D0A60C3', 'X-Loadout-Preview': encoded },
    body: frame,
  }), env)
  const result = await publish.json()
  const meta = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${result.compactCode}/meta`), env)
  assert.equal(meta.status, 200)
  const metadata = await meta.json()
  assert.equal(metadata.title, '训练场测试')
  assert.equal(metadata.preview.sigils[0].level, 15)
  assert.equal(metadata.preview.sigils[1].name, '怒发冲冠 V+')
  assert.equal(metadata.preview.sigils[0].primaryLevel, 14)
  assert.equal(metadata.preview.sigils[1].secondaryLevel, 7)
  assert.equal(metadata.preview.sigils[2].name, '体力 V+')
  assert.equal(metadata.preview.sigils[2].secondaryLevel, 15)
  assert.equal(metadata.preview.sigils[3].name, '躲避性能+')
  assert.equal(metadata.preview.weaponSkills[0].name, '超凡技艺')
  assert.equal(metadata.preview.summons[0].mainTrait, '攻击力')
  assert.equal(metadata.preview.summons[0].subParamValue, 12)
  assert.equal(metadata.preview.masterySkills[0].count, 2)
  assert.equal(metadata.preview.combinedSkills[0].rawLevel, 55)
  assert.deepEqual(metadata.preview.combinedSkills[0].sources, [
    '因子01 · 攻击力',
    '武器 · [绝霸]布里欧纳克 · 攻击力 Lv5',
    '武器祝福 · 勇气之石 · 攻击力 Lv10',
  ])
  assert.equal(metadata.preview.masteryLabel, '真谛：魔法连锁')
  assert.match(metadata.preview.weaponIcon, /assets\/weapons\/cmn_imgequ_wp1602\.png/)
  assert.match(metadata.preview.sigils[0].icon, /assets\/traits\/cmn_icskill_05_02\.png/)
  assert.match(metadata.preview.summons[0].icon, /assets\/summons\/cmn_icitmsmn02_3f00\.png/)
  assert.match(metadata.preview.masterySkills[0].icon, /assets\/traits\/cmn_icskill_05_02\.png/)
  assert.match(metadata.preview.combinedSkills[0].icon, /assets\/traits\/cmn_icskill_05_02\.png/)
  assert.equal(Object.hasOwn(metadata, 'ownerCode'), false)

  const catalog = await worker.fetch(new Request('https://share.example/api/v1/loadouts?character=%E4%BC%8A%E6%AC%A7'), env)
  assert.equal(catalog.status, 200)
  assert.equal((await catalog.json()).items[0].title, '训练场测试')

  for (const query of ['[绝霸]布里欧纳克', '伤害上限', '勇气之石', '魔法连锁']) {
    const search = await worker.fetch(new Request(`https://share.example/api/v1/loadouts?q=${encodeURIComponent(query)}`), env)
    assert.equal(search.status, 200)
    assert.equal((await search.json()).items[0].title, '训练场测试')
  }
  const missingSearch = await worker.fetch(new Request('https://share.example/api/v1/loadouts?q=%E4%B8%8D%E5%AD%98%E5%9C%A8%E7%9A%84%E9%85%8D%E8%A3%85'), env)
  assert.deepEqual((await missingSearch.json()).items, [])

  const refreshed = { ...preview, sigils: [{ name: '快速冷却+', level: 15 }], combinedSkills: [{ name: '快速冷却', level: 45, rawLevel: 60 }] }
  const republish = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Preview': Buffer.from(JSON.stringify(refreshed)).toString('base64url') }, body: frame,
  }), env)
  assert.equal(republish.status, 200)
  const refreshedMeta = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${result.compactCode}/meta`), env)
  const refreshedMetadata = await refreshedMeta.json()
  assert.equal(refreshedMetadata.preview.sigils[0].name, '快速冷却+')
  assert.equal(refreshedMetadata.preview.combinedSkills[0].rawLevel, 60)
})

test('detail page includes summon effects, mastery nodes, and merged skill sections', async () => {
  const env = { LOADOUTS: makeR2() }
  const frame = makeFrame()
  const preview = Buffer.from(JSON.stringify({
    wrightstone: { name: 'Sequestration Wrightstone', traits: [] },
    summons: [{ name: '巴哈姆特', rank: 5, mainTrait: '攻击力' }],
    masterySkills: [{ name: '魔法连锁' }],
    overLimit: [{ index: 0, attributeHash: '6CB38EF3', name: '昏厥值', level: 10, value: 20, unit: 'flat' }],
    combinedSkills: [{ name: '攻击力', level: 45 }],
  })).toString('base64url')
  const publish = await worker.fetch(new Request('https://share.example/api/v1/loadouts', { method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Preview': preview }, body: frame }), env)
  const result = await publish.json()
  const page = await worker.fetch(new Request(`https://share.example/s/${result.compactCode}`), env)
  const html = await page.text()
  assert.match(html, /主加护 \/ 副词条 \/ 阶级/)
  assert.match(html, /专精技能/)
  assert.match(html, /合并技能等级/)
  assert.match(html, /上限突破/)
  assert.match(html, /overLimit\(p\.overLimit\|\|\[\]\)/)
  assert.match(html, /t\.skill\+' Lv'/)
  assert.match(html, /t\.sigil\+' Lv'/)
  assert.match(html, /隔绝之祝福/)
  assert.match(html, /t\.direction\+': '/)
  assert.match(html, /class="detail-icon"/)
  assert.match(html, /class="detail-back" href="\/c\/gran\?lang=zh">← 返回格兰配装<\/a>/)
  assert.doesNotMatch(html, /class="detail-actions"[^]*返回格兰配装/)
  assert.equal((html.match(/<div class="detail-column">/g) || []).length, 3)
  assert.doesNotMatch(html, /动态加成汇总/)
})

test('English pages keep navigation and official game names in English', async () => {
  const env = { LOADOUTS: makeR2() }
  const frame = makeFrame()
  const preview = Buffer.from(JSON.stringify({
    characterHash: '4D0A60C3', characterName: '伊欧', weaponHash: '02352554', weaponName: '星晶武器',
    sigils: [{ hash: '0053599E', name: '快速冷却 V+', primaryHash: '318D12E9', primary: '快速冷却', primaryLevel: 15, secondaryHash: '50079A1C', secondary: '攻击力', secondaryLevel: 15 }],
    abilities: [{ hash: '74AB18E8', name: '专注' }],
    summons: [{ typeHash: '0033943A', name: '巴哈姆特', mainTraitHash: '0DE887A0', mainTrait: '天星之炼', subParamHash: '00D171E0', subParam: '暴击率（低·最高20%）' }],
    masteryCount: 1, masteryCat: 'SB_ATK', masterySkills: [{ hash: '1F52146F', rank: 'EX阶专精技能', effect: '昏厥值+0.4' }],
  })).toString('base64url')
  const published = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Preview': preview }, body: frame,
  }), env)
  const result = await published.json()
  const page = await worker.fetch(new Request(`https://share.example/s/${result.compactCode}?lang=en`), env)
  const html = await page.text()
  assert.match(html, /Io · CHARACTER LOADOUT/)
  assert.match(html, /href="\/c\/io\?lang=en"/)
  assert.match(html, /Loadout Details/)
  assert.doesNotMatch(html, /返回|配装详情|角色配装|搜索/)
  const metadata = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${result.compactCode}/meta?lang=en`), env).then(response => response.json())
  assert.equal(metadata.preview.weaponName, 'Brionac')
  assert.equal(metadata.preview.sigils[0].primary, 'Quick Cooldown')
  assert.equal(metadata.preview.sigils[0].secondary, 'ATK')
  assert.equal(metadata.preview.summons[0].mainTrait, 'Celestial Nyx')
  assert.equal(metadata.preview.summons[0].subParam, 'Critical Hit Rate (Low · Max 20%)')
  assert.equal(metadata.preview.masterySkills[0].effect, 'Stun Power +0.4')
})

test('awakened weapon hashes resolve to the official base weapon name and icon', async () => {
  const env = { LOADOUTS: makeR2() }
  const preview = Buffer.from(JSON.stringify({
    characterName: '菲迪埃尔', weaponHash: 'F0B8CF77', weaponName: '[黑榫]幽冥华冠',
  })).toString('base64url')
  const published = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Preview': preview }, body: makeFrame(),
  }), env)
  const result = await published.json()
  const metadata = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${result.compactCode}/meta?lang=en`), env).then(response => response.json())
  assert.equal(metadata.preview.weaponName, 'Stygian Ornament')
  assert.match(metadata.preview.weaponIcon, /assets\/weapons\//)
})

test('legacy Chinese previews use official English name fallbacks', async () => {
  const env = { LOADOUTS: makeR2() }
  const preview = Buffer.from(JSON.stringify({
    abilities: [{ name: '非凡旅程' }],
    wrightstone: { name: '隔绝之祝福', traits: [] },
    summons: [{ name: '罗兰 · 传说 · 特殊', subParam: '普通攻击伤害上限', subParamLevel: 9, subParamValue: 100 }],
  })).toString('base64url')
  const published = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Preview': preview }, body: makeFrame(),
  }), env)
  const result = await published.json()
  const metadata = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${result.compactCode}/meta?lang=en`), env).then(response => response.json())
  assert.equal(metadata.preview.abilities[0].name, 'Frozen Blade')
  assert.equal(metadata.preview.wrightstone.name, 'Sequestration Wrightstone')
  assert.equal(metadata.preview.summons[0].name, 'Rolan · Legendary · Special')
  assert.equal(metadata.preview.summons[0].subParam, 'Attack DMG Cap (High · Max 100%)')
})

test('DLC factor item and primary hashes never leak Chinese into English pages', async () => {
  const env = { LOADOUTS: makeR2() }
  const preview = Buffer.from(JSON.stringify({
    sigils: [
      { hash: '04BD9F6B', name: '浪迹天涯 + 协同攻击', primaryHash: 'D029FE08', secondaryHash: '3FEC5F80', secondary: '协同攻击' },
      { hash: '332E9B30', name: '狂战士+', primaryHash: 'EE85CD1F', secondaryHash: '05F2ECDC', secondary: '怒涛' },
      { hash: '938DB625', name: '斯巴达+', primaryHash: '3D8153A1', secondaryHash: '6085DA25', secondary: '自愈' },
    ],
  })).toString('base64url')
  const published = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Preview': preview }, body: makeFrame(),
  }), env)
  const result = await published.json()
  const metadata = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${result.compactCode}/meta?lang=en`), env).then(response => response.json())
  assert.deepEqual(metadata.preview.sigils.map(item => item.name), ['Fatebreaker V+', 'Berserker Echo+', 'Spartan Echo+'])
  assert.deepEqual(metadata.preview.sigils.map(item => item.primary), ['Fatebreaker', 'Berserker Echo', 'Spartan Echo'])
  assert.doesNotMatch(JSON.stringify(metadata.preview.sigils), /[\u3400-\u9fff]/u)
})

test('catalog and character routes expose all 29 unique character pages without portrait panels', async () => {
  const env = { LOADOUTS: makeR2() }
  const catalog = await worker.fetch(new Request('https://share.example/'), env)
  assert.equal(catalog.status, 200)
  const catalogHtml = await catalog.text()
  assert.equal((catalogHtml.match(/href="\/c\//g) || []).length, 29)
  assert.match(catalogHtml, /class="roster-all active" href="\/\?lang=zh" aria-current="page"/)
  assert.match(catalogHtml, /搜索角色、武器、因子、祝福、技能、专精/)
  assert.match(catalogHtml, /event\.key==='Enter'/)
  const page = await worker.fetch(new Request('https://share.example/c/fediel'), env)
  assert.equal(page.status, 200)
  const pageHtml = await page.text()
  assert.match(pageHtml, /菲迪埃尔/)
  assert.match(pageHtml, /assets\/avatars\/cmn_mini_s_pl2900\.png/)
  assert.match(pageHtml, /class="roster-all" href="\/\?lang=zh"/)
  assert.match(pageHtml, /搜索此角色的武器、因子、祝福、技能或专精/)
  assert.match(pageHtml, /if\(fixedCharacter\)params\.set\('character',fixedCharacter\)/)
  assert.match(pageHtml, /scrollIntoView\(\{block:'nearest',inline:'center'\}\)/)
  assert.doesNotMatch(pageHtml, /Fediel|CHARACTER LOADOUTS/)
  assert.doesNotMatch(pageHtml, /assets\/characters\//)
})

test('detail actions share one fixed button box and old previews inherit trait levels', async () => {
  const env = { LOADOUTS: makeR2() }
  const frame = makeFrame()
  const preview = Buffer.from(JSON.stringify({
    characterName: '伊欧',
    sigils: [{ name: '体力 V+', level: 15, primary: '体力', secondary: '伤害上限' }],
  })).toString('base64url')
  const published = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Preview': preview }, body: frame,
  }), env)
  const result = await published.json()
  const page = await worker.fetch(new Request(`https://share.example/s/${result.compactCode}`), env)
  const html = await page.text()
  assert.match(html, /\.button\{display:inline-flex;align-items:center;justify-content:center;box-sizing:border-box;/)
  assert.match(html, /\.roster-bar\{--roster-item-height:73px;/)
  assert.match(html, /\.roster-all\{display:grid;height:var\(--roster-item-height\);/)
  assert.match(html, /\.roster-chip\{flex:0 0 70px;display:grid;height:var\(--roster-item-height\);/)
  assert.match(html, /const secondaryLevel=s\.secondary\?\(Number\(s\.secondaryLevel\)\|\|primaryLevel\):0/)

  const meta = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${result.compactCode}/meta`), env)
  assert.equal((await meta.json()).preview.sigils[0].secondaryLevel, 15)
})

test('character names, routes, hashes, and avatars stay on the same roster record', async () => {
  const { CHARACTER_ROSTER, characterByIdentity } = await import('../src/roster.js')
  assert.equal(CHARACTER_ROSTER.length, 29)
  assert.equal(new Set(CHARACTER_ROSTER.map(character => character.slug)).size, 29)
  assert.equal(new Set(CHARACTER_ROSTER.map(character => character.plId)).size, 29)
  assert.equal(new Set(CHARACTER_ROSTER.map(character => character.hash)).size, 29)

  const expected = {
    ferry: 'pl0700', lancelot: 'pl0800', zeta: 'pl1600',
    sandalphon: 'pl2100', seofon: 'pl2200', tweyen: 'pl2300', fediel: 'pl2900',
  }
  for (const [slug, plId] of Object.entries(expected)) {
    const character = CHARACTER_ROSTER.find(item => item.slug === slug)
    assert.equal(character.plId, plId)
    assert.equal(character.iconFile, `cmn_mini_s_${plId}.png`)
    assert.equal(existsSync(new URL(`../public/assets/avatars/${character.iconFile}`, import.meta.url)), true)
  }

  assert.equal(characterByIdentity('塞达').slug, 'zeta')
  assert.equal(characterByIdentity('', '0D21B430').slug, 'zeta')
})
