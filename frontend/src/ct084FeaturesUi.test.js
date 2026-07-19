import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const read = name => readFileSync(new URL(name, import.meta.url), 'utf8')

const patchTool = read('./components/PatchTool.vue')
const home = read('./components/HomeJournal.vue')
const ctPage = read('./components/CT084Features.vue')
const uiI18n = read('./i18n-ui.js')
const ctCatalogBackend = read('../../ct084_catalog.go')
const ctRuntimeBackend = read('../../ct084_runtime.go')

test('the three CT 0.8.4 routes share one categorized component and unique planned art', () => {
  for (const [id, mode] of [
    ['ctCombat', 'combat'],
    ['ctCharacters', 'characters'],
    ['ctQuest', 'quest'],
  ]) {
    assert.match(patchTool, new RegExp(`${id}: \\(\\) => import\\('\\.\\/CT084Features\\.vue'\\)`))
    assert.match(patchTool, new RegExp(`<CT084Features[^>]*?activeTab === '${id}'[^>]*?mode="${mode}"`))
    assert.match(patchTool, new RegExp(`functionArt[\\s\\S]*?${id}: ${id}Art`))
    assert.match(patchTool, new RegExp(`functionStickers[\\s\\S]*?${id}: ${id}Sticker`))
  }

  assert.match(patchTool, /id:\s*'memory'[\s\S]*?items:\s*\[[^\]]*'ctCombat'[^\]]*'ctCharacters'[^\]]*'ctQuest'[^\]]*\]/)
  assert.match(home, /id:\s*'ctCombat'/)
  assert.match(home, /id:\s*'ctCharacters'/)
  assert.match(home, /id:\s*'ctQuest'/)

  assert.match(patchTool, /ct-combat-official-edge-safe\.webp/)
  assert.match(patchTool, /ct-characters-official-edge-safe\.webp/)
  assert.match(patchTool, /ct-quest-official-edge-safe\.webp/)
  assert.match(patchTool, /stickers\/ct-combat\.webp/)
  assert.match(patchTool, /stickers\/ct-characters\.webp/)
  assert.match(patchTool, /stickers\/ct-quest\.webp/)

  const artMap = patchTool.match(/const functionArt = \{([\s\S]*?)\n\}/)?.[1] || ''
  const stickerMap = patchTool.match(/const functionStickers = \{([\s\S]*?)\n\}/)?.[1] || ''
  for (const id of ['ctCombat', 'ctCharacters', 'ctQuest']) {
    assert.doesNotMatch(artMap, new RegExp(`${id}: progressionArt`))
    assert.doesNotMatch(stickerMap, new RegExp(`${id}: progressionSticker`))
  }
  assert.doesNotMatch(patchTool, /currentArt[^\n]*\|\|\s*progressionArt/)
  assert.doesNotMatch(patchTool, /currentSticker[^\n]*\|\|\s*progressionSticker/)
})

test('catalog presentation filters by mode and search while naming the active conflict', async () => {
  const {
    buildCT084Groups,
    buildCT084StatusIndex,
    findActiveCT084Conflict,
  } = await import(`./ct084FeatureView.js?contract=${Date.now()}`)

  const features = [
    { id: 'ct084-1', mode: 'characters', name: '一击集满无痛肉身', group: '巴萨拉卡', character: '巴萨拉卡', groupPath: ['角色修改', '巴萨拉卡'], conflicts: ['ct084-2'] },
    { id: 'ct084-2', mode: 'characters', name: '无限打击层数', group: '巴恩', character: '巴恩', groupPath: ['角色修改', '巴恩'], conflicts: ['ct084-1'] },
    { id: 'ct084-3', mode: 'quest', name: '自动收集任务宝箱', group: '任务修改', character: '', groupPath: ['任务修改'], conflicts: [] },
  ]
  const groups = buildCT084Groups(features, 'characters', '巴萨拉卡')
  assert.deepEqual(groups.map(group => [group.key, group.features.map(feature => feature.id)]), [
    ['巴萨拉卡', ['ct084-1']],
  ])

  const statuses = buildCT084StatusIndex([
    { id: 'ct084-1', enabled: false, available: false, rvas: [], currentBytes: [], error: '' },
    { id: 'ct084-2', enabled: false, available: true, rvas: [123], currentBytes: ['90'], error: 'recovery is required' },
  ])
  assert.equal(findActiveCT084Conflict(features[0], statuses, features)?.name, '无限打击层数')
})

test('the shared page owns the full CT lifecycle and changes switches only after verified refresh', () => {
  for (const api of [
    'CharaAcquire',
    'CharaRelease',
    'CT084GetCatalog',
    'CT084GetStatusesOwned',
    'CT084SetEnabledOwned',
    'CT084ReleaseOwned',
  ]) assert.match(ctPage, new RegExp(`\\b${api}\\b`), `${api} binding`)

  assert.match(ctPage, /CharaAcquire\(nextRuntimeAcquireRequestID\(\)\)/)
  assert.match(ctPage, /const verifiedStatuses = await fetchVerifiedStatuses\(acquiredOwnerToken\)/)
  assert.match(ctPage, /async function releaseCT084PageOwner\(ownerToken\)[\s\S]*?await CT084ReleaseOwned\(ownerToken\)[\s\S]*?await CharaRelease\(ownerToken\)/)
  assert.match(ctPage, /onBeforeUnmount\(\(\) => \{[\s\S]*?queueRuntimeLeaseRelease\([^;]*?releaseCT084PageOwner/)

  const toggleBody = ctPage.match(/async function setFeatureEnabled\([^)]*\) \{([\s\S]*?)\n\}/)?.[1] || ''
  const writeAt = toggleBody.indexOf('await CT084SetEnabledOwned(')
  const refreshAt = toggleBody.indexOf('await fetchVerifiedStatuses(')
  const publishAt = toggleBody.indexOf('applyStatuses(')
  assert.ok(writeAt >= 0 && refreshAt > writeAt && publishAt > refreshAt, 'write, verified refresh, then publish')
  assert.doesNotMatch(toggleBody.slice(0, publishAt), /\.enabled\s*=/, 'must not optimistically mutate enabled state')

  assert.match(ctPage, /sessionStorage\.getItem\(OFFLINE_CONFIRMATION_KEY\)/)
  assert.match(ctPage, /sessionStorage\.setItem\(OFFLINE_CONFIRMATION_KEY/)
  assert.match(ctPage, /role="dialog"[^>]*aria-modal="true"/)
  assert.match(ctPage, /仅离线\/单机使用/)
  assert.match(ctPage, /aria-live="polite"/)
  assert.doesNotMatch(ctPage, /任务得分倍率|动作速度|队伍监测|选中素材/, 'unimplemented Task 7 controls must not be advertised')
})

test('feature browsing remains keyboard-readable and reflows at the four target widths', () => {
  assert.match(ctPage, /type="search"[^>]*placeholder="输入关键词筛选"/)
  assert.match(ctPage, /class="ct-group-disclosure"[^>]*:aria-label/)
  assert.match(ctPage, /:aria-expanded="currentGroup\?\.key === group\.key"/)
  assert.match(ctPage, /role="switch"/)
  assert.match(ctPage, /:aria-checked="statusFor\(feature\)\.enabled"/)
  assert.match(ctPage, /:aria-busy="busyFeatureID === feature\.id"/)
  assert.match(ctPage, /与「\{\{ displayFeatureName\(activeConflictFor\(feature\)\) \}\}」互斥/)
  assert.match(ctPage, /<details class="ct-technical ui-disclosure">/)

  assert.match(ctPage, /@media\s*\(min-width\s*:\s*1024px\)[\s\S]*?\.ct-feature-workspace\s*\{[^}]*grid-template-columns\s*:\s*minmax\(146px,30fr\)\s+minmax\(0,70fr\)/i)
  assert.match(ctPage, /@media\s*\(max-width\s*:\s*1023px\)[\s\S]*?\.ct-feature-workspace\s*\{[^}]*grid-template-columns\s*:\s*minmax\(0,1fr\)/i)
  assert.match(ctPage, /@media\s*\(max-width\s*:\s*767px\)/)
  assert.match(ctPage, /@media\s*\(max-width\s*:\s*480px\)/)
  assert.match(ctPage, /@media\s*\(prefers-reduced-motion\s*:\s*reduce\)/)

  const pageRule = ctPage.match(/\.ct084-page\s*\{([^}]*)\}/)?.[1] || ''
  const workspaceRule = ctPage.match(/\.ct-feature-workspace\s*\{([^}]*)\}/)?.[1] || ''
  assert.doesNotMatch(`${pageRule}\n${workspaceRule}`, /overflow(?:-y)?\s*:\s*(?:auto|scroll)/, 'the shell owns the only main scroll container')
})

test('the component bindings match the owned backend methods Wails generates from', () => {
  assert.match(ctCatalogBackend, /func \(a \*App\) CT084GetCatalog\(\) \(\[\]CT084Feature, error\)/)
  assert.match(ctRuntimeBackend, /func \(a \*App\) CT084GetStatusesOwned\(token string\) \(\[\]CT084FeatureStatus, error\)/)
  assert.match(ctRuntimeBackend, /func \(a \*App\) CT084SetEnabledOwned\(token, id string, enabled bool\) \(CT084FeatureStatus, error\)/)
  assert.match(ctRuntimeBackend, /func \(a \*App\) CT084ReleaseOwned\(token string\) error/)
})

test('new navigation, safety, state and recovery copy is covered by the UI translation layer', () => {
  for (const label of [
    '战斗规则补丁',
    '角色机制补丁',
    '任务与便利补丁',
    '仅离线/单机使用',
    '恢复全部并断开',
    '搜索名称、角色或分组',
    '首次启用时定位并保存原字节',
    '写后回读状态不一致',
    '需要恢复',
    '互斥占用',
  ]) {
    assert.match(uiI18n, new RegExp(`'${label.replace(/[.*+?^${}()|[\\]\\]/g, '\\$&')}'\\s*:`), `${label} translation`)
  }
})
