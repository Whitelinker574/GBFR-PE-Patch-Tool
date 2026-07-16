<script setup>
import { nextTick, onBeforeUnmount, ref } from 'vue'

const open = ref(false)
const title = ref('请确认操作')
const message = ref('')
const detail = ref('')
const tone = ref('warning')
const confirmLabel = ref('确认')
const cancelLabel = ref('取消')
const confirmButton = ref(null)
let resolver = null

function settle(result) {
  if (!open.value) return
  open.value = false
  const pending = resolver
  resolver = null
  pending?.(result)
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
  open.value = true
  nextTick(() => confirmButton.value?.focus())
  return new Promise(resolve => { resolver = resolve })
}

function onKeydown(event) {
  if (!open.value) return
  if (event.key === 'Escape') settle(false)
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
        <section class="journal-dialog" :class="tone" role="alertdialog" aria-modal="true" :aria-label="title">
          <header>
            <span class="seal" aria-hidden="true">!</span>
            <div><small>GBFR PATCH TOOL</small><h2>{{ title }}</h2></div>
          </header>
          <div class="rule"><i></i><b></b><i></i></div>
          <p class="message">{{ message }}</p>
          <p v-if="detail" class="detail">{{ detail }}</p>
          <footer>
            <button class="cancel" type="button" @click="settle(false)">{{ cancelLabel }}</button>
            <button ref="confirmButton" class="confirm" type="button" @click="settle(true)">{{ confirmLabel }}</button>
          </footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.dialog-backdrop{position:fixed;z-index:9999;inset:0;display:grid;place-items:center;padding:28px;background:rgba(41,31,21,.45)}
.journal-dialog{--accent:#8a6635;position:relative;width:min(500px,calc(100vw - 46px));overflow:hidden;padding:21px 23px 20px;border:1px solid #9f7a45;border-left:3px solid var(--accent);border-radius:1px;color:#51463a;background:#f5e7c7;box-shadow:none;font-family:var(--font-ui)}
.journal-dialog.danger{--accent:#914d42}.journal-dialog.info{--accent:#806637}.journal-dialog.success{--accent:#667453}
header{display:flex;align-items:center;gap:12px}.seal{display:grid;place-items:center;width:32px;height:32px;flex:0 0 32px;border:1px solid var(--accent);border-radius:50%;color:var(--accent);background:transparent;font:900 17px/1 Georgia,serif}header small{display:block;color:#8c6938;font-size:9px;font-weight:800;letter-spacing:.12em}h2{margin:3px 0 0;color:#4d4236;font-size:16px;line-height:1.35;font-weight:800;letter-spacing:0}
.rule{display:block;height:1px;margin:15px 0 14px;background:rgba(123,88,43,.28)}.rule i,.rule b{display:none}
.message,.detail{white-space:pre-line}.message{margin:0;color:#55493b;font-size:13px;font-weight:700;line-height:1.72}.detail{margin:11px 0 0;padding:10px 11px;border:1px solid rgba(127,91,42,.24);border-left:3px solid var(--accent);color:#6d5c45;background:#edddba;font-size:11px;font-weight:650;line-height:1.65;word-break:break-all}
footer{display:flex;justify-content:flex-end;gap:9px;margin-top:18px}button{min-width:86px;min-height:35px;padding:7px 14px;border-radius:1px;font:750 12px/1 var(--font-ui);cursor:pointer;box-shadow:none;transition:color .1s ease,background-color .1s ease,border-color .1s ease}button:focus-visible{outline:2px solid rgba(126,91,42,.5);outline-offset:2px}.cancel{color:#66543c;border:1px solid rgba(127,91,43,.38);background:#ead8b2}.cancel:hover{border-color:#8a6635;background:#f3e5c5}.confirm{color:#fff9e9;border:1px solid var(--accent);background:var(--accent)}.confirm:hover{background:#735128}
.journal-dialog-enter-active,.journal-dialog-leave-active{transition:opacity .12s ease}.journal-dialog-enter-from,.journal-dialog-leave-to{opacity:0}
</style>
