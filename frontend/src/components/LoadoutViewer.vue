<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { FindSaveFiles, LoadoutList, LoadoutPreviewList, SelectLogsLoadoutShares, SelectProgressionSave } from '../../wailsjs/go/backend/App'
import { characterAssetIcon, traitAssetIcon, weaponAssetIcon } from '../gameAssetIcons'
import skillIconFiles from '../loadoutSkillIcons.json'
import CapturedLoadoutPreview from './CapturedLoadoutPreview.vue'
import LoadoutEditor from './LoadoutEditor.vue'

const props = defineProps({ pendingImport: { type: Object, default: null } })
const emit = defineEmits(['status', 'editing-change', 'import-consumed'])

const slots = ref([])
const savePath = ref('')
const loading = ref(false)
const groups = ref([])
const selectedChara = ref('')
const expanded = ref(new Set())
const mode = ref('view') // view | edit | logs | logs-preview
const previews = ref(new Map())
const previewLoading = ref(false)
const logsCandidates = ref([])
const logsLoading = ref(false)
const selectedLogsCandidate = ref(null)
const viewerRoot = ref(null)
const logsPendingImport = ref(null)
let previewRequestId = 0

const CAT_LABELS = { SB_ATK: '真谛（攻击盘）', SB_DEF: '觉醒（防御盘）', SB_LIMIT: '秘义（界限盘）' }

function catLabel(cat) { return CAT_LABELS[cat] || '基础盘' }
function assetPath(folder, file) {
  if (!file) return ''
  return `/loadout-icons/${folder}/${String(file).split('/').map(part => encodeURIComponent(part).replace(/'/g, '%27')).join('/')}`
}
function skillIcon(skill) {
  const verifiedFile = skillIconFiles[skill?.key || ''] || ''
  return assetPath('skills', verifiedFile || 'Plain_Skill_Frame.png')
}
function traitIcon(name, hash = '') { return traitAssetIcon({ name, hash }) }

const currentGroup = computed(() => groups.value.find(g => g.charaName === selectedChara.value) || null)
const isEditing = computed(() => mode.value === 'edit' && !!currentGroup.value)
const effectivePendingImport = computed(() => props.pendingImport || logsPendingImport.value)
const pendingCodeForEditor = computed(() => {
  if (!effectivePendingImport.value?.code || !currentGroup.value) return ''
  return String(currentGroup.value.charaHash).toUpperCase() === String(effectivePendingImport.value.charaHash || '').toUpperCase()
    ? effectivePendingImport.value.code
    : ''
})
const presetCount = computed(() => {
  let n = 0
  for (const g of groups.value) for (const lo of g.loadouts) if (!lo.isParty) n++
  return n
})
function previewFor(loadout) { return previews.value.get(loadout?.unitId) || null }
function formatNumber(value, digits = 0) {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return '—'
  return numeric.toLocaleString('zh-CN', { maximumFractionDigits: digits })
}
async function loadPreviews() {
  if (!savePath.value || !currentGroup.value) return
  const path = savePath.value
  const charaHash = currentGroup.value.charaHash
  const requestId = ++previewRequestId
  previewLoading.value = true
  try {
    const result = await LoadoutPreviewList(path, charaHash)
    if (requestId !== previewRequestId || path !== savePath.value || charaHash !== currentGroup.value?.charaHash) return
    previews.value = new Map((result || []).map(entry => [entry.unitId, entry]))
  } catch (err) {
    if (requestId !== previewRequestId) return
    previews.value = new Map()
    emit('status', `配装数值预览失败：${String(err)}`, 'error')
  } finally {
    if (requestId === previewRequestId) previewLoading.value = false
  }
}

function masterySummary(lo) {
  const order = ['R1', 'R2', 'R3', 'EX']
  const byRank = new Map()
  for (const m of lo.mastery || []) {
    const rank = m.rank || 'unknown'
    const current = byRank.get(rank) || { rank, label: m.rankLabel || rank, count: 0 }
    current.count += 1
    byRank.set(rank, current)
  }
  return [...byRank.values()].sort((a, b) => order.indexOf(a.rank) - order.indexOf(b.rank))
}

function masteryGrouped(lo) {
  const byRankAndCat = new Map()
  for (const m of lo.mastery || []) {
    const key = `${m.rank || 'unknown'}:${m.cat || ''}`
    if (!byRankAndCat.has(key)) byRankAndCat.set(key, { key, rankLabel: m.rankLabel || m.rank, cat: m.cat, nodes: [] })
    byRankAndCat.get(key).nodes.push(m)
  }
  return [...byRankAndCat.values()]
}

function toggle(lo) {
  const next = new Set(expanded.value)
  if (next.has(lo.unitId)) next.delete(lo.unitId)
  else next.add(lo.unitId)
  expanded.value = next
}

function enterEdit() {
  if (!savePath.value || !currentGroup.value) return
  mode.value = 'edit'
}

function leaveEdit() {
  mode.value = 'view'
}

async function activatePendingImport() {
  const pending = effectivePendingImport.value
  if (!pending?.code) return
  if (!savePath.value || !groups.value.length) {
	selectedLogsCandidate.value = null
	mode.value = 'view'
	emit('status', `已暂存 ${pending.charaName || '角色'} 配装，请先选择目标存档`, 'info')
	return
  }
  const target = groups.value.find(group => String(group.charaHash).toUpperCase() === String(pending.charaHash || '').toUpperCase())
  if (!target) {
    emit('status', `目标存档没有 ${pending.charaName || pending.charaHash} 的配装槽`, 'error')
    return
  }
  selectedChara.value = target.charaName
  mode.value = 'edit'
  await nextTick()
}

async function browseLogsLoadouts() {
	if (logsLoading.value) return
	logsLoading.value = true
	try {
		logsCandidates.value = await SelectLogsLoadoutShares() || []
		mode.value = 'logs'
	} catch (error) {
		emit('status', String(error), 'error')
	} finally {
		logsLoading.value = false
	}
}

function openLogsLibrary() {
	selectedLogsCandidate.value = null
	mode.value = 'logs'
}

function closeLogsLibrary() {
	selectedLogsCandidate.value = null
	mode.value = 'view'
}

function previewLogsCandidate(candidate) {
	selectedLogsCandidate.value = candidate
	mode.value = 'logs-preview'
	nextTick(() => viewerRoot.value?.closest('.tool-center-scroll,.workspace-scroll')?.scrollTo({ top: 0 }))
}

function leaveLogsPreview() {
	selectedLogsCandidate.value = null
	mode.value = 'logs'
}

async function deployLogsCandidate(candidate) {
	logsPendingImport.value = {
		code: candidate.compatibilityCode,
		charaHash: candidate.characterHash,
		charaName: candidate.characterName,
		requestId: Date.now(),
	}
	await activatePendingImport()
}

function consumePendingImport() {
	if (logsPendingImport.value) logsPendingImport.value = null
	else emit('import-consumed')
}

watch(isEditing, value => emit('editing-change', value), { immediate: true })
watch(currentGroup, value => {
  if (!value && mode.value === 'edit') mode.value = 'view'
  previews.value = new Map()
  previewRequestId++
  previewLoading.value = false
  if (value && savePath.value) loadPreviews()
})

async function load(path) {
  if (!path) return
  loading.value = true
  try {
    const result = await LoadoutList(path)
    groups.value = result || []
    // 展开态按 unitId 记录，而 unitId 在不同存档间会复用，
    // 换档后必须清空，否则新档的同号卡片会凭空展开。
    if (savePath.value !== path) expanded.value = new Set()
    savePath.value = path
    if (!groups.value.find(g => g.charaName === selectedChara.value)) {
      const richest = [...groups.value].sort((a, b) => {
        const score = group => Math.max(0, ...(group.loadouts || []).filter(item => !item.isParty).map(item =>
          (item.mastery?.length || 0) * 100 + (item.sigils?.length || 0) * 10 + (item.skills?.length || 0)
        ))
        return score(b) - score(a)
      })[0]
      selectedChara.value = richest?.charaName || ''
    }
    emit('status', `已读取 ${groups.value.length} 个角色、${presetCount.value} 套配装预设`, 'success')
		await activatePendingImport()
  } catch (err) {
    emit('status', String(err), 'error')
  } finally {
    loading.value = false
  }
}

async function browse() {
  try {
    const path = await SelectProgressionSave()
    if (path) await load(path)
  } catch (err) {
    emit('status', String(err), 'error')
  }
}

onMounted(async () => {
  try {
    slots.value = (await FindSaveFiles()) || []
  } catch { /* 找不到默认存档目录时静默，仍可手动浏览 */ }
})

watch(() => effectivePendingImport.value?.requestId, () => { activatePendingImport() })

</script>

<template>
  <div ref="viewerRoot" class="loadout-viewer ui-page is-wide ui-page-stack" :class="{ editing: isEditing }">
    <section v-if="isEditing" class="editor-workspace ui-page is-fluid" aria-label="配装编辑工作区">
      <header class="editor-workspace-bar ui-card">
        <button type="button" class="back-button ui-btn" @click="leaveEdit">
          <span aria-hidden="true">←</span> 返回配装列表
        </button>
        <div class="editor-workspace-title">
          <small>角色配装工作台</small>
          <strong>{{ currentGroup.charaName }}</strong>
          <span>构造或从真实背包选择资源，配置因子、技能与三方向专精</span>
        </div>
        <div class="editor-workspace-meta">
          <span class="preset-count-badge"><b>{{ currentGroup.loadouts.filter(l => !l.isParty).length }}</b> 套已有预设</span>
          <small :title="savePath">{{ savePath }}</small>
        </div>
      </header>
      <div class="editor-workspace-content">
        <LoadoutEditor :save-path="savePath" :chara-hash="currentGroup.charaHash" :chara-name="currentGroup.charaName"
          :loadouts="currentGroup.loadouts" :pending-import-code="pendingCodeForEditor"
          @import-consumed="consumePendingImport" @status="(m, t) => emit('status', m, t)" @reload="load(savePath)" />
      </div>
    </section>

	<section v-else-if="mode === 'logs-preview' && selectedLogsCandidate" class="logs-workspace" aria-label="Logs 配装预览">
	  <header class="subpage-bar ui-card">
		<button type="button" class="back-button ui-btn" @click="leaveLogsPreview"><span aria-hidden="true">←</span> 返回 Logs 配装库</button>
		<div><small>GBFR Logs 配装预览</small><strong>{{ selectedLogsCandidate.characterName }} · {{ selectedLogsCandidate.playerName || '未记录玩家名' }}</strong></div>
	  </header>
	  <CapturedLoadoutPreview :loadout="selectedLogsCandidate.preview" source-label="GBFR Logs v1 · 最终导入预览">
		<template #actions>
		  <button type="button" class="ui-btn is-primary" @click="deployLogsCandidate(selectedLogsCandidate)">选择存档并导入</button>
		</template>
	  </CapturedLoadoutPreview>
	</section>

	<section v-else-if="mode === 'logs'" class="logs-workspace" aria-label="GBFR Logs 配装库">
	  <header class="subpage-bar logs-library-bar ui-card">
		<button type="button" class="back-button logs-back-button ui-btn is-ghost" @click="closeLogsLibrary"><span aria-hidden="true">←</span> 返回配装预设</button>
		<div class="logs-header-copy"><small>多角色配装导入</small><strong>GBFR Logs 配装库</strong><span>一次读取日志中的全部队员；先预览每名角色，再选择是否部署到存档。</span></div>
		<button type="button" class="logs-source-button ui-btn is-primary" :disabled="logsLoading" @click="browseLogsLoadouts">{{ logsLoading ? '正在解析…' : logsCandidates.length ? '更换 Logs 数据库' : '选择 Logs 数据库' }}</button>
	  </header>
	  <div v-if="logsCandidates.length" class="logs-candidate-grid">
		<article v-for="candidate in logsCandidates" :key="`${candidate.logTime}-${candidate.playerName}-${candidate.ownerCode}`" class="logs-candidate-card ui-card is-flat">
		  <header>
			<img v-if="characterAssetIcon(candidate.characterHash)" :src="characterAssetIcon(candidate.characterHash)" alt="" />
			<span v-else class="logs-character-fallback" aria-hidden="true">◇</span>
			<div><small>{{ candidate.playerName || '未记录玩家名' }}</small><strong>{{ candidate.characterName }}</strong><span>{{ new Date(candidate.logTime).toLocaleString('zh-CN') }}</span></div>
		  </header>
		  <dl>
			<div><dt>武器</dt><dd>{{ candidate.weaponName || '未记录' }}</dd></div>
			<div><dt>因子</dt><dd>{{ candidate.sigilCount }} / 12</dd></div>
			<div><dt>上限突破</dt><dd>{{ candidate.overLimitCount }} / 4</dd></div>
		  </dl>
		  <div class="logs-card-actions">
			<button type="button" class="ui-btn" @click="previewLogsCandidate(candidate)">预览实际配装</button>
			<button type="button" class="ui-btn is-primary" @click="deployLogsCandidate(candidate)">导入到存档</button>
		  </div>
		</article>
	  </div>
	  <div v-else class="logs-empty ui-empty"><strong>选择一个 GBFR Logs 数据库</strong><span>数据库中的不同场次与角色会分别解析成可预览的配装卡片。</span></div>
	</section>

    <template v-else>
	  <section v-if="effectivePendingImport?.code" class="section ui-notice is-info"><strong>已捕获 {{ effectivePendingImport.charaName || '角色' }} 配装</strong><span>选择目标存档后将自动进入分项导入，不会直接写档。</span></section>
      <section class="section ui-card ui-panel">
        <div class="section-title ui-section-title"><span>选择存档</span><small>读取游戏内保存的配装预设（每角色 15 槽）</small></div>
        <div class="save-row ui-actions">
          <button v-for="slot in slots" :key="slot.path" class="action ui-btn" :class="{ 'is-primary': savePath === slot.path }" :disabled="loading" @click="load(slot.path)">存档位 {{ slot.index }}</button>
          <button class="action ui-btn" :disabled="loading" @click="browse">浏览…</button>
          <button class="action ui-btn is-ghost" :disabled="loading || !savePath" @click="load(savePath)">刷新</button>
        </div>
        <div v-if="savePath" class="path-line ui-hint ui-truncate" :title="savePath">{{ savePath }}</div>
      </section>

	  <button type="button" class="logs-library-entry ui-card" @click="openLogsLibrary">
		<span class="logs-entry-mark" aria-hidden="true">L</span>
		<span><small>外部战斗记录</small><strong>从 GBFR Logs 批量获取队伍配装</strong><em>独立解析数据库中的多名角色，可逐个预览后再导入。</em></span>
		<b aria-hidden="true">→</b>
	  </button>

      <section v-if="groups.length" class="section ui-card ui-panel">
        <div class="section-title ui-section-title">
          <span>角色</span><small>{{ groups.length }} 个角色 · {{ presetCount }} 套预设</small>
          <button type="button" class="edit-launch ui-btn is-primary" :disabled="!currentGroup" @click="enterEdit">
            编辑 {{ currentGroup?.charaName || '' }} 配装 <span aria-hidden="true">→</span>
          </button>
        </div>
        <div class="chara-row">
          <button v-for="g in groups" :key="g.charaHash" class="chara-chip ui-chip" :class="{ 'is-on': selectedChara === g.charaName }" @click="selectedChara = g.charaName">
            <img v-if="characterAssetIcon(g.charaHash)" :src="characterAssetIcon(g.charaHash)" alt="" />
            <span class="chara-chip-name" :title="g.charaName">{{ g.charaName }}</span><i>{{ g.loadouts.filter(l => !l.isParty).length }}</i>
          </button>
        </div>
      </section>

      <section v-if="currentGroup" class="section ui-card ui-panel">
        <div class="section-title ui-section-title"><span>{{ currentGroup.charaName }} 的配装</span><small>{{ previewLoading ? '正在估算每套配装…' : '当前实时配装置顶；点击卡片展开因子与专精明细' }}</small></div>
        <div class="card-grid ui-card-grid">
          <article v-for="lo in currentGroup.loadouts" :key="lo.unitId" class="loadout-card ui-card is-flat" :class="{ open: expanded.has(lo.unitId), party: lo.isParty }">
            <button type="button" class="loadout-card-toggle" :aria-expanded="expanded.has(lo.unitId)" @click="toggle(lo)">
              <img v-if="weaponAssetIcon({ hash: lo.weaponHash })" class="loadout-weapon-icon" :src="weaponAssetIcon({ hash: lo.weaponHash })" alt="" />
              <b v-if="!lo.isParty">槽{{ String(lo.slot).padStart(2, '0') }}</b>
              <b v-else class="party-tag">队伍{{ lo.slot }}</b>
              <strong>{{ lo.name || (lo.isParty ? '当前实时配装' : '(未命名)') }}</strong>
              <span class="wep">{{ lo.weaponName || '未收录武器' }}</span>
              <em>{{ (lo.sigils || []).length }}因子 · {{ (lo.mastery || []).length }}专精</em>
              <span class="expand-mark">{{ expanded.has(lo.unitId) ? '收起' : '展开' }}<i aria-hidden="true">⌄</i></span>
            </button>
            <div v-if="lo.weapon" class="weapon-loadout-summary">
              <span><b>{{ lo.weapon.name || lo.weaponName }}</b><small>Lv{{ lo.weapon.level }} · 觉醒 {{ lo.weapon.awakening }} · 超凡 {{ lo.weapon.transcendence }}</small></span>
              <i v-for="skill in lo.weapon.skills" :key="`${skill.slot}-${skill.traitHash}`" :title="skill.effect || skill.unlockCondition">
                {{ skill.name || '未收录武器技能' }} <em>Lv{{ skill.level }}</em>
              </i>
              <i v-if="lo.weapon.wrightstone" class="wrightstone-chip">祝福 · {{ lo.weapon.wrightstone.name || '未收录祝福' }}</i>
              <i v-for="trait in lo.weapon.wrightstone?.traits || []" :key="`${lo.unitId}-stone-${trait.index}-${trait.hash}`" class="wrightstone-chip">{{ trait.name || trait.hash }} <em>Lv{{ trait.level }}</em></i>
            </div>
            <div class="skills-line">
              <span>技能</span>
              <i v-for="s in lo.skills" :key="s.hash"><img :src="skillIcon(s)" alt="" />{{ s.name || '未收录技能' }}</i>
            </div>
            <div class="mastery-summary">
              <span>专精</span>
              <i v-for="t in masterySummary(lo)" :key="t.rank">{{ t.label }} {{ t.count }}点</i>
              <i v-if="!(lo.mastery || []).length" class="dim">未保存</i>
            </div>
            <div v-if="previewFor(lo)?.finalStats" class="loadout-stat-strip">
              <span><small>HP</small><b>≈{{ formatNumber(previewFor(lo).finalStats.hp) }}</b></span>
              <span><small>攻击</small><b>≈{{ formatNumber(previewFor(lo).finalStats.attack) }}</b></span>
              <span><small>暴击</small><b>≈{{ formatNumber(previewFor(lo).finalStats.critRate, 1) }}%</b></span>
              <span><small>昏厥</small><b>≈{{ formatNumber(previewFor(lo).finalStats.stunPower, 1) }}</b></span>
            </div>
            <p v-if="previewFor(lo)?.error" class="preview-error">{{ previewFor(lo).error }}</p>
            <div v-if="expanded.has(lo.unitId)" class="detail">
              <div class="detail-block sigil-detail-block">
                <h4>因子（{{ (lo.sigils || []).length }}）</h4>
                <ul class="sigil-detail-list">
                  <li v-for="s in lo.sigils" :key="s.slotId" class="sigil-detail-item">
                    <div class="sigil-detail-title">
                      <img v-if="traitIcon(s.primaryTraitName, s.primaryTraitHash)" :src="traitIcon(s.primaryTraitName, s.primaryTraitHash)" alt="" />
                      <span v-else class="sigil-icon-fallback" aria-hidden="true">◇</span>
                      <b>{{ s.name || '因子' }}</b>
                      <small>因子 Lv{{ s.level }}</small>
                    </div>
                    <div v-if="!s.missing" class="sigil-traits">
                      <span v-if="s.primaryTraitName"><i>主</i>{{ s.primaryTraitName }}<em>Lv{{ s.primaryTraitLevel }}</em></span>
                      <span v-if="s.secondaryTraitName"><i>副</i>{{ s.secondaryTraitName }}<em>Lv{{ s.secondaryTraitLevel }}</em></span>
                    </div>
                    <small v-if="s.missing" class="warn">原背包因子已不存在</small>
                  </li>
                </ul>
              </div>
              <div v-for="grp in masteryGrouped(lo)" :key="grp.key" class="detail-block">
                <h4>{{ grp.rankLabel }} · {{ catLabel(grp.cat) }}（{{ grp.nodes.length }}点）</h4>
                <ul>
                  <li v-for="m in grp.nodes" :key="m.hash">
                    <b v-if="m.name">{{ m.name }} — </b>{{ m.desc || '未收录效果' }}
                  </li>
                </ul>
              </div>
            </div>
          </article>
        </div>
      </section>

      <section v-else-if="!loading && savePath && !groups.length" class="section ui-card ui-panel">
        <p class="empty ui-empty">该存档中没有已保存的配装预设。</p>
      </section>

      <section v-else-if="!loading && !savePath" class="section ui-card ui-panel is-compact">
        <p class="empty ui-empty">选择存档位或浏览文件后，这里会显示真实角色与配装预设。</p>
      </section>
    </template>
  </div>
</template>

<style scoped>
.loadout-viewer { width:100%; max-width:100%; min-width:0; overflow-x:hidden; font-size:var(--fs-md); container:loadout-viewer / inline-size; }
.loadout-viewer.editing { width:100%; height:100%; min-height:0; gap:0; overflow:hidden; }
.section { width:100%; max-width:100%; min-width:0; overflow:hidden; }
.section-title { min-width:0; flex-wrap:wrap; }
.section-title > small { min-width:0; overflow-wrap:anywhere; }
.edit-launch { margin-left:auto; }
.edit-launch span { font-size:var(--fs-lg); }
.editor-workspace { min-width:0; height:100%; min-height:0; display:flex; flex-direction:column; gap:14px; overflow:hidden; }
.editor-workspace-bar { position:sticky; z-index:20; top:0; min-width:0; min-height:72px; display:grid; grid-template-columns:auto minmax(0,1fr) minmax(180px,280px); gap:var(--space-5); align-items:center; padding:var(--space-3) var(--space-5); background:var(--surface-card-pop); box-shadow:var(--shadow-2); }
.back-button span { font-size:1rem; }
.editor-workspace-title { min-width:0; display:grid; grid-template-columns:auto minmax(0,1fr); align-items:baseline; column-gap:9px; row-gap:2px; }
.editor-workspace-title small { grid-column:1/-1; color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-semibold); letter-spacing:.08em; }
.editor-workspace-title strong { color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); font-weight:var(--fw-bold); }
.editor-workspace-title span { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-secondary); font-size:var(--fs-sm); }
.editor-workspace-meta { min-width:0; display:flex; flex-direction:column; align-items:flex-end; gap:2px; text-align:right; }
.preset-count-badge { display:inline-flex; align-items:baseline; gap:5px; padding:3px 8px; border:1px solid var(--line-soft); border-radius:12px; background:rgba(139,103,55,.07); color:var(--text-secondary); font-size:var(--fs-sm); white-space:nowrap; }
.preset-count-badge b { color:var(--accent-hover); font-size:var(--fs-md); }
.editor-workspace-meta small { max-width:100%; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-muted); font-size:var(--fs-xs); }
.editor-workspace-content { min-width:0; min-height:0; flex:1; overflow:auto; scrollbar-gutter:stable; overscroll-behavior:contain; padding:0 2px 2px; }
.editor-workspace-content :deep(.loadout-editor) { height:100%; min-height:0; }
.path-line { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.chara-row { display:grid; grid-template-columns:repeat(auto-fit, minmax(156px, 1fr)); gap:var(--space-2); }
.chara-chip { width:100%; min-width:0; display:inline-flex; align-items:center; justify-content:flex-start; gap:5px; }
.chara-chip img { flex:0 0 auto; width:27px; height:27px; object-fit:cover; border-radius:6px; }
.chara-chip-name { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.chara-chip i { flex:0 0 auto; margin-left:auto; color:var(--text-muted); font-size:var(--fs-xs); font-style:normal; }
.chara-chip.is-on i { color:var(--accent-soft); }
.card-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,420px),1fr)); gap:var(--space-4); align-items:start; min-width:0; }
.loadout-card { min-width:0; display:flex; flex-direction:column; gap:var(--space-3); padding:var(--space-4); border-left:3px solid var(--accent); background:var(--surface-card-pop); }
.loadout-card.party { grid-column:1 / -1; order:-1; border-left-color:var(--success); background:linear-gradient(110deg,rgba(74,139,105,.1),rgba(255,253,247,.92) 52%); box-shadow:inset 0 0 0 1px rgba(74,139,105,.08); }
.loadout-card.party .loadout-card-toggle { grid-template-columns:62px auto minmax(120px,1.2fr) minmax(120px,.8fr) auto auto; }
.loadout-card.open { border-color:var(--border-strong); grid-column:1/-1; }
.loadout-card-toggle { width:100%; min-width:0; min-height:var(--control-height-sm); display:grid; grid-template-columns:62px auto minmax(120px,1fr) minmax(100px,.72fr) auto auto; align-items:center; gap:var(--space-3); padding:0; border:0; background:transparent; color:inherit; text-align:left; cursor:pointer; user-select:none; }
.loadout-weapon-icon { width:62px; height:44px; object-fit:contain; border-radius:6px; background:rgba(255,255,255,.55); }
.loadout-card-toggle:hover strong { color:var(--accent-hover); }
.loadout-card-toggle b { color:var(--accent); font-size:var(--fs-sm); }
.loadout-card-toggle b.party-tag { color:var(--success-ink); }
.loadout-card-toggle strong,.loadout-card-toggle .wep,.loadout-card-toggle em { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.loadout-card-toggle strong { color:var(--text-primary); font-size:var(--fs-md); transition:color var(--dur-base) var(--ease-out); }
.loadout-card-toggle .wep { color:var(--text-secondary); font-size:var(--fs-sm); }
.loadout-card-toggle em { color:var(--text-muted); font-size:var(--fs-xs); font-style:normal; }
.expand-mark { display:inline-flex; align-items:center; justify-content:flex-end; gap:3px; color:var(--accent-hover); font-size:var(--fs-xs); font-weight:var(--fw-semibold); white-space:nowrap; }
.expand-mark i { display:inline-block; font-size:14px; font-style:normal; transition:transform var(--dur-base) var(--ease-out); }
.loadout-card.open .expand-mark i { transform:rotate(180deg); }
.weapon-loadout-summary { min-width:0; display:flex; flex-wrap:wrap; gap:var(--space-2); align-items:center; padding:7px 9px; border:1px solid rgba(139,103,55,.16); border-radius:8px; background:rgba(139,103,55,.045); }
.weapon-loadout-summary > span { min-width:150px; display:flex; flex-direction:column; margin-right:3px; }
.weapon-loadout-summary > span b { color:var(--text-primary); font-size:var(--fs-sm); }
.weapon-loadout-summary > span small { color:var(--text-muted); font-size:var(--fs-xs); }
.weapon-loadout-summary > i { padding:2px 7px; border:1px solid var(--line-soft); border-radius:10px; background:var(--panel-solid); color:var(--text-secondary); font-size:var(--fs-xs); font-style:normal; }
.weapon-loadout-summary > i em { color:var(--accent); font-style:normal; }
.weapon-loadout-summary > i.wrightstone-chip { border-color:rgba(123,89,154,.28); background:rgba(123,89,154,.08); color:#6c4c82; }
.skills-line, .mastery-summary { display:flex; flex-wrap:wrap; gap:var(--space-2); align-items:center; font-size:var(--fs-sm); }
.skills-line span, .mastery-summary span { color:var(--text-muted); }
.skills-line i, .mastery-summary i { font-style:normal; padding:1px 8px; border:1px solid var(--line-soft); border-radius:12px; background:var(--panel-solid); color:var(--text-secondary); }
.skills-line i { display:inline-flex; align-items:center; gap:5px; padding-left:3px; }
.skills-line i img { width:24px; height:24px; border-radius:50%; object-fit:cover; }
.mastery-summary i b { font-weight:700; color:var(--amber); }
.mastery-summary i.dim { color:var(--text-muted); border-style:dashed; background:none; }
.detail { display:flex; flex-direction:column; gap:9px; padding-top:5px; border-top:1px dashed var(--line); }
.detail-block h4 { margin:0 0 5px; font-size:.74rem; color:var(--gold); }
.detail-block ul { margin:0; padding-left:17px; display:flex; flex-direction:column; gap:2px; }
.detail-block li { font-size:.72rem; line-height:1.5; color:var(--text-secondary); }
.detail-block li b { color:var(--text-primary); }
.detail-block li small { color:var(--text-muted); margin-left:5px; }
.detail-block li small.warn { color:var(--red); }
.sigil-detail-list { list-style:none; padding:0; display:grid; grid-template-columns:repeat(auto-fill,minmax(260px,1fr)); gap:7px; }
.sigil-detail-item { min-width:0; padding:var(--space-3) var(--space-4); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-card); }
.sigil-detail-title { min-width:0; display:flex; align-items:center; gap:7px; }
.sigil-detail-title > img { width:30px; height:30px; flex:0 0 30px; border:1px solid var(--line-gold); border-radius:7px; object-fit:cover; }
.sigil-icon-fallback { width:30px; height:30px; flex:0 0 30px; display:grid; place-items:center; border:1px solid var(--border-strong); border-radius:var(--radius-sm); background:var(--accent-soft); color:var(--accent-hover); font-size:1rem; }
.sigil-detail-title b { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-primary); font-size:var(--fs-sm); }
.sigil-detail-title small { margin-left:auto; flex:0 0 auto; color:var(--text-muted); font-size:var(--fs-xs); }
.sigil-traits { display:flex; flex-direction:column; gap:3px; margin-top:5px; }
.sigil-traits span { min-width:0; display:grid; grid-template-columns:20px minmax(0,1fr) auto; gap:var(--space-2); align-items:center; color:var(--text-secondary); font-size:var(--fs-sm); line-height:1.35; }
.sigil-traits i { width:20px; height:18px; display:grid; place-items:center; border:1px solid var(--border-strong); border-radius:var(--radius-sm); color:var(--accent-hover); background:var(--accent-soft); font-size:var(--fs-xs); font-style:normal; font-weight:var(--fw-bold); }
.sigil-traits em { color:var(--accent); font-size:var(--fs-xs); font-style:normal; font-weight:var(--fw-semibold); }
.preview-error { margin:var(--space-3) 0 0; color:var(--danger-ink,#a6473f); font-size:var(--fs-xs); line-height:1.5; }
.loadout-stat-strip { display:grid; grid-template-columns:repeat(4,minmax(72px,1fr)); gap:4px; padding-top:var(--space-2); border-top:1px dashed var(--line-soft); }
.loadout-stat-strip span { min-width:0; display:flex; flex-direction:column; padding:4px 6px; background:rgba(139,103,55,.045); }
.loadout-stat-strip small { color:var(--text-muted); font-size:10px; }
.loadout-stat-strip b { overflow:hidden; text-overflow:ellipsis; color:var(--text-primary); font-size:var(--fs-xs); font-variant-numeric:tabular-nums; white-space:nowrap; }
.empty { margin:0; }
.logs-library-entry { width:100%; min-width:0; display:grid; grid-template-columns:42px minmax(0,1fr) auto; align-items:center; gap:var(--space-3); padding:var(--space-4); border:1px solid var(--border-strong); border-left:4px solid var(--accent); background:var(--surface-card-pop); color:inherit; text-align:left; cursor:pointer; box-shadow:var(--shadow-1); }
.logs-library-entry:hover { border-color:var(--accent); background:var(--accent-soft); }
.logs-library-entry > span:nth-child(2) { min-width:0; display:grid; gap:2px; }
.logs-library-entry small,.logs-library-entry strong,.logs-library-entry em { min-width:0; overflow-wrap:anywhere; }
.logs-library-entry small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.logs-library-entry strong { color:var(--text-primary); font-size:var(--fs-md); }
.logs-library-entry em { color:var(--text-muted); font-size:var(--fs-xs); font-style:normal; }
.logs-library-entry > b { color:var(--accent-hover); font-size:var(--fs-xl); }
.logs-entry-mark { width:42px; height:42px; display:grid; place-items:center; border:1px solid var(--border-strong); border-radius:var(--radius-sm); background:var(--surface-sunken); color:var(--accent-hover); font-family:var(--font-display); font-size:var(--fs-lg); font-weight:var(--fw-bold); }
.logs-workspace { width:100%; min-width:0; display:grid; gap:var(--space-4); }
.subpage-bar { position:sticky; z-index:10; top:0; min-width:0; display:grid; grid-template-columns:auto minmax(0,1fr) auto; align-items:center; gap:var(--space-4); padding:var(--space-4); background:var(--surface-card-pop); box-shadow:var(--shadow-2); }
.subpage-bar > div { min-width:0; display:grid; gap:2px; }
.subpage-bar small,.subpage-bar strong,.subpage-bar span { min-width:0; overflow-wrap:anywhere; }
.subpage-bar small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.subpage-bar strong { color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); }
.subpage-bar span { color:var(--text-muted); font-size:var(--fs-xs); }
.logs-library-bar { grid-template-areas:"back back" "copy action"; grid-template-columns:minmax(0,1fr) auto; align-items:end; row-gap:var(--space-2); }
.logs-back-button { grid-area:back; justify-self:start; width:auto; }
.logs-header-copy { grid-area:copy; align-self:center; }
.logs-source-button { grid-area:action; justify-self:end; width:max-content; max-width:100%; }
.logs-candidate-grid { min-width:0; display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,300px),1fr)); gap:var(--space-4); align-items:stretch; }
.logs-candidate-card { min-width:0; display:flex; flex-direction:column; gap:var(--space-3); padding:var(--space-4); border-left:3px solid var(--accent); background:var(--surface-card-pop); }
.logs-candidate-card > header { min-width:0; display:grid; grid-template-columns:52px minmax(0,1fr); gap:var(--space-3); align-items:center; }
.logs-candidate-card > header img,.logs-character-fallback { width:52px; height:52px; display:grid; place-items:center; overflow:hidden; border:1px solid var(--border-strong); border-radius:var(--radius-sm); background:var(--surface-sunken); object-fit:cover; }
.logs-candidate-card > header div { min-width:0; display:grid; gap:2px; }
.logs-candidate-card > header small,.logs-candidate-card > header strong,.logs-candidate-card > header span { min-width:0; overflow-wrap:anywhere; }
.logs-candidate-card > header small,.logs-candidate-card > header span { color:var(--text-muted); font-size:var(--fs-xs); }
.logs-candidate-card > header strong { color:var(--text-primary); font-size:var(--fs-lg); }
.logs-candidate-card dl { min-width:0; display:grid; grid-template-columns:minmax(0,1.5fr) repeat(2,minmax(72px,.65fr)); gap:var(--space-2); margin:0; }
.logs-candidate-card dl > div { min-width:0; padding:var(--space-2); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.logs-candidate-card dt { color:var(--text-muted); font-size:var(--fs-2xs); }
.logs-candidate-card dd { min-width:0; margin:2px 0 0; overflow-wrap:anywhere; color:var(--text-secondary); font-size:var(--fs-xs); }
.logs-card-actions { min-width:0; display:flex; flex-wrap:wrap; gap:var(--space-2); margin-top:auto; }
.logs-card-actions .ui-btn { flex:1 1 132px; min-width:0; }
.logs-empty { min-height:220px; display:flex; flex-direction:column; justify-content:center; gap:var(--space-2); }
@container loadout-viewer (max-width:900px) {
  .section-title .edit-launch { width:100%; margin-left:0; }
  .editor-workspace-bar { grid-template-columns:1fr auto; }
  .editor-workspace-title { grid-column:1/-1; grid-row:1; }
  .back-button { grid-row:2; }
  .editor-workspace-meta { grid-row:2; }
  .loadout-card-toggle { grid-template-columns:52px auto minmax(0,1fr) auto; }
  .loadout-card.party .loadout-card-toggle { grid-template-columns:52px auto minmax(0,1fr) auto; }
  .loadout-card-toggle > b { grid-column:2; grid-row:1; }
  .loadout-card-toggle > strong { grid-column:3; grid-row:1; }
  .loadout-card-toggle .wep { grid-column:2/4; grid-row:2; }
  .loadout-card-toggle > em { grid-column:2/4; grid-row:3; }
  .loadout-card-toggle .expand-mark { grid-column:4; grid-row:1/4; }
  .loadout-weapon-icon { width:52px; height:40px; grid-row:1/3; }
  .loadout-stat-strip { grid-template-columns:repeat(2,minmax(70px,1fr)); }
}
@container loadout-viewer (max-width:560px) {
	.logs-library-entry { grid-template-columns:36px minmax(0,1fr); }
	.logs-entry-mark { width:36px; height:36px; }
	.logs-library-entry > b { display:none; }
	.subpage-bar:not(.logs-library-bar) { grid-template-columns:minmax(0,1fr); }
	.subpage-bar:not(.logs-library-bar) > .back-button,.subpage-bar:not(.logs-library-bar) > div { grid-column:1; }
	.subpage-bar:not(.logs-library-bar) > .back-button { grid-row:1; justify-self:start; width:auto; }
	.subpage-bar:not(.logs-library-bar) > div { grid-row:2; width:100%; }
	.logs-library-bar { grid-template-areas:"back" "copy" "action"; grid-template-columns:minmax(0,1fr); }
	.logs-source-button { justify-self:start; }
	.logs-candidate-card dl { grid-template-columns:minmax(0,1fr); }
}
</style>
