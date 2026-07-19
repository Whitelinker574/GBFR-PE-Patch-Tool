<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { matchText } from '../utils/matchText.js'

const props = defineProps({
  modelValue: { type: String, default: '' },
  options: { type: Array, default: () => [] },
  placeholder: { type: String, default: '尚未选择' },
  searchPlaceholder: { type: String, default: '输入关键词筛选' },
  disabled: { type: Boolean, default: false },
  optional: { type: Boolean, default: false },
  detailKey: { type: String, default: '' },
  iconResolver: { type: Function, default: null },
})

const emit = defineEmits(['update:modelValue', 'pick'])
const rootEl = ref(null)
const searchEl = ref(null)
const listEl = ref(null)
const open = ref(false)
const query = ref('')
const highlight = ref(0)

const selected = computed(() => props.options.find(option => option.internalId === props.modelValue) || null)
const filtered = computed(() => {
  const q = query.value.trim()
  if (!q) return props.options
  return props.options.filter(option => matchText(option.displayName, q) || matchText(option.internalId, q))
})

function optionIcon(option) {
  return option && props.iconResolver ? props.iconResolver(option) || '' : ''
}

function openDropdown() {
  if (props.disabled) return
  open.value = true
  query.value = ''
  const index = filtered.value.findIndex(option => option.internalId === props.modelValue)
  highlight.value = index >= 0 ? index : 0
  nextTick(() => searchEl.value?.focus())
}

function closeDropdown() { open.value = false }

function commit(option) {
  if (!option) return
  emit('update:modelValue', option.internalId)
  emit('pick', option)
  closeDropdown()
}

function clearSelection() {
  emit('update:modelValue', '')
  emit('pick', null)
  closeDropdown()
}

function scrollToHighlight() {
  nextTick(() => listEl.value?.querySelector('.catalog-option.highlight')?.scrollIntoView({ block: 'nearest' }))
}

function onKey(event) {
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    highlight.value = Math.min(highlight.value + 1, filtered.value.length - 1)
    scrollToHighlight()
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    highlight.value = Math.max(highlight.value - 1, 0)
    scrollToHighlight()
  } else if (event.key === 'Enter') {
    event.preventDefault()
    commit(filtered.value[highlight.value])
  } else if (event.key === 'Escape') {
    event.preventDefault()
    closeDropdown()
  }
}

function onDocumentPointer(event) {
  if (open.value && rootEl.value && !rootEl.value.contains(event.target)) closeDropdown()
}

watch(query, () => { highlight.value = 0 })
watch(open, value => value
  ? document.addEventListener('mousedown', onDocumentPointer)
  : document.removeEventListener('mousedown', onDocumentPointer))
onBeforeUnmount(() => document.removeEventListener('mousedown', onDocumentPointer))
</script>

<template>
  <div ref="rootEl" class="catalog-select" :class="{ open, disabled }">
    <button type="button" class="catalog-trigger ui-btn" :disabled="disabled" @click="open ? closeDropdown() : openDropdown()">
      <img v-if="optionIcon(selected)" class="catalog-icon" :src="optionIcon(selected)" alt="" />
      <span :class="selected ? 'catalog-value' : 'catalog-placeholder'">{{ selected?.displayName || placeholder }}</span>
      <span v-if="optional && selected" class="catalog-clear" role="button" tabindex="0" title="清除选择" @click.stop="clearSelection" @keydown.enter.stop="clearSelection">×</span>
      <span class="catalog-chevron" aria-hidden="true">{{ open ? '▴' : '▾' }}</span>
    </button>
    <div v-if="open" class="catalog-popover ui-card">
      <div class="catalog-search">
        <input ref="searchEl" v-model="query" class="catalog-search-input ui-input" :placeholder="searchPlaceholder" @keydown="onKey">
      </div>
      <div ref="listEl" class="catalog-options">
        <div v-if="!filtered.length" class="catalog-empty ui-empty">没有匹配项</div>
        <button
          v-for="(option, index) in filtered"
          :key="option.internalId"
          type="button"
          class="catalog-option ui-row"
          :class="{ highlight: index === highlight, selected: option.internalId === modelValue }"
          @mousedown.prevent="commit(option)"
          @mouseenter="highlight = index"
        >
          <span class="catalog-option-main">
            <img v-if="optionIcon(option)" class="catalog-icon" :src="optionIcon(option)" alt="" />
            <span>{{ option.displayName }}</span>
          </span>
          <small v-if="detailKey && option[detailKey] != null">{{ option[detailKey] }}</small>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.catalog-select { position:relative; width:100%; min-width:0; }
.catalog-trigger { width:100%; min-height:var(--control-height); justify-content:flex-start; gap:var(--space-3); padding-inline:var(--space-4); text-align:left; }
.open .catalog-trigger { border-color:var(--accent-border); background:var(--surface-field-hover); box-shadow:var(--focus-ring); }
.catalog-value,.catalog-placeholder { min-width:0; flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.catalog-icon { width:28px; height:28px; flex:0 0 28px; object-fit:cover; border:1px solid var(--line-soft); border-radius:6px; background:var(--surface-field); }
.catalog-placeholder { color:var(--text-muted); font-size:var(--fs-sm); font-weight:var(--fw-normal); }
.catalog-clear { display:grid; width:24px; height:24px; flex:0 0 24px; place-items:center; border-radius:var(--radius-sm); color:var(--danger-ink); font-size:var(--fs-base); font-weight:var(--fw-bold); }
.catalog-clear:hover,.catalog-clear:focus-visible { background:var(--danger-bg); }
.catalog-chevron { flex:0 0 auto; color:var(--accent); font-size:var(--fs-xs); }
.catalog-popover { position:absolute; z-index:var(--z-dropdown); top:calc(100% + var(--space-1)); right:0; left:0; overflow:hidden; background:var(--surface-card-pop); box-shadow:var(--shadow-2); }
.catalog-search { padding:var(--space-3); border-bottom:1px solid var(--border-soft); background:var(--surface-field); }
.catalog-search-input { width:100%; }
.catalog-options { max-height:236px; overflow-y:auto; overscroll-behavior:contain; }
.catalog-option { width:100%; min-height:var(--control-height); justify-content:space-between; gap:var(--space-4); border-width:0 0 1px; border-radius:0; background:var(--surface-card-pop); color:var(--text-secondary); box-shadow:none; text-align:left; cursor:pointer; }
.catalog-option:last-child { border-bottom:0; }
.catalog-option-main { min-width:0; flex:1; display:flex; align-items:center; gap:var(--space-3); }
.catalog-option-main > span { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.catalog-option:hover,.catalog-option.highlight,.catalog-option.selected { color:var(--text-primary); background:var(--surface-row-hover); box-shadow:3px 0 0 var(--selected-bar) inset; }
.catalog-option small { flex:0 0 auto; color:var(--text-muted); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.catalog-option:hover small,.catalog-option.highlight small,.catalog-option.selected small { color:var(--text-secondary); }
.catalog-empty { padding:var(--space-7) var(--space-4); color:var(--text-muted); font-size:var(--fs-sm); }
</style>
