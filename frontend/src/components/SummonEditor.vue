<script setup>
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { CharaAcquire, CharaRelease, SummonGetAll, SummonGetOptions, SummonUpdateOwned } from '../../wailsjs/go/main/App'
import { summonAssetIcon, traitAssetIcon } from '../gameAssetIcons'
import { matchText } from '../utils/matchText.js'
import { nextRuntimeAcquireRequestID, queueRuntimeLeaseRelease, releaseRuntimeLease } from '../runtimeLeaseManager.js'
import LegalityIndicator from './LegalityIndicator.vue'

const emit = defineEmits(['status'])
const RUNTIME_LEASE_SCOPE = 'summon-editor'
const connected = ref(false)
const loading = ref(false)
const saving = ref(false)
const pid = ref(0)
const summons = ref([])
const options = reactive({ types: [], traits: [], subParams: [] })
const filter = ref('')
const traitFilter = ref('')
const selectedIndex = ref(-1)
const edit = reactive({ typeHash: '', mainTraitHash: '', subParamHash: '', mainTraitLevel: '1', subParamLevel: '1', rank: '1' })
let disposed = false
let lifecycleEpoch = 0
let connectionOwnerToken = ''

const typeByHash = computed(() => new Map(options.types.map((item) => [item.hash, item])))
const traitByHash = computed(() => new Map(options.traits.map((item) => [item.hash, item])))
const subParamByHash = computed(() => new Map(options.subParams.map((item) => [item.hash, item])))
const currentSubParam = computed(() => {
  const h = Number.parseInt(String(edit.subParamHash).trim(), 16)
  return Number.isNaN(h) ? null : subParamByHash.value.get(h) || null
})
const subParamMaxLevel = computed(() => {
  const sp = currentSubParam.value
  return sp && Number.isInteger(sp.maxLevel) && sp.maxLevel > 0 ? sp.maxLevel : 9
})
function subParamValueLabel(level) {
  const sp = currentSubParam.value
  const idx = Number.parseInt(level, 10)
  if (!sp || !Array.isArray(sp.values) || idx < 0 || idx >= sp.values.length) return String(idx)
  const v = sp.values[idx]
  return sp.isPercent ? `${idx} → +${v}%` : `${idx} → +${v}`
}
const filteredTraits = computed(() => {
  const query = traitFilter.value.trim()
  if (!query) return options.traits
  return options.traits.filter((item) => matchText(optionLabel(item), query))
})
const filteredSummons = computed(() => {
  const query = filter.value.trim()
  if (!query) return summons.value
  return summons.value.filter((item) => [item.index, nameForType(item.typeHash), nameForTrait(item.mainTraitHash), nameForSubParam(item.subParamHash), hex(item.typeHash)]
    .some((value) => matchText(value, query)))
})
const selected = computed(() => summons.value.find((item) => item.index === selectedIndex.value))
const currentMainTraitIsLegacy = computed(() => {
  if (!selected.value) return false
  const mainHash = parsedHashOrNull(edit.mainTraitHash)
  return mainHash !== null &&
    mainHash === (selected.value.mainTraitHash >>> 0) &&
    !traitByHash.value.has(mainHash)
})
const rarityLabelsByCost = { 3: 'I', 4: 'II', 5: 'III' }

function hex(value) { return '0x' + Number(value || 0).toString(16).toUpperCase().padStart(8, '0') }
function nameForType(hash) { return typeByHash.value.get(hash)?.name || hex(hash) }
function nameForTrait(hash) { return traitByHash.value.get(hash)?.name || (hash ? hex(hash) : '无') }
function nameForSubParam(hash) { return subParamByHash.value.get(hash)?.name || (hash ? hex(hash) : '无') }
function optionLabel(item) { return `${item.name} · ${hex(item.hash)}` }
function rarityLabel(item) {
  const cost = typeByHash.value.get(item.typeHash)?.cost
  return rarityLabelsByCost[cost] || String(item.rank)
}
function summonIcon(item) { return summonAssetIcon({ typeHash: item?.typeHash }) }
function currentTraitIcon() {
  const hash = parsedHashOrNull(edit.mainTraitHash)
  const trait = hash == null ? null : traitByHash.value.get(hash)
  return hash == null ? '' : traitAssetIcon({ hash, name: trait?.name })
}
function parseHash(value, label) {
  const text = String(value).trim()
  const parsed = /^0x/i.test(text) ? Number.parseInt(text, 16) : Number.parseInt(text, 10)
  if (!Number.isInteger(parsed) || parsed < 0 || parsed > 0xFFFFFFFF) throw new Error(`${label}必须是 32 位无符号整数`)
  return parsed >>> 0
}
function parsedHashOrNull(value) {
  try { return parseHash(value, 'Hash') } catch (_) { return null }
}
function traitMax(hash) {
  const parsed = parseHash(hash, '因子')
  const known = traitByHash.value.get(parsed)?.maxLevel
  if (Number.isInteger(known) && known > 0) return known
  if (currentMainTraitIsLegacy.value && selected.value && parsed === (selected.value.mainTraitHash >>> 0)) {
    return selected.value.mainTraitLevel
  }
  return 15
}

const legality = computed(() => {
  if (!selected.value) return { status: 'impossible', writable: false, message: '请先选择召唤石' }
  const typeHash = parsedHashOrNull(edit.typeHash)
  const mainHash = parsedHashOrNull(edit.mainTraitHash)
  const subHash = parsedHashOrNull(edit.subParamHash)
  const mainLevel = Number.parseInt(edit.mainTraitLevel, 10)
  const subLevel = Number.parseInt(edit.subParamLevel, 10)
  if (typeHash === null || mainHash === null || subHash === null || !Number.isInteger(mainLevel) || !Number.isInteger(subLevel)) {
    return { status: 'impossible', writable: false, message: '存在无法编码的 Hash 或等级' }
  }
  if (!subHash) return { status: 'impossible', writable: false, message: '副参数不能为空，后端不会接受 Hash 0' }
  if (typeHash !== (selected.value.typeHash >>> 0)) return { status: 'impossible', writable: false, message: '当前保存函数不支持更换召唤石种类' }
  if (subLevel < 0 || subLevel > subParamMaxLevel.value) return { status: 'impossible', writable: false, message: `副参数档位必须为 0 到 ${subParamMaxLevel.value}，越界会读取错误数值表` }
  if (mainLevel < 0 || mainLevel > 0x7FFFFFFF) return { status: 'impossible', writable: false, message: '主因子等级超出存档可写范围' }
  const legacyMainUnchanged = !traitByHash.value.has(mainHash) &&
    mainHash === (selected.value.mainTraitHash >>> 0) &&
    mainLevel === selected.value.mainTraitLevel
  const reasons = []
  if (!typeByHash.value.has(typeHash)) reasons.push('种类不在本地资料库中')
  if (!mainHash) reasons.push('主因子为空，游戏内通常不会自然生成')
  else if (!traitByHash.value.has(mainHash) && !legacyMainUnchanged) reasons.push('非天然主因子只能原样保留')
  if (subHash && !subParamByHash.value.has(subHash)) reasons.push('副参数不在本地资料库中')
  const max = traitByHash.value.get(mainHash)?.maxLevel
  if (Number.isInteger(max) && mainLevel > max) reasons.push(`主因子等级超过已知上限 ${max}`)
  if (reasons.length) return { status: 'impossible', writable: false, message: `${reasons.join('；')}；后端会拒绝这次写入` }
  if (legacyMainUnchanged) return { status: 'unknown', writable: true, message: '当前主因子为非天然或旧值；主因子已锁定，仍可修改副词条和 Rank' }
  return { status: 'unknown', writable: true, message: 'Hash 与等级可写；召唤石的完整天然主因子/副参数词池尚未完全验证' }
})

async function load() {
  if (loading.value || saving.value) return
  const epoch = ++lifecycleEpoch
  loading.value = true
  let acquiredOwnerToken = ''
  try {
    const info = await CharaAcquire(nextRuntimeAcquireRequestID())
    acquiredOwnerToken = String(info?.ownerToken || '')
    if (!acquiredOwnerToken) throw new Error('后端未返回召唤石连接所有权令牌')
    if (disposed || epoch !== lifecycleEpoch) {
      queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, CharaRelease)
      return
    }
    connectionOwnerToken = acquiredOwnerToken
    connected.value = true
    pid.value = info.pid
    const [catalog, items] = await Promise.all([SummonGetOptions(), SummonGetAll()])
    if (disposed || epoch !== lifecycleEpoch) return
    Object.assign(options, catalog || { types: [], traits: [], subParams: [] })
    summons.value = Array.isArray(items) ? items : []
    if (selectedIndex.value >= 0) select(summons.value.find((item) => item.index === selectedIndex.value), true)
  } catch (err) {
    let cleanupError = null
    if (acquiredOwnerToken) {
      try {
        await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, CharaRelease)
        if (connectionOwnerToken === acquiredOwnerToken) connectionOwnerToken = ''
      } catch (nextError) { cleanupError = nextError }
    }
    if (!cleanupError && connectionOwnerToken === '') {
      connected.value = false
      pid.value = 0
    }
    if (!disposed && epoch === lifecycleEpoch) {
      emit('status', cleanupError ? `${String(err)}；释放召唤石连接也失败：${String(cleanupError)}` : String(err), 'error')
    }
  } finally {
    if (!disposed && epoch === lifecycleEpoch) loading.value = false
  }
}

async function disconnect(silent = false) {
  if (loading.value || saving.value) return false
  const epoch = ++lifecycleEpoch
  const ownerToken = connectionOwnerToken
  loading.value = true
  try {
    if (ownerToken) await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, ownerToken, CharaRelease)
    if (disposed || epoch !== lifecycleEpoch) return false
    if (connectionOwnerToken === ownerToken) connectionOwnerToken = ''
    connected.value = false
    pid.value = 0
    summons.value = []
    selectedIndex.value = -1
    if (!silent) emit('status', '已断开召唤石内存连接', 'success')
    return true
  } catch (err) {
    if (!silent && !disposed && epoch === lifecycleEpoch) emit('status', String(err), 'error')
    return false
  } finally {
    if (!disposed && epoch === lifecycleEpoch) loading.value = false
  }
}

function select(item, force = false) {
  if (!item || (!force && (loading.value || saving.value))) return
  selectedIndex.value = item.index
  edit.typeHash = hex(item.typeHash)
  edit.mainTraitHash = hex(item.mainTraitHash)
  edit.subParamHash = hex(item.subParamHash)
  edit.mainTraitLevel = String(item.mainTraitLevel)
  edit.subParamLevel = String(item.subParamLevel)
  edit.rank = String(item.rank)
}

async function save() {
  if (loading.value || saving.value) return
  if (!selected.value) { emit('status', '请选择召唤石', 'error'); return }
  if (!legality.value.writable) { emit('status', legality.value.message, 'error'); return }
  let update
  try {
    const mainTraitHash = parseHash(edit.mainTraitHash, '主因子')
    const subParamHash = parseHash(edit.subParamHash, '副参数')
    update = {
      index: selected.value.index,
      typeHash: parseHash(edit.typeHash, '种类'),
      mainTraitHash,
      subParamHash,
      mainTraitLevel: Number.parseInt(edit.mainTraitLevel, 10),
      subParamLevel: Number.parseInt(edit.subParamLevel, 10),
      rank: Number.parseInt(edit.rank, 10),
    }
    if (!Number.isInteger(update.mainTraitLevel) || update.mainTraitLevel < 0 || update.mainTraitLevel > 0x7FFFFFFF) throw new Error('主因子等级超出存档可写范围')
    if (!Number.isInteger(update.subParamLevel) || update.subParamLevel < 0 || update.subParamLevel > subParamMaxLevel.value) throw new Error(`副参数等级必须为 0 到 ${subParamMaxLevel.value}`)
  } catch (err) {
    emit('status', String(err.message || err), 'error')
    return
  }
  const epoch = ++lifecycleEpoch
  saving.value = true
  try {
    const ownerToken = connectionOwnerToken
    if (!ownerToken) throw new Error('当前页面不再持有召唤石连接所有权')
    const updated = await SummonUpdateOwned(ownerToken, update)
    if (disposed || epoch !== lifecycleEpoch) return
    const index = summons.value.findIndex((item) => item.index === updated.index)
    if (index >= 0) summons.value.splice(index, 1, updated)
    select(updated, true)
    emit('status', '召唤石已写入并保存', 'success')
  } catch (err) {
    if (!disposed && epoch === lifecycleEpoch) emit('status', String(err), 'error')
  } finally {
    if (!disposed && epoch === lifecycleEpoch) saving.value = false
  }
}

onBeforeUnmount(() => {
  disposed = true
  lifecycleEpoch += 1
  const ownerToken = connectionOwnerToken
  connectionOwnerToken = ''
  if (ownerToken) queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, ownerToken, CharaRelease)
})
</script>

<template>
  <div class="summon-page ui-page is-wide ui-page-stack">
    <section class="summon-intro ui-card ui-panel">
      <header class="ui-split">
        <div class="intro-copy">
          <h2 class="ui-section-title">召唤石编辑</h2>
          <p class="ui-section-copy">打开游戏内召唤石背包后读取；种类只读，主因子与副词条可以修改。</p>
        </div>
        <span v-if="connected" class="pid ui-tag is-info">PID {{ pid }}</span>
      </header>
      <div class="summon-toolbar ui-toolbar">
        <button type="button" class="ui-btn is-primary" @click="load" :disabled="loading || saving">
          {{ loading ? '读取中...' : connected ? '刷新背包' : '连接背包' }}
        </button>
        <button v-if="connected" type="button" class="ui-btn is-ghost" @click="disconnect()" :disabled="loading || saving">
          断开连接
        </button>
        <span v-if="connected" class="ui-tag">{{ summons.length }} 颗召唤石</span>
      </div>
    </section>

    <section v-if="connected" class="workspace ui-card-grid">
      <aside class="summon-list-panel ui-card ui-panel is-compact">
        <div class="ui-split">
          <h3 class="ui-section-title">背包列表</h3>
          <span class="ui-tag">{{ filteredSummons.length }} / {{ summons.length }}</span>
        </div>
        <input v-model="filter" class="summon-search ui-input" placeholder="搜索名称、因子或 Hash" />
        <div class="summon-list ui-list ui-scroll-region">
          <button
            v-for="item in filteredSummons"
            :key="item.index"
            type="button"
            class="summon-row ui-row"
            :class="{ 'is-on': item.index === selectedIndex }"
            :disabled="loading || saving"
            @click="select(item)"
          >
            <img v-if="summonIcon(item)" class="summon-item-icon" :src="summonIcon(item)" alt="" />
            <span class="slot">#{{ item.index + 1 }}</span>
            <span class="name ui-truncate">{{ nameForType(item.typeHash) }}</span>
            <span class="rank ui-tag">{{ rarityLabel(item) }}</span>
          </button>
          <p v-if="!filteredSummons.length" class="ui-empty">
            {{ summons.length ? '无匹配召唤石，请更换搜索关键词。' : '未读取到召唤石，请打开游戏内召唤石背包后刷新。' }}
          </p>
        </div>
      </aside>

      <section class="summon-editor-panel ui-card ui-panel">
        <template v-if="selected">
          <header class="editor-head ui-split">
            <div class="editor-title-with-icon">
              <img v-if="summonIcon(selected)" :src="summonIcon(selected)" alt="" />
              <h3 class="ui-section-title ui-truncate">{{ nameForType(selected.typeHash) }}</h3>
            </div>
            <span class="ui-tag">槽位 #{{ selected.index + 1 }}</span>
          </header>

          <section class="field-group ui-card is-flat">
            <h4 class="group-title">召唤石信息</h4>
            <div class="ui-form-grid">
              <label class="ui-field">
                <span class="ui-field-label">种类</span>
                <select v-model="edit.typeHash" class="ui-select" disabled>
                  <option v-for="item in options.types" :key="item.hash" :value="hex(item.hash)">{{ optionLabel(item) }}</option>
                </select>
              </label>
              <label class="ui-field">
                <span class="ui-field-label">稀有度</span>
                <span class="readonly-value">{{ rarityLabel(selected) }}</span>
              </label>
            </div>
          </section>

          <section class="field-group ui-card is-flat">
            <h4 class="group-title">主因子</h4>
            <div class="ui-form-grid">
              <label class="ui-field wide-field">
                <span class="ui-field-label">搜索</span>
                <input v-model="traitFilter" class="ui-input" placeholder="按名称或 Hash 筛选" />
              </label>
              <label class="ui-field wide-field">
                <span class="ui-field-label">因子</span>
                <span class="trait-select-row">
                  <img v-if="currentTraitIcon()" :src="currentTraitIcon()" alt="" />
                  <select v-model="edit.mainTraitHash" class="ui-select" :disabled="loading || saving">
                    <option v-if="currentMainTraitIsLegacy" :value="edit.mainTraitHash">当前值（非天然） · {{ edit.mainTraitHash }}</option>
                    <option v-for="item in filteredTraits" :key="item.hash" :value="hex(item.hash)">{{ optionLabel(item) }}</option>
                  </select>
                </span>
              </label>
              <label class="ui-field wide-field">
                <span class="ui-field-label">主因子等级</span>
                <span class="number-combo ui-control-group">
                  <input v-model="edit.mainTraitLevel" class="ui-input" type="number" min="0" :max="traitMax(edit.mainTraitHash)" :disabled="loading || saving || currentMainTraitIsLegacy" />
                  <button type="button" class="ui-btn is-sm" :disabled="loading || saving || currentMainTraitIsLegacy" @click="edit.mainTraitLevel=String(traitMax(edit.mainTraitHash))">最大</button>
                </span>
              </label>
            </div>
          </section>

          <section class="field-group ui-card is-flat">
            <h4 class="group-title">副词条</h4>
            <div class="ui-form-grid">
              <label class="ui-field wide-field">
                <span class="ui-field-label">副词条</span>
                <select v-model="edit.subParamHash" class="ui-select" :disabled="loading || saving">
                  <option v-for="item in options.subParams" :key="item.hash" :value="hex(item.hash)">{{ optionLabel(item) }}</option>
                </select>
              </label>
              <label class="ui-field wide-field">
                <span class="ui-field-label">副参数等级</span>
                <span class="number-combo ui-control-group">
                  <select v-model="edit.subParamLevel" class="ui-select" :disabled="loading || saving">
                    <option v-for="level in (subParamMaxLevel + 1)" :key="level - 1" :value="String(level - 1)">{{ subParamValueLabel(level - 1) }}</option>
                  </select>
                  <button type="button" class="ui-btn is-sm" :disabled="loading || saving" @click="edit.subParamLevel=String(subParamMaxLevel)">最大</button>
                </span>
              </label>
            </div>
          </section>

          <div class="save-row ui-toolbar">
            <LegalityIndicator :status="legality.status" :message="legality.message" />
            <button type="button" class="ui-btn is-primary" @click="save" :disabled="loading || saving || !legality.writable">
              {{ saving ? '写入中...' : '写入召唤石' }}
            </button>
          </div>
        </template>
        <p v-else class="ui-empty">从左侧选择一颗召唤石开始编辑。</p>
      </section>
    </section>
  </div>
</template>

<style scoped>
.summon-page { padding-bottom:var(--space-8); }
.intro-copy { display:flex; min-width:0; flex-direction:column; gap:var(--space-2); }
.pid { flex:0 0 auto; font-family:var(--font-data); }
.summon-toolbar { padding:0; border:0; background:transparent; }
.workspace {
  --ui-grid-min:360px;
  grid-template-columns:minmax(280px,.8fr) minmax(0,1.2fr);
}
.summon-list-panel,
.summon-editor-panel { min-block-size:0; }
.summon-list { max-height:min(58vh,540px); padding-right:var(--space-1); }
.summon-row {
  width:100%;
  grid-template-columns:42px 52px minmax(0,1fr) auto;
  text-align:left;
  cursor:pointer;
}
.summon-item-icon { width:38px; height:38px; object-fit:contain; border:1px solid var(--line-soft); border-radius:7px; background:var(--surface-field); }
.slot { color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); }
.name { color:var(--text-primary); font-size:var(--fs-md); font-weight:var(--fw-semibold); }
.rank { justify-self:end; color:var(--accent-hover); }
.editor-head { padding-bottom:var(--space-3); border-bottom:1px solid var(--border-soft); }
.editor-title-with-icon { min-width:0; display:flex; align-items:center; gap:var(--space-3); }
.editor-title-with-icon img { width:52px; height:52px; flex:0 0 52px; object-fit:contain; border:1px solid var(--line-soft); border-radius:8px; background:var(--surface-field); }
.field-group { display:flex; flex-direction:column; gap:var(--space-4); padding:var(--space-4); background:var(--surface-sunken); }
.field-group .ui-form-grid { --ui-grid-min:180px; }
.group-title { margin:0; color:var(--text-primary); font-size:var(--fs-md); font-weight:var(--fw-bold); }
.wide-field { grid-column:1 / -1; }
.trait-select-row { display:flex; min-width:0; align-items:center; gap:var(--space-3); }
.trait-select-row img { width:38px; height:38px; flex:0 0 38px; object-fit:cover; border:1px solid var(--line-soft); border-radius:7px; background:var(--surface-field); }
.trait-select-row .ui-select { min-width:0; flex:1; }
.readonly-value { display:flex; min-height:var(--control-height); align-items:center; padding:0 var(--space-4); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-card-pop); color:var(--text-secondary); }
.number-combo > .ui-btn { flex:0 0 auto; }
.save-row { justify-content:space-between; align-items:center; }
.save-row > :first-child { min-width:0; flex:1 1 280px; }

@container ui-page (max-width:760px) {
  .workspace { grid-template-columns:minmax(0,1fr); }
  .summon-list { max-height:280px; }
}
@container ui-page (max-width:520px) {
  .summon-row { grid-template-columns:36px 42px minmax(0,1fr) auto; }
  .save-row { align-items:stretch; }
  .save-row > .ui-btn { width:100%; }
}
</style>
