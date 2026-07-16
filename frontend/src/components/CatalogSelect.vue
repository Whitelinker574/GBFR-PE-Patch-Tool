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
.catalog-select{position:relative;min-width:0;width:100%}.catalog-trigger{width:100%;min-height:34px;display:flex;align-items:center;gap:8px;padding:7px 10px;border:1px solid rgba(126,91,42,.36);border-radius:2px;color:#514638;background:#fff9e8;text-align:left;cursor:pointer}.catalog-trigger:hover,.open .catalog-trigger{border-color:rgba(126,91,42,.5);box-shadow:inset 3px 0 #9a7440}.catalog-trigger:disabled{cursor:not-allowed;opacity:.48}.catalog-value,.catalog-placeholder{min-width:0;flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.catalog-placeholder{color:#98866d;font-size:.92em}.catalog-clear{padding:0 4px;color:#92624e;font-weight:800}.catalog-chevron{color:#7b603d;font-size:.72rem}.catalog-popover{position:absolute;z-index:40;top:calc(100% + 3px);left:0;right:0;overflow:hidden;border:1px solid rgba(112,77,31,.48);border-radius:2px;background:#f3e4c3;box-shadow:0 9px 22px rgba(77,52,21,.2)}.catalog-search{padding:7px;border-bottom:1px solid rgba(126,91,42,.24);background:#ead8b2}.catalog-search input{width:100%;min-height:30px;padding:5px 8px;border:1px solid rgba(126,91,42,.32);border-radius:2px;color:#514638;background:#fff9e8;outline:0}.catalog-options{max-height:226px;overflow-y:auto}.catalog-option{width:100%;min-height:36px;display:flex;align-items:center;justify-content:space-between;gap:10px;padding:7px 10px;border:0;border-bottom:1px solid rgba(126,91,42,.18);color:#574a39;background:#f8edcf;text-align:left;cursor:pointer}.catalog-option:nth-child(even){background:#f0dfba}.catalog-option:hover,.catalog-option.highlight,.catalog-option.selected{color:#443725;background:#ead5a8;box-shadow:inset 3px 0 #9a7440}.catalog-option small{flex:0 0 auto;color:#866e4c;font-size:.72em}.catalog-empty{padding:14px;color:#927f65;text-align:center;font-size:.78rem}
</style>
