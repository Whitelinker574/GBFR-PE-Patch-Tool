<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { matchText } from '../utils/matchText.js'

const props = defineProps({
  modelValue: { type: Number, default: 0 },
  options: { type: Array, default: () => [] },
  optional: { type: Boolean, default: false },
  placeholder: { type: String, default: '未选择' },
  disabled: { type: Boolean, default: false },
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
    <button type="button" class="picker-selected" @click="open ? closeDropdown() : openDropdown()" :disabled="disabled">
      <span v-if="selected" class="picker-label" :title="selected.displayName">{{ selected.displayName }}</span>
      <span v-else class="picker-placeholder">{{ placeholder }}</span>
      <span v-if="optional && selected" class="picker-inline-clear" role="button" tabindex="0" @click.stop="clearSelection" @keydown.enter.stop="clearSelection" title="移除">✕</span>
      <span class="cheveron">{{ open ? '▴' : '▾' }}</span>
    </button>
    <div v-if="open" class="picker-dropdown">
      <div class="picker-search">
        <input ref="searchEl" v-model="query" @keydown="onKey" placeholder="搜索名称 / 拼音 / hex" />
      </div>
      <div class="picker-list" ref="listEl">
        <div v-if="!filtered.length" class="picker-none">无匹配</div>
        <div
          v-for="(opt, i) in filtered"
          :key="opt.hash"
          class="opt"
          :class="{ hi: i === highlight, selected: (opt.hash >>> 0) === (modelValue >>> 0) }"
          @mousedown.prevent="commit(opt)"
          @mouseenter="highlight = i"
          :title="`${opt.displayName} · ${hex(opt.hash)}`"
        >
          <span class="opt-name">
            {{ opt.displayName }}
            <span v-if="opt.source === 'memory-only'" class="opt-tag">补</span>
          </span>
          <span v-if="opt.maxLevel != null" class="opt-max">Lv {{ opt.maxLevel }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 样式统一到 CatalogSelect(Gemini 重设计)：羊皮纸色阶#fdf6e4→#efe1c0 + 金棕#9a7440，
   去掉原来的青色强调与双色渐变；选项名单行不换行。 */
.picker { position: relative; flex: 1; font-family:inherit; }
.picker-selected { display:flex; align-items:center; gap:8px; width:100%; min-height:34px; padding:7px 11px; border:1px solid rgba(154,116,64,.4); border-radius:6px; background:#fdf6e4; color:#4e4438; cursor:pointer; font-family:inherit; font-size:.75rem; line-height:1.35; text-align:left; transition:border-color .18s ease,background-color .18s ease,box-shadow .18s ease; }
.picker-selected:hover,.picker-open .picker-selected { border-color:#9a7440; background:#fffdf6; box-shadow:0 0 0 1px rgba(154,116,64,.12); }
.picker-selected:disabled { opacity:.48; cursor:not-allowed; }
.picker-label, .picker-placeholder { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.picker-placeholder { color:#9c8a73; }
.picker-inline-clear { flex-shrink:0; background:transparent; border:0; padding:0 4px; color:#a6473d; font:inherit; font-size:.8rem; line-height:1; cursor:pointer; font-weight:800; font-family:inherit; }
.picker-inline-clear:hover,.picker-inline-clear:focus-visible { color:#8a3a30; outline:none; }
.cheveron { color:#9a7440; font-size:.7rem; flex-shrink:0; }
.picker-dropdown { position:absolute; top:calc(100% + 4px); left:0; right:0; z-index:40; overflow:hidden; background:#fdf6e4; border:1px solid rgba(154,116,64,.32); border-radius:6px; box-shadow:0 8px 24px rgba(78,68,56,.14),0 2px 8px rgba(78,68,56,.05); }
.picker-search { display:flex; align-items:center; gap:6px; padding:8px; border-bottom:1px solid rgba(154,116,64,.15); background:#f6ebd4; }
.picker-search input { flex:1; min-height:30px; padding:5px 9px; border:1px solid rgba(154,116,64,.3); border-radius:4px; background:#fffdf6; color:#4e4438; outline:none; font-family:inherit; font-size:.78rem; }
.picker-search input:focus { border-color:#9a7440; }
.picker-list { max-height:236px; overflow-y:auto; }
.opt { min-height:36px;display:flex; justify-content:space-between; align-items:center; gap:10px; padding:8px 12px; border:0; border-bottom:1px solid rgba(154,116,64,.09); font-size:.72rem; line-height:1.35; color:#6f6152; background:transparent; cursor:pointer; font-family:inherit; transition:background-color .14s ease,color .14s ease; }
.opt:nth-child(even) { background:rgba(154,116,64,.035); }
.opt:hover, .opt.hi, .opt.selected { color:#4e4438; background:#efe1c0; box-shadow:inset 3px 0 #9a7440; }
.opt-name { flex:1; min-width:0; display:inline-flex; align-items:center; gap:6px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.opt-tag { flex:0 0 auto; padding:1px 5px; border:1px solid rgba(154,116,64,.4); border-radius:20px; color:#8a6a34; background:rgba(154,116,64,.1); font-size:.6rem; letter-spacing:.5px; font-family:inherit; }
.opt-max { flex:0 0 auto; color:#9c8a73; font-size:.82em; font-weight:600; }
.opt:hover .opt-max,.opt.hi .opt-max,.opt.selected .opt-max { color:#7a664d; }
.picker-none { padding:14px; text-align:center; color:#9c8a73; font-size:.72rem; }
</style>
