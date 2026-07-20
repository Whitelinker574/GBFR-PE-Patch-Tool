<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import OpenCC from 'opencc-js/t2cn'
import { LoadBadgeState, SetAllBadgeStates, SetBadgeState } from '../../wailsjs/go/main/App'
import ConfirmDialog from './ConfirmDialog.vue'
import { matchText } from '../utils/matchText.js'
import { language } from '../i18n.js'

const props = defineProps({ savePath: { type: String, default: '' } })
const emit = defineEmits(['status'])

const badges = ref([])
const loading = ref(false)
const saving = ref(false)
const query = ref('')
const filter = ref('all')
const markViewed = ref(true)
const state = ref(null)
const listRef = ref(null)
const scrollTop = ref(0)
const viewportHeight = ref(520)
const confirmDialog = ref(null)
const rowHeight = 52
const overscan = 5
let resizeObserver = null
const toSimplified = OpenCC.Converter({ from: 'tw', to: 'cn' })

const copy = computed(() => language.value === 'en' ? {
  editor: 'Title Record Editor', chooseSave: 'Select a save to load title records', loading: 'Loading the 1700-bit title vectors…',
  summary: 'Title Unlocks', viewed: 'Viewed', claimed: 'Rewards Claimed', scope: 'Only unlock and optional viewed records are changed. Reward-claim records are preserved.',
  searchLabel: 'Search titles', searchPlaceholder: 'Search by name or ID', filterLabel: 'Filter title status', syncViewed: 'Mark records as viewed when writing', results: 'results',
  clearAll: 'Clear All Unlocks', unlockAll: 'Unlock All', writing: 'Writing…', list: 'Title List', unlocked: 'Unlocked', locked: 'Locked', empty: 'No titles match the current filters.',
  filters: { all: 'All', unlocked: 'Unlocked', locked: 'Locked', unviewed: 'Not Viewed' },
} : {
  editor: '称号记录编辑', chooseSave: '选择存档后读取称号记录', loading: '正在读取 1700 位称号向量…',
  summary: '称号解锁', viewed: '已查看', claimed: '已领奖', scope: '只修改解锁与可选的已查看记录；奖励领取记录不会修改。',
  searchLabel: '搜索称号', searchPlaceholder: '搜索名称、拼音或编号', filterLabel: '称号状态筛选', syncViewed: '写入时同步标记已查看', results: '条结果',
  clearAll: '清除全部解锁', unlockAll: '全部解锁', writing: '写入中…', list: '称号列表', unlocked: '已解锁', locked: '未解锁', empty: '没有符合筛选条件的称号',
  filters: { all: '全部', unlocked: '已解锁', locked: '未解锁', unviewed: '未查看' },
})
const filterOptions = computed(() => ['all', 'unlocked', 'locked', 'unviewed'].map(id => ({ id, label: copy.value.filters[id] })))

function isolatedError(error, englishFallback) {
  const raw = String(error || '')
  return language.value === 'en' && /[\u3400-\u9fff]/u.test(raw) ? englishFallback : raw
}

const filteredBadges = computed(() => {
  const needle = query.value.trim()
  return badges.value.filter(badge => {
    if (filter.value === 'unlocked' && !badge.unlocked) return false
    if (filter.value === 'locked' && badge.unlocked) return false
    if (filter.value === 'unviewed' && badge.viewed) return false
    if (!needle) return true
    return [badge.id, badge.nameZh, badge.nameZhSimplified, badge.nameEn].some(value => matchText(String(value || ''), needle))
  })
})

const startIndex = computed(() => Math.max(0, Math.floor(scrollTop.value / rowHeight) - overscan))
const visibleCount = computed(() => Math.ceil(viewportHeight.value / rowHeight) + overscan * 2)
const visibleBadges = computed(() => filteredBadges.value.slice(startIndex.value, startIndex.value + visibleCount.value))
const topSpacer = computed(() => startIndex.value * rowHeight)
const bottomSpacer = computed(() => Math.max(0, (filteredBadges.value.length - startIndex.value - visibleBadges.value.length) * rowHeight))

function badgeName(badge) {
  return (language.value === 'en' ? badge.nameEn : badge.nameZhSimplified) || (language.value === 'en' ? `Title #${badge.id}` : `称号 #${badge.id}`)
}

function prepareBadges(items) {
  return (items || []).map(badge => ({
    ...badge,
    nameZhSimplified: toSimplified(badge.nameZh || ''),
  }))
}

function resetScroll() {
  scrollTop.value = 0
  nextTick(() => {
    if (listRef.value) listRef.value.scrollTop = 0
  })
}

function onScroll(event) {
  scrollTop.value = event.currentTarget.scrollTop
}

async function load() {
  badges.value = []
  state.value = null
  resetScroll()
  if (!props.savePath) return
  loading.value = true
  try {
    const result = await LoadBadgeState(props.savePath)
    state.value = result || null
    badges.value = prepareBadges(result?.badges)
  } catch (err) {
    emit('status', isolatedError(err, 'Failed to load title records.'), 'error')
  } finally {
    loading.value = false
  }
}

async function toggleBadge(badge) {
  if (saving.value || !props.savePath) return
  saving.value = true
  try {
    const result = await SetBadgeState(props.savePath, badge.id, !badge.unlocked, markViewed.value)
    emit('status', language.value === 'en'
      ? `Title #${badge.id} was written and ${result.verified} record was verified.`
      : `称号 #${badge.id} 已写入并回读验证 ${result.verified} 项`, 'success')
    await load()
  } catch (err) {
    emit('status', isolatedError(err, 'Failed to write the title record.'), 'error')
  } finally {
    saving.value = false
  }
}

async function setAll(unlocked) {
  if (saving.value || !props.savePath) return
  const confirmed = await confirmDialog.value?.ask({
    title: language.value === 'en'
      ? (unlocked ? 'Unlock All Title Records' : 'Clear All Title Unlock Records')
      : (unlocked ? '解锁全部称号记录' : '清除全部称号解锁记录'),
    message: language.value === 'en'
      ? `This will modify ${state.value?.total || badges.value.length} title unlock records in the catalog.`
      : `将修改目录中的 ${state.value?.total || badges.value.length} 条称号解锁记录。`,
    detail: language.value === 'en'
      ? (markViewed.value ? 'The same records will also be marked as viewed. Reward-claim records are preserved.' : 'Viewed and reward-claim records are both preserved.')
      : (markViewed.value ? '同时把对应记录标记为已查看；奖励领取记录不会修改。' : '已查看与奖励领取记录都不会修改。'),
    tone: unlocked ? 'warning' : 'danger',
    confirmLabel: language.value === 'en'
      ? (unlocked ? 'Back Up and Unlock' : 'Back Up and Clear')
      : (unlocked ? '备份并解锁' : '备份并清除'),
  })
  if (!confirmed) return
  saving.value = true
  try {
    const result = await SetAllBadgeStates(props.savePath, unlocked, markViewed.value)
    emit('status', language.value === 'en'
      ? `${result.verified} title records were written and verified.`
      : `已写入并回读验证 ${result.verified} 条称号记录`, 'success')
    await load()
  } catch (err) {
    emit('status', isolatedError(err, 'Failed to update title records.'), 'error')
  } finally {
    saving.value = false
  }
}

watch(() => props.savePath, load, { immediate: true })
watch([query, filter, language], resetScroll)
watch(listRef, (nextList, previousList) => {
  if (previousList) resizeObserver?.unobserve(previousList)
  if (nextList) resizeObserver?.observe(nextList)
})

onMounted(() => {
  resizeObserver = new ResizeObserver(entries => {
    viewportHeight.value = entries[0]?.contentRect?.height || 520
  })
  if (listRef.value) resizeObserver.observe(listRef.value)
})
onBeforeUnmount(() => resizeObserver?.disconnect())
</script>

<template>
  <section class="badge-editor ui-card" :aria-label="copy.editor">
    <div v-if="!savePath" class="badge-empty ui-empty">{{ copy.chooseSave }}</div>
    <div v-else-if="loading" class="badge-empty ui-empty">{{ copy.loading }}</div>
    <template v-else-if="state">
      <header class="badge-summary">
        <div>
          <span>{{ copy.summary }}</span>
          <strong>{{ state.unlockedCount }} / {{ state.total }}</strong>
          <small>{{ copy.viewed }} {{ state.viewedCount }} · {{ copy.claimed }} {{ state.rewardClaimedCount }}</small>
        </div>
        <p>{{ copy.scope }}</p>
      </header>

      <div class="badge-tools">
        <input v-model="query" class="ui-input badge-search" :aria-label="copy.searchLabel" :placeholder="copy.searchPlaceholder">
        <div class="badge-filters" role="group" :aria-label="copy.filterLabel">
          <button v-for="option in filterOptions" :key="option.id" type="button" class="ui-btn is-sm" :class="{ on: filter === option.id }" @click="filter = option.id">{{ option.label }}</button>
        </div>
      </div>

      <div class="badge-batch">
        <label><input v-model="markViewed" type="checkbox"> {{ copy.syncViewed }}</label>
        <span>{{ filteredBadges.length }} {{ copy.results }}</span>
        <button type="button" class="ui-btn is-sm" :disabled="saving" @click="setAll(false)">{{ copy.clearAll }}</button>
        <button type="button" class="ui-btn is-sm is-primary" :disabled="saving" @click="setAll(true)">{{ saving ? copy.writing : copy.unlockAll }}</button>
      </div>

      <div ref="listRef" class="badge-list" role="list" :aria-label="copy.list" @scroll="onScroll">
        <div :style="{ height: `${topSpacer}px` }" aria-hidden="true"></div>
        <label v-for="badge in visibleBadges" :key="badge.id" v-memo="[badge.id, badge.unlocked, badge.viewed, badge.rewardClaimed, language]" class="badge-row ui-row" role="listitem">
          <input type="checkbox" :checked="badge.unlocked" :disabled="saving" @change="toggleBadge(badge)">
          <span class="badge-id">#{{ badge.id }}</span>
          <span class="badge-name"><b>{{ badgeName(badge) }}</b></span>
          <span class="badge-state" :class="{ on: badge.unlocked }">{{ badge.unlocked ? copy.unlocked : copy.locked }}</span>
          <span class="badge-flags"><i v-if="badge.viewed">{{ copy.viewed }}</i><i v-if="badge.rewardClaimed">{{ copy.claimed }}</i></span>
        </label>
        <div :style="{ height: `${bottomSpacer}px` }" aria-hidden="true"></div>
        <div v-if="!filteredBadges.length" class="badge-empty ui-empty">{{ copy.empty }}</div>
      </div>
    </template>
    <ConfirmDialog ref="confirmDialog" />
  </section>
</template>

<style scoped>
.badge-editor {
  min-width:0;
  min-height:0;
  flex:1;
  display:flex;
  flex-direction:column;
  overflow:hidden;
  container-type:inline-size;
}
.badge-summary,.badge-tools,.badge-batch {
  display:flex;
  align-items:center;
  gap:var(--space-3);
  padding:var(--space-3) var(--space-4);
  border-bottom:1px solid var(--border-soft);
}
.badge-summary { justify-content:space-between; background:var(--surface-sunken); }
.badge-summary > div { display:flex; align-items:baseline; gap:var(--space-3); }
.badge-summary span,.badge-summary small { color:var(--text-secondary); font-size:var(--fs-sm); }
.badge-summary strong { color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-xl); }
.badge-summary p { margin:0; color:var(--text-secondary); font-size:var(--fs-xs); }
.badge-tools { flex-wrap:wrap; }
.badge-search { min-width:190px; flex:1; }
.badge-filters { display:flex; flex-wrap:wrap; gap:var(--space-2); }
.badge-filters .on { border-color:var(--selected-border); color:var(--selected-fg); background:var(--selected-bg); }
.badge-batch { flex-wrap:wrap; color:var(--text-secondary); background:var(--surface-sunken); font-size:var(--fs-sm); }
.badge-batch label { display:flex; align-items:center; gap:var(--space-2); margin-right:auto; }
.badge-list { height:clamp(240px,52dvh,560px); min-height:220px; flex:none; overflow-x:hidden; overflow-y:auto; overscroll-behavior:contain; scrollbar-width:thin; scrollbar-color:var(--border-default) transparent; }
.badge-row {
  height:52px;
  display:grid;
  grid-template-columns:20px 54px minmax(160px,1fr) 64px minmax(0,110px);
  align-items:center;
  gap:var(--space-3);
  padding:0 var(--space-4);
  border:0;
  border-bottom:1px solid var(--border-soft);
  border-radius:0;
  background:transparent;
}
.badge-row:hover { background:var(--surface-hover); }
.badge-id { color:var(--text-tertiary); font-family:var(--font-data); font-size:var(--fs-xs); }
.badge-name { min-width:0; display:flex; flex-direction:column; }
.badge-name b { overflow:hidden; color:var(--text-primary); font-size:var(--fs-sm); text-overflow:ellipsis; white-space:nowrap; }
.badge-state { color:var(--text-tertiary); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.badge-state.on { color:var(--success-ink); }
.badge-flags { min-width:0; display:flex; justify-content:flex-end; gap:var(--space-1); }
.badge-flags i { padding:2px 5px; border:1px solid var(--border-soft); border-radius:var(--radius-pill); color:var(--text-tertiary); font-size:var(--fs-2xs); font-style:normal; white-space:nowrap; }
.badge-empty { min-height:180px; flex:1; }
input[type="checkbox"] { accent-color:var(--accent); }

@container (max-width:620px) {
  .badge-summary { align-items:flex-start; flex-direction:column; }
  .badge-summary p { max-width:none; }
  .badge-search { width:100%; flex-basis:100%; }
  .badge-filters { width:100%; }
  .badge-filters .ui-btn { flex:1; }
  .badge-batch > span { width:100%; order:2; }
  .badge-batch .ui-btn { flex:1; order:3; }
  .badge-row { grid-template-columns:18px 42px minmax(0,1fr) 58px; padding-inline:var(--space-3); gap:var(--space-2); }
  .badge-flags { display:none; }
}
</style>
