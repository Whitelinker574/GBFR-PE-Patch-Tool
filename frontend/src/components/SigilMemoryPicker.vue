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
.picker { position: relative; flex: 1; font-family:inherit; }
.picker-selected { display:flex; align-items:center; gap:6px; width:100%; min-height:34px; padding:6px 9px; border:1px solid rgba(132,95,43,.36); border-radius:2px; background:rgba(255,249,229,.62); color:#514638; box-shadow:none; cursor:pointer; font-family:inherit; font-size:.75rem; line-height:1.35; text-align:left; }
.picker-selected:hover { border-color:rgba(65,137,151,.48); box-shadow:inset 3px 0 #4ba8b6,inset 0 1px rgba(255,255,255,.74); }
.picker-selected:disabled { opacity:.45; cursor:not-allowed; }
.picker-open .picker-selected { border-color:rgba(106,75,33,.5); border-bottom-left-radius:0; border-bottom-right-radius:0; box-shadow:inset 0 -3px #4ba8b6; }
.picker-label, .picker-placeholder { flex:1; min-width:0; overflow:visible; text-overflow:clip; white-space:normal; overflow-wrap:anywhere; }
.picker-placeholder { color:#9a876c; }
.picker-inline-clear { flex-shrink:0; background:transparent; border:0; padding:2px 6px; color:#9a7c5d; font:inherit; font-size:.8rem; line-height:1; cursor:pointer; border-radius:2px 5px 2px 5px; font-family:inherit; }
.picker-inline-clear:hover,.picker-inline-clear:focus-visible { color:#9a4d43; background:rgba(162,86,69,.1); outline:none; }
.cheveron { color:#8b6d43; font-size:.7rem; flex-shrink:0; }
.picker-dropdown { position:absolute; top:100%; left:0; right:0; z-index:20; overflow:hidden; background:linear-gradient(145deg,#fff9e8,#ead6aa); border:1px solid rgba(108,76,33,.5); border-top:none; border-radius:0 0 3px 8px; box-shadow:0 9px 22px rgba(77,52,21,.22),inset 0 0 0 1px rgba(255,255,255,.45); }
.picker-search { display:flex; align-items:center; gap:6px; padding:7px 10px; border-bottom:1px solid rgba(124,88,40,.24); background:rgba(255,249,229,.64); }
.picker-search input { flex:1; background:transparent; border:none; color:#514638; outline:none; font-family:inherit; font-size:.78rem; padding:2px 0; }
.clear-btn { background:transparent; border:1px solid rgba(255,255,255,.15); border-radius:4px; padding:2px 8px; font-size:.68rem; color:rgba(255,255,255,.55); cursor:pointer; font-family:inherit; }
.clear-btn:hover { color:#67e8f9; border-color:rgba(103,232,249,.35); }
.picker-list { max-height:220px; overflow-y:auto; padding:4px; }
.opt { min-height:36px;display:flex; justify-content:space-between; align-items:flex-start; gap:10px; margin:2px 0; padding:7px 10px; border:1px solid transparent; border-bottom-color:rgba(123,88,43,.2); font-size:.72rem; line-height:1.35; color:#5b4d3c; background:rgba(255,251,235,.5); cursor:pointer; font-family:inherit; }
.opt:last-child { border-bottom:0; }
.opt:nth-child(even) { background:rgba(226,207,166,.4); }
.opt:hover, .opt.hi, .opt.selected { color:#493e31; border-color:rgba(108,76,34,.38); background:linear-gradient(90deg,#fff9e5,#e7d3a8); box-shadow:inset 4px 0 #4ba8b6; }
.opt.selected::before { content:"✓ "; color:#397d88; }
.opt-name { display:inline-flex; align-items:flex-start; gap:6px; min-width:0; overflow:visible; text-overflow:clip; white-space:normal; overflow-wrap:anywhere; }
.opt-tag { padding:1px 5px; border:1px solid rgba(63,128,136,.3); border-radius:2px 5px 2px 5px; color:#39727a; background:#f4efd9; font-size:.6rem; letter-spacing:.5px; font-family:inherit; }
.opt-max { color:#907a5d; font-size:.7rem; font-weight:700; flex-shrink:0; }
.picker-none { padding:14px; text-align:center; color:#9b876d; font-size:.72rem; }
</style>
