<script setup>
const props = defineProps({
  slots: { type: Array, default: () => [] },
  modelValue: { type: String, default: '' },
  busy: { type: Boolean, default: false },
  loaded: { type: Boolean, default: false },
  summary: { type: String, default: '' },
  helper: { type: String, default: '与其他离线编辑页面使用同一组游戏存档' },
})

const emit = defineEmits(['update:modelValue', 'select', 'browse'])

function saveSlotLabel(slot) {
  const fileName = String(slot?.name || slot?.path || '').split(/[\\/]/).pop()
  const match = fileName.match(/SaveData(\d+)/i)
  return match ? `存档 ${match[1]}` : fileName.replace(/\.dat$/i, '')
}

function select(path) {
  emit('update:modelValue', path)
  emit('select', path)
}
</script>

<template>
  <section class="save-source-picker ui-card compact-save-bar">
    <div class="source-title">
      <div><strong>选择存档槽</strong><small>{{ helper }}</small></div>
      <span v-if="loaded && summary" class="ui-tag is-success">{{ summary }}</span>
    </div>
    <div class="save-slots">
      <button v-for="slot in slots" :key="slot.index ?? slot.path" class="slot-choice ui-btn is-sm" :class="{ on: modelValue === slot.path, 'is-primary': modelValue === slot.path }" :title="slot.name || slot.path" :disabled="busy" @click="select(slot.path)">{{ saveSlotLabel(slot) }}</button>
      <button class="slot-choice secondary ui-btn is-sm" :disabled="busy" @click="emit('browse')">选择其他存档</button>
    </div>
    <div class="selected-save" :class="{ empty: !modelValue }" :title="modelValue">{{ modelValue || '尚未选择存档' }}</div>
  </section>
</template>

<style scoped>
.compact-save-bar { min-width:0; padding:var(--space-4) var(--space-5); }
.source-title { display:flex; align-items:flex-start; justify-content:space-between; gap:var(--space-4); }
.source-title strong { display:block; color:var(--text-primary); font-size:var(--fs-md); }
.source-title small { display:block; margin-top:2px; color:var(--text-muted); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.save-slots { display:grid; grid-template-columns:repeat(auto-fit,minmax(118px,1fr)); gap:var(--space-2); margin-top:var(--space-3); }
.slot-choice.on { border-color:var(--selected-border); }
.selected-save { min-width:0; margin-top:var(--space-3); padding:8px 10px; overflow:hidden; border:1px solid var(--line-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); color:var(--text-secondary); font-family:var(--font-data); font-size:var(--fs-xs); text-overflow:ellipsis; white-space:nowrap; }
.selected-save.empty { color:var(--text-muted); font-family:var(--font-ui); }
@container ui-page (max-width:560px) {
  .source-title { align-items:stretch; flex-direction:column; }
}
</style>
