<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { CloseSaveDiff, ExportFateEpisodeEvidence, ExportSaveDiffCSV, ExportSaveDiffJSON, FateEpisodeInspect, FindSaveFiles, InfinityRuleCatalog, OpenSaveDiff, SaveDiffPage, SelectSaveDiffFile } from '../../wailsjs/go/backend/App'
import { characterNamePairByPLID } from '../characterRoster.js'
import { language } from '../i18n.js'

const emit = defineEmits(['status'])
const leftPath = ref('')
const rightPath = ref('')
const summary = ref(null)
const items = ref([])
const search = ref('')
const status = ref('different')
const loading = ref(false)
const exporting = ref(false)
const nextCursor = ref(0)
const hasMore = ref(false)
const totalFiltered = ref(0)
const mode = ref('diff')
const fatePath = ref('')
const fateStatus = ref(null)
const fateLoading = ref(false)
const fateExporting = ref(false)
const fateSelectedCode = ref('')
const infinityCatalog = ref(null)
const infinityLoading = ref(false)
const saveSlots = ref([])

const tx = (zh, en) => language.value === 'en' ? en : zh
const canCompare = computed(() => !!leftPath.value && !!rightPath.value && !loading.value)
const statusOptions = computed(() => [
  { value: 'different', label: tx('仅差异', 'Differences Only') },
  { value: 'changed', label: tx('已修改', 'Changed') },
  { value: 'added', label: tx('右侧新增', 'Added on Right') },
  { value: 'removed', label: tx('右侧缺少', 'Missing on Right') },
  { value: 'all', label: tx('全部记录', 'All Records') },
])
const fateComplete = computed(() => fateStatus.value?.completed === fateStatus.value?.total && fateStatus.value?.missionCompleted === fateStatus.value?.missionTotal)
const selectedFateCharacter = computed(() => {
  const characters = fateStatus.value?.characters || []
  return characters.find(item => item.code === fateSelectedCode.value) || characters[0] || null
})
const infinityRulesByQuest = computed(() => {
  const groups = new Map()
  for (const rule of infinityCatalog.value?.rules || []) {
    if (!groups.has(rule.questId)) groups.set(rule.questId, [])
    groups.get(rule.questId).push(rule)
  }
  return [...groups].map(([questId, rules]) => ({ questId, rules }))
})

function fileName(path) {
  return String(path || '').split(/[\\/]/).pop() || tx('尚未选择', 'Not Selected')
}
function saveSlotLabel(slot) {
  const index = Number(slot?.index)
  return Number.isFinite(index) && index > 0 ? tx(`存档位 ${index}`, `Save Slot ${index}`) : fileName(slot?.path || slot?.name)
}
function clearComparisonResult() {
  summary.value = null
  items.value = []
  nextCursor.value = 0
  hasMore.value = false
  totalFiltered.value = 0
}
function setComparePath(side, path) {
  if (side === 'left') leftPath.value = path
  else rightPath.value = path
  clearComparisonResult()
}
async function scanSaveSlots() {
  try { saveSlots.value = await FindSaveFiles() || [] }
  catch { saveSlots.value = [] }
}
function formatID(value) {
  const number = Number(value) >>> 0
  return `${number} · 0x${number.toString(16).toUpperCase().padStart(8, '0')}`
}
function statusLabel(value) {
  return {
    added: tx('右侧新增', 'Added on Right'),
    removed: tx('右侧缺少', 'Missing on Right'),
    changed: tx('内容变化', 'Changed'),
    unchanged: tx('相同', 'Unchanged'),
  }[value] || value
}
async function choose(side) {
  try {
    const path = await SelectSaveDiffFile()
    if (!path) return
    setComparePath(side, path)
  } catch (error) { emit('status', String(error), 'error') }
}
async function fetchPage(reset = false) {
  if (!summary.value || loading.value) return
  loading.value = true
  try {
    const page = await SaveDiffPage(reset ? 0 : nextCursor.value, 80, search.value.trim(), status.value)
    items.value = reset ? (page.items || []) : [...items.value, ...(page.items || [])]
    nextCursor.value = page.nextCursor || 0
    hasMore.value = !!page.hasMore
    totalFiltered.value = page.totalFiltered || 0
  } catch (error) { emit('status', String(error), 'error') } finally { loading.value = false }
}
async function compare() {
  if (!canCompare.value) return
  loading.value = true
  try {
    summary.value = await OpenSaveDiff(leftPath.value, rightPath.value)
    items.value = []
    nextCursor.value = 0
    hasMore.value = false
    totalFiltered.value = 0
    loading.value = false
    await fetchPage(true)
    emit('status', summary.value.different
      ? tx(`发现 ${summary.value.different} 条差异`, `${summary.value.different} differences found`)
      : tx('两份存档的逻辑记录完全一致', 'The two saves contain identical logical records'), 'success')
  } catch (error) {
    summary.value = null
    emit('status', String(error), 'error')
  } finally { loading.value = false }
}
async function reset() {
  await CloseSaveDiff()
  clearComparisonResult()
}
async function exportDiff() {
  if (!summary.value || exporting.value) return
  exporting.value = true
  try {
    const path = await ExportSaveDiffJSON()
    if (path) emit('status', tx('脱敏差分已导出', 'Sanitized diff exported'), 'success')
  } catch (error) { emit('status', String(error), 'error') } finally { exporting.value = false }
}
async function exportCSV() {
  if (!summary.value || exporting.value) return
  exporting.value = true
  try {
    const path = await ExportSaveDiffCSV()
    if (path) emit('status', tx('脱敏 CSV 已导出', 'Sanitized CSV exported'), 'success')
  } catch (error) { emit('status', String(error), 'error') } finally { exporting.value = false }
}
function applyFilter() { fetchPage(true) }
function fateCharacterName(code) {
  const names = characterNamePairByPLID(code)
  return names ? names[language.value === 'en' ? 1 : 0] : code
}
function fateEpisodeTitle(episode) {
  return language.value === 'en' ? (episode?.titleEn || episode?.key) : (episode?.titleZh || episode?.key)
}
function fateRequirement(episode) {
  const parts = []
  if (episode?.requiredLevel > 1) parts.push(tx(`角色 Lv${episode.requiredLevel}`, `Character Lv${episode.requiredLevel}`))
  if (episode?.requiredQuestId && episode.requiredQuestId !== '00000000') parts.push(tx(`前置任务 ${episode.requiredQuestId}`, `Prerequisite ${episode.requiredQuestId}`))
  if (episode?.missionQuestId && episode.missionQuestId !== '00000000') parts.push(tx(`战斗任务 ${episode.missionQuestId}`, `Battle Mission ${episode.missionQuestId}`))
  return parts.length ? parts.join(' · ') : tx('无额外等级或任务要求', 'No additional level or quest requirement')
}
async function chooseFateSave() {
  try {
    const path = await SelectSaveDiffFile()
    if (!path) return
    fatePath.value = path
    await inspectFate()
  } catch (error) { emit('status', String(error), 'error') }
}
async function selectFateSave(path) {
  if (!path || fateLoading.value) return
  fatePath.value = path
  await inspectFate()
}
async function inspectFate() {
  if (!fatePath.value || fateLoading.value) return
  fateLoading.value = true
  try {
    fateStatus.value = await FateEpisodeInspect(fatePath.value)
    const characters = fateStatus.value?.characters || []
    if (!characters.some(item => item.code === fateSelectedCode.value)) {
      fateSelectedCode.value = (characters.find(item => item.completed < item.total) || characters[0])?.code || ''
    }
    emit('status', tx(`已读取 ${fateStatus.value.completed}/${fateStatus.value.total} 篇命运篇章`, `Read ${fateStatus.value.completed}/${fateStatus.value.total} Fate Episodes`), 'success')
  } catch (error) {
    fateStatus.value = null
    emit('status', String(error), 'error')
  } finally { fateLoading.value = false }
}
async function exportFateEvidence() {
  if (!fatePath.value || !fateStatus.value || fateExporting.value) return
  fateExporting.value = true
  try {
    const path = await ExportFateEpisodeEvidence(fatePath.value)
    if (path) emit('status', tx('命运篇章只读证据已导出', 'Read-only Fate evidence exported'), 'success')
  } catch (error) {
    emit('status', String(error), 'error')
  } finally { fateExporting.value = false }
}
async function openInfinityRules() {
  mode.value = 'infinity'
  if (infinityCatalog.value || infinityLoading.value) return
  infinityLoading.value = true
  try {
    infinityCatalog.value = await InfinityRuleCatalog()
    emit('status', tx('已读取 25 条无尽模式官方规则', 'Loaded 25 official Endless rules'), 'success')
  } catch (error) {
    emit('status', String(error), 'error')
  } finally { infinityLoading.value = false }
}
function infinityName(rule) { return language.value === 'en' ? rule.nameEn : rule.nameZh }
function infinityDescription(rule) { return language.value === 'en' ? rule.descriptionEn : rule.descriptionZh }
onMounted(scanSaveSlots)
onBeforeUnmount(() => { void CloseSaveDiff().catch(() => {}) })
</script>

<template>
  <section class="save-diff-lab ui-page-stack" :aria-label="tx('存档实验室', 'Save Laboratory')">
    <nav class="lab-modes ui-seg" :aria-label="tx('存档实验室功能', 'Save Laboratory Modes')">
      <button type="button" class="ui-seg-btn" :class="{ 'is-on': mode === 'diff' }" @click="mode = 'diff'">{{ tx('双存档差分', 'Save Comparison') }}</button>
      <button type="button" class="ui-seg-btn" :class="{ 'is-on': mode === 'fate' }" @click="mode = 'fate'">{{ tx('命运篇章', 'Fate Episodes') }}</button>
      <button type="button" class="ui-seg-btn" :class="{ 'is-on': mode === 'infinity' }" @click="openInfinityRules">{{ tx('无尽规则', 'Endless Rules') }}</button>
    </nav>

    <div v-if="mode === 'diff'" class="lab-boundary ui-notice is-info">
      <strong>{{ tx('严格只读', 'Strictly Read Only') }}</strong>
      <span>{{ tx('两份源存档只会被读取和解析，不会备份、修改或写回。导出文件不含源路径、文件名或原始字段值。', 'Both source saves are read and parsed only. They are never backed up, modified, or written back. Exports omit source paths, file names, and raw field values.') }}</span>
    </div>

    <section v-if="mode === 'diff'" class="source-grid" aria-label="存档来源">
      <button type="button" class="source-file ui-card is-flat" :class="{ ready: leftPath }" :disabled="loading" @click="choose('left')">
        <small>{{ tx('基准存档', 'Baseline Save') }}</small><strong>{{ fileName(leftPath) }}</strong><span>{{ tx('选择左侧文件', 'Choose Left File') }}</span>
      </button>
      <div class="compare-mark" aria-hidden="true">⇄</div>
      <button type="button" class="source-file ui-card is-flat" :class="{ ready: rightPath }" :disabled="loading" @click="choose('right')">
        <small>{{ tx('对照存档', 'Comparison Save') }}</small><strong>{{ fileName(rightPath) }}</strong><span>{{ tx('选择右侧文件', 'Choose Right File') }}</span>
      </button>
      <button type="button" class="ui-btn is-primary compare-button" :disabled="!canCompare" @click="compare">{{ loading ? tx('正在解析…', 'Parsing…') : tx('开始只读比较', 'Compare Read Only') }}</button>
    </section>
    <section v-if="mode === 'diff' && saveSlots.length" class="known-save-pickers ui-card is-flat" :aria-label="tx('快速选择已识别存档位', 'Quick Select Detected Save Slots')">
      <div><span>{{ tx('基准存档快捷选择', 'Baseline Quick Select') }}</span><div><button v-for="slot in saveSlots" :key="`left-${slot.path}`" type="button" class="ui-btn is-sm" :class="{ 'is-primary': leftPath === slot.path }" :disabled="loading" @click="setComparePath('left', slot.path)">{{ saveSlotLabel(slot) }}</button></div></div>
      <div><span>{{ tx('对照存档快捷选择', 'Comparison Quick Select') }}</span><div><button v-for="slot in saveSlots" :key="`right-${slot.path}`" type="button" class="ui-btn is-sm" :class="{ 'is-primary': rightPath === slot.path }" :disabled="loading" @click="setComparePath('right', slot.path)">{{ saveSlotLabel(slot) }}</button></div></div>
    </section>

    <template v-if="mode === 'diff' && summary">
      <section class="diff-summary ui-stat-grid">
        <article class="ui-card ui-stat"><small>{{ tx('差异总数', 'Differences') }}</small><strong>{{ summary.different }}</strong><span>{{ summary.leftRecords }} / {{ summary.rightRecords }} {{ tx('条记录', 'records') }}</span></article>
        <article class="ui-card ui-stat is-changed"><small>{{ tx('内容变化', 'Changed') }}</small><strong>{{ summary.changed }}</strong><span>{{ tx('同一位置，值或长度变化', 'Same location, changed values or length') }}</span></article>
        <article class="ui-card ui-stat is-added"><small>{{ tx('右侧新增', 'Added on Right') }}</small><strong>{{ summary.added }}</strong><span>{{ tx('只存在于对照存档', 'Only in comparison save') }}</span></article>
        <article class="ui-card ui-stat is-removed"><small>{{ tx('右侧缺少', 'Missing on Right') }}</small><strong>{{ summary.removed }}</strong><span>{{ tx('只存在于基准存档', 'Only in baseline save') }}</span></article>
      </section>

      <div class="diff-toolbar ui-card is-flat">
        <label><span>{{ tx('搜索 ID、语义名或内容哈希', 'Search ID, semantic name, or content hash') }}</span><input v-model="search" class="ui-input" :placeholder="tx('例如：2803 / 0x00000AF3 / Weapon ID', 'For example: 2803 / 0x00000AF3 / Weapon ID')" @keyup.enter="applyFilter" /></label>
        <label><span>{{ tx('显示范围', 'Record Scope') }}</span><select v-model="status" class="ui-select" @change="applyFilter"><option v-for="option in statusOptions" :key="option.value" :value="option.value">{{ option.label }}</option></select></label>
        <button type="button" class="ui-btn" :disabled="loading" @click="applyFilter">{{ tx('筛选', 'Filter') }}</button>
        <button type="button" class="ui-btn" :disabled="exporting" @click="exportDiff">{{ exporting ? tx('正在导出…', 'Exporting…') : tx('导出脱敏 JSON', 'Export Sanitized JSON') }}</button>
        <button type="button" class="ui-btn" :disabled="exporting" @click="exportCSV">{{ tx('导出脱敏 CSV', 'Export Sanitized CSV') }}</button>
        <button type="button" class="ui-btn is-ghost" :disabled="loading" @click="reset">{{ tx('清空比较', 'Clear Comparison') }}</button>
      </div>

      <div class="diff-count"><span>{{ tx(`当前筛选 ${totalFiltered} 条`, `${totalFiltered} records in current filter`) }}</span><small>{{ tx(`SlotData 版本 ${summary.leftVersion} → ${summary.rightVersion}`, `SlotData version ${summary.leftVersion} → ${summary.rightVersion}`) }}</small></div>

      <section class="diff-list" aria-live="polite">
        <article v-for="entry in items" :key="entry.key" class="diff-row ui-card is-flat" :class="`is-${entry.status}`">
          <div class="record-identity"><span class="status-badge">{{ statusLabel(entry.status) }}</span><strong>{{ entry.semanticName || tx('未命名记录', 'Unnamed Record') }}</strong><small>{{ entry.section }} · {{ entry.valueType }} · L#{{ entry.leftOccurrence >= 0 ? entry.leftOccurrence + 1 : '—' }} / R#{{ entry.rightOccurrence >= 0 ? entry.rightOccurrence + 1 : '—' }}</small></div>
          <dl><div><dt>IDType</dt><dd>{{ formatID(entry.idType) }}</dd></div><div><dt>UnitID</dt><dd>{{ formatID(entry.unitId) }}</dd></div></dl>
          <div class="value-side"><small>{{ tx('基准', 'Baseline') }} · #{{ entry.leftIndex >= 0 ? entry.leftIndex : '—' }} · {{ entry.leftCount }} {{ tx('项', 'values') }}</small><code>{{ entry.leftPreview || '—' }}</code><span>{{ entry.leftHash || '—' }}</span></div>
          <div class="value-side"><small>{{ tx('对照', 'Comparison') }} · #{{ entry.rightIndex >= 0 ? entry.rightIndex : '—' }} · {{ entry.rightCount }} {{ tx('项', 'values') }}</small><code>{{ entry.rightPreview || '—' }}</code><span>{{ entry.rightHash || '—' }}</span></div>
        </article>
        <p v-if="!items.length && !loading" class="ui-empty">{{ tx('当前筛选没有记录。', 'No records match the current filter.') }}</p>
      </section>
      <button v-if="hasMore" type="button" class="load-more ui-btn" :disabled="loading" @click="fetchPage(false)">{{ loading ? tx('正在读取…', 'Loading…') : tx('加载更多记录', 'Load More Records') }}</button>
    </template>

    <template v-if="mode === 'fate'">
      <div class="lab-boundary ui-notice is-info"><strong>{{ tx('命运篇章只读审计', 'Read-Only Fate Audit') }}</strong><span>{{ tx('只读取 DLC 2.0.2 已确认的 319 条命运篇章状态和 56 个对应任务完成记录；当前构建不会写入存档。', 'Reads the 319 confirmed DLC 2.0.2 Fate states and 56 matching mission completion records. This build does not write save data.') }}</span></div>
      <section class="fate-source ui-card is-flat">
        <button type="button" class="source-file" :class="{ ready: fatePath }" :disabled="fateLoading" @click="chooseFateSave"><small>{{ tx('目标存档', 'Target Save') }}</small><strong>{{ fileName(fatePath) }}</strong><span>{{ tx('选择后立即检查结构', 'Select and validate immediately') }}</span></button>
        <button type="button" class="ui-btn" :disabled="!fatePath || fateLoading" @click="inspectFate">{{ fateLoading ? tx('正在检查…', 'Checking…') : tx('重新检查', 'Check Again') }}</button>
        <div v-if="saveSlots.length" class="fate-save-slots"><span>{{ tx('已识别存档位', 'Detected Save Slots') }}</span><div><button v-for="slot in saveSlots" :key="`fate-${slot.path}`" type="button" class="ui-btn is-sm" :class="{ 'is-primary': fatePath === slot.path }" :disabled="fateLoading" @click="selectFateSave(slot.path)">{{ saveSlotLabel(slot) }}</button></div></div>
      </section>
      <template v-if="fateStatus">
        <section class="fate-summary ui-stat-grid">
          <article class="ui-card ui-stat"><small>{{ tx('已完成篇章', 'Completed Episodes') }}</small><strong>{{ fateStatus.completed }} / {{ fateStatus.total }}</strong><span>{{ tx('按 29 名角色逐篇校验', 'Verified across 29 characters') }}</span></article>
          <article class="ui-card ui-stat"><small>{{ tx('战斗篇章任务', 'Battle Missions') }}</small><strong>{{ fateStatus.missionCompleted }} / {{ fateStatus.missionTotal }}</strong><span>{{ tx('任务 ID 保持原值，只补完成状态', 'Mission IDs stay unchanged; only completion is raised') }}</span></article>
          <article class="ui-card ui-stat"><small>{{ tx('待完成', 'Remaining') }}</small><strong>{{ fateStatus.total - fateStatus.completed }}</strong><span>{{ fateComplete ? tx('当前记录已全部完成', 'All records are complete') : tx('仅用于审计，不提供写入', 'Audit only; writing is unavailable') }}</span></article>
          <article class="ui-card ui-stat"><small>{{ tx('辅助记录', 'Auxiliary Rows') }}</small><strong>{{ fateStatus.auxiliaryPreserved }} / 5</strong><span>REMI · {{ tx('仅验证存在性', 'presence is validated only') }}</span></article>
        </section>
        <section class="fate-character-grid" :aria-label="tx('各角色命运篇章进度', 'Fate progress by character')">
          <button v-for="character in fateStatus.characters" :key="character.code" type="button" class="fate-character ui-card is-flat" :class="{ complete: character.completed === character.total, selected: character.code === selectedFateCharacter?.code }" @click="fateSelectedCode = character.code"><span><strong>{{ fateCharacterName(character.code) }}</strong><small>{{ character.code }}</small></span><b>{{ character.completed }}/{{ character.total }}</b></button>
        </section>
        <section v-if="selectedFateCharacter" class="fate-detail" :aria-label="tx(`${fateCharacterName(selectedFateCharacter.code)}命运篇章明细`, `${fateCharacterName(selectedFateCharacter.code)} Fate Episode Details`)">
          <header><div><small>{{ fateStatus.dataVersion }} · {{ tx('解包表逐篇校验', 'Per-Episode Unpacked-Table Validation') }}</small><h3>{{ fateCharacterName(selectedFateCharacter.code) }}</h3></div><span><b>HP +{{ selectedFateCharacter.completedStaticHp }}</b><b>{{ tx('攻击', 'ATK') }} +{{ selectedFateCharacter.completedStaticAttack }}</b><small>{{ tx('仅汇总已完成篇章在 chara_status_fate.tbl 中的静态奖励', 'Only completed static rewards from chara_status_fate.tbl are totalled') }}</small></span></header>
          <div class="fate-episode-list">
            <article v-for="episode in selectedFateCharacter.episodes" :key="episode.key" class="fate-episode" :class="{ complete: episode.completed }">
              <i>{{ String(episode.index + 1).padStart(2, '0') }}</i><span><strong>{{ fateEpisodeTitle(episode) }}</strong><small>{{ fateRequirement(episode) }}</small><code>{{ episode.key }} · 0x{{ episode.hash }}</code></span><em v-if="episode.hasStaticBonus">HP +{{ episode.staticHp }} · {{ tx('攻击', 'ATK') }} +{{ episode.staticAttack }}</em><b>{{ episode.completed ? tx('已完成', 'Completed') : tx('未完成', 'Incomplete') }}</b>
            </article>
          </div>
        </section>
        <div class="fate-actions ui-card is-flat"><span><strong>{{ tx('命运篇章保持只读研究', 'Fate Episodes Remain Read-Only Research') }}</strong><small>{{ tx('当前字段目录可以审计完成状态和静态加成，但尚未完成两份独立存档、领取状态与游戏内重读验证；本构建不会写入命运篇章。', 'The current catalog can audit completion and static bonuses, but two independent saves, claim states, and in-game reload verification are still missing. This build does not write Fate Episode progress.') }}</small></span><div class="fate-action-buttons"><button type="button" class="ui-btn" :disabled="fateExporting" @click="exportFateEvidence">{{ fateExporting ? tx('正在导出…', 'Exporting…') : tx('导出只读证据', 'Export Read-Only Evidence') }}</button></div></div>
      </template>
    </template>

    <template v-if="mode === 'infinity'">
      <div class="lab-boundary ui-notice is-info"><strong>{{ tx('2.0.2 官方规则图鉴', 'Official 2.0.2 Rule Atlas') }}</strong><span>{{ tx('名称、说明和参数来自 infinity_rule / infinity_rule_effect 与官方文本表。未知效果 Id 只保留原值，不猜测语义。', 'Names, descriptions, and parameters come from infinity_rule / infinity_rule_effect and official text tables. Unknown effect IDs retain raw values without invented semantics.') }}</span></div>
      <p v-if="infinityLoading" class="ui-empty">{{ tx('正在读取无尽模式规则…', 'Loading Endless rules…') }}</p>
      <template v-else-if="infinityCatalog">
        <section class="infinity-summary ui-stat-grid">
          <article class="ui-card ui-stat"><small>{{ tx('任务组', 'Quest Groups') }}</small><strong>{{ infinityRulesByQuest.length }}</strong><span>{{ tx('按任务 ID 分组', 'Grouped by quest ID') }}</span></article>
          <article class="ui-card ui-stat"><small>{{ tx('规则', 'Rules') }}</small><strong>{{ infinityCatalog.rules.length }}</strong><span>{{ tx('官方中英文本', 'Official Chinese and English text') }}</span></article>
          <article class="ui-card ui-stat"><small>{{ tx('难度阶梯', 'Difficulty Tiers') }}</small><strong>{{ infinityCatalog.difficulties.length }}</strong><span>{{ tx('原始等级与战力范围', 'Raw level and power ranges') }}</span></article>
        </section>
        <section class="infinity-difficulties" :aria-label="tx('无尽模式难度表', 'Endless Difficulty Table')">
          <article v-for="tier in infinityCatalog.difficulties" :key="tier.SortOrder"><small>{{ tx('阶梯', 'Tier') }} {{ tier.SortOrder + 1 }}</small><strong>Lv{{ tier.EnemyMinLevel }}–{{ tier.EnemyMaxLevel }}</strong><span>PWR {{ tier.Power }}</span></article>
        </section>
        <section class="infinity-quest-list">
          <article v-for="group in infinityRulesByQuest" :key="group.questId" class="infinity-quest">
            <header><span><small>{{ tx('任务 ID', 'Quest ID') }}</small><strong>{{ group.questId }}</strong></span><b>{{ group.rules.length }} {{ tx('条规则', 'rules') }}</b></header>
            <div class="infinity-rule-list">
              <details v-for="rule in group.rules" :key="rule.nameKey" class="infinity-rule">
                <summary><strong>{{ infinityName(rule) }}</strong><small>{{ rule.nameKey }}</small></summary>
                <p>{{ infinityDescription(rule) }}</p>
                <div v-if="rule.effects?.length" class="infinity-effect-row"><code v-for="effect in rule.effects" :key="effect.key">{{ effect.key }} · Id {{ effect.id }} · {{ effect.value }}</code></div>
                <small v-else class="infinity-no-effect">{{ tx('该条规则没有独立效果参数行', 'This rule has no separate effect parameter row') }}</small>
              </details>
            </div>
          </article>
        </section>
      </template>
    </template>
  </section>
</template>

<style scoped>
.save-diff-lab { min-width:0; container:save-diff / inline-size; }
.lab-modes { justify-self:start; }
.lab-boundary { align-items:start; }
.source-grid { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) 36px minmax(0,1fr) auto; gap:var(--space-3); align-items:stretch; }
.source-file { min-width:0; display:grid; gap:3px; padding:var(--space-4); border-left:3px solid var(--border-strong); color:inherit; text-align:left; cursor:pointer; }
.source-file:hover,.source-file.ready { border-left-color:var(--accent); background:var(--accent-soft); }
.source-file small,.source-file span { color:var(--text-muted); font-size:var(--fs-xs); }
.source-file strong { min-width:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-primary); font-size:var(--fs-md); white-space:nowrap; }
.compare-mark { pointer-events:none; display:grid; place-items:center; color:var(--accent); font-size:var(--fs-xl); }
.compare-button { align-self:center; }
.known-save-pickers { min-width:0; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-3); padding:var(--space-3); }.known-save-pickers > div,.fate-save-slots { min-width:0; display:grid; gap:6px; }.known-save-pickers > div > span,.fate-save-slots > span { color:var(--text-muted); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }.known-save-pickers > div > div,.fate-save-slots > div { min-width:0; display:flex; flex-wrap:wrap; gap:6px; }
.diff-summary { grid-template-columns:repeat(4,minmax(0,1fr)); }
.diff-summary .ui-stat { min-width:0; padding:var(--space-3); border-top:2px solid var(--border-strong); }
.diff-summary .ui-stat.is-changed { border-top-color:var(--warning); }
.diff-summary .ui-stat.is-added { border-top-color:var(--success); }
.diff-summary .ui-stat.is-removed { border-top-color:var(--danger); }
.diff-summary small,.diff-summary span { color:var(--text-muted); font-size:var(--fs-2xs); }
.diff-summary strong { color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-xl); }
.diff-toolbar { min-width:0; display:grid; grid-template-columns:minmax(240px,1fr) minmax(150px,.35fr) repeat(4,auto); gap:var(--space-2); align-items:end; padding:var(--space-3); }
.diff-toolbar label { min-width:0; display:grid; gap:4px; color:var(--text-muted); font-size:var(--fs-xs); }
.diff-count { display:flex; justify-content:space-between; gap:var(--space-3); color:var(--text-secondary); font-size:var(--fs-xs); }
.diff-count small { color:var(--text-muted); }
.diff-list { min-width:0; display:grid; gap:6px; }
.diff-row { min-width:0; display:grid; grid-template-columns:minmax(150px,.7fr) minmax(230px,.8fr) repeat(2,minmax(180px,1fr)); gap:var(--space-3); align-items:center; padding:var(--space-3); border-left:3px solid var(--border-strong); }
.diff-row.is-changed { border-left-color:var(--warning); }
.diff-row.is-added { border-left-color:var(--success); }
.diff-row.is-removed { border-left-color:var(--danger); }
.record-identity,.value-side { min-width:0; display:grid; gap:2px; }
.record-identity strong { min-width:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-primary); white-space:nowrap; }
.record-identity small,.value-side small,.value-side span { min-width:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-muted); font-size:var(--fs-2xs); white-space:nowrap; }
.status-badge { justify-self:start; padding:2px 6px; border:1px solid var(--border-soft); color:var(--accent); font-size:var(--fs-2xs); font-weight:var(--fw-bold); }
.diff-row dl { min-width:0; display:grid; gap:4px; margin:0; }
.diff-row dl div { min-width:0; display:grid; grid-template-columns:52px minmax(0,1fr); gap:var(--space-2); }
.diff-row dt { color:var(--text-muted); font-size:var(--fs-2xs); }
.diff-row dd { min-width:0; margin:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-secondary); font-family:var(--font-data); font-size:var(--fs-2xs); white-space:nowrap; }
.value-side code { min-width:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-xs); white-space:nowrap; }
.load-more { justify-self:center; }
.fate-source { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-3); align-items:center; padding:var(--space-3); }
.fate-save-slots { grid-column:1 / -1; padding-top:var(--space-2); border-top:1px solid var(--border-soft); }
.fate-source .source-file { border:0; border-left:3px solid var(--border-strong); background:transparent; }
.fate-summary { grid-template-columns:repeat(4,minmax(0,1fr)); }
.fate-summary .ui-stat { min-width:0; padding:var(--space-3); border-top:2px solid var(--border-strong); }
.fate-summary small,.fate-summary span { color:var(--text-muted); font-size:var(--fs-2xs); }
.fate-summary strong { color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-xl); }
.fate-character-grid { min-width:0; display:grid; grid-template-columns:repeat(auto-fill,minmax(160px,1fr)); gap:6px; }
.fate-character { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-2); padding:var(--space-2) var(--space-3); border-left:3px solid var(--warning); color:inherit; text-align:left; cursor:pointer; }
.fate-character.complete { border-left-color:var(--success); }
.fate-character.selected { outline:2px solid var(--accent-border); outline-offset:-2px; background:var(--accent-soft); }
.fate-character span { min-width:0; display:grid; }
.fate-character strong { min-width:0; overflow:hidden; text-overflow:ellipsis; color:var(--text-primary); font-size:var(--fs-sm); white-space:nowrap; }
.fate-character small { color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-2xs); }
.fate-character b { flex:0 0 auto; color:var(--text-secondary); font-family:var(--font-data); font-size:var(--fs-xs); }
.fate-detail { min-width:0; display:grid; gap:var(--space-3); padding-top:var(--space-3); border-top:1px solid var(--border-soft); }
.fate-detail > header { min-width:0; display:flex; align-items:end; justify-content:space-between; gap:var(--space-4); }
.fate-detail > header div,.fate-detail > header span { min-width:0; display:grid; gap:2px; }
.fate-detail > header small { color:var(--text-muted); font-size:var(--fs-2xs); }
.fate-detail > header h3 { margin:0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); letter-spacing:0; }
.fate-detail > header span { grid-template-columns:repeat(2,auto); justify-content:end; text-align:right; }
.fate-detail > header span b { color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); }
.fate-detail > header span small { grid-column:1/-1; max-width:64ch; }
.fate-episode-list { min-width:0; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:6px var(--space-3); }
.fate-episode { min-width:0; display:grid; grid-template-columns:28px minmax(0,1fr) auto; gap:var(--space-2); align-items:center; padding:8px 0; border-bottom:1px solid var(--border-soft); }
.fate-episode > i { grid-row:1/3; display:grid; place-items:center; width:28px; height:28px; border:1px solid var(--border-soft); color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-2xs); font-style:normal; }
.fate-episode > span { min-width:0; display:grid; gap:2px; }
.fate-episode strong,.fate-episode small,.fate-episode code { min-width:0; overflow-wrap:anywhere; }
.fate-episode strong { color:var(--text-primary); font-size:var(--fs-xs); }
.fate-episode small,.fate-episode code { color:var(--text-muted); font-size:var(--fs-2xs); }
.fate-episode > em { grid-column:2; color:var(--text-secondary); font-size:var(--fs-2xs); font-style:normal; }
.fate-episode > b { grid-column:3; grid-row:1/3; color:var(--warning-ink); font-size:var(--fs-2xs); }
.fate-episode.complete > i { border-color:var(--success-border); color:var(--success); }
.fate-episode.complete > b { color:var(--success); }
.fate-actions { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-4); padding:var(--space-4); border-left:3px solid var(--accent); }
.fate-actions span { min-width:0; display:grid; gap:2px; }
.fate-actions strong { color:var(--text-primary); }
.fate-actions small { color:var(--text-muted); font-size:var(--fs-xs); }
.fate-action-buttons { min-width:0; display:flex; flex-wrap:wrap; justify-content:flex-end; gap:var(--space-2); }
.infinity-summary { grid-template-columns:repeat(3,minmax(0,1fr)); }
.infinity-summary .ui-stat { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:2px var(--space-3); align-items:center; min-height:58px; padding:var(--space-3); border-top:2px solid var(--accent); }
.infinity-summary .ui-stat small,.infinity-summary .ui-stat span { grid-column:1; min-width:0; }
.infinity-summary .ui-stat strong { grid-column:2; grid-row:1/3; }
.infinity-summary small,.infinity-summary span { color:var(--text-muted); font-size:var(--fs-2xs); }
.infinity-summary strong { color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-xl); }
.infinity-difficulties { min-width:0; display:grid; grid-template-columns:repeat(5,minmax(0,1fr)); gap:1px; background:var(--border-soft); }
.infinity-difficulties article { min-width:0; display:grid; gap:2px; padding:var(--space-3); background:var(--surface-soft); }
.infinity-difficulties small { color:var(--text-muted); font-size:var(--fs-2xs); }
.infinity-difficulties strong { color:var(--text-primary); font-family:var(--font-data); }
.infinity-difficulties span { color:var(--text-secondary); font-family:var(--font-data); font-size:var(--fs-xs); }
.infinity-quest-list { min-width:0; display:grid; gap:var(--space-4); }
.infinity-quest { min-width:0; display:grid; grid-template-columns:minmax(140px,.24fr) minmax(0,1fr); gap:var(--space-4); padding-top:var(--space-3); border-top:1px solid var(--border-soft); }
.infinity-quest > header { min-width:0; display:flex; align-items:start; justify-content:space-between; gap:var(--space-2); }
.infinity-quest > header span { min-width:0; display:grid; gap:2px; }
.infinity-quest > header small { color:var(--text-muted); font-size:var(--fs-2xs); }
.infinity-quest > header strong { color:var(--text-primary); font-family:var(--font-data); overflow-wrap:anywhere; }
.infinity-quest > header b { flex:0 0 auto; color:var(--accent); font-size:var(--fs-xs); }
.infinity-rule-list { min-width:0; display:grid; gap:4px; }
.infinity-rule { min-width:0; border-bottom:1px solid var(--border-soft); }
.infinity-rule summary { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-2); padding:7px 0; cursor:pointer; }
.infinity-rule summary strong { min-width:0; color:var(--text-primary); font-size:var(--fs-sm); overflow-wrap:anywhere; }
.infinity-rule summary small { color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-2xs); overflow-wrap:anywhere; }
.infinity-rule p { margin:0 0 var(--space-2); color:var(--text-secondary); font-size:var(--fs-xs); line-height:1.55; white-space:pre-line; }
.infinity-effect-row { min-width:0; display:flex; flex-wrap:wrap; gap:4px; padding-bottom:var(--space-2); }
.infinity-effect-row code { padding:2px 4px; background:var(--surface-soft); color:var(--text-muted); font-size:var(--fs-2xs); overflow-wrap:anywhere; }
.infinity-no-effect { display:block; padding-bottom:var(--space-2); color:var(--text-muted); font-size:var(--fs-2xs); }
@container save-diff (max-width:980px) {
  .source-grid { grid-template-columns:minmax(0,1fr) 28px minmax(0,1fr); }
  .compare-button { grid-column:1/-1; justify-self:center; }
  .diff-toolbar { grid-template-columns:minmax(0,1fr) minmax(150px,.45fr) repeat(4,auto); }
  .diff-toolbar label:first-child { grid-column:1/-1; }
  .diff-row { grid-template-columns:minmax(140px,.6fr) minmax(220px,1fr); }
}
@container save-diff (max-width:620px) {
  .source-grid { grid-template-columns:minmax(0,1fr); }
  .known-save-pickers { grid-template-columns:minmax(0,1fr); }
  .compare-mark { min-height:24px; transform:rotate(90deg); }
  .compare-button { grid-column:1; width:100%; }
  .diff-summary { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .fate-summary { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .infinity-summary,.infinity-difficulties { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .diff-toolbar { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .diff-toolbar label { grid-column:1/-1; }
  .diff-toolbar .ui-btn { width:100%; }
  .diff-count { display:grid; }
  .diff-row { grid-template-columns:minmax(0,1fr); align-items:start; }
  .record-identity strong,.record-identity small,.value-side small,.value-side span,.value-side code { white-space:normal; overflow-wrap:anywhere; }
  .fate-source { grid-template-columns:minmax(0,1fr); }
  .fate-source .ui-btn { width:100%; }
  .fate-actions { align-items:stretch; flex-direction:column; }
  .fate-actions .ui-btn { width:100%; }
  .fate-action-buttons { width:100%; display:grid; grid-template-columns:minmax(0,1fr); }
  .fate-detail > header { align-items:start; flex-direction:column; }
  .fate-detail > header span { justify-content:start; text-align:left; }
  .fate-episode-list { grid-template-columns:minmax(0,1fr); }
  .infinity-quest { grid-template-columns:minmax(0,1fr); }
  .infinity-rule summary { grid-template-columns:minmax(0,1fr); }
}
</style>
