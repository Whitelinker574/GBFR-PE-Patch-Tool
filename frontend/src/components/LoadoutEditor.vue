<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { LoadoutApply, LoadoutEditContext, LoadoutExport, LoadoutImport, LoadoutSimulate, MasteryNodePool, MasterySummarize } from '../../wailsjs/go/main/App'
import { groupMasteryNodes, resolveMasteryHashes } from '../loadoutMastery'
import ConfirmDialog from './ConfirmDialog.vue'

const props = defineProps({
  savePath: { type: String, default: '' },
  charaHash: { type: String, default: '' },
  charaName: { type: String, default: '' },
  loadouts: { type: Array, default: () => [] },
})
const emit = defineEmits(['status', 'reload'])

const confirmDialog = ref(null)
const ctx = ref(null)
const loading = ref(false)
const applying = ref(false)
const sharing = ref(false)

const targetSlot = ref(0)          // 目标预设槽 unitId
const op = ref('write')            // write | clone | clear
const form = ref({ name: '', weaponSlotId: 0, sigilSlotIds: [], skillHashes: [], masterySource: 0 })
const cloneFrom = ref(0)
const sigilSearch = ref('')

// 名称字节数（后端上限 63）
function utf8Bytes(s) { return new TextEncoder().encode(s || '').length }
const nameBytes = computed(() => utf8Bytes(form.value.name))
const nameTooLong = computed(() => nameBytes.value > 63)

const slots = computed(() => ctx.value?.slots || [])
const occupiedSlots = computed(() => slots.value.filter(s => s.occupied))
const masterySources = computed(() => ctx.value?.masterySources || [])
const filteredSigils = computed(() => {
  const query = sigilSearch.value.trim().toLocaleLowerCase()
  if (!query) return ctx.value?.sigils || []
  return (ctx.value?.sigils || []).filter(item => [item.name, item.primaryTraitName, item.secondaryTraitName]
    .some(value => String(value || '').toLocaleLowerCase().includes(query)))
})
const SKILL_ICONS = {
  '花耀七闪': 'https://cdn.gbf.wiki/relink/Flowery_Seven.png',
  '魔力漩涡': 'https://cdn.gbf.wiki/relink/Mystic_Vortex.png',
  '寒冰': 'https://cdn.gbf.wiki/relink/Freeze.png',
  '专注': 'https://cdn.gbf.wiki/relink/Concentration.png',
  '治愈之风': 'https://cdn.gbf.wiki/relink/Healing_Winds.png',
  '雷霆': 'https://cdn.gbf.wiki/relink/Lightning.png',
  '魔洞': 'https://cdn.gbf.wiki/relink/Gravity_Well.png',
  '火焰': 'https://cdn.gbf.wiki/relink/Fire.png',
}
function skillIcon(name) { return SKILL_ICONS[name] || '' }

// 专精：复制现有 or 自由配置（4 档 10/10/10/20）
const masteryMode = ref('free')     // copy | free
const masteryPool = ref([])         // [{rank,label,cap,nodes}]
const masteryPick = ref({})         // { R1:[hash...], R2:[], R3:[], EX:[] }
const masteryRankTab = ref('R1')
const masteryDirection = ref('')    // 2阶起锁定的专精方向
const CAT_ABBR = { SB_ATK: '攻', SB_DEF: '防', SB_LIMIT: '界' }
function catAbbr(cat) { return CAT_ABBR[cat] || '基' }
const activeRankPool = computed(() => masteryPool.value.find(p => p.rank === masteryRankTab.value) || null)
const activeRankGroups = computed(() => groupMasteryNodes(activeRankPool.value?.nodes || []))
function rankPicked(rank) { return (masteryPick.value[rank] || []).length }
const masteryTotal = computed(() => Object.values(masteryPick.value).reduce((n, a) => n + a.length, 0))
function toggleNode(rank, hash, cap) {
  const node = masteryNodeByHash.value.get(hash)
  if (masteryNodeDisabled(rank, node)) return
  const arr = masteryPick.value[rank] || (masteryPick.value[rank] = [])
  const i = arr.indexOf(hash)
  if (i >= 0) arr.splice(i, 1)
  else if (arr.length < cap) arr.push(hash)
}

function chooseMasteryDirection(cat) {
  masteryDirection.value = cat
  // 方向切换时只移除另两个方向的“具名专精技能”；普通子词条原样保留。
  for (const rank of ['R2', 'R3']) {
    masteryPick.value[rank] = (masteryPick.value[rank] || []).filter(hash => {
      const node = masteryNodeByHash.value.get(hash)
      return !node?.name || node.cat === cat
    })
  }
}

function masteryNodeDisabled(rank, node) {
  return ['R2', 'R3'].includes(rank) && !!node?.name && (!masteryDirection.value || node.cat !== masteryDirection.value)
}
async function loadMasteryPool() {
  masteryPool.value = []
  masteryPick.value = { R1: [], R2: [], R3: [], EX: [] }
  if (!ctx.value?.ownerCode) return
  try { masteryPool.value = (await MasteryNodePool(ctx.value.ownerCode)) || [] }
  catch (err) { emit('status', String(err), 'error') }
}

const selectedSlot = computed(() => slots.value.find(s => s.unitId === targetSlot.value) || null)
const selectedLoadout = computed(() => props.loadouts.find(item => item.unitId === targetSlot.value) || null)
const selectedMasteryHashes = computed(() => resolveMasteryHashes({
  mode: masteryMode.value,
  picks: masteryPick.value,
  sourceId: form.value.masterySource,
  sources: masterySources.value,
}))
const masteryNodeByHash = computed(() => {
  const result = new Map()
  for (const pool of masteryPool.value) for (const node of pool.nodes || []) result.set(node.hash, { ...node, rank: pool.rank, rankLabel: pool.label })
  return result
})
const selectedMasteryDetails = computed(() => selectedMasteryHashes.value.map(hash => masteryNodeByHash.value.get(hash)).filter(Boolean))
const selectedSkills = computed(() => form.value.skillHashes.map(hash => ctx.value?.skills?.find(skill => skill.hash === hash)).filter(Boolean))
const masterySummary = ref(null)
const summarizingMastery = ref(false)

// ── 配装模拟器：随所选因子实时算「词条加成汇总」 ──
const bonuses = ref([])
const simulating = ref(false)
let simTimer = null
let masteryTimer = null
function refreshSim() {
  clearTimeout(simTimer)
  simTimer = setTimeout(async () => {
    const ids = form.value.sigilSlotIds.filter(Boolean)
    if (!props.savePath || !ids.length) { bonuses.value = []; return }
    simulating.value = true
    try { bonuses.value = (await LoadoutSimulate(props.savePath, ids)) || [] }
    catch { bonuses.value = [] }
    finally { simulating.value = false }
  }, 180)
}
watch(() => form.value.sigilSlotIds.slice(), refreshSim, { deep: true })
const catClass = (label) => ({ '攻击类': 'atk', '基础能力': 'base', '防御类': 'def', '支援类': 'sup' }[label] || 'misc')

function refreshMasterySummary() {
  clearTimeout(masteryTimer)
  masteryTimer = setTimeout(async () => {
    const hashes = selectedMasteryHashes.value
    if (!ctx.value?.ownerCode || !hashes.length) { masterySummary.value = null; return }
    summarizingMastery.value = true
    try { masterySummary.value = await MasterySummarize(ctx.value.ownerCode, hashes) }
    catch { masterySummary.value = null }
    finally { summarizingMastery.value = false }
  }, 100)
}
watch(() => selectedMasteryHashes.value.slice(), refreshMasterySummary, { deep: true })

function setMasteryHashes(hashes) {
  masteryPick.value = { R1: [], R2: [], R3: [], EX: [] }
  for (const value of hashes || []) {
    const hash = typeof value === 'string' ? value : value.hash
    const rank = (typeof value === 'object' && value.rank) || masteryNodeByHash.value.get(hash)?.rank
    if (rank && masteryPick.value[rank]) masteryPick.value[rank].push(hash)
  }
  const r2Counts = new Map()
  for (const hash of masteryPick.value.R2) {
    const node = masteryNodeByHash.value.get(hash)
    if (node?.cat) r2Counts.set(node.cat, (r2Counts.get(node.cat) || 0) + 1)
  }
  masteryDirection.value = [...r2Counts.entries()].sort((a, b) => b[1] - a[1])[0]?.[0] || ''
}

function hydrateFromTarget() {
  const loadout = selectedLoadout.value
  form.value = {
    name: loadout?.name || '',
    weaponSlotId: loadout?.weaponSlotId || 0,
    sigilSlotIds: (loadout?.sigils || []).map(item => item.slotId).filter(Boolean),
    skillHashes: (loadout?.skills || []).map(item => item.hash).filter(Boolean),
    masterySource: loadout?.unitId || 0,
  }
  setMasteryHashes(loadout?.mastery || [])
  const fullestRank = ['R1', 'R2', 'R3', 'EX'].find(rank => rankPicked(rank) < (masteryPool.value.find(pool => pool.rank === rank)?.cap || 0))
  masteryRankTab.value = fullestRank || 'EX'
}

function selectTarget(unitId) {
  if (targetSlot.value === unitId) return
  targetSlot.value = unitId
  hydrateFromTarget()
}

async function loadCtx() {
  if (!props.savePath || !props.charaHash) return
  loading.value = true
  ctx.value = null
  try {
    ctx.value = await LoadoutEditContext(props.savePath, props.charaHash)
    await loadMasteryPool()
    // 默认打开该角色内容最完整的一套，真实存档可直接看到 12 因子、4 技能与 50 专精。
    const richest = [...props.loadouts].filter(item => !item.isParty).sort((a, b) =>
      (b.mastery?.length || 0) - (a.mastery?.length || 0) || (b.sigils?.length || 0) - (a.sigils?.length || 0)
    )[0]
    const empty = ctx.value.slots.find(s => !s.occupied)
    targetSlot.value = richest?.unitId || (empty || ctx.value.slots[0])?.unitId || 0
    if (occupiedSlots.value.length) cloneFrom.value = occupiedSlots.value[0].unitId
    if (masterySources.value.length) form.value.masterySource = masterySources.value[0].unitId
    hydrateFromTarget()
  } catch (err) {
    emit('status', String(err), 'error')
  } finally {
    loading.value = false
  }
}

watch(() => [props.savePath, props.charaHash], loadCtx, { immediate: true })

function toggleSigil(slotId) {
  const arr = form.value.sigilSlotIds
  const i = arr.indexOf(slotId)
  if (i >= 0) arr.splice(i, 1)
  else if (arr.length < 12) arr.push(slotId)
}
function toggleSkill(hash) {
  const arr = form.value.skillHashes
  const i = arr.indexOf(hash)
  if (i >= 0) arr.splice(i, 1)
  else if (arr.length < 4) arr.push(hash)
}

const writeInvalid = computed(() => {
  if (op.value === 'clear') return false
  if (op.value === 'clone') return !cloneFrom.value || cloneFrom.value === targetSlot.value
  return !form.value.name.trim() || nameTooLong.value || (masteryMode.value === 'free' && masteryTotal.value !== 0 && masteryTotal.value !== 50)
})

onBeforeUnmount(() => { clearTimeout(simTimer); clearTimeout(masteryTimer) })

function opLabel() {
  return op.value === 'write' ? '写入' : op.value === 'clone' ? '克隆' : '清空'
}

async function exportCurrentLoadout() {
  if (!selectedLoadout.value || selectedLoadout.value.isParty) return
  sharing.value = true
  try {
    const output = await LoadoutExport(props.savePath, selectedLoadout.value.unitId)
    if (output) emit('status', `已导出单套配装：${output}`, 'success')
  } catch (err) {
    emit('status', String(err), 'error')
  } finally {
    sharing.value = false
  }
}

async function importLoadout() {
  sharing.value = true
  try {
    const draft = await LoadoutImport(props.savePath, props.charaHash)
    if (!draft) return
    form.value.name = draft.name || form.value.name
    form.value.weaponSlotId = draft.weaponSlotId || 0
    form.value.sigilSlotIds = [...(draft.sigilSlotIds || [])]
    form.value.skillHashes = [...(draft.skillHashes || [])]
    masteryMode.value = 'free'
    setMasteryHashes(draft.masteryHashes || [])
    masteryRankTab.value = 'EX'
    if (draft.missing?.length) {
      emit('status', `已载入草稿，但当前存档缺少 ${draft.missing.length} 项资源：${draft.missing.join('；')}`, 'error')
    } else {
      emit('status', '单套配装已完整映射到当前存档；请核对右侧总计后选择目标槽写入', 'success')
    }
  } catch (err) {
    emit('status', String(err), 'error')
  } finally {
    sharing.value = false
  }
}

async function apply() {
  if (!props.savePath || !targetSlot.value) return
  const slotNo = selectedSlot.value?.slot ?? '?'
  const occupied = selectedSlot.value?.occupied
  const verb = op.value === 'clear' ? '清空' : (occupied ? '覆盖' : '写入')
  const detail = op.value === 'clear'
    ? `将清空【${props.charaName}·槽${String(slotNo).padStart(2, '0')}】的配装。`
    : `将${verb}【${props.charaName}·槽${String(slotNo).padStart(2, '0')}】的配装。请确认游戏已完全退出；工具会先自动备份原存档，再写入并回读验证。`
  const confirmed = await confirmDialog.value?.ask({
    title: '写入存档前确认',
    detail,
    tone: 'warning',
    confirmLabel: '备份并写入',
  })
  if (!confirmed) return

  const w = { unitId: targetSlot.value, expectCharaHash: props.charaHash, op: op.value }
  if (op.value === 'write') {
    w.name = form.value.name
    w.weaponSlotId = form.value.weaponSlotId || 0
    w.sigilSlotIds = [...form.value.sigilSlotIds]
    w.skillHashes = [...form.value.skillHashes]
    if (masteryMode.value === 'free') {
      w.masteryHashes = [...selectedMasteryHashes.value]
    } else {
      const src = masterySources.value.find(m => m.unitId === form.value.masterySource)
      w.masteryHashes = src ? [...src.nodeHashes] : []
    }
  } else if (op.value === 'clone') {
    w.cloneFromUnitId = cloneFrom.value
  }

  applying.value = true
  try {
    const res = await LoadoutApply(props.savePath, '', [w])
    emit('status', `已${opLabel()}并验证 ${res.verifiedFields} 项${res.backupPath ? '（已自动备份）' : ''}`, 'success')
    emit('reload')
    await loadCtx()
  } catch (err) {
    emit('status', String(err), 'error')
  } finally {
    applying.value = false
  }
}
</script>

<template>
  <div class="loadout-editor">
    <div v-if="loading" class="hint">正在读取该角色可用资源…</div>
    <template v-else-if="ctx">
      <div class="editor-layout">
      <main class="editor-form">
      <div class="ed-head">
        <strong>{{ charaName }}</strong>
        <span v-if="ctx.ownerCode" class="owner">{{ ctx.ownerCode }}</span>
        <span v-else class="owner warn">未能确定角色码（仅可用通用武器）</span>
        <span class="source-badge">真实存档 · {{ selectedLoadout ? '槽' + String(selectedLoadout.slot).padStart(2, '0') : '新配装' }}</span>
      </div>

      <!-- 目标槽 -->
      <div class="ed-field">
        <label>目标槽位</label>
        <div class="slot-grid">
          <button v-for="s in slots" :key="s.unitId" class="slot-btn" :class="{ on: targetSlot === s.unitId, occ: s.occupied }"
            @click="selectTarget(s.unitId)" :title="s.occupied ? s.name : '空槽'">
            {{ String(s.slot).padStart(2, '0') }}
          </button>
        </div>
        <small v-if="selectedSlot?.occupied" class="occ-warn">该槽已有配装「{{ selectedSlot.name || '(未命名)' }}」，写入将覆盖它</small>
      </div>

      <!-- 操作 -->
      <div class="ed-field">
        <label>操作</label>
        <div class="op-row">
          <button class="op-btn" :class="{ on: op === 'write' }" @click="op = 'write'">自定义写入</button>
          <button class="op-btn" :class="{ on: op === 'clone' }" @click="op = 'clone'" :disabled="!occupiedSlots.length">克隆现有</button>
          <button class="op-btn" :class="{ on: op === 'clear' }" @click="op = 'clear'">清空</button>
        </div>
      </div>

      <!-- 克隆源 -->
      <div v-if="op === 'clone'" class="ed-field">
        <label>克隆来源</label>
        <select v-model.number="cloneFrom" class="ed-select">
          <option v-for="s in occupiedSlots" :key="s.unitId" :value="s.unitId" :disabled="s.unitId === targetSlot">
            槽{{ String(s.slot).padStart(2, '0') }} · {{ s.name || '(未命名)' }}
          </option>
        </select>
      </div>

      <!-- 自定义写入表单 -->
      <template v-if="op === 'write'">
        <div class="ed-field">
          <label>配装名称 <em :class="{ over: nameTooLong }">{{ nameBytes }}/63 字节</em></label>
          <input v-model="form.name" class="ed-input" maxlength="30" placeholder="给这套配装起个名字" />
        </div>

        <div class="ed-field">
          <label>武器（{{ ctx.weapons.length }} 可选）</label>
          <select v-model.number="form.weaponSlotId" class="ed-select">
            <option :value="0">— 不设置武器 —</option>
            <option v-for="w in ctx.weapons" :key="w.slotId" :value="w.slotId">
              {{ w.name }}{{ w.ownerCode ? '' : '（通用）' }}
            </option>
          </select>
        </div>

        <div class="ed-field">
          <label>因子（{{ form.sigilSlotIds.length }}/12 · 池 {{ ctx.sigils.length }}）</label>
          <div class="bag-toolbar">
            <strong>从真实背包选择</strong>
            <input v-model="sigilSearch" placeholder="搜索因子或主 / 副词条" />
            <span>{{ filteredSigils.length }} 件</span>
          </div>
          <div class="pick-grid sigils">
            <button v-for="s in filteredSigils" :key="s.slotId" class="pick sigil-pick" :class="{ on: form.sigilSlotIds.includes(s.slotId) }"
              @click="toggleSigil(s.slotId)" :title="s.generic ? '通用因子' : '角色因子'">
              <span class="sigil-glyph">{{ s.generic ? '◇' : '◆' }}</span>
              <span class="sigil-copy"><b>{{ s.name }}</b><small>{{ s.primaryTraitName }} Lv{{ s.primaryTraitLevel }}<template v-if="s.secondaryTraitName"> · {{ s.secondaryTraitName }} Lv{{ s.secondaryTraitLevel }}</template></small></span>
              <i v-if="s.level">Lv{{ s.level }}</i>
            </button>
          </div>
        </div>

        <div class="ed-field">
          <label>技能（{{ form.skillHashes.length }}/4 · 池 {{ ctx.skills.length }}）</label>
          <div class="pick-grid">
            <button v-for="s in ctx.skills" :key="s.hash" class="pick" :class="{ on: form.skillHashes.includes(s.hash) }" @click="toggleSkill(s.hash)">
              <img v-if="skillIcon(s.name)" class="skill-icon" :src="skillIcon(s.name)" alt="" />{{ s.name || s.hash }}
            </button>
            <span v-if="!ctx.skills.length" class="empty">该角色现有配装未记录技能，无法自定义技能</span>
          </div>
        </div>

        <div class="ed-field">
          <label>专精 <em v-if="masteryMode === 'free'">已点 {{ masteryTotal }}/50</em></label>
          <div class="op-row">
            <button class="op-btn" :class="{ on: masteryMode === 'copy' }" @click="masteryMode = 'copy'">复制现有</button>
            <button class="op-btn" :class="{ on: masteryMode === 'free' }" @click="masteryMode = 'free'" :disabled="!ctx.ownerCode">自由配置</button>
          </div>

          <template v-if="masteryMode === 'copy'">
            <select v-model.number="form.masterySource" class="ed-select">
              <option :value="0">— 不设置专精 —</option>
              <option v-for="m in masterySources" :key="m.unitId" :value="m.unitId">
                复制自 槽{{ String(m.slot).padStart(2, '0') }}「{{ m.name || '未命名' }}」（{{ m.nodeCount }} 节点）
              </option>
            </select>
            <small class="hint">整套复制自该角色已有配装，保证游戏认可</small>
          </template>

          <template v-else>
            <div class="rank-tabs">
              <button v-for="p in masteryPool" :key="p.rank" class="rank-tab" :class="{ on: masteryRankTab === p.rank }" @click="masteryRankTab = p.rank">
                {{ p.label }}<i :class="{ full: rankPicked(p.rank) >= p.cap }">{{ rankPicked(p.rank) }}/{{ p.cap }}</i>
              </button>
            </div>
            <div v-if="['R2', 'R3'].includes(masteryRankTab)" class="direction-picker">
              <div>
                <strong>2阶起主方向</strong>
                <small>3阶必须沿用；其他方向只可配置普通子词条</small>
              </div>
              <button v-for="grp in activeRankGroups" :key="grp.cat" :class="{ on: masteryDirection === grp.cat }" @click="chooseMasteryDirection(grp.cat)">
                {{ grp.label }}
              </button>
            </div>
            <div v-if="masteryRankTab === 'R1'" class="mastery-rule">
              1阶三个方向互不排斥：每组达到 3 项即可激活该方向的1阶专精技能。
            </div>
            <div v-else-if="masteryRankTab === 'EX'" class="mastery-rule ex-rule">
              <b>官方名称：EX阶专精技能</b><span>MLv30→50 解锁 · 本阶仍按真谛、觉醒、秘义整理，不是第四种专精类型。</span>
            </div>
            <div v-if="activeRankPool" class="mastery-groups">
              <section v-for="grp in activeRankGroups" :key="grp.cat" class="mastery-group" :class="'cat-' + catAbbr(grp.cat)">
                <header>
                  <span class="cat-mark">{{ catAbbr(grp.cat) }}</span>
                  <strong>{{ grp.label }}</strong>
                  <em>{{ grp.nodes.filter(n => (masteryPick[activeRankPool.rank] || []).includes(n.hash)).length }}/{{ grp.nodes.length }}</em>
                  <i v-if="['R2', 'R3'].includes(activeRankPool.rank) && masteryDirection === grp.cat">主方向</i>
                </header>
                <div class="node-list">
                  <button v-for="n in grp.nodes" :key="n.hash" class="node"
                    :class="{ on: (masteryPick[activeRankPool.rank] || []).includes(n.hash), named: n.name, disabled: masteryNodeDisabled(activeRankPool.rank, n) }"
                    :disabled="masteryNodeDisabled(activeRankPool.rank, n)"
                    @click="toggleNode(activeRankPool.rank, n.hash, activeRankPool.cap)">
                    <span class="node-check">{{ (masteryPick[activeRankPool.rank] || []).includes(n.hash) ? '✓' : '' }}</span>
                    <span><b v-if="n.name">{{ n.name }}</b><small>{{ n.desc }}</small></span>
                    <em v-if="n.name">专精技能</em>
                  </button>
                </div>
              </section>
            </div>
            <small class="hint">满级配置必须为 10 / 10 / 10 / 20；页面展示的是解包专精节点与真实存档选择，写入前仍会校验角色归属和各阶配额。</small>
          </template>
        </div>
      </template>

      <div class="ed-actions">
        <button class="apply-btn" :disabled="applying || writeInvalid" @click="apply">
          {{ applying ? '写入中…' : opLabel() + '到槽' + String(selectedSlot?.slot ?? 0).padStart(2, '0') }}
        </button>
        <small class="safety">写入前自动备份 · 写后回读验证 · 建议先在副本存档上试</small>
      </div>
      </main>

      <aside class="result-sidebar">
        <section class="result-card result-overview">
          <header><strong>角色效果总计</strong><span>随当前槽实时更新</span></header>
          <div class="result-metrics">
            <div><b>{{ form.sigilSlotIds.length }}</b><span>/12 因子</span></div>
            <div><b>{{ form.skillHashes.length }}</b><span>/4 技能</span></div>
            <div><b>{{ selectedMasteryHashes.length }}</b><span>/50 专精</span></div>
          </div>
        </section>

        <section class="result-card">
          <header><strong>专精生效结果</strong><span>{{ summarizingMastery ? '解析中…' : (masterySummary?.primaryLabel || '未形成2阶主方向') }}</span></header>
          <div v-if="masterySummary" class="mastery-result">
            <article v-for="rank in masterySummary.ranks" :key="rank.rank" class="mastery-result-rank">
              <div class="rank-line"><b>{{ rank.label }}</b><em>{{ rank.count }}/{{ rank.cap }}</em></div>
              <template v-if="rank.rank !== 'EX'">
                <div v-for="cat in rank.categories" :key="cat.cat" class="mastery-result-cat" :class="{ active: cat.active }">
                  <span>{{ cat.specialization || cat.label }}</span>
                  <b>{{ cat.count }}/{{ cat.threshold }}</b>
                  <i>{{ cat.active ? '生效' : cat.reason }}</i>
                </div>
              </template>
              <div v-else class="ex-result">
                EX阶专精技能 {{ rank.count }}/{{ rank.cap }} · 按三方向归类，不构成第四专精
              </div>
            </article>
          </div>
          <small v-else class="hint">当前没有可解析的专精节点。</small>
        </section>

        <section class="result-card">
          <header><strong>总计加成</strong><span>{{ simulating ? '计算中…' : bonuses.length + ' 条' }}</span></header>
          <div v-if="bonuses.length" class="sim-list">
            <div v-for="b in bonuses" :key="b.traitId" class="sim-row" :class="catClass(b.catLabel)">
              <span class="sim-name">{{ b.name }}<i v-if="b.capped" class="sim-cap" title="已达词条上限">封顶</i></span>
              <span class="sim-lv">Lv{{ b.level }}<small v-if="b.rawLevel !== b.level">/{{ b.rawLevel }}</small></span>
              <span class="sim-eff">{{ b.effect }}</span>
            </div>
          </div>
          <small v-else-if="!simulating" class="hint">当前因子没有可汇总的词条。</small>
        </section>

        <section class="result-card">
          <header><strong>技能效果</strong><span>{{ selectedSkills.length }} 主动技能 · {{ selectedMasteryDetails.length }} 专精节点</span></header>
          <div class="skill-chips">
            <span v-for="skill in selectedSkills" :key="skill.hash">{{ skill.name || skill.hash }}</span>
            <i v-if="!selectedSkills.length">未配置主动技能</i>
          </div>
          <div v-if="selectedMasteryDetails.length" class="mastery-effect-list">
            <div v-for="node in selectedMasteryDetails" :key="node.hash" class="mastery-effect">
              <span>{{ node.rankLabel }} · {{ node.catLabel }}</span>
              <b v-if="node.name">{{ node.name }}</b>
              <p>{{ node.desc }}</p>
            </div>
          </div>
        </section>

        <section class="result-card share-card">
          <header><strong>分享单套配装</strong><span>不做批量 · 不包含存档</span></header>
          <p>导出时使用物品与主副词条指纹；导入只匹配当前存档已拥有的同一物品，并先载入为草稿。</p>
          <div class="share-actions">
            <button :disabled="sharing || !selectedLoadout || selectedLoadout.isParty" @click="exportCurrentLoadout">导出当前槽</button>
            <button :disabled="sharing" @click="importLoadout">导入单套配装</button>
          </div>
        </section>
      </aside>
      </div>
    </template>
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<style scoped>
.loadout-editor { min-width:0; }
.editor-layout { display:grid; grid-template-columns:minmax(0, 1.55fr) minmax(340px, .75fr); gap:14px; align-items:start; }
.editor-form { min-width:0; display:flex; flex-direction:column; gap:13px; }
.hint { font-size:.72rem; color:var(--text-muted); }
.ed-head { display:flex; align-items:baseline; gap:9px; }
.ed-head strong { font-size:.9rem; color:var(--text-primary); }
.ed-head .owner { font-size:.68rem; color:var(--gold); padding:1px 7px; border:1px solid var(--line-soft); border-radius:10px; }
.ed-head .owner.warn { color:var(--red); }
.source-badge { margin-left:auto; font-size:.66rem; font-weight:700; color:var(--text-muted); padding:3px 8px; border:1px solid var(--line-soft); border-radius:10px; background:var(--panel-solid); }
.ed-field { display:flex; flex-direction:column; gap:6px; }
.ed-field > label { font-size:.74rem; font-weight:700; color:var(--text-secondary); }
.ed-field > label em { font-style:normal; font-weight:600; color:var(--text-muted); margin-left:6px; }
.ed-field > label em.over { color:var(--red); }
.slot-grid { display:grid; grid-template-columns:repeat(auto-fill, minmax(38px, 1fr)); gap:5px; }
.slot-btn { min-height:30px; border:1px solid var(--line-soft); border-radius:5px; background:var(--sky-900); color:var(--text-muted); font-size:.72rem; cursor:pointer; user-select:none; }
.slot-btn.occ { color:var(--text-primary); border-color:var(--line-gold); }
.slot-btn.on { border-color:#765126; background:#8b6737; color:#fff9e9; }
.occ-warn { font-size:.68rem; color:var(--amber); }
.op-row, .ed-actions { display:flex; flex-wrap:wrap; gap:7px; align-items:center; }
.op-btn { min-height:30px; padding:0 13px; border:1px solid var(--line-gold); border-radius:6px; background:var(--sky-900); color:var(--text-primary); font-size:.76rem; cursor:pointer; user-select:none; }
.op-btn.on { border-color:#765126; background:#8b6737; color:#fff9e9; }
.op-btn:disabled { opacity:.4; cursor:not-allowed; }
.ed-input, .ed-select { min-height:32px; padding:0 10px; border:1px solid var(--line-gold); border-radius:6px; background:var(--panel-solid); color:var(--text-primary); font-size:.78rem; }
.ed-input:focus, .ed-select:focus { outline:2px solid rgba(154,116,64,.5); outline-offset:1px; }
.pick-grid { display:flex; flex-wrap:wrap; gap:6px; max-height:180px; overflow-y:auto; padding:2px; }
.pick-grid.sigils { max-height:300px; display:grid; grid-template-columns:repeat(auto-fill,minmax(210px,1fr)); align-items:start; }
.pick { display:inline-flex; align-items:center; gap:5px; padding:3px 9px; border:1px solid var(--line-soft); border-radius:12px; background:var(--panel-solid); color:var(--text-secondary); font-size:.72rem; cursor:pointer; user-select:none; }
.pick:hover { border-color:var(--line-gold); }
.pick.on { border-color:#765126; background:#8b6737; color:#fff9e9; }
.pick i { font-style:normal; margin-left:5px; opacity:.7; font-size:.64rem; }
.skill-icon { width:22px; height:22px; flex:0 0 22px; object-fit:cover; border-radius:4px; box-shadow:0 0 0 1px rgba(118,81,38,.28); }
.bag-toolbar { display:grid; grid-template-columns:auto minmax(150px,1fr) auto; gap:8px; align-items:center; padding:7px 8px; border:1px solid var(--line-soft); border-radius:7px; background:var(--panel-solid); }
.bag-toolbar strong { font-size:.67rem; color:var(--text-primary); }
.bag-toolbar input { min-height:28px; padding:0 8px; border:1px solid var(--line-soft); border-radius:5px; background:var(--panel); color:var(--text-primary); font-size:.68rem; }
.bag-toolbar input:focus { outline:1px solid var(--line-gold); }
.bag-toolbar span { font-size:.62rem; color:var(--text-muted); }
.sigil-pick { min-width:0; min-height:48px; display:grid; grid-template-columns:24px minmax(0,1fr) auto; gap:7px; align-items:center; padding:6px 8px; border-radius:7px; text-align:left; }
.sigil-glyph { width:23px; height:23px; display:grid; place-items:center; border:1px solid var(--line-gold); border-radius:50%; background:rgba(139,103,55,.08); color:var(--gold); font-size:.78rem; }
.sigil-copy { min-width:0; display:flex; flex-direction:column; gap:2px; }
.sigil-copy b { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-primary); font-size:.68rem; }
.sigil-copy small { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-muted); font-size:.59rem; }
.sigil-pick.on .sigil-copy b, .sigil-pick.on .sigil-copy small, .sigil-pick.on .sigil-glyph { color:inherit; }
.empty { font-size:.7rem; color:var(--text-muted); }
.apply-btn { min-height:34px; padding:0 18px; border:1px solid #765126; border-radius:6px; background:#8b6737; color:#fff9e9; font-size:.8rem; font-weight:700; cursor:pointer; }
.apply-btn:hover:not(:disabled) { background:#76552d; }
.apply-btn:disabled { opacity:.45; cursor:not-allowed; }
.safety { font-size:.66rem; color:var(--text-muted); }
/* 右侧角色总计：始终保留因子加成、技能与专精效果 */
.result-sidebar { position:sticky; top:8px; min-width:0; max-height:calc(100vh - 116px); overflow-y:auto; display:flex; flex-direction:column; gap:10px; padding-right:2px; }
.result-card { min-width:0; padding:11px; border:1px solid var(--line); border-radius:8px; background:linear-gradient(155deg, var(--panel-solid), var(--sky-900)); box-shadow:0 5px 16px rgba(27,35,44,.06); }
.result-card > header { display:flex; align-items:baseline; justify-content:space-between; gap:8px; margin-bottom:8px; }
.result-card > header strong { font-size:.76rem; color:var(--text-primary); }
.result-card > header span { font-size:.64rem; color:var(--text-muted); text-align:right; }
.result-overview { border-color:var(--line-gold); background:linear-gradient(145deg, rgba(139,103,55,.12), var(--panel-solid)); }
.result-metrics { display:grid; grid-template-columns:repeat(3,1fr); gap:6px; }
.result-metrics div { display:flex; align-items:baseline; justify-content:center; gap:2px; padding:7px 4px; border:1px solid var(--line-soft); border-radius:6px; background:var(--panel); }
.result-metrics b { font-size:1rem; color:var(--gold); font-variant-numeric:tabular-nums; }
.result-metrics span { font-size:.62rem; color:var(--text-muted); }
.sim-list { display:flex; flex-direction:column; gap:2px; max-height:320px; overflow-y:auto; border:1px solid var(--border-soft); border-radius:var(--radius-md); background:var(--surface-card-pop); padding:4px; }
.sim-row { display:grid; grid-template-columns:minmax(5.5em,.8fr) 3.5em minmax(0,1.5fr); gap:7px; align-items:baseline; padding:4px 7px; border-radius:var(--radius-sm); font-size:.68rem; border-left:3px solid var(--border-soft); }
.sim-row.atk { border-left-color:var(--danger); }
.sim-row.base { border-left-color:var(--accent); }
.sim-row.def { border-left-color:var(--info); }
.sim-row.sup { border-left-color:var(--success); }
.sim-row:nth-child(even) { background:var(--surface-row); }
.sim-name { font-weight:800; color:var(--text-primary); white-space:nowrap; }
.sim-cap { font-style:normal; margin-left:4px; font-size:.6rem; color:var(--warning-ink); }
.sim-lv { color:var(--text-secondary); font-variant-numeric:tabular-nums; }
.sim-lv small { color:var(--text-muted); }
.sim-eff { color:var(--text-secondary); white-space:pre-line; line-height:1.4; }
.skill-chips { display:flex; flex-wrap:wrap; gap:5px; margin-bottom:8px; }
.skill-chips span { padding:3px 8px; border:1px solid var(--line-gold); border-radius:12px; background:rgba(139,103,55,.1); color:var(--text-primary); font-size:.68rem; }
.skill-chips i { font-style:normal; color:var(--text-muted); font-size:.68rem; }
.mastery-effect-list { max-height:390px; overflow-y:auto; display:flex; flex-direction:column; gap:4px; padding-right:2px; }
.mastery-effect { padding:6px 7px; border-left:3px solid var(--line-gold); border-radius:4px; background:var(--surface-row); }
.mastery-effect > span { display:block; font-size:.59rem; color:var(--text-muted); }
.mastery-effect > b { display:block; margin-top:2px; font-size:.67rem; color:var(--gold); }
.mastery-effect > p { margin:2px 0 0; font-size:.67rem; line-height:1.4; color:var(--text-secondary); }
.share-card > p { margin:0 0 8px; font-size:.64rem; line-height:1.5; color:var(--text-muted); }
.share-actions { display:grid; grid-template-columns:1fr 1fr; gap:6px; }
.share-actions button { min-height:30px; border:1px solid var(--line-gold); border-radius:6px; background:var(--panel); color:var(--text-primary); font-size:.68rem; cursor:pointer; }
.share-actions button:first-child { background:#8b6737; color:#fff9e9; }
.share-actions button:disabled { opacity:.42; cursor:not-allowed; }
.mastery-result { display:flex; flex-direction:column; gap:7px; }
.mastery-result-rank { padding-top:6px; border-top:1px dashed var(--line-soft); }
.mastery-result-rank:first-child { padding-top:0; border-top:0; }
.rank-line { display:flex; justify-content:space-between; align-items:center; margin-bottom:4px; }
.rank-line b { font-size:.68rem; color:var(--text-primary); }
.rank-line em { font-style:normal; font-size:.62rem; color:var(--gold); font-variant-numeric:tabular-nums; }
.mastery-result-cat { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:1px 7px; padding:3px 5px; border-radius:4px; opacity:.68; }
.mastery-result-cat.active { opacity:1; background:rgba(63,125,92,.1); }
.mastery-result-cat span { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-size:.64rem; color:var(--text-secondary); }
.mastery-result-cat b { font-size:.62rem; color:var(--text-muted); }
.mastery-result-cat i { grid-column:1/-1; font-style:normal; font-size:.58rem; color:var(--text-muted); }
.mastery-result-cat.active i { color:#3f7d5c; font-weight:800; }
.ex-result { padding:5px 6px; border-radius:4px; background:rgba(139,103,55,.09); font-size:.63rem; line-height:1.45; color:var(--text-secondary); }
/* 专精自由配置 */
.rank-tabs { display:flex; flex-wrap:wrap; gap:6px; }
.rank-tab { min-height:28px; padding:0 12px; border:1px solid var(--border-strong); border-radius:var(--radius-sm); background:var(--surface-card-pop); color:var(--text-secondary); font-size:.74rem; cursor:pointer; user-select:none; }
.rank-tab.on { background:var(--selected-bg); border-color:var(--selected-border); color:var(--selected-fg); }
.rank-tab i { font-style:normal; margin-left:5px; font-size:.64rem; opacity:.8; }
.rank-tab i.full { color:var(--success); font-weight:800; }
.rank-tab.on i.full { color:#bfe6cd; }
.direction-picker { display:flex; flex-wrap:wrap; align-items:center; gap:6px; padding:8px; border:1px solid var(--line-soft); border-radius:7px; background:var(--panel-solid); }
.direction-picker > div { margin-right:auto; display:flex; flex-direction:column; gap:1px; }
.direction-picker strong { font-size:.68rem; color:var(--text-primary); }
.direction-picker small { font-size:.6rem; color:var(--text-muted); }
.direction-picker button { min-height:27px; padding:0 10px; border:1px solid var(--line-soft); border-radius:5px; background:var(--panel); color:var(--text-secondary); font-size:.68rem; cursor:pointer; }
.direction-picker button.on { border-color:#765126; background:#8b6737; color:#fff9e9; }
.mastery-rule { display:flex; gap:6px; align-items:center; padding:7px 9px; border:1px solid var(--line-soft); border-radius:6px; background:rgba(63,125,92,.07); color:var(--text-secondary); font-size:.66rem; line-height:1.45; }
.mastery-rule.ex-rule { flex-direction:column; align-items:flex-start; border-color:var(--line-gold); background:rgba(139,103,55,.08); }
.mastery-rule b { color:var(--gold); }
.mastery-groups { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:7px; max-height:510px; overflow-y:auto; padding:2px; }
.mastery-group { min-width:0; padding:7px; border:1px solid var(--line-soft); border-radius:7px; background:var(--panel-solid); }
.mastery-group > header { display:flex; align-items:center; gap:5px; margin-bottom:6px; }
.mastery-group > header .cat-mark { width:17px; height:17px; display:grid; place-items:center; border-radius:4px; color:#fff; font-size:.59rem; font-weight:900; }
.mastery-group.cat-攻 .cat-mark { background:var(--danger); }
.mastery-group.cat-防 .cat-mark { background:var(--info); }
.mastery-group.cat-界 .cat-mark { background:var(--accent); }
.mastery-group > header strong { font-size:.7rem; color:var(--text-primary); }
.mastery-group > header em { margin-left:auto; font-style:normal; font-size:.61rem; color:var(--text-muted); }
.mastery-group > header i { font-style:normal; padding:1px 5px; border-radius:8px; background:#3f7d5c; color:#fff; font-size:.56rem; }
.node-list { display:flex; flex-direction:column; gap:4px; }
.node { width:100%; display:grid; grid-template-columns:16px minmax(0,1fr) auto; gap:6px; align-items:start; padding:6px; border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-card-pop); color:var(--text-secondary); text-align:left; cursor:pointer; user-select:none; }
.node-check { width:15px; height:15px; display:grid; place-items:center; margin-top:1px; border:1px solid var(--line-soft); border-radius:3px; color:#fff; font-size:.6rem; }
.node > span:nth-child(2) { min-width:0; display:flex; flex-direction:column; gap:2px; }
.node b { font-size:.65rem; color:var(--gold); }
.node small { font-size:.62rem; line-height:1.38; color:var(--text-secondary); }
.node > em { font-style:normal; font-size:.55rem; color:var(--gold); white-space:nowrap; }
.node.on { border-color:var(--selected-border); background:var(--selected-bg); color:var(--selected-fg); }
.node.on .node-check { border-color:rgba(255,255,255,.55); background:rgba(255,255,255,.22); }
.node.on b, .node.on small, .node.on > em { color:inherit; }
.node.disabled { opacity:.42; cursor:not-allowed; }
.ed-field .hint { font-size:.66rem; color:var(--text-muted); }
@media (max-width:1180px) {
  .editor-layout { grid-template-columns:1fr; }
  .result-sidebar { position:static; max-height:none; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); }
}
@media (max-width:760px) {
  .mastery-groups, .result-sidebar { grid-template-columns:1fr; }
  .source-badge { width:100%; margin-left:0; }
  .ed-head { flex-wrap:wrap; }
}
</style>
