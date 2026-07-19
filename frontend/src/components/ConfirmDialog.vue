<script setup>
import { nextTick, onBeforeUnmount, ref } from 'vue'

const open = ref(false)
const title = ref('请确认操作')
const message = ref('')
const detail = ref('')
const tone = ref('warning')
const confirmLabel = ref('确认')
const cancelLabel = ref('取消')
const dialog = ref(null)
const cancelButton = ref(null)
const confirmButton = ref(null)
let resolver = null
let lastFocused = null

function settle(result) {
  if (!open.value) return
  open.value = false
  const pending = resolver
  resolver = null
  pending?.(result)
  nextTick(() => lastFocused?.focus?.())
}

function ask(options = {}) {
  if (resolver) resolver(false)
  const config = typeof options === 'string' ? { message: options } : options
  title.value = config.title || '请确认操作'
  message.value = config.message || ''
  detail.value = config.detail || ''
  tone.value = config.tone || 'warning'
  confirmLabel.value = config.confirmLabel || '确认'
  cancelLabel.value = config.cancelLabel || '取消'
  lastFocused = document.activeElement
  open.value = true
  nextTick(() => {
    const safeFirst = tone.value === 'warning' || tone.value === 'danger'
    ;(safeFirst ? cancelButton.value : confirmButton.value)?.focus()
  })
  return new Promise(resolve => { resolver = resolve })
}

function onKeydown(event) {
  if (!open.value) return
  if (event.key === 'Escape') {
    event.preventDefault()
    event.stopImmediatePropagation()
    settle(false)
    return
  }
  if (event.key !== 'Tab') return
  const controls = [...(dialog.value?.querySelectorAll('button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])') || [])]
  if (!controls.length) return
  const first = controls[0]
  const last = controls[controls.length - 1]
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}

function toneMark(value) {
  return ({ danger: '×', success: '✓', info: 'i', warning: '!' })[value] || '!'
}

window.addEventListener('keydown', onKeydown)
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  if (resolver) resolver(false)
})

defineExpose({ ask })
</script>

<template>
  <Teleport to="body">
    <Transition name="journal-dialog">
      <div v-if="open" class="dialog-backdrop" @mousedown.self="settle(false)">
        <section ref="dialog" class="journal-dialog ui-card" :class="tone" :role="tone === 'warning' || tone === 'danger' ? 'alertdialog' : 'dialog'" aria-modal="true" aria-labelledby="confirm-dialog-title" aria-describedby="confirm-dialog-content">
          <header>
            <span class="seal" aria-hidden="true">{{ toneMark(tone) }}</span>
            <div><small>GBFR PATCH TOOL</small><h2 id="confirm-dialog-title">{{ title }}</h2></div>
          </header>
          <div class="rule"><i></i><b></b><i></i></div>
          <div id="confirm-dialog-content" class="dialog-content">
            <p class="message">{{ message }}</p>
            <p v-if="detail" class="detail">{{ detail }}</p>
          </div>
          <footer>
            <button ref="cancelButton" class="cancel ui-btn" type="button" @click="settle(false)">{{ cancelLabel }}</button>
            <button ref="confirmButton" class="confirm ui-btn is-primary" :class="{ 'is-danger': tone === 'danger' }" type="button" @click="settle(true)">{{ confirmLabel }}</button>
          </footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.dialog-backdrop {
  position:fixed;
  z-index:var(--z-dialog);
  inset:0;
  display:grid;
  place-items:center;
  padding:16px;
  background:rgba(41,31,21,.46);
}
.journal-dialog {
  --dialog-accent:var(--warning);
  width:min(480px,100%);
  max-height:calc(100dvh - 32px);
  display:flex;
  flex-direction:column;
  overflow:hidden;
  padding:var(--space-6);
  border-left:4px solid var(--dialog-accent);
  background:var(--surface-card-pop);
  box-shadow:var(--shadow-3);
  font-family:var(--font-ui);
}
.journal-dialog.danger { --dialog-accent:var(--danger); }
.journal-dialog.info { --dialog-accent:var(--info); }
.journal-dialog.success { --dialog-accent:var(--success); }
.journal-dialog header {
  flex:0 0 auto;
  display:flex;
  align-items:center;
  gap:var(--space-4);
}
.seal {
  width:36px;
  height:36px;
  flex:0 0 36px;
  display:grid;
  place-items:center;
  border:1px solid var(--dialog-accent);
  border-radius:50%;
  color:var(--dialog-accent);
  background:color-mix(in srgb,var(--dialog-accent) 9%,transparent);
  font-family:var(--font-data);
  font-size:var(--fs-lg);
  font-weight:var(--fw-bold);
}
header > div { min-width:0; }
header small {
  display:block;
  color:var(--accent);
  font-size:var(--fs-xs);
  font-weight:var(--fw-bold);
  letter-spacing:.08em;
}
h2 {
  margin:2px 0 0;
  color:var(--text-primary);
  font-family:var(--font-display);
  font-size:var(--fs-lg);
  font-weight:var(--fw-bold);
  line-height:var(--lh-tight);
}
.rule {
  flex:0 0 auto;
  height:1px;
  margin:var(--space-6) 0 var(--space-5);
  background:var(--border-default);
}
.rule i,.rule b { display:none; }
.dialog-content {
  min-height:0;
  overflow:auto;
  overscroll-behavior:contain;
}
.message,.detail { white-space:pre-line; }
.message {
  margin:0;
  color:var(--text-primary);
  font-size:var(--fs-md);
  font-weight:var(--fw-semibold);
  line-height:var(--lh-relaxed);
}
.detail {
  margin:var(--space-4) 0 0;
  padding:var(--space-4);
  overflow-wrap:anywhere;
  border:1px solid var(--border-default);
  border-left:3px solid var(--dialog-accent);
  border-radius:var(--radius-sm);
  color:var(--text-secondary);
  background:var(--surface-field);
  font-size:var(--fs-sm);
  font-weight:var(--fw-normal);
  line-height:var(--lh-normal);
}
footer {
  flex:0 0 auto;
  display:flex;
  justify-content:flex-end;
  flex-wrap:wrap;
  gap:var(--space-3);
  margin-top:var(--space-6);
}
footer .ui-btn { min-width:92px; }
.journal-dialog-enter-active,.journal-dialog-leave-active {
  transition:opacity var(--dur-fast) var(--ease-out),transform var(--dur-fast) var(--ease-out);
}
.journal-dialog-enter-from,.journal-dialog-leave-to { opacity:0; transform:translateY(4px) scale(.99); }

@media (max-width:520px) {
  .journal-dialog { padding:var(--space-5); }
  footer { align-items:stretch; flex-direction:column-reverse; }
  footer .ui-btn { width:100%; }
}
@media (max-height:620px) {
  .journal-dialog { padding:var(--space-5); }
  .rule { margin-block:var(--space-4); }
  footer { margin-top:var(--space-4); }
}
@media (prefers-reduced-motion:reduce) {
  .journal-dialog-enter-active,.journal-dialog-leave-active { transition:none; }
}
</style>
