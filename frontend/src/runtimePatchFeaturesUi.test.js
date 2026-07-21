import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const read = name => readFileSync(new URL(name, import.meta.url), 'utf8')

const patchTool = read('./components/PatchTool.vue')
const home = read('./components/HomeJournal.vue')
const patchPage = read('./components/RuntimePatchFeatures.vue')
const uiI18n = read('./i18n-ui.js')
const patchCatalogBackend = read('../../runtime_patch_catalog.go')
const patchRuntimeBackend = read('../../runtime_patch_runtime.go')
const productionCatalog = JSON.parse(read('../../data/runtime_patch_catalog.json'))

test('one runtime patch operation gate blocks writes and disconnects during a delayed refresh, then invalidates stale publication on reset', async () => {
  const { createRuntimePatchOperationGate } = await import(`./runtimePatchOperationGate.js?gate=${Date.now()}`)
  const observed = []
  const gate = createRuntimePatchOperationGate(current => observed.push(current))
  const refreshToken = gate.begin('refresh')
  assert.ok(refreshToken)
  assert.equal(gate.current?.kind, 'refresh')

  let resolveRefresh
  const delayedRefresh = new Promise(resolve => { resolveRefresh = resolve })
  let published = 'last-verified'
  const pending = (async () => {
    const result = await delayedRefresh
    if (gate.isCurrent(refreshToken)) published = result
    gate.finish(refreshToken)
  })()

  assert.equal(gate.begin('feature', 'runtime-patch-1'), null)
  assert.equal(gate.begin('disconnect'), null)
  gate.reset()
  assert.equal(gate.busy, false)
  assert.equal(observed.at(-1), null)

  resolveRefresh('stale-refresh')
  await pending
  assert.equal(published, 'last-verified')

  const writeToken = gate.begin('feature', 'runtime-patch-1')
  assert.ok(writeToken)
  assert.equal(gate.current?.featureID, 'runtime-patch-1')
  gate.finish(writeToken)
  assert.equal(gate.busy, false)
})

test('the runtime patch page routes refresh, writes, connect and disconnect through the same reactive gate', () => {
  assert.match(patchPage, /createRuntimePatchOperationGate/)
  assert.match(patchPage, /const operationGate = createRuntimePatchOperationGate\(\(operation\) => \{\s*activeOperation\.value = operation\s*\}\)/)
  assert.match(patchPage, /const operationBusy = computed\(\(\) => activeOperation\.value !== null\)/)
  assert.match(patchPage, /function clearConnectionState\(\) \{[\s\S]*?operationGate\.reset\(\)/)

  for (const [name, kind, featureArgument = ''] of [
    ['connect', 'connect'],
    ['disconnect', 'disconnect'],
    ['refreshStatuses', 'refresh'],
    ['setFeatureEnabled', 'feature', ', feature.id'],
  ]) {
    assert.match(patchPage, new RegExp(`async function ${name}\\([^)]*\\) \\{[\\s\\S]*?beginOperation\\('${kind}'${featureArgument.replace('.', '\\.') }\\)`), `${name} shared gate`)
  }

  assert.match(patchPage, /function featureDisabled\([^)]*\) \{[\s\S]*?interactionLocked\.value/)
  assert.match(patchPage, /:disabled="operationBusy"[^>]*@click="connected \? disconnect\(\) : connect\(\)"/)
})

test('a disconnect retry keeps runtime patch writes locked until its exact owner and epoch are finally released', () => {
  assert.match(patchPage, /const releasePending = ref\(false\)/)
  assert.match(patchPage, /const interactionLocked = computed\(\(\) => operationBusy\.value \|\| releasePending\.value\)/)
  assert.match(patchPage, /function completeRuntimeRelease\(expectedOwnerToken, expectedEpoch, notification\) \{[\s\S]*?disposed[\s\S]*?lifecycleEpoch !== expectedEpoch[\s\S]*?connectionOwnerToken !== expectedOwnerToken[\s\S]*?notification\?\.ownerToken !== expectedOwnerToken[\s\S]*?clearConnectionState\(\)/)
  assert.match(patchPage, /releaseRuntimeLease\([\s\S]*?releaseRuntimePatchPageOwner,[\s\S]*?notification => completeRuntimeRelease\(ownerToken, epoch, notification\)[\s\S]*?\)/)
  assert.match(patchPage, /catch \(error\) \{[\s\S]*?releasePending\.value = true[\s\S]*?正在后台重试恢复/)
  assert.match(patchPage, /function featureDisabled\([^)]*\) \{[\s\S]*?interactionLocked\.value/)
})

test('the three live-patch routes share one persistent categorized session and unique art', () => {
  assert.match(patchTool, /import RuntimePatchFeatures from ['"]\.\/RuntimePatchFeatures\.vue['"]/)
  for (const [id, mode] of [
    ['patchCombat', 'combat'],
    ['patchCharacters', 'characters'],
    ['patchQuest', 'quest'],
  ]) {
    assert.match(patchTool, new RegExp(`${id}: '${mode}'`))
    assert.match(patchTool, new RegExp(`functionArt[\\s\\S]*?${id}: ${id}Art`))
    assert.match(patchTool, new RegExp(`functionStickers[\\s\\S]*?${id}: ${id}Sticker`))
  }

  assert.equal((patchTool.match(/<RuntimePatchFeatures\b/g) || []).length, 1, 'all three tabs must use one component instance')
  assert.match(patchTool, /<RuntimePatchFeatures[\s\S]*?v-if="runtimePatchesMounted"[\s\S]*?v-show="isRuntimePatchTab"[\s\S]*?:mode="runtimePatchMode"/)
  assert.match(patchTool, /watch\(activeTab,[\s\S]*?runtimePatchesMounted\.value = true/)
  assert.match(patchTool, /@session-change="updateCTFeatureSession"/)
  assert.match(patchTool, /ctFeatureSession\.connected[\s\S]*?实时补丁常驻/)

  assert.match(patchTool, /id:\s*'memory'[\s\S]*?items:\s*\[[^\]]*'patchCombat'[^\]]*'patchCharacters'[^\]]*'patchQuest'[^\]]*\]/)
  assert.match(home, /id:\s*'patchCombat'/)
  assert.match(home, /id:\s*'patchCharacters'/)
  assert.match(home, /id:\s*'patchQuest'/)

  assert.match(patchTool, /patch-combat-official-edge-safe\.webp/)
  assert.match(patchTool, /patch-characters-official-edge-safe\.webp/)
  assert.match(patchTool, /patch-quest-official-edge-safe\.webp/)
  assert.match(patchTool, /stickers\/patch-combat\.webp/)
  assert.match(patchTool, /stickers\/patch-characters\.webp/)
  assert.match(patchTool, /stickers\/patch-quest\.webp/)

  const artMap = patchTool.match(/const functionArt = \{([\s\S]*?)\n\}/)?.[1] || ''
  const stickerMap = patchTool.match(/const functionStickers = \{([\s\S]*?)\n\}/)?.[1] || ''
  for (const id of ['patchCombat', 'patchCharacters', 'patchQuest']) {
    assert.doesNotMatch(artMap, new RegExp(`${id}: progressionArt`))
    assert.doesNotMatch(stickerMap, new RegExp(`${id}: progressionSticker`))
  }
  assert.doesNotMatch(patchTool, /currentArt[^\n]*\|\|\s*progressionArt/)
  assert.doesNotMatch(patchTool, /currentSticker[^\n]*\|\|\s*progressionSticker/)
})

test('page navigation hides the persistent patch session without unmounting or restoring it', () => {
  assert.match(patchTool, /<section v-show="activeTab !== 'home'" class="tool-stage"/)
  assert.doesNotMatch(patchTool, /<section v-else :key="activeTab" class="tool-stage"/)
  assert.match(patchPage, /const emit = defineEmits\(\['status', 'session-change'\]\)/)
  assert.match(patchPage, /emit\('session-change',[\s\S]*?connected:/)
  assert.match(patchPage, /onBeforeUnmount\(\(\) => \{[\s\S]*?queueRuntimeLeaseRelease\([^;]*?releaseRuntimePatchPageOwner/)
})

test('unverified runtime extensions remain visibly labelled as candidates', () => {
  assert.match(patchPage, /v-if="feature\.evidenceNote"/)
  assert.match(patchPage, /startsWith\('candidate_'\)/)
  assert.match(patchPage, /class="feature-evidence"/)
  assert.match(patchPage, /\{\{ tr\(feature\.evidenceNote\) \}\}/)
  assert.match(patchPage, /\.feature-evidence\.is-candidate\s*\{[^}]*color\s*:\s*var\(--warning-ink\)/is)
})

test('catalog presentation filters by mode and search while naming the active conflict', async () => {
  const {
    buildRuntimePatchGroups,
    buildRuntimePatchStatusIndex,
    findActiveRuntimePatchConflict,
  } = await import(`./runtimePatchFeatureView.js?contract=${Date.now()}`)

  const features = [
    { id: 'runtime-patch-1', mode: 'characters', name: '一击集满无痛肉身', group: '巴萨拉卡', character: '巴萨拉卡', groupPath: ['角色修改', '巴萨拉卡'], conflicts: ['runtime-patch-2'] },
    { id: 'runtime-patch-2', mode: 'characters', name: '无限打击层数', group: '巴恩', character: '巴恩', groupPath: ['角色修改', '巴恩'], conflicts: ['runtime-patch-1'] },
    { id: 'runtime-patch-3', mode: 'quest', name: '自动收集任务宝箱', group: '任务修改', character: '', groupPath: ['任务修改'], conflicts: [] },
  ]
  const groups = buildRuntimePatchGroups(features, 'characters', '巴萨拉卡')
  assert.deepEqual(groups.map(group => [group.key, group.features.map(feature => feature.id)]), [
    ['巴萨拉卡', ['runtime-patch-1']],
  ])
  const englishGroups = buildRuntimePatchGroups(features, 'characters', 'Vaseraga', {
    featureLabel: feature => feature.id === 'runtime-patch-1' ? 'Instant Grynoth' : feature.name,
    groupLabel: group => group === '巴萨拉卡' ? 'Vaseraga' : group,
  })
  assert.deepEqual(englishGroups.map(group => [group.key, group.label, group.features.map(feature => feature.id)]), [
    ['巴萨拉卡', 'Vaseraga', ['runtime-patch-1']],
  ])

  const statuses = buildRuntimePatchStatusIndex([
    { id: 'runtime-patch-1', enabled: false, available: false, rvas: [], currentBytes: [], error: '' },
    { id: 'runtime-patch-2', enabled: false, available: true, rvas: [123], currentBytes: ['90'], error: 'recovery is required' },
  ])
  assert.equal(findActiveRuntimePatchConflict(features[0], statuses, features)?.name, '无限打击层数')
})

test('verified runtime patch statuses form an exact one-to-one set with the catalog', async () => {
  const { validateRuntimePatchStatusSet } = await import(`./runtimePatchFeatureView.js?status-set=${Date.now()}`)
  const catalog = [
    { id: 'runtime-patch-1', sites: [{ enableBytes: 'kJE=' }] },
    { id: 'runtime-patch-2', sites: [{ enableBytes: 'zA==' }] },
  ]
  const valid = [
    { id: 'runtime-patch-2', enabled: false, available: true, rvas: [], currentBytes: [], error: '' },
    { id: 'runtime-patch-1', enabled: true, available: true, rvas: [4096], currentBytes: ['90 91'], error: '' },
  ]
  assert.equal(validateRuntimePatchStatusSet(catalog, valid), valid, 'status order may differ when IDs still match exactly')

  assert.throws(() => validateRuntimePatchStatusSet(catalog, [{ id: 'runtime-patch-1' }]), /数量.*目录/)
  assert.throws(() => validateRuntimePatchStatusSet(catalog, [...valid, { id: 'runtime-patch-extra' }]), /数量.*目录/)
  assert.throws(() => validateRuntimePatchStatusSet(catalog, [{ id: 'runtime-patch-1' }, { id: 'runtime-patch-1' }]), /重复/)
  assert.throws(() => validateRuntimePatchStatusSet(catalog, [{ id: 'runtime-patch-1' }, { id: '' }]), /不能为空/)
  assert.throws(() => validateRuntimePatchStatusSet(catalog, [{ id: 'runtime-patch-1' }, { id: 'runtime-patch-extra' }]), /目录外.*runtime-patch-extra/)
  assert.throws(() => validateRuntimePatchStatusSet(catalog, [{ id: 'runtime-patch-1' }, { id: ' runtime-patch-2 ' }]), /目录外/, 'ID equality is exact, not trim-coerced')

  const coercedBoolean = valid.map(status => ({ ...status, rvas: [...status.rvas], currentBytes: [...status.currentBytes] }))
  coercedBoolean[0].enabled = 'false'
  assert.throws(() => validateRuntimePatchStatusSet(catalog, coercedBoolean), /enabled.*布尔值/)

  const mutateValid = (mutate) => {
    const next = valid.map(status => ({ ...status, rvas: [...status.rvas], currentBytes: [...status.currentBytes] }))
    mutate(next)
    return next
  }
  for (const [label, malformed, expected] of [
    ['available is not coerced', mutateValid(next => { next[0].available = 1 }), /available.*布尔值/],
    ['error is a string', mutateValid(next => { next[0].error = null }), /error.*字符串/],
    ['rvas is an array', mutateValid(next => { next[0].rvas = {} }), /rvas.*数组/],
    ['currentBytes is an array', mutateValid(next => { next[0].currentBytes = null }), /currentBytes.*数组/],
    ['owned arrays have equal lengths', mutateValid(next => { next[1].currentBytes = [] }), /rvas.*currentBytes.*长度/],
    ['owned arrays match site count', mutateValid(next => {
      next[1].rvas.push(8192)
      next[1].currentBytes.push('90 91')
    }), /写入点数量.*目录/],
    ['RVA is a non-negative safe integer', mutateValid(next => { next[1].rvas[0] = 1.5 }), /RVA.*安全整数/],
    ['current bytes are hex pairs', mutateValid(next => { next[1].currentBytes[0] = 'GG' }), /当前字节.*十六进制/],
    ['current bytes match patch width', mutateValid(next => { next[1].currentBytes[0] = '90' }), /当前字节.*长度/],
    ['enabled state owns write sites', mutateValid(next => {
      next[1].rvas = []
      next[1].currentBytes = []
    }), /已开启.*写入点/],
    ['enabled state is available', mutateValid(next => { next[1].available = false }), /已开启.*available/],
    ['enabled state has no error', mutateValid(next => { next[1].error = 'foreign bytes' }), /已开启.*error/],
    ['enabled bytes equal catalog patch', mutateValid(next => { next[1].currentBytes[0] = '90 90' }), /已开启.*目录补丁/],
  ]) {
    assert.throws(() => validateRuntimePatchStatusSet(catalog, malformed), expected, label)
  }
})

test('the shared page owns the full runtime patch lifecycle and changes switches only after verified refresh', () => {
  for (const api of [
    'CharaAcquire',
    'CharaRelease',
    'RuntimePatchGetCatalog',
    'RuntimePatchGetStatusesOwned',
    'RuntimePatchSetEnabledOwned',
    'RuntimePatchReleaseOwned',
  ]) assert.match(patchPage, new RegExp(`\\b${api}\\b`), `${api} binding`)

  assert.match(patchPage, /CharaAcquire\(nextRuntimeAcquireRequestID\(\)\)/)
  assert.match(patchPage, /const verifiedStatuses = await fetchVerifiedStatuses\(acquiredOwnerToken\)/)
  assert.match(patchPage, /async function releaseRuntimePatchPageOwner\(ownerToken\)[\s\S]*?await RuntimePatchReleaseOwned\(ownerToken\)[\s\S]*?await CharaRelease\(ownerToken\)/)
  assert.match(patchPage, /onBeforeUnmount\(\(\) => \{[\s\S]*?queueRuntimeLeaseRelease\([^;]*?releaseRuntimePatchPageOwner/)

  const toggleBody = patchPage.match(/async function setFeatureEnabled\([^)]*\) \{([\s\S]*?)\n\}/)?.[1] || ''
  const writeAt = toggleBody.indexOf('await RuntimePatchSetEnabledOwned(')
  const refreshAt = toggleBody.indexOf('await fetchVerifiedStatuses(')
  const publishAt = toggleBody.indexOf('applyStatuses(')
  assert.ok(writeAt >= 0 && refreshAt > writeAt && publishAt > refreshAt, 'write, verified refresh, then publish')
  assert.doesNotMatch(toggleBody.slice(0, publishAt), /\.enabled\s*=/, 'must not optimistically mutate enabled state')

  assert.match(patchPage, /sessionStorage\.getItem\(OFFLINE_CONFIRMATION_KEY\)/)
  assert.match(patchPage, /sessionStorage\.setItem\(OFFLINE_CONFIRMATION_KEY/)
  assert.match(patchPage, /role="dialog"[^>]*aria-modal="true"/)
  assert.match(patchPage, /仅离线\/单机使用/)
  assert.match(patchPage, /aria-live="polite"/)
  assert.doesNotMatch(patchPage, /任务得分倍率|动作速度|队伍监测|选中素材/, 'unimplemented Task 7 controls must not be advertised')

  const statusFetchBody = patchPage.match(/async function fetchVerifiedStatuses\([^)]*\) \{([\s\S]*?)\n\}/)?.[1] || ''
  assert.match(statusFetchBody, /validateRuntimePatchStatusSet\(catalog\.value, await RuntimePatchGetStatusesOwned\(ownerToken\)\)/)
  assert.doesNotMatch(statusFetchBody, /new Set\(/, 'the shared validator owns all exact-set semantics')
  assert.doesNotMatch(patchPage, /function normalizeStatuses|!!status\?\.enabled|String\(status\?\.error/, 'malformed backend DTO fields must never be coerced into plausible UI state')
})

test('feature browsing remains keyboard-readable and reflows from its actual tool-panel width', () => {
  assert.match(patchPage, /type="search"[^>]*:placeholder="tr\('输入关键词筛选'\)"/)
  assert.match(patchPage, /class="patch-group-disclosure"[^>]*:aria-label/)
  assert.match(patchPage, /:aria-expanded="currentGroup\?\.key === group\.key"/)
  assert.match(patchPage, /role="switch"/)
  assert.match(patchPage, /:aria-checked="statusFor\(feature\)\.enabled"/)
  assert.match(patchPage, /:aria-busy="busyFeatureID === feature\.id"/)
  assert.match(patchPage, /tr\('与「'\)[\s\S]*?displayFeatureName\(activeConflictFor\(feature\)\)[\s\S]*?tr\('」互斥；先恢复该功能后才能启用。'\)/)
  assert.match(patchPage, /<details class="patch-technical ui-disclosure">/)

  assert.match(patchPage, /@container\s+tool-panel\s*\(min-width\s*:\s*680px\)[\s\S]*?\.patch-feature-workspace\s*\{[^}]*grid-template-columns\s*:\s*minmax\(146px,30fr\)\s+minmax\(0,70fr\)/i)
  assert.match(patchPage, /@container\s+tool-panel\s*\(max-width\s*:\s*679px\)[\s\S]*?\.patch-browser-head\s*\{[^}]*flex-direction\s*:\s*column/i)
  assert.match(patchPage, /@container\s+tool-panel\s*\(max-width\s*:\s*520px\)/)
  assert.match(patchPage, /@container\s+tool-panel\s*\(max-width\s*:\s*340px\)[\s\S]*?\.patch-browser-head \.ui-section-copy\s*\{[^}]*display\s*:\s*none/i)
  assert.doesNotMatch(patchPage, /@media\s*\((?:min|max)-width/, 'component layout must follow panel width, not the outer viewport')
  assert.match(patchPage, /@media\s*\(prefers-reduced-motion\s*:\s*reduce\)/)

  const pageRule = patchPage.match(/\.runtime-patch-page\s*\{([^}]*)\}/)?.[1] || ''
  const workspaceRule = patchPage.match(/\.patch-feature-workspace\s*\{([^}]*)\}/)?.[1] || ''
  assert.doesNotMatch(`${pageRule}\n${workspaceRule}`, /overflow(?:-y)?\s*:\s*(?:auto|scroll)/, 'the shell owns the only main scroll container')
})

test('the component bindings match the owned backend methods Wails generates from', () => {
  assert.match(patchCatalogBackend, /func \(a \*App\) RuntimePatchGetCatalog\(\) \(\[\]RuntimePatchFeature, error\)/)
  assert.match(patchRuntimeBackend, /func \(a \*App\) RuntimePatchGetStatusesOwned\(token string\) \(\[\]RuntimePatchFeatureStatus, error\)/)
  assert.match(patchRuntimeBackend, /func \(a \*App\) RuntimePatchSetEnabledOwned\(token, id string, enabled bool\) \(RuntimePatchFeatureStatus, error\)/)
  assert.match(patchRuntimeBackend, /func \(a \*App\) RuntimePatchReleaseOwned\(token string\) error/)
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

test('all 58 production runtime patch features, groups and dynamic page messages render without Chinese in English mode', async () => {
  const {
    runtimePatchEnglishFeatureNames,
    translateRuntimePatchFeatureName,
    translateRuntimePatchGroupName,
    translateRuntimePatchText,
  } = await import(`./runtimePatchTranslations.js?complete=${Date.now()}`)
  const cjk = /[\u3400-\u9fff]/u

  assert.equal(productionCatalog.features.length, 58, 'the production fixture must remain the audited live-feature catalog')
  assert.equal(Object.keys(runtimePatchEnglishFeatureNames).length, productionCatalog.features.length)
  for (const feature of productionCatalog.features) {
    const englishName = translateRuntimePatchFeatureName(feature, 'en')
    const englishGroup = translateRuntimePatchGroupName(feature.character || feature.group, 'en')
    assert.ok(englishName && englishName !== feature.displayName, `${feature.id} needs a dedicated English name`)
    assert.doesNotMatch(englishName, cjk, `${feature.id} English name`)
    assert.doesNotMatch(englishGroup, cjk, `${feature.id} English group`)
    assert.doesNotMatch(translateRuntimePatchText(feature.displayName, 'en'), cjk, `${feature.id} dynamic-name replacement`)
  }

  const dynamicSamples = [
    '正在读取实时功能目录…',
    '功能目录已就绪；连接游戏后可读取实时状态。',
    '已读取 58 项已验证补丁',
    '读取实时功能目录失败：未知错误',
    '后端未返回连接所有权令牌',
    '已连接游戏进程 PID 1234',
    '全部实时补丁已恢复，并已断开游戏进程',
    '安全断开暂未完成，正在后台重试恢复：未知错误',
    '实时补丁状态已回读',
    '刷新状态失败：未知错误',
    '当前页面不再持有连接所有权',
    '无限闪避写后回读状态不一致',
    '无限闪避已开启',
    '无限闪避已恢复默认',
    '无限闪避操作失败：未知错误',
    '回读中', '已开启', '需要恢复', '未连接', '互斥占用', '不可用', '默认',
    '正在安全恢复并断开', '游戏进程已连接', '连接游戏后读取实时状态',
    '已开启 3 项', '等待恢复', '已验证连接', '刷新状态', '处理中…',
    '重试安全恢复', '恢复全部并断开', '连接游戏进程',
    '战斗规则目录', '58 项',
    '搜索名称、角色或分组', '输入关键词筛选', '正在读取功能目录…',
    '没有匹配的功能', '换一个角色名、功能名或分组关键词。', '当前分组',
    '战斗规则分组', '战斗规则', '3 项已验证补丁',
    '与「无限格挡」互斥；先恢复该功能后才能启用。',
    '已回读 2 个写入点', '首次启用时定位并保存原字节', '连接后读取状态',
    '恢复默认', '开启', '技术详情', '目录 ID', '写入点', '冲突组',
    '偏移 4 · 当前字节 90 90', '未读取',
    '首次启用确认', '仅离线/单机使用',
    '这些功能会直接修改游戏运行时规则。请确认当前不在联机房间，并只在离线或单机内容中使用。本次打开应用只确认一次。',
    '即将开启', '取消', '确认仅在单机使用并开启',
    '实时补丁回读状态 runtime-patch-1 的 enabled 必须是布尔值',
    '实时补丁回读状态 runtime-patch-1 的 RVA[0] 必须是非负安全整数',
    '实时补丁回读状态 runtime-patch-1 的当前字节[0] 必须是空值或空格分隔的十六进制字节',
    '实时功能目录 runtime-patch-1 的补丁字节无效',
    '实时补丁回读状态 runtime-patch-1 已开启，但当前字节[0] 与目录补丁不一致',
  ]
  for (const sample of dynamicSamples) {
    const translated = translateRuntimePatchText(sample, 'en')
    assert.doesNotMatch(translated, cjk, `missing runtime patch page translation for: ${sample}`)
  }
})

test('the runtime patch component localizes catalog search, announcements, feature names and every static template label explicitly', () => {
  assert.match(patchPage, /import \{ language \} from '\.\.\/i18n\.js'/)
  assert.match(patchPage, /translateRuntimePatchFeatureName[\s\S]*?translateRuntimePatchGroupName[\s\S]*?translateRuntimePatchText/)
  assert.match(patchPage, /function tr\(value\) \{[\s\S]*?translateRuntimePatchText\(value, language\.value\)/)
  assert.match(patchPage, /buildRuntimePatchGroups\([\s\S]*?featureLabel:[\s\S]*?translateRuntimePatchFeatureName[\s\S]*?groupLabel:[\s\S]*?translateRuntimePatchGroupName/)
  assert.match(patchPage, /function announce\([^)]*\) \{[\s\S]*?const translatedMessage = tr\(message\)[\s\S]*?liveMessage\.value = translatedMessage[\s\S]*?emit\('status', translatedMessage/)
  assert.match(patchPage, /function displayFeatureName\(feature\) \{[\s\S]*?translateRuntimePatchFeatureName\(feature, language\.value\)/)

  const template = patchPage.match(/<template>([\s\S]*?)<\/template>/)?.[1] || ''
  for (const label of [
    '游戏进程已连接', '恢复全部并断开',
    '搜索名称、角色或分组', '没有匹配的功能', '首次启用时定位并保存原字节',
    '技术详情', '目录 ID', '首次启用确认', '仅离线/单机使用', '取消',
  ]) {
    const sourceLine = template.split('\n').find(line => line.includes(label)) || ''
    assert.match(sourceLine, /\btr\(/, `${label} must use component-local translation`)
  }
})
