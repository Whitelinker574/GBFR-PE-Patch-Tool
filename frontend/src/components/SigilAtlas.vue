<script setup>
import { computed, onMounted, ref } from 'vue'
import { traitAssetIcon } from '../gameAssetIcons'
import { language } from '../i18n.js'
import { sigilAtlasStore } from '../sigilAtlasStore'
import { matchText } from '../utils/matchText'

const emit = defineEmits(['status', 'construct', 'optimize', 'share-note'])
const loading = ref(false)
const error = ref('')
const atlas = ref({ dataVersion: '', sigils: [], traits: [] })
const query = ref('')
const category = ref('all')
const structure = ref('all')
const firstTrait = ref('')
const secondTrait = ref('')
const expandedId = ref('')
const page = ref(1)
const pageSize = 60

const tx = (zh, en) => language.value === 'en' ? en : zh
const categories = computed(() => [...new Set((atlas.value.sigils || []).map(item => item.category).filter(Boolean))].sort())
const filtered = computed(() => {
  const first = firstTrait.value
  const second = secondTrait.value
  return (atlas.value.sigils || []).filter(item => {
    if (category.value !== 'all' && item.category !== category.value) return false
    if (structure.value === 'single' && item.supportsSecondaryTrait) return false
    if (structure.value === 'dual' && !item.supportsSecondaryTrait) return false
    if (structure.value === 'constructible' && !item.constructible) return false
    if (first && item.primaryTraitId !== first) return false
    if (second && !(item.secondaryTraits || []).some(trait => trait.internalId === second)) return false
    return matchText(item.searchText, query.value)
  })
})
const pageCount = computed(() => Math.max(1, Math.ceil(filtered.value.length / pageSize)))
const visible = computed(() => filtered.value.slice((page.value - 1) * pageSize, page.value * pageSize))

function resetPage() { page.value = 1 }
function traitIcon(trait) { return traitAssetIcon({ internalId: trait?.internalId, hash: trait?.hash, name: trait?.displayName }) }
function entryIcon(entry) { return traitAssetIcon({ internalId: entry?.primaryTraitId, name: entry?.primaryTraitName }) }
function categoryLabel(value) {
  const labels = {
    normal: ['通用因子', 'General'], character_sigil: ['角色专属', 'Character'],
    support_sigil: ['辅助型', 'Support'], special_sigil: ['特殊因子', 'Special'],
    opus_sigil: ['Alpha / Beta / Gamma', 'Alpha / Beta / Gamma'], dlc: ['DLC', 'DLC'],
  }
  return labels[value] ? tx(...labels[value]) : value || tx('未分类', 'Uncategorized')
}
function toggle(entry) { expandedId.value = expandedId.value === entry.internalId ? '' : entry.internalId }
function sendToConstructor(entry) {
  const secondary = secondTrait.value && (entry.secondaryTraits || []).some(trait => trait.internalId === secondTrait.value) ? secondTrait.value : ''
  emit('construct', {
    sigilId: entry.internalId,
    primaryTraitId: entry.primaryTraitId,
    secondaryTraitId: secondary,
  })
}
function selectedTraits(entry) {
  const secondary = secondTrait.value
    ? (entry.secondaryTraits || []).find(trait => trait.internalId === secondTrait.value)
    : null
  return [
    { internalId: entry.primaryTraitId, name: entry.primaryTraitName },
    ...(secondary ? [{ internalId: secondary.internalId, name: secondary.displayName }] : []),
  ].filter(item => item.internalId)
}
function sendToOptimizer(entry) {
  const traits = selectedTraits(entry)
  emit('optimize', { traitIds: traits.map(item => item.internalId), traitNames: traits.map(item => item.name), requestId: Date.now() })
}
function sendToShareNote(entry) {
  const names = selectedTraits(entry).map(item => item.name).filter(Boolean)
  emit('share-note', {
    description: tx(`核心因子方向：${names.join(' + ') || entry.displayName}`, `Core sigil direction: ${names.join(' + ') || entry.displayName}`),
    requestId: Date.now(),
  })
}
function swapTraits() {
  const previous = firstTrait.value
  firstTrait.value = secondTrait.value
  secondTrait.value = previous
  resetPage()
}
async function load() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    atlas.value = await sigilAtlasStore.load(language.value)
  } catch (reason) {
    error.value = String(reason)
    emit('status', error.value, 'error')
  } finally {
    loading.value = false
  }
}
onMounted(load)
</script>

<template>
  <section class="atlas-page" :aria-label="tx('因子图鉴与组合查找', 'Sigil Atlas and Combination Search')">
    <header class="atlas-heading">
      <div><small>{{ atlas.dataVersion || 'GBFR 2.0.2' }}</small><h2>{{ tx('因子图鉴与组合查找', 'Sigil Atlas & Combination Search') }}</h2><p>{{ tx('目录、等级和副词条池与存档、内存和配装构造共用同一套数据。', 'Catalog, levels, and secondary pools use the same data as save, memory, and loadout construction.') }}</p></div>
      <span class="atlas-count"><b>{{ filtered.length }}</b> / {{ atlas.sigils.length }}</span>
    </header>

    <div class="atlas-toolbar ui-card is-flat">
      <label class="atlas-search"><span>{{ tx('搜索', 'Search') }}</span><input v-model="query" class="ui-input" :placeholder="tx('名称、技能、内部 ID 或 Hash', 'Name, trait, internal ID, or hash')" @input="resetPage" /></label>
      <label><span>{{ tx('类别', 'Category') }}</span><select v-model="category" class="ui-select" @change="resetPage"><option value="all">{{ tx('全部类别', 'All Categories') }}</option><option v-for="item in categories" :key="item" :value="item">{{ categoryLabel(item) }}</option></select></label>
      <label><span>{{ tx('结构', 'Structure') }}</span><select v-model="structure" class="ui-select" @change="resetPage"><option value="all">{{ tx('全部结构', 'All Structures') }}</option><option value="single">{{ tx('单词条', 'Single Trait') }}</option><option value="dual">{{ tx('双词条', 'Dual Trait') }}</option><option value="constructible">{{ tx('可构造', 'Constructible') }}</option></select></label>
    </div>

    <div class="combo-finder ui-card is-flat">
      <div class="combo-copy"><small>{{ tx('合法组合反查', 'Legal Combination Lookup') }}</small><strong>{{ tx('指定主词条和副词条', 'Choose a Primary and Secondary Trait') }}</strong><span>{{ tx('只返回游戏目录中允许该方向的因子外壳，不拼接不存在的名称。', 'Only shells that permit this direction are returned; no invented item names.') }}</span></div>
      <select v-model="firstTrait" class="ui-select" @change="resetPage"><option value="">{{ tx('任意主词条', 'Any Primary Trait') }}</option><option v-for="trait in atlas.traits" :key="trait.internalId" :value="trait.internalId">{{ trait.displayName }}</option></select>
      <button type="button" class="ui-btn is-ghost is-square" :title="tx('交换方向', 'Swap Direction')" :aria-label="tx('交换主副词条方向', 'Swap primary and secondary direction')" @click="swapTraits">⇄</button>
      <select v-model="secondTrait" class="ui-select" @change="resetPage"><option value="">{{ tx('任意副词条', 'Any Secondary Trait') }}</option><option v-for="trait in atlas.traits" :key="trait.internalId" :value="trait.internalId">{{ trait.displayName }}</option></select>
    </div>

    <p v-if="loading" class="ui-empty">{{ tx('正在读取 2.0.2 因子目录…', 'Loading the 2.0.2 sigil catalog…') }}</p>
    <div v-else-if="error" class="ui-notice is-danger"><strong>{{ tx('目录读取失败', 'Catalog Load Failed') }}</strong><span>{{ error }}</span><button type="button" class="ui-btn" @click="load">{{ tx('重试', 'Retry') }}</button></div>
    <p v-else-if="!visible.length" class="ui-empty">{{ tx('没有符合条件的合法因子。', 'No legal sigils match these filters.') }}</p>
    <div v-else class="atlas-grid">
      <article v-for="entry in visible" :key="entry.internalId" class="atlas-entry ui-card is-flat" :class="{ open: expandedId === entry.internalId }">
        <button type="button" class="atlas-entry-head" :aria-expanded="expandedId === entry.internalId" @click="toggle(entry)">
          <img v-if="entryIcon(entry)" :src="entryIcon(entry)" alt="" loading="lazy" decoding="async" />
          <span v-else class="atlas-icon-fallback" aria-hidden="true">◇</span>
          <span class="atlas-entry-copy"><small>{{ categoryLabel(entry.category) }} · {{ entry.internalId }}</small><strong>{{ entry.displayName }}</strong><em>{{ entry.primaryTraitName }} · Lv{{ entry.firstTraitMaxLevel || '—' }}</em></span>
          <span class="atlas-badges"><i :class="{ ok: entry.verified }">{{ entry.verified ? tx('已核对', 'Verified') : tx('待核对', 'Review') }}</i><i v-if="entry.supportsSecondaryTrait">+ {{ entry.secondaryTraits?.length || 0 }}</i></span>
          <b aria-hidden="true">⌄</b>
        </button>
        <div v-if="expandedId === entry.internalId" class="atlas-entry-detail">
          <dl><div><dt>{{ tx('因子等级', 'Sigil Levels') }}</dt><dd>{{ entry.allowedSigilLevels?.join(' / ') || '—' }}</dd></div><div><dt>{{ tx('主词条等级', 'Primary Levels') }}</dt><dd>{{ entry.allowedFirstTraitLevels?.join(' / ') || '—' }}</dd></div><div><dt>{{ tx('目录来源', 'Catalog Source') }}</dt><dd>{{ entry.source || tx('本机 2.0.2 表', 'Local 2.0.2 Tables') }}</dd></div></dl>
          <div v-if="entry.secondaryTraits?.length" class="secondary-pool"><small>{{ tx('允许的副词条', 'Allowed Secondary Traits') }}</small><span v-for="trait in entry.secondaryTraits" :key="trait.internalId"><img v-if="traitIcon(trait)" :src="traitIcon(trait)" alt="" loading="lazy" />{{ trait.displayName }} <em>Lv{{ trait.maxLevel }}</em></span></div>
          <p v-else>{{ entry.supportsSecondaryTrait ? tx('当前正式目录没有通过写入验证的副词条。', 'No secondary trait has passed write validation in the current catalog.') : tx('该因子为单词条结构。', 'This sigil has a single-trait structure.') }}</p>
          <div class="atlas-entry-actions"><span>{{ secondTrait ? tx('会同时使用当前反查的副词条', 'The selected lookup secondary trait will also be used') : tx('当前以主词条为目标，副词条可在后续页面继续选择', 'The primary trait is used now; choose a secondary trait later') }}</span><div><button type="button" class="ui-btn" @click="sendToOptimizer(entry)">{{ tx('送入优化目标', 'Use as Optimization Target') }}</button><button type="button" class="ui-btn" @click="sendToShareNote(entry)">{{ tx('加入分享图说明', 'Add to Share Description') }}</button><button type="button" class="ui-btn is-primary" :disabled="!entry.constructible" @click="sendToConstructor(entry)">{{ entry.constructible ? tx('送入配装构造器', 'Send to Loadout Constructor') : tx('该外壳不可构造', 'Shell Is Not Constructible') }}</button></div></div>
        </div>
      </article>
    </div>
    <nav v-if="pageCount > 1" class="atlas-pagination" :aria-label="tx('图鉴分页', 'Atlas Pagination')"><button type="button" class="ui-btn" :disabled="page <= 1" @click="page--">←</button><span>{{ page }} / {{ pageCount }}</span><button type="button" class="ui-btn" :disabled="page >= pageCount" @click="page++">→</button></nav>
  </section>
</template>

<style scoped>
.atlas-page { min-width:0; display:grid; gap:var(--space-4); container:atlas / inline-size; }
.atlas-heading { min-width:0; display:flex; align-items:end; justify-content:space-between; gap:var(--space-4); padding:2px var(--space-2); border-bottom:1px solid var(--border-soft); }
.atlas-heading > div { min-width:0; }
.atlas-heading small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.atlas-heading h2 { margin:2px 0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-xl); letter-spacing:0; }
.atlas-heading p { margin:0 0 var(--space-3); color:var(--text-muted); font-size:var(--fs-sm); }
.atlas-count { flex:0 0 auto; padding:var(--space-2) var(--space-3); border-left:2px solid var(--accent); color:var(--text-muted); font-size:var(--fs-xs); }
.atlas-count b { color:var(--accent-hover); font-size:var(--fs-lg); }
.atlas-toolbar { min-width:0; display:grid; grid-template-columns:minmax(240px,1.5fr) repeat(2,minmax(150px,.6fr)); gap:var(--space-3); padding:var(--space-3); }
.atlas-toolbar label,.atlas-search { min-width:0; display:grid; gap:4px; color:var(--text-muted); font-size:var(--fs-xs); }
.combo-finder { min-width:0; display:grid; grid-template-columns:minmax(220px,1fr) minmax(170px,.65fr) var(--control-height) minmax(170px,.65fr); gap:var(--space-3); align-items:end; padding:var(--space-4); border-left:3px solid var(--accent); }
.combo-copy { min-width:0; display:grid; gap:2px; }
.combo-copy small { color:var(--accent); font-size:var(--fs-2xs); font-weight:var(--fw-bold); }
.combo-copy strong { color:var(--text-primary); font-size:var(--fs-md); }
.combo-copy span { color:var(--text-muted); font-size:var(--fs-xs); line-height:1.4; }
.atlas-grid { min-width:0; display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,360px),1fr)); gap:var(--space-3); align-items:start; }
.atlas-entry { min-width:0; padding:0; overflow:hidden; border-left:3px solid var(--border-strong); }
.atlas-entry.open { grid-column:1 / -1; border-left-color:var(--accent); }
.atlas-entry-head { width:100%; min-width:0; display:grid; grid-template-columns:42px minmax(0,1fr) auto auto; gap:var(--space-3); align-items:center; padding:var(--space-3); border:0; background:transparent; color:inherit; text-align:left; cursor:pointer; }
.atlas-entry-head > img,.atlas-icon-fallback { width:42px; height:42px; display:grid; place-items:center; border:1px solid var(--border-strong); border-radius:var(--radius-sm); background:var(--surface-sunken); object-fit:cover; }
.atlas-entry-copy { min-width:0; display:grid; gap:2px; }
.atlas-entry-copy small,.atlas-entry-copy strong,.atlas-entry-copy em { min-width:0; overflow-wrap:anywhere; }
.atlas-entry-copy small { color:var(--text-muted); font-size:var(--fs-2xs); }
.atlas-entry-copy strong { color:var(--text-primary); font-size:var(--fs-md); }
.atlas-entry-copy em { color:var(--text-secondary); font-size:var(--fs-xs); font-style:normal; }
.atlas-badges { display:flex; flex-wrap:wrap; justify-content:flex-end; gap:4px; }
.atlas-badges i { padding:2px 6px; border:1px solid var(--border-soft); border-radius:var(--radius-sm); color:var(--text-muted); font-size:var(--fs-2xs); font-style:normal; }
.atlas-badges i.ok { border-color:var(--success); color:var(--success-ink); background:var(--success-bg); }
.atlas-entry-head > b { color:var(--accent); }
.atlas-entry.open .atlas-entry-head > b { transform:rotate(180deg); }
.atlas-entry-detail { min-width:0; display:grid; gap:var(--space-3); padding:0 var(--space-4) var(--space-4) 69px; border-top:1px dashed var(--border-soft); }
.atlas-entry-detail dl { min-width:0; display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:var(--space-2); margin:var(--space-3) 0 0; }
.atlas-entry-detail dl div { min-width:0; padding:var(--space-2); background:var(--surface-sunken); }
.atlas-entry-detail dt { color:var(--text-muted); font-size:var(--fs-2xs); }
.atlas-entry-detail dd { margin:2px 0 0; overflow-wrap:anywhere; color:var(--text-secondary); font-size:var(--fs-xs); }
.secondary-pool { min-width:0; display:flex; flex-wrap:wrap; gap:6px; align-items:center; }
.secondary-pool > small { width:100%; color:var(--text-muted); font-size:var(--fs-xs); }
.secondary-pool span { display:inline-flex; align-items:center; gap:5px; padding:3px 7px 3px 3px; border:1px solid var(--border-soft); border-radius:var(--radius-sm); color:var(--text-secondary); font-size:var(--fs-xs); }
.secondary-pool img { width:22px; height:22px; border-radius:4px; object-fit:cover; }
.secondary-pool em { color:var(--accent); font-style:normal; }
.atlas-entry-detail p { margin:0; color:var(--text-muted); font-size:var(--fs-xs); }
.atlas-entry-actions { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-3); padding-top:var(--space-2); border-top:1px solid var(--border-soft); }
.atlas-entry-actions span { min-width:0; color:var(--text-muted); font-size:var(--fs-xs); overflow-wrap:anywhere; }
.atlas-entry-actions > div { min-width:0; display:flex; flex-wrap:wrap; justify-content:flex-end; gap:var(--space-2); }
.atlas-entry-actions .ui-btn { flex:0 0 auto; }
.atlas-pagination { display:flex; justify-content:center; align-items:center; gap:var(--space-3); }
.atlas-pagination span { color:var(--text-secondary); font-variant-numeric:tabular-nums; }
@container atlas (max-width:760px) {
  .atlas-toolbar { grid-template-columns:1fr 1fr; }
  .atlas-search { grid-column:1 / -1; }
  .combo-finder { grid-template-columns:1fr var(--control-height) 1fr; }
  .combo-copy { grid-column:1 / -1; }
}
@container atlas (max-width:500px) {
  .atlas-heading { align-items:start; }
  .atlas-toolbar,.combo-finder { grid-template-columns:minmax(0,1fr); }
  .combo-copy,.atlas-search { grid-column:auto; }
  .combo-finder .is-square { justify-self:start; }
  .atlas-entry-head { grid-template-columns:38px minmax(0,1fr) auto; }
  .atlas-badges { grid-column:2 / -1; justify-content:flex-start; }
  .atlas-entry-detail { padding-left:var(--space-4); }
  .atlas-entry-detail dl { grid-template-columns:1fr; }
  .atlas-entry-actions { align-items:stretch; flex-direction:column; }
  .atlas-entry-actions > div,.atlas-entry-actions .ui-btn { width:100%; }
}
</style>
