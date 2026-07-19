<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: String, default: 'unknown' },
  message: { type: String, default: '' },
  compact: { type: Boolean, default: false },
})

const meta = computed(() => ({
  legal: { icon: '✓', label: '合法', hint: '符合当前已验证的游戏规则' },
  forced: { icon: '!', label: '可强制写入', hint: '组合不合法或不会正常生效，但结构仍可写入' },
  unknown: { icon: '?', label: '合法性未完全验证', hint: '可以写入，但社区数据不足以断言天然可获得' },
  impossible: { icon: '×', label: '无法写入', hint: '缺少必要字段或当前记录结构无法表达该组合' },
}[props.status] || { icon: '?', label: '未检验', hint: '尚无足够信息' }))
</script>

<template>
  <div class="legality" :class="[status, { compact }]" :title="message || meta.hint">
    <span class="icon">{{ meta.icon }}</span>
    <span class="text"><strong>{{ meta.label }}</strong><small v-if="!compact">{{ message || meta.hint }}</small></span>
  </div>
</template>

<style scoped>
.legality { min-width:0; display:flex; align-items:flex-start; gap:7px; padding:3px 0 3px 8px; border:0; border-left:2px solid currentColor; border-radius:0; background:transparent; }
.icon { width:18px; height:18px; flex:0 0 18px; display:grid; place-items:center; border:1px solid currentColor; border-radius:50%; font-size:var(--fs-xs); font-weight:var(--fw-bold); background:transparent; }
.text { min-width:0; display:flex; flex-direction:column; gap:2px; }
.text strong { color:currentColor; font-size:var(--fs-sm); font-weight:var(--fw-bold); line-height:var(--lh-tight); white-space:nowrap; }
.text small { max-width:none; color:var(--text-secondary); font-size:var(--fs-xs); line-height:var(--lh-normal); overflow:visible; text-overflow:clip; white-space:normal; overflow-wrap:anywhere; }
.legal { color:var(--success-ink); }
.forced { color:var(--warning-ink); }
.unknown { color:var(--text-secondary); }
.impossible { color:var(--danger-ink); }
.compact { padding:5px 8px; }
.compact .text strong { font-size:var(--fs-sm); }
</style>
