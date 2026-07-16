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
    <button type="button" class="catalog-trigger" :disabled="disabled" @click="open ? closeDropdown() : openDropdown()">
      <span :class="selected ? 'catalog-value' : 'catalog-placeholder'">{{ selected?.displayName || placeholder }}</span>
      <span v-if="optional && selected" class="catalog-clear" role="button" tabindex="0" title="清除选择" @click.stop="clearSelection" @keydown.enter.stop="clearSelection">×</span>
      <span class="catalog-chevron" aria-hidden="true">{{ open ? '▴' : '▾' }}</span>
    </button>
    <div v-if="open" class="catalog-popover">
      <div class="catalog-search">
        <input ref="searchEl" v-model="query" :placeholder="searchPlaceholder" @keydown="onKey">
      </div>
      <div ref="listEl" class="catalog-options">
        <div v-if="!filtered.length" class="catalog-empty">没有匹配项</div>
        <button
          v-for="(option, index) in filtered"
          :key="option.internalId"
          type="button"
          class="catalog-option"
          :class="{ highlight: index === highlight, selected: option.internalId === modelValue }"
          @mousedown.prevent="commit(option)"
          @mouseenter="highlight = index"
        >
          <span>{{ option.displayName }}</span>
          <small v-if="detailKey && option[detailKey] != null">{{ option[detailKey] }}</small>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 下拉框：配色统一到卡片羊皮纸色阶(#fdf6e4→#efe1c0) + 金棕#9a7440，圆角/阴影与整体一致（据 Gemini 重设计） */
.catalog-select{position:relative;min-width:0;width:100%}
.catalog-trigger{width:100%;min-height:34px;display:flex;align-items:center;gap:8px;padding:7px 11px;border:1px solid rgba(154,116,64,.4);border-radius:6px;color:#4e4438;background:#fdf6e4;text-align:left;cursor:pointer;transition:border-color .18s ease,background-color .18s ease,box-shadow .18s ease}
.catalog-trigger:hover,.open .catalog-trigger{border-color:#9a7440;background:#fffdf6;box-shadow:0 0 0 1px rgba(154,116,64,.12)}
.catalog-trigger:disabled{cursor:not-allowed;opacity:.48}
.catalog-value,.catalog-placeholder{min-width:0;flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.catalog-placeholder{color:#9c8a73;font-size:.92em}
.catalog-clear{padding:0 4px;color:#a6473d;font-weight:800}
.catalog-chevron{color:#9a7440;font-size:.72rem}
.catalog-popover{position:absolute;z-index:40;top:calc(100% + 4px);left:0;right:0;overflow:hidden;border:1px solid rgba(154,116,64,.32);border-radius:6px;background:#fdf6e4;box-shadow:0 8px 24px rgba(78,68,56,.14),0 2px 8px rgba(78,68,56,.05)}
.catalog-search{padding:8px;border-bottom:1px solid rgba(154,116,64,.15);background:#f6ebd4}
.catalog-search input{width:100%;min-height:30px;padding:5px 9px;border:1px solid rgba(154,116,64,.3);border-radius:4px;color:#4e4438;background:#fffdf6;outline:0}
.catalog-search input:focus{border-color:#9a7440}
.catalog-options{max-height:236px;overflow-y:auto}
.catalog-option{width:100%;min-height:36px;display:flex;align-items:center;justify-content:space-between;gap:10px;padding:8px 12px;border:0;border-bottom:1px solid rgba(154,116,64,.09);color:#6f6152;background:transparent;text-align:left;cursor:pointer;transition:background-color .14s ease,color .14s ease}
.catalog-option:nth-child(even){background:rgba(154,116,64,.035)}
.catalog-option>span:first-child{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.catalog-option:hover,.catalog-option.highlight,.catalog-option.selected{color:#4e4438;background:#efe1c0;box-shadow:inset 3px 0 #9a7440}
.catalog-option small{flex:0 0 auto;color:#9c8a73;font-size:.82em;font-weight:600}
.catalog-option:hover small,.catalog-option.highlight small,.catalog-option.selected small{color:#7a664d}
.catalog-empty{padding:14px;color:#9c8a73;text-align:center;font-size:.78rem}
</style>
