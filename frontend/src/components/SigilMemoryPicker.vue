<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { matchText } from '../utils/matchText.js'

const props = defineProps({
  modelValue: { type: Number, default: 0 },
  options: { type: Array, default: () => [] },
  optional: { type: Boolean, default: false },
  placeholder: { type: String, default: '未选择' },
  disabled: { type: Boolean, default: false },
  iconResolver: { type: Function, default: null },
})

const emit = defineEmits(['update:modelValue', 'pick', 'open', 'close'])

const open = ref(false)
const query = ref('')
const highlight = ref(0)
const rootEl = ref(null)
const searchEl = ref(null)
const listEl = ref(null)

function hex(v) { return '0x' + (Number(v) >>> 0).toString(16).toUpperCase().padStart(8, '0') }

const selected = computed(() => props.options.find(o => (o.hash >>> 0) === (props.modelValue >>> 0)) || null)

const filtered = computed(() => {
  const q = query.value.trim()
  if (!q) return props.options
  return props.options.filter(o => matchText(o.displayName, q) || matchText(hex(o.hash), q))
})

function optionIcon(option) {
  return option && props.iconResolver ? props.iconResolver(option) || '' : ''
}

function openDropdown() {
  if (props.disabled) return
  open.value = true
  query.value = ''
  const idx = filtered.value.findIndex(o => (o.hash >>> 0) === (props.modelValue >>> 0))
  highlight.value = idx >= 0 ? idx : 0
  emit('open')
  nextTick(() => searchEl.value?.focus())
}

function closeDropdown() {
  if (!open.value) return
  open.value = false
  emit('close')
}

function commit(opt) {
  if (!opt) return
  emit('update:modelValue', opt.hash >>> 0)
  emit('pick', opt)
  closeDropdown()
}

function clearSelection() {
  if (props.disabled) return
  emit('update:modelValue', 0)
  emit('pick', null)
  closeDropdown()
}

function onKey(e) {
  const list = filtered.value
  if (e.key === 'ArrowDown') { e.preventDefault(); highlight.value = Math.min(highlight.value + 1, list.length - 1); scrollToHighlight() }
  else if (e.key === 'ArrowUp') { e.preventDefault(); highlight.value = Math.max(highlight.value - 1, 0); scrollToHighlight() }
  else if (e.key === 'Enter') { e.preventDefault(); commit(list[highlight.value]) }
  else if (e.key === 'Escape') { e.preventDefault(); closeDropdown() }
}

function scrollToHighlight() {
  nextTick(() => {
    const el = listEl.value?.querySelector('.opt.hi')
    el?.scrollIntoView({ block: 'nearest' })
  })
}

watch(query, () => { highlight.value = 0 })

function onDocClick(e) {
  if (!open.value) return
  if (rootEl.value && !rootEl.value.contains(e.target)) closeDropdown()
}

watch(open, (v) => {
  if (v) document.addEventListener('mousedown', onDocClick)
  else document.removeEventListener('mousedown', onDocClick)
})

onBeforeUnmount(() => document.removeEventListener('mousedown', onDocClick))
</script>

<template>
  <div class="picker" :class="{ 'picker-open': open, disabled }" ref="rootEl">
    <div class="picker-control">
      <button type="button" class="picker-selected ui-btn" @click="open ? closeDropdown() : openDropdown()" :disabled="disabled" :aria-expanded="open">
        <img v-if="optionIcon(selected)" class="picker-icon" :src="optionIcon(selected)" alt="" />
        <span v-if="selected" class="picker-label" :title="selected.displayName">{{ selected.displayName }}</span>
        <span v-else class="picker-placeholder">{{ placeholder }}</span>
        <span class="picker-chevron" aria-hidden="true">{{ open ? '▴' : '▾' }}</span>
      </button>
      <button v-if="optional && selected" type="button" class="picker-inline-clear ui-btn is-icon is-ghost" :disabled="disabled" @click="clearSelection" title="移除当前选择" aria-label="移除当前选择">×</button>
    </div>
    <div v-if="open" class="picker-dropdown ui-card is-raised">
      <div class="picker-search">
        <input ref="searchEl" v-model="query" class="ui-input" @keydown="onKey" placeholder="搜索名称 / 拼音 / Hash" aria-label="搜索因子或词条" />
      </div>
      <div class="picker-list ui-scroll-region" ref="listEl">
        <div v-if="!filtered.length" class="ui-empty picker-none">无匹配结果</div>
        <button
          v-for="(opt, i) in filtered"
          :key="opt.hash"
          type="button"
          class="opt ui-row"
          :class="{ hi: i === highlight, 'is-on': (opt.hash >>> 0) === (modelValue >>> 0) }"
          @click="commit(opt)"
          @mouseenter="highlight = i"
          :title="`${opt.displayName} · ${hex(opt.hash)}`"
        >
          <span class="opt-name">
            <img v-if="optionIcon(opt)" class="picker-icon" :src="optionIcon(opt)" alt="" />
            {{ opt.displayName }}
          </span>
          <span v-if="opt.maxLevel != null" class="opt-max">Lv {{ opt.maxLevel }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.picker { position:relative; min-width:0; flex:1; }
.picker-control { display:grid; min-width:0; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-2); }
.picker-selected { width:100%; justify-content:flex-start; text-align:left; }
.picker-label, .picker-placeholder { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.picker-icon { width:28px; height:28px; flex:0 0 28px; object-fit:cover; border:1px solid var(--line-soft); border-radius:6px; background:var(--surface-field); }
.picker-placeholder { color:var(--text-muted); }
.picker-inline-clear { color:var(--danger-ink); }
.picker-chevron { flex:0 0 auto; color:var(--accent); font-size:var(--fs-xs); }
.picker-dropdown { position:absolute; top:calc(100% + var(--space-1)); left:0; right:0; z-index:var(--z-dropdown); overflow:hidden; }
.picker-search { display:flex; padding:var(--space-3); border-bottom:1px solid var(--border-soft); background:var(--surface-field); }
.picker-list { max-height:236px; overflow-y:auto; }
.opt { width:100%; min-height:var(--control-height); justify-content:space-between; border-width:0 0 1px; border-radius:0; box-shadow:none; text-align:left; }
.opt:last-child { border-bottom:0; }
.opt.hi { background:var(--surface-row-hover); }
.opt-name { flex:1; min-width:0; display:inline-flex; align-items:center; gap:6px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.opt-max { flex:0 0 auto; color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); }
.picker-none { padding:var(--space-6); }
</style>
