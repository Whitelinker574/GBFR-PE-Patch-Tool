<script setup>
import { language } from '../i18n.js'

defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: '' },
  characterName: { type: String, default: '' },
  subtitle: { type: String, default: '' },
  image: { type: String, default: '' },
  busy: { type: Boolean, default: false },
  result: { type: Object, default: null },
  error: { type: String, default: '' },
})

const emit = defineEmits(['update:title', 'close', 'submit', 'copy'])
const tx = (zh, en) => language.value === 'en' ? en : zh
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="publish-backdrop" @click.self="emit('close')" @keydown.esc="emit('close')">
      <section class="publish-dialog ui-card" role="dialog" aria-modal="true" :aria-label="tx('上传配装', 'Publish Loadout')">
        <header>
          <div><small>{{ tx('社区配装图鉴', 'Community Loadout Archive') }}</small><strong>{{ tx('上传并复制分享链接', 'Publish & Copy Link') }}</strong></div>
          <button type="button" class="ui-btn is-ghost is-sm" :aria-label="tx('关闭', 'Close')" @click="emit('close')">×</button>
        </header>
        <div class="publish-identity">
          <img v-if="image" :src="image" alt="" />
          <span v-else aria-hidden="true">◇</span>
          <div><strong>{{ characterName }}</strong><small>{{ subtitle || tx('未记录武器', 'Weapon Not Recorded') }}</small></div>
        </div>
        <label class="publish-title"><span>{{ tx('分享标题', 'Share Title') }}</span><input :value="title" class="ui-input" maxlength="80" :disabled="busy || !!result" :placeholder="tx('例如：泽塔常规毕业配装', 'For example: Zeta Endgame Loadout')" @input="emit('update:title', $event.target.value)" /></label>
        <p>{{ tx('标题可以重复；完全相同的配装会沿用原短码和首次标题。线上只保存脱敏后的单套配装。', 'Titles may be reused. An identical loadout reuses its original code and first title. Only the sanitized loadout is stored online.') }}</p>
        <div v-if="result" class="publish-result">
          <div><small>{{ result.reused ? tx('已沿用现有短码', 'Existing Code Reused') : tx('短码已生成', 'Code Created') }}</small><strong>{{ result.code }}</strong><span>{{ result.url }}</span></div>
          <button type="button" class="ui-btn is-primary" @click="emit('copy')">{{ tx('复制链接', 'Copy Link') }}</button>
        </div>
        <p v-if="error" class="publish-error" role="alert">{{ error }}</p>
        <footer>
          <button type="button" class="ui-btn" @click="emit('close')">{{ result ? tx('完成', 'Done') : tx('取消', 'Cancel') }}</button>
          <button v-if="!result" type="button" class="ui-btn is-primary" :disabled="busy" @click="emit('submit')">{{ busy ? tx('正在上传…', 'Publishing…') : tx('上传并复制链接', 'Publish & Copy Link') }}</button>
        </footer>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.publish-backdrop { position:fixed; z-index:10000; inset:0; display:grid; place-items:center; padding:16px; background:rgba(38,28,17,.58); backdrop-filter:blur(8px); }
.publish-dialog { width:100%; max-width:min(560px,calc(100vw - 32px)); max-height:calc(100vh - 32px); display:flex; flex-direction:column; gap:var(--space-4); overflow:auto; padding:var(--space-5); border-color:var(--accent-border); background:var(--surface-card-pop); box-shadow:var(--shadow-4); }
.publish-dialog > header { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-3); align-items:start; padding-bottom:var(--space-3); border-bottom:1px solid var(--border-soft); }
.publish-dialog > header > div,.publish-identity > div,.publish-result > div { min-width:0; display:grid; gap:2px; }
.publish-dialog > header small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.publish-dialog > header strong { overflow-wrap:anywhere; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-lg); }
.publish-dialog > p { margin:0; color:var(--text-secondary); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.publish-identity { min-width:0; display:grid; grid-template-columns:52px minmax(0,1fr); gap:var(--space-3); align-items:center; }
.publish-identity img,.publish-identity > span { width:52px; height:52px; display:grid; place-items:center; border:1px solid var(--border-strong); border-radius:var(--radius-sm); background:var(--surface-sunken); object-fit:cover; }
.publish-identity strong,.publish-identity small { min-width:0; overflow-wrap:anywhere; }.publish-identity small { color:var(--text-muted); }
.publish-title { min-width:0; display:grid; gap:var(--space-2); color:var(--text-secondary); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }.publish-title input { width:100%; min-width:0; }
.publish-result { min-width:0; display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-3); align-items:center; padding:var(--space-4); border-left:3px solid var(--success); background:var(--success-bg); }
.publish-result small { color:var(--success-ink); font-size:var(--fs-2xs); }.publish-result strong { color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-lg); }.publish-result span { min-width:0; overflow-wrap:anywhere; color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-2xs); }
.publish-error { color:var(--danger) !important; }
.publish-dialog > footer { min-width:0; display:flex; justify-content:flex-end; gap:var(--space-2); }
@media(max-width:520px) { .publish-result { grid-template-columns:minmax(0,1fr); }.publish-result .ui-btn,.publish-dialog > footer .ui-btn { width:100%; }.publish-dialog > footer { display:grid; grid-template-columns:minmax(0,1fr); } }
</style>
