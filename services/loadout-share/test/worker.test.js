import test from 'node:test'
import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import { DatabaseSync } from 'node:sqlite'
import { decode } from '@msgpack/msgpack'
import worker, {
  displayCode,
  buildD1CatalogQuery,
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
  let getCalls = 0
  let listCalls = 0
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
      getCalls += 1
      return wrap(objects.get(key))
    },
    async put(key, value, options) {
      objects.set(key, {
        bytes: typeof value === 'string' ? new TextEncoder().encode(value) : new Uint8Array(value),
        customMetadata: options.customMetadata,
      })
    },
    async list(options = {}) {
      listCalls += 1
      const prefix = options.prefix || ''
      const keys = [...objects.keys()].filter(key => key.startsWith(prefix)).sort()
      const start = Math.max(0, Number(options.cursor) || 0)
      const limit = Number.isFinite(options.limit) ? Math.max(0, options.limit) : keys.length
      const selected = keys.slice(start, start + limit)
      const next = start + selected.length
      return { objects: selected.map(key => ({ key })), truncated: next < keys.length, cursor: next < keys.length ? String(next) : undefined }
    },
    getCalls: () => getCalls,
    listCalls: () => listCalls,
  }
}

function makeCommunityDB({ failCatalog = false, failWrites = false } = {}) {
  const loadouts = new Map()
  const likes = new Set()
  const comments = []
  let catalogQueries = 0
  return {
    loadouts,
    likes,
    comments,
    catalogQueries: () => catalogQueries,
    prepare(sql) {
      let values = []
      const statement = {
        bind(...args) { values = args; return statement },
        async run() {
          if (failWrites && (sql.startsWith('INSERT INTO loadouts') || sql.startsWith('INSERT OR IGNORE INTO loadouts'))) throw new Error('simulated D1 write outage')
          if (sql.startsWith('INSERT INTO loadouts') || sql.startsWith('INSERT OR IGNORE INTO loadouts')) {
            const [code, title, characterName, characterHash, createdAt, characterSlug = '', weaponName = '', weaponNameEn = '', searchText = '', searchTextEn = '', previewJSON = '{}', previewEnJSON = '{}', catalogReady = 0, updatedAt = createdAt, titleSort = title] = values
            const previous = loadouts.get(code)
            loadouts.set(code, {
              code, title, character_name: characterName, character_hash: characterHash, created_at: previous?.created_at || createdAt,
              likes_count: previous?.likes_count || 0, character_slug: characterSlug, weapon_name: weaponName, weapon_name_en: weaponNameEn,
              search_text: searchText, search_text_en: searchTextEn, preview_json: previewJSON, preview_en_json: previewEnJSON,
              catalog_ready: catalogReady, updated_at: updatedAt, title_sort: titleSort,
            })
          } else if (sql.startsWith('INSERT OR IGNORE INTO likes')) {
            likes.add(`${values[0]}\u0000${values[1]}`)
          } else if (sql.startsWith('DELETE FROM likes')) {
            likes.delete(`${values[0]}\u0000${values[1]}`)
          } else if (sql.startsWith('UPDATE loadouts SET likes_count')) {
            const code = values[1]
            const entry = loadouts.get(code)
            if (entry) entry.likes_count = [...likes].filter(value => value.startsWith(`${code}\u0000`)).length
          } else if (sql.startsWith('INSERT INTO comments')) {
            comments.push({ code: values[0], author: values[1], body: values[2], visitor_key: values[3], created_at: values[4], deleted: 0 })
          }
          return { success: true }
        },
        async first() {
          if (sql.startsWith('SELECT likes_count FROM loadouts')) return loadouts.get(values[0]) || null
          return null
        },
        async all() {
          if (sql.startsWith('SELECT /* catalog-v2 */')) {
            catalogQueries += 1
            if (failCatalog) throw new Error('simulated D1 catalog outage')
            const sort = sql.includes('ORDER BY title_sort ASC') ? 'name' : sql.includes('ORDER BY likes_count DESC') ? 'likes' : 'time'
            let offset = 0
            const character = sql.includes('character_slug = ?') ? values[offset++] : ''
            const englishSearch = sql.includes('search_text_en LIKE ?')
            const hasSearch = englishSearch || sql.includes('search_text LIKE ?')
            const query = hasSearch ? String(values[offset++]).replace(/^%|%$/g, '').replace(/\\([\\%_])/g, '$1') : ''
            const hasCursor = /AND \((?:created_at|title_sort|likes_count) [<>] \?/.test(sql)
            const cursor = hasCursor ? { value: values[offset], code: values[offset + 2] } : null
            const settings = {
              character, query, englishSearch, sort, cursor,
              limit: Number(values.at(-1)) - 1,
            }
            let rows = [...loadouts.values()].filter(row => row.catalog_ready === 1)
            if (settings.character) rows = rows.filter(row => row.character_slug === settings.character)
            if (settings.query) rows = rows.filter(row => row[settings.englishSearch ? 'search_text_en' : 'search_text'].includes(settings.query))
            if (settings.sort === 'name') rows.sort((a, b) => a.title_sort.localeCompare(b.title_sort) || a.code.localeCompare(b.code))
            else if (settings.sort === 'likes') rows.sort((a, b) => b.likes_count - a.likes_count || a.code.localeCompare(b.code))
            else rows.sort((a, b) => b.created_at.localeCompare(a.created_at) || b.code.localeCompare(a.code))
            if (settings.cursor) {
              const cursor = settings.cursor
              rows = rows.filter(row => {
                if (settings.sort === 'name') return row.title_sort > cursor.value || (row.title_sort === cursor.value && row.code > cursor.code)
                if (settings.sort === 'likes') return row.likes_count < cursor.value || (row.likes_count === cursor.value && row.code > cursor.code)
                return row.created_at < cursor.value || (row.created_at === cursor.value && row.code < cursor.code)
              })
            }
            return { results: rows.slice(0, settings.limit + 1) }
          }
          if (sql.startsWith('SELECT code, likes_count FROM loadouts WHERE code IN')) {
            catalogQueries += 1
            return { results: values.map(code => loadouts.get(code)).filter(Boolean).map(({ code, likes_count }) => ({ code, likes_count })) }
          }
          if (sql.startsWith('SELECT id, author, body, created_at FROM comments')) {
            return { results: comments.filter(item => item.code === values[0] && item.deleted === 0).map((item, index) => ({ id: index + 1, author: item.author, body: item.body, created_at: item.created_at })) }
          }
          return { results: [] }
        },
      }
      return statement
    },
  }
}

async function publishCatalogPreview(env, { title = '目录测试', characterName = '伊欧', characterHash = '4D0A60C3', weaponName = '星晶武器', sigilName = '快速冷却 V+' } = {}) {
  const preview = Buffer.from(JSON.stringify({
    characterName, characterHash, weaponHash: '02352554', weaponName,
    sigils: [{ name: sigilName, level: 15, primaryHash: '318D12E9', primary: '快速冷却', primaryLevel: 15 }],
  })).toString('base64url')
  return worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Title-B64': Buffer.from(title).toString('base64'), 'X-Loadout-Preview': preview }, body: makeFrame(crypto.getRandomValues(new Uint8Array(8))),
  }), env).then(response => response.json())
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

test('partial v11 runtime JSON uploads only captured scopes', async () => {
  let packed
  const source = {
    format: 'gbfr-loadout', version: 11, charaHash: '0D21B430', charaName: '泽塔', ownerCode: 'PL1600', name: '实时毕业配装',
    weaponHash: '02352554', weaponName: '阿尔贝斯之枪',
    sigils: [{ index: 0, hash: '2D7F2E70', name: '攻击力 V+', level: 15, primaryTraitHash: '50079A1C', primaryTraitLevel: 15, secondaryTraitHash: 'DC584F60', secondaryTraitLevel: 15 }],
    weaponSkillHashes: ['7EDD69D0', '887AE0B0', '887AE0B0', '887AE0B0', '020DB733'],
    weapon: { storedHash: '02352554', xp: 162540, uncap: 6, mirage: 99, awakening: 10, transcendence: 7, skillHashes: ['7EDD69D0', '887AE0B0', '887AE0B0', '887AE0B0', '020DB733'] },
    overLimit: Array.from({ length: 4 }, (_, index) => ({ index, attributeHash: index === 0 ? '52A207B5' : '', level: index === 0 ? 10 : 0 })),
    sourceKind: 'runtime', capturedFields: ['stats', 'sigils', 'weapon', 'weaponSkills', 'overLimit'], progressionPolicy: 'endgame-max',
  }
  const result = await loadoutJSONToFrame(JSON.stringify(source), bytes => { packed = bytes; return new Uint8Array([1, 2, 3]) })
  const wire = decode(packed)
  assert.equal(wire[0], 11)
  assert.equal(wire[8].length, 0)
  assert.equal(wire[9].length, 0)
  assert.equal(wire[11].length, 0)
  assert.equal(wire[12], null)
  assert.equal(wire[13][2], 6)
  assert.equal(wire[15], 'runtime')
  assert.deepEqual(wire[16], source.capturedFields)
  assert.equal(result.preview.weaponName, '阿尔贝斯之枪')
})

test('partial v11 JSON accepts a captured weapon without sigils', async () => {
  const source = {
    format: 'gbfr-loadout', version: 11, charaHash: '0D21B430', charaName: '泽塔', ownerCode: 'PL1600', name: '武器记录',
    weaponHash: '02352554', weaponName: '阿尔贝斯之枪', sigils: [],
    weapon: { storedHash: '02352554', xp: 162540, uncap: 6, mirage: 99, awakening: 10, transcendence: 7, skillHashes: [] },
    sourceKind: 'logs-db', capturedFields: ['weapon'], progressionPolicy: 'endgame-max',
  }
  let packed
  await loadoutJSONToFrame(JSON.stringify(source), bytes => { packed = bytes; return new Uint8Array([1, 2, 3]) })
  const wire = decode(packed)
  assert.equal(wire[7].length, 0)
  assert.equal(wire[13][0], 0x02352554)
  assert.equal(wire[13][10].length, 0)
  assert.deepEqual(wire[16], ['weapon'])
})

test('partial v11 JSON rejects unknown provenance and undeclared payloads', async () => {
  const base = {
    format: 'gbfr-loadout', version: 11, charaHash: '0D21B430', charaName: '泽塔', ownerCode: 'PL1600', name: '实时配装',
    sigils: [{ index: 0, hash: '2D7F2E70', name: '攻击力 V+', level: 15, primaryTraitHash: '50079A1C', primaryTraitLevel: 15 }],
    sourceKind: 'runtime', capturedFields: ['sigils'], progressionPolicy: 'endgame-max',
  }
  for (const mutate of [
    source => { source.sourceKind = 'other' },
    source => { source.progressionPolicy = 'exact' },
    source => { source.capturedFields = ['sigils', 'sigils'] },
    source => { source.skills = [{ hash: '12345678', name: '测试', key: 'test' }] },
    source => { source.weaponHash = '02352554' },
    source => { source.capturedFields = ['sigils', 'weapon'] },
    source => { source.capturedFields = ['sigils', 'weaponSkills']; source.weaponSkillHashes = Array(5).fill('887AE0B0') },
    source => { source.capturedFields = ['sigils', 'wrightstone'] },
    source => { source.capturedFields = ['sigils', 'weapon']; source.weapon = { storedHash: '02352554', xp: 0, uncap: 0, mirage: 0, awakening: 0, transcendence: 0, skillHashes: ['7EDD69D0'] } },
    source => { source.capturedFields = ['sigils', 'weapon']; source.weapon = { storedHash: '02352554', xp: 0, uncap: 0, mirage: 0, awakening: 0, transcendence: 0, skillHashes: [], wrightstoneReference: '09E6F629' } },
  ]) {
    const source = structuredClone(base)
    mutate(source)
    await assert.rejects(() => loadoutJSONToFrame(JSON.stringify(source)), /来源|策略|捕获字段|未声明|武器/)
  }
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

  const catalog = await worker.fetch(new Request('https://share.example/api/v1/loadouts'), env).then(response => response.json())
  assert.deepEqual(catalog.items, [], 'a frame without a valid preview must not enter the public catalog')
})

test('12-character legacy prefixes resolve only when exactly one stored frame matches', async () => {
  const r2 = makeR2()
  const env = { LOADOUTS: r2 }
  const frame = makeFrame()
  const published = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST', headers: { 'Content-Type': 'application/octet-stream' }, body: frame,
  }), env).then(response => response.json())
  const prefix = published.compactCode.slice(0, 12)

  const loaded = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${displayCode(prefix)}`), env)
  assert.equal(loaded.status, 200)
  assert.deepEqual(new Uint8Array(await loaded.arrayBuffer()), frame)
  assert.equal((await worker.fetch(new Request(`https://share.example/s/${prefix}`), env)).status, 200)
  assert.equal((await worker.fetch(new Request(`https://share.example/download/${prefix}.gbfr-loadout`), env)).status, 200)

  const collisionCode = prefix + (published.compactCode.endsWith('0000') ? '1111' : '0000')
  await r2.put(`v1/${collisionCode}`, frame, { customMetadata: { sha256: 'collision', protocol: 'GBLC1' } })
  const ambiguous = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${prefix}`), env)
  assert.equal(ambiguous.status, 409)
  assert.match((await ambiguous.json()).error, /完整 16 位短码/)
})

test('catalog cards show and update deduplicated likes without opening the detail link', async () => {
  const community = makeCommunityDB()
  const env = { LOADOUTS: makeR2(), COMMUNITY_DB: community }
  const preview = Buffer.from(JSON.stringify({ characterHash: '4D0A60C3', characterName: '伊欧', weaponName: '星晶武器', sigils: [{ name: '迅捷能力 V+' }] })).toString('base64url')
  const published = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Preview': preview }, body: makeFrame(),
  }), env).then(response => response.json())

  const firstCatalog = await worker.fetch(new Request('https://share.example/api/v1/loadouts'), env).then(response => response.json())
  assert.equal(firstCatalog.items[0].likes, 0)
  assert.deepEqual(Object.keys(firstCatalog.items[0].preview).sort(), ['masteryCount', 'masteryLabel', 'sigils', 'weaponSkills'])
  assert.equal(community.catalogQueries(), 1, 'one catalog page should use one batched like query')

  const like = async (visitorKey, liked) => worker.fetch(new Request(`https://share.example/api/v1/loadouts/${published.compactCode}/like`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ visitorKey, liked }),
    }), env)
  assert.equal((await like('visitor-alpha', true)).status, 200)
  assert.equal((await like('visitor-alpha', true)).status, 200)
  assert.equal((await like('visitor-beta', true)).status, 200)
  const removed = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${published.compactCode}/like`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ visitorKey: 'visitor-alpha', liked: false }),
  }), env).then(response => response.json())
  assert.equal(removed.liked, false)
  assert.equal(community.likes.size, 1, 'a second click must remove the same anonymous visitor\'s like')
  const refreshed = await worker.fetch(new Request('https://share.example/api/v1/loadouts'), env).then(response => response.json())
  assert.equal(refreshed.items[0].likes, 1)

  const page = await worker.fetch(new Request('https://share.example/'), env).then(response => response.text())
  assert.match(page, /rel="icon" href="\/favicon\.ico\?v=2"/)
  assert.match(page, /<article class="loadout-card"/)
  assert.match(page, /class="loadout-card-main"/)
  assert.match(page, /class="card-like(?:'|\+)/)
  assert.match(page, /event\.preventDefault\(\);event\.stopPropagation\(\);likeCard\(button\)/)
  assert.match(page, /aria-pressed="'\+liked\+'"/)
  assert.match(page, /GitHub 下载应用/)
  assert.match(page, /github\.com\/Whitelinker574\/GBFR-PE-Patch-Tool\/releases\/latest/)
  assert.match(page, /content-visibility:auto/)
  assert.match(page, /loading="lazy" decoding="async"/)
  assert.match(page, /<select id="sort"/)
  assert.match(page, /<option value="time">最新<\/option>/)
  assert.match(page, /<option value="name">名称<\/option>/)
  assert.match(page, /<option value="likes">点赞<\/option>/)
  assert.match(page, /params\.set\('sort',document\.querySelector\('#sort'\)\?\.value\|\|'time'\)/)
  assert.match(page, /document\.querySelector\('#sort'\)\?\.addEventListener\('change',load\)/)
  assert.match(page, /let nextCursor=''/)
  assert.match(page, /if\(more&&nextCursor\)params\.set\('cursor',nextCursor\)/)
  assert.match(page, /document\.querySelector\('#load-more'\)\?\.addEventListener\('click',\(\)=>load\(true\)\)/)
  assert.match(page, /id="load-more"/)
  assert.match(page, /Date\.parse\(b\.createdAt\|\|0\)-Date\.parse\(a\.createdAt\|\|0\)/)
  assert.match(page, /hint\.rel='prefetch';hint\.href=link\.href/)
  for (const [, script] of page.matchAll(/<script>([\s\S]*?)<\/script>/g)) {
    assert.doesNotThrow(() => new Function(script), 'every catalog inline script must remain valid JavaScript')
  }
})

test('community mutations reject non-JSON and bodies above the 2 KiB boundary before parsing', async () => {
  const community = makeCommunityDB()
  const env = { LOADOUTS: makeR2(), COMMUNITY_DB: community }
  const published = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/octet-stream' },
    body: makeFrame(),
  }), env).then(response => response.json())
  const endpoint = `https://share.example/api/v1/loadouts/${published.compactCode}/comments`

  const wrongType = await worker.fetch(new Request(endpoint, {
    method: 'POST',
    headers: { 'Content-Type': 'text/plain' },
    body: JSON.stringify({ visitorKey: 'visitor-alpha', body: 'hello' }),
  }), env)
  assert.equal(wrongType.status, 415)

  const oversized = await worker.fetch(new Request(endpoint, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ visitorKey: 'visitor-alpha', body: 'x'.repeat(3 * 1024) }),
  }), env)
  assert.equal(oversized.status, 413)
  assert.match((await oversized.json()).error, /2 KiB/)
  assert.equal(community.comments.length, 0)
})

test('landing page escapes untrusted titles before server-side HTML rendering', async () => {
  const env = { LOADOUTS: makeR2() }
  const payload = '</title><img src=x onerror=alert(document.domain)>'
  const published = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Title': payload },
    body: makeFrame(),
  }), env).then(response => response.json())
  const html = await worker.fetch(new Request(`https://share.example/s/${published.compactCode}`), env).then(response => response.text())
  assert.equal(html.includes(payload), false)
  assert.match(html, /&lt;\/title&gt;&lt;img src=x onerror=alert\(document\.domain\)&gt;/)
})

test('catalog search returns a bounded globally ordered R2 page and stable cursor', async () => {
  const r2 = makeR2()
  const env = { LOADOUTS: r2 }
  for (let index = 0; index < 60; index += 1) {
    const code = index.toString(32).toUpperCase().padStart(16, '0')
    const metadata = {
      code,
      title: `搜索配装 ${index}`,
      characterHash: '4D0A60C3',
      characterName: '伊欧',
      preview: { characterHash: '4D0A60C3', characterName: '伊欧', weaponName: '星晶武器', sigils: [{ name: '快速冷却 V+' }] },
    }
    await r2.put(`meta/v1/${code}.json`, JSON.stringify(metadata), { customMetadata: { sha256: code } })
  }

  const result = await worker.fetch(new Request('https://share.example/api/v1/loadouts?q=快速冷却&limit=12'), env).then(response => response.json())
  assert.equal(result.items.length, 12)
  assert.equal(result.truncated, true)
  assert.ok(result.cursor)
  const second = await worker.fetch(new Request(`https://share.example/api/v1/loadouts?q=快速冷却&limit=12&cursor=${encodeURIComponent(result.cursor)}`), env).then(response => response.json())
  assert.equal(second.items.length, 12)
  assert.equal(new Set([...result.items, ...second.items].map(item => item.code)).size, 24)
  assert.equal(result.items[0].code > result.items.at(-1).code, true)
})

test('filtered catalogs scan beyond the first R2 page for Io and DLC characters', async () => {
  const r2 = makeR2()
  const env = { LOADOUTS: r2 }
  for (let index = 0; index < 30; index += 1) {
    const code = index.toString(32).toUpperCase().padStart(16, '0')
    await r2.put(`meta/v1/${code}.json`, JSON.stringify({
      code, title: `古兰配装 ${index}`, characterHash: '2A26B1B2', characterName: '古兰',
      preview: { characterHash: '2A26B1B2', characterName: '古兰', weaponName: '旅行者之剑', sigils: [{ name: '攻击力 V+' }] },
    }), { customMetadata: { sha256: code } })
  }
  await r2.put('meta/v1/ZZZZZZZZZZZZZZZY.json', JSON.stringify({
    code: 'ZZZZZZZZZZZZZZZY', title: '伊欧常规毕业配装', characterHash: '4D0A60C3', characterName: '伊欧',
    preview: { characterHash: '4D0A60C3', characterName: '伊欧', weaponName: '星晶武器', sigils: [{ name: '迅捷能力 V+' }] },
  }), { customMetadata: { sha256: 'io' } })
  await r2.put('meta/v1/ZZZZZZZZZZZZZZYX.json', JSON.stringify({
    code: 'ZZZZZZZZZZZZZZYX', title: '卡莉奥丝特罗配装', characterHash: 'E7053919', characterName: '卡莉奥丝特罗',
    preview: { characterHash: 'E7053919', characterName: '卡莉奥丝特罗', weaponName: '真典', sigils: [{ name: '迅捷能力 V+' }] },
  }), { customMetadata: { sha256: 'cagliostro' } })
  await r2.put('meta/v1/ZZZZZZZZZZZZZZZZ.json', JSON.stringify({
    code: 'ZZZZZZZZZZZZZZZZ', title: '菲迪埃尔常规毕业配装', characterHash: '74DD4C79', characterName: '菲迪埃尔',
    preview: { characterHash: '74DD4C79', characterName: '菲迪埃尔', weaponName: 'DLC 武器', sigils: [{ name: '伤害上限 V+' }] },
  }), { customMetadata: { sha256: 'dlc' } })

  const io = await worker.fetch(new Request('https://share.example/api/v1/loadouts?character=io&limit=24'), env).then(response => response.json())
  assert.equal(io.items.length, 1)
  assert.equal(io.items[0].title, '伊欧常规毕业配装')
  const dlc = await worker.fetch(new Request('https://share.example/api/v1/loadouts?character=%E8%8F%B2%E8%BF%AA%E5%9F%83%E5%B0%94&limit=24'), env).then(response => response.json())
  assert.equal(dlc.items.length, 1)
  assert.equal(dlc.items[0].title, '菲迪埃尔常规毕业配装')
})

test('D1 is the primary catalog index and publishing dual-writes searchable card data', async () => {
  const r2 = makeR2()
  const community = makeCommunityDB()
  const env = { LOADOUTS: r2, COMMUNITY_DB: community }
  const published = await publishCatalogPreview(env, { title: '伊欧快速冷却毕业配装' })

  const indexed = community.loadouts.get(published.compactCode)
  assert.equal(indexed.catalog_ready, 1)
  assert.equal(indexed.character_slug, 'io')
  assert.equal(indexed.weapon_name, '[绝霸]布里欧纳克')
  assert.match(indexed.search_text, /快速冷却/)
  assert.equal(JSON.parse(indexed.preview_json).sigils[0].name, '快速冷却 V+')

  const catalog = await worker.fetch(new Request('https://share.example/api/v1/loadouts?character=io&q=快速冷却'), env).then(response => response.json())
  assert.equal(catalog.items.length, 1)
  assert.equal(catalog.items[0].code, published.compactCode)
  assert.equal(r2.listCalls(), 0, 'healthy D1 catalog reads must not scan R2 metadata')
})

test('publishing reports a retryable failure until the configured D1 catalog accepts the dual write', async () => {
  const r2 = makeR2()
  const preview = Buffer.from(JSON.stringify({
    characterName: '伊欧', characterHash: '4D0A60C3', weaponHash: '02352554', weaponName: '星晶武器',
    sigils: [{ name: '快速冷却 V+', primaryHash: '318D12E9', primary: '快速冷却', primaryLevel: 15 }],
  })).toString('base64url')
  const frame = makeFrame(new Uint8Array([9, 8, 7, 6]))
  const request = () => new Request('https://share.example/api/v1/loadouts', {
    method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Title-B64': Buffer.from('双写重试').toString('base64'), 'X-Loadout-Preview': preview }, body: frame,
  })
  const failed = await worker.fetch(request(), { LOADOUTS: r2, COMMUNITY_DB: makeCommunityDB({ failWrites: true }) })
  assert.equal(failed.status, 503)
  assert.equal((await failed.json()).retryable, true)

  const healthy = makeCommunityDB()
  const retried = await worker.fetch(request(), { LOADOUTS: r2, COMMUNITY_DB: healthy })
  assert.equal(retried.status, 200)
  const published = await retried.json()
  assert.equal(healthy.loadouts.get(published.compactCode).catalog_ready, 1)
})

test('catalog automatically falls back to R2 when the D1 index is unavailable', async () => {
  const r2 = makeR2()
  const healthy = makeCommunityDB()
  const published = await publishCatalogPreview({ LOADOUTS: r2, COMMUNITY_DB: healthy }, { title: '回退测试配装' })
  const failing = makeCommunityDB({ failCatalog: true })
  const result = await worker.fetch(new Request('https://share.example/api/v1/loadouts?q=回退测试'), { LOADOUTS: r2, COMMUNITY_DB: failing }).then(response => response.json())

  assert.equal(result.items[0].code, published.compactCode)
  assert.equal(result.indexSource, 'r2-fallback')
  assert.ok(r2.listCalls() > 0)
})

test('R2 fallback preserves global time, name, and likes ordering with D1-compatible cursors', async () => {
  const r2 = makeR2()
  const healthy = makeCommunityDB()
  const env = { LOADOUTS: r2, COMMUNITY_DB: healthy }
  const published = []
  for (const [index, title] of ['乙方案', '甲方案', '丙方案'].entries()) {
    const item = await publishCatalogPreview(env, { title })
    const metadataKey = `meta/v1/${item.compactCode}.json`
    const stored = r2.objects.get(metadataKey)
    const metadata = JSON.parse(new TextDecoder().decode(stored.bytes))
    metadata.createdAt = `2026-07-2${index + 1}T00:00:00.000Z`
    stored.bytes = new TextEncoder().encode(JSON.stringify(metadata))
    const row = healthy.loadouts.get(item.compactCode)
    row.created_at = metadata.createdAt
    row.likes_count = [3, 9, 5][index]
    published.push(item.compactCode)
  }
  const failing = makeCommunityDB({ failCatalog: true })
  for (const code of published) failing.loadouts.set(code, healthy.loadouts.get(code))
  const expected = {
    time: [...published].sort((left, right) => healthy.loadouts.get(right).created_at.localeCompare(healthy.loadouts.get(left).created_at) || right.localeCompare(left)),
    name: [...published].sort((left, right) => {
      const leftTitle = healthy.loadouts.get(left).title_sort
      const rightTitle = healthy.loadouts.get(right).title_sort
      return leftTitle === rightTitle ? left.localeCompare(right) : leftTitle < rightTitle ? -1 : 1
    }),
    likes: [...published].sort((left, right) => healthy.loadouts.get(right).likes_count - healthy.loadouts.get(left).likes_count || left.localeCompare(right)),
  }
  for (const sort of ['time', 'name', 'likes']) {
    const first = await worker.fetch(new Request(`https://share.example/api/v1/loadouts?sort=${sort}&limit=1`), { LOADOUTS: r2, COMMUNITY_DB: failing }).then(response => response.json())
    const second = await worker.fetch(new Request(`https://share.example/api/v1/loadouts?sort=${sort}&limit=1&cursor=${encodeURIComponent(first.cursor)}`), { LOADOUTS: r2, COMMUNITY_DB: failing }).then(response => response.json())
    assert.deepEqual([first.items[0].code, second.items[0].code], expected[sort].slice(0, 2), sort)
    assert.equal(first.indexSource, 'r2-fallback')
  }
})

test('R2 catalog fails closed for likes ordering when community counts are unavailable', async () => {
  const r2 = makeR2()
  await publishCatalogPreview({ LOADOUTS: r2 }, { title: '无互动索引配装' })
  const response = await worker.fetch(new Request('https://share.example/api/v1/loadouts?sort=likes'), { LOADOUTS: r2 })
  assert.equal(response.status, 503)
  assert.match((await response.json()).error, /点赞排序暂时不可用/)
})

test('R2 fallback returns an explicit failure instead of a partial globally sorted page past the proof bound', async () => {
  const r2 = makeR2()
  for (let index = 0; index <= 1000; index += 1) {
    const code = index.toString(32).toUpperCase().padStart(16, '0')
    await r2.put(`meta/v1/${code}.json`, JSON.stringify({
      code,
      title: `全局排序边界 ${index}`,
      characterHash: '4D0A60C3',
      characterName: '伊欧',
      preview: {
        characterHash: '4D0A60C3',
        characterName: '伊欧',
        weaponName: '星晶武器',
        sigils: [{ name: '快速冷却 V+' }],
      },
    }), { customMetadata: { sha256: code } })
  }
  const response = await worker.fetch(
    new Request('https://share.example/api/v1/loadouts?sort=time&limit=24'),
    { LOADOUTS: r2, COMMUNITY_DB: makeCommunityDB({ failCatalog: true }) },
  )
  assert.equal(response.status, 503)
  const result = await response.json()
  assert.match(result.error, /超过 1000 条安全扫描上限/)
  assert.match(result.error, /无法证明全局排序/)
  assert.equal(result.items, undefined)
})

test('D1 catalog cursors remain stable for time, name, and likes ordering', async () => {
  const r2 = makeR2()
  const community = makeCommunityDB()
  const env = { LOADOUTS: r2, COMMUNITY_DB: community }
  const titles = ['乙方案', '甲方案', '丙方案']
  for (let index = 0; index < titles.length; index += 1) {
    const result = await publishCatalogPreview(env, { title: titles[index] })
    const row = community.loadouts.get(result.compactCode)
    row.created_at = `2026-07-2${index + 1}T00:00:00.000Z`
    row.likes_count = index === 0 ? 8 : index
  }
  for (const sort of ['time', 'name', 'likes']) {
    const first = await worker.fetch(new Request(`https://share.example/api/v1/loadouts?sort=${sort}&limit=1`), env).then(response => response.json())
    const second = await worker.fetch(new Request(`https://share.example/api/v1/loadouts?sort=${sort}&limit=1&cursor=${encodeURIComponent(first.cursor)}`), env).then(response => response.json())
    assert.equal(first.items.length, 1)
    assert.equal(second.items.length, 1)
    assert.notEqual(first.items[0].code, second.items[0].code)
  }
  assert.equal(r2.listCalls(), 0)
})

test('catalog backfill is token-protected and migrates one bounded R2 page into D1', async () => {
  const r2 = makeR2()
  const community = makeCommunityDB()
  const published = await publishCatalogPreview({ LOADOUTS: r2 }, { title: '待回填配装' })
  const env = { LOADOUTS: r2, COMMUNITY_DB: community, CATALOG_ADMIN_TOKEN: 'test-maintainer-token' }
  const denied = await worker.fetch(new Request('https://share.example/api/internal/catalog/backfill', { method: 'POST' }), env)
  assert.equal(denied.status, 403)
  const migrated = await worker.fetch(new Request('https://share.example/api/internal/catalog/backfill?limit=10', {
    method: 'POST', headers: { Authorization: 'Bearer test-maintainer-token' },
  }), env).then(response => response.json())
  assert.equal(migrated.indexed, 1)
  assert.equal(community.loadouts.get(published.compactCode).catalog_ready, 1)
})

test('catalog backfill reports D1 write failures instead of claiming rows were indexed', async () => {
  const r2 = makeR2()
  await publishCatalogPreview({ LOADOUTS: r2 }, { title: '回填失败样本' })
  const response = await worker.fetch(new Request('https://share.example/api/internal/catalog/backfill', {
    method: 'POST', headers: { Authorization: 'Bearer test-maintainer-token' },
  }), { LOADOUTS: r2, COMMUNITY_DB: makeCommunityDB({ failWrites: true }), CATALOG_ADMIN_TOKEN: 'test-maintainer-token' })
  assert.equal(response.status, 503)
  const result = await response.json()
  assert.equal(result.indexed, 0)
  assert.equal(result.failed, 1)
})

test('D1 catalog stores separate Chinese and English summaries and search text', async () => {
  const r2 = makeR2()
  const community = makeCommunityDB()
  const env = { LOADOUTS: r2, COMMUNITY_DB: community }
  await publishCatalogPreview(env, { title: '双语目录', sigilName: '快速冷却 V+' })
  const english = await worker.fetch(new Request('https://share.example/api/v1/loadouts?lang=en&q=Quick%20Cooldown'), env).then(response => response.json())
  assert.equal(english.items.length, 1)
  assert.equal(english.items[0].preview.sigils[0].name, 'Quick Cooldown V+')
  assert.equal(english.items[0].weaponName, 'Brionac')
})

test('D1 catalog rejects malformed and cross-sort cursors', async () => {
  const r2 = makeR2()
  const community = makeCommunityDB()
  const env = { LOADOUTS: r2, COMMUNITY_DB: community }
  await publishCatalogPreview(env)
  await publishCatalogPreview(env, { title: '第二套游标样本' })
  const first = await worker.fetch(new Request('https://share.example/api/v1/loadouts?sort=time&limit=1'), env).then(response => response.json())
  const malformed = await worker.fetch(new Request('https://share.example/api/v1/loadouts?cursor=not-a-cursor'), env)
  assert.equal(malformed.status, 400)
  if (first.cursor) {
    const crossSort = await worker.fetch(new Request(`https://share.example/api/v1/loadouts?sort=name&cursor=${encodeURIComponent(first.cursor)}`), env)
    assert.equal(crossSort.status, 400)
  }
})

test('D1 catalog query plans use the matching global and character sort indexes', () => {
  const db = new DatabaseSync(':memory:')
  db.exec(readFileSync(new URL('../migrations/0001_community.sql', import.meta.url), 'utf8'))
  db.exec(readFileSync(new URL('../migrations/0002_catalog_index.sql', import.meta.url), 'utf8'))
  for (const sort of ['time', 'name', 'likes']) {
    for (const characterSlug of ['', 'io']) {
      const built = buildD1CatalogQuery({ characterSlug, sort, limit: 24 })
      const plan = db.prepare(`EXPLAIN QUERY PLAN ${built.sql}`).all(...built.bindings).map(row => row.detail).join('\n')
      const expected = characterSlug ? `loadouts_catalog_character_${sort}` : `loadouts_catalog_${sort}`
      assert.match(plan, new RegExp(expected), `${sort}/${characterSlug || 'all'} plan:\n${plan}`)
      assert.doesNotMatch(plan, /SCAN loadouts(?:\s|$)/, `${sort}/${characterSlug || 'all'} unexpectedly scans loadouts`)
    }
  }
  db.close()
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

test('publishing stores a sanitized complete preview and keeps first metadata immutable on replay', async () => {
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
  assert.equal(meta.headers.get('Cache-Control'), 'public, max-age=60, stale-while-revalidate=300')
  const metadata = await meta.json()
  const originalMetadata = structuredClone(metadata)
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
  assert.deepEqual(refreshedMetadata, originalMetadata)
})

test('upload routes reject oversized streamed bodies before parsing', async () => {
  const env = { LOADOUTS: makeR2() }
  const raw = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST', headers: { 'Content-Type': 'application/octet-stream' }, body: new Uint8Array(8 * 1024 + 1),
  }), env)
  assert.equal(raw.status, 413)

  const json = await worker.fetch(new Request('https://share.example/api/v1/loadouts/import', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: new Uint8Array(1024 * 1024 + 1),
  }), env)
  assert.equal(json.status, 413)
})

test('source-less merged skills rebuild their recorded sources from the loadout preview', async () => {
  const env = { LOADOUTS: makeR2() }
  const preview = {
    characterHash: '4D0A60C3', characterName: '伊欧', weaponHash: '02352554', weaponName: '星晶武器',
    sigils: [{ name: '体力 V+', level: 15, primaryHash: 'F372F096', primary: '体力', primaryLevel: 15, secondaryHash: 'DC584F60', secondary: '伤害上限', secondaryLevel: 15 }],
    weaponSkills: [{ hash: 'DC584F60', name: '伤害上限', level: 5 }],
    wrightstone: { name: '隔绝之祝福', traits: [{ hash: 'DC584F60', name: '伤害上限', level: 10 }] },
    summons: [{ typeHash: '0033943A', name: '巴哈姆特', mainTraitHash: 'DC584F60', mainTrait: '伤害上限', mainTraitLevel: 15 }],
    combinedSkills: [{ hash: 'SKILL_020_00', name: '伤害上限', level: 45, rawLevel: 45, sources: [] }],
  }
  const published = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Preview': Buffer.from(JSON.stringify(preview)).toString('base64url') }, body: makeFrame(),
  }), env).then(response => response.json())
  const metadata = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${published.compactCode}/meta`), env).then(response => response.json())
  assert.deepEqual(metadata.preview.combinedSkills[0].sources, [
    '因子01 · 伤害上限 Lv15',
    '武器 · [绝霸]布里欧纳克 · 伤害上限 Lv5',
    '武器祝福 · 隔绝之祝福 · 伤害上限 Lv10',
    '召唤石01 · 黑龙伊弗欧 · 传说 · 伤害 · 伤害上限 Lv15',
  ])
})

test('legacy sigil shells use the actual trait title and primary trait icon', async () => {
  const env = { LOADOUTS: makeR2() }
  const preview = Buffer.from(JSON.stringify({
    characterName: '巴恩',
    sigils: [
      { hash: 'B5B23F02', name: 'HP V+', primaryHash: 'F372F096', primary: '体力', secondaryHash: '48A95B8D', secondary: '金刚' },
      { hash: '80C94A24', name: 'Precise Wrath V+', primaryHash: '7EDD69D0', primary: '怒发冲冠', secondaryHash: 'DC584F60', secondary: '伤害上限' },
      { hash: 'F1D8F754', name: 'Divergence V+', primaryHash: 'F26BAEA5', primary: '分歧', secondaryHash: '0DE887A0', secondary: '天星之炼' },
      { hash: '673C5D8F', name: '勇士之觉醒+', primaryHash: '2E65A774', primary: '勇士的信念', secondaryHash: '16EFF868', secondary: '勇士的毅力' },
	  { hash: '95CC3CB8', name: '群青的剑光 V+', primaryHash: 'D176D262', primary: '群青的剑光', secondaryHash: '461A8E07', secondary: '群青的逆境' },
	  { hash: 'D8A464F1', name: '刃姬的小夜曲 V+', primaryHash: '7B5B081D', primary: '刃姬的小夜曲', secondaryHash: '9ACE140B', secondary: '刃姬的轮舞曲' },
	  { hash: '23953FD4', name: '雷狼的弹匣 V+', primaryHash: '7D75D904', primary: '雷狼的弹匣', secondaryHash: 'BE3404B9', secondary: '雷狼的慧眼' },
    ],
  })).toString('base64url')
  const published = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Preview': preview }, body: makeFrame(),
  }), env).then(response => response.json())
  const metadata = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${published.compactCode}/meta`), env).then(response => response.json())
  assert.deepEqual(metadata.preview.sigils.map(item => item.name), ['体力 V+', '怒发冲冠 V+', '分歧 V+', '勇士之觉醒+', '群青之觉醒+', '刃姬之觉醒+', '雷狼之觉醒+'])
  assert.equal(metadata.preview.sigils[0].icon, metadata.preview.sigils[0].primaryIcon)
  assert.equal(metadata.preview.sigils[1].icon, metadata.preview.sigils[1].primaryIcon)
  assert.equal(metadata.preview.sigils[2].icon, metadata.preview.sigils[2].primaryIcon)
  assert.doesNotMatch(metadata.preview.sigils[0].icon, /cmn_icskill_05_00\.png/)
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
  assert.match(html, /id="share-image">生成分享图<\/button>/)
  assert.match(html, /canvas\.width=1600;canvas\.height=900/)
  assert.match(html, /new ClipboardItem\(\{'image\/png':blob\}\)/)
  assert.match(html, /anchor\.download=title\.replace/)
  assert.match(html, /class="detail-back" href="\/c\/gran\?lang=zh">← 返回古兰配装<\/a>/)
  assert.doesNotMatch(html, /class="detail-actions"[^]*返回古兰配装/)
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
  assert.match(html, /id="share-image">Share Image<\/button>/)
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

test('public pages use a transparent track and parchment-brass vertical scrollbar', async () => {
  const html = await worker.fetch(new Request('https://share.example/'), { LOADOUTS: makeR2() }).then(response => response.text())
  assert.match(html, /html,body\{scrollbar-width:thin;scrollbar-color:rgba\(126,89,40,\.38\) transparent\}/)
  assert.match(html, /html\{scrollbar-gutter:stable;background:#e9dfcc\}/)
  assert.match(html, /\*\{scrollbar-width:thin;scrollbar-color:rgba\(126,89,40,\.38\) transparent\}/)
  assert.match(html, /::-webkit-scrollbar-track\{background:transparent\}/)
  assert.match(html, /::-webkit-scrollbar-thumb\{[^}]*border-radius:999px[^}]*background:rgba\(126,89,40,\.38\)/)
  assert.match(html, /::-webkit-scrollbar-corner\{background:transparent\}/)
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
	assert.equal(characterByIdentity('格兰').name, '古兰')
})

test('favicon reuses the desktop app identity instead of the browser fallback', async () => {
  assert.equal(existsSync(new URL('../public/favicon.ico', import.meta.url)), true)
  let requestedPath = ''
  const response = await worker.fetch(new Request('https://share.example/favicon.ico'), {
    ASSETS: {
      fetch(request) {
        requestedPath = new URL(request.url).pathname
        return new Response(new Uint8Array([0, 1, 2]), { headers: { 'Content-Type': 'image/x-icon' } })
      },
    },
  })
  assert.equal(response.status, 200)
  assert.equal(response.headers.get('Content-Type'), 'image/x-icon')
  assert.equal(requestedPath, '/favicon.ico')
})

test('legacy previews are canonicalized with extracted hash-specific Chinese names', async () => {
  const env = { LOADOUTS: makeR2() }
  const preview = Buffer.from(JSON.stringify({
    characterName: '塞达',
    characterHash: '0D21B430',
    sigils: [
      { hash: '119B24A8', name: '变幻自如之觉醒+', primaryHash: '0CD6C625', primary: '变换自如的迅刃', secondaryHash: 'A3B49220', secondary: '变换自如的妖剑士' },
      { hash: '6AAE4B8F', name: '剑圣之觉醒+', primaryHash: '77C809F5', primary: '剑圣的练气' },
      { hash: '426AD20E', name: '钳蟹的共鸣 V+', primaryHash: '082033CB', primary: '钳蟹的共鸣', secondaryHash: 'D3B8C21F', secondary: '终极钳蟹因子' },
    ],
  })).toString('base64url')
  const published = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST', headers: { 'Content-Type': 'application/octet-stream', 'X-Loadout-Preview': preview }, body: makeFrame(),
  }), env).then(response => response.json())
  const metadata = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${published.compactCode}/meta?lang=zh`), env).then(response => response.json())
  assert.equal(metadata.preview.characterName, '泽塔')
  assert.equal(metadata.preview.sigils[0].primary, '变幻自如的迅刃')
  assert.equal(metadata.preview.sigils[0].secondary, '变幻自如的妖剑士')
  assert.equal(metadata.preview.sigils[1].primary, '剑圣的炼气')
  assert.equal(metadata.preview.sigils[2].name, '永恒钳蟹因子+')
})
