<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  OverLimitAcquire,
  OverLimitGetOptions,
  OverLimitGetStatus,
  OverLimitRelease,
  OverLimitScan,
  OverLimitSetAllOwned,
} from '../../wailsjs/go/backend/App'
import { nextRuntimeAcquireRequestID, queueRuntimeLeaseRelease, releaseRuntimeLease } from '../runtimeLeaseManager.js'

const emit = defineEmits(['status'])
const RUNTIME_LEASE_SCOPE = 'overlimit'

const loading = ref(false)
const options = reactive({ attributes: [], levels: [] })
const status = reactive({ found: false, hooked: false, address: 0, rva: 0, selectedAddr: 0, currentBytes: '', slots: [] })
const edits = reactive([
  { attribute: 0, level: 0, value: 0 },
  { attribute: 0, level: 0, value: 0 },
  { attribute: 0, level: 0, value: 0 },
  { attribute: 0, level: 0, value: 0 },
])
let lifecycleEpoch = 0
let disposed = false
let enableRequest = Promise.resolve()
let hookOwnerToken = ''

function isCurrent(epoch) {
  return !disposed && epoch === lifecycleEpoch
}

onMounted(() => {
  OverLimitGetOptions()
    .then((res) => {
      options.attributes = res.attributes || []
      options.levels = res.levels || []
    })
    .catch((err) => { if (!disposed) emit('status', String(err), 'error') })
})

onBeforeUnmount(() => {
  disposed = true
  lifecycleEpoch += 1
  loading.value = false
  const ownerToken = hookOwnerToken
  hookOwnerToken = ''
  if (ownerToken) queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, ownerToken, OverLimitRelease)
})

function formatHex(value) {
  if (!value) return '-'
  return '0x' + Number(value).toString(16).toUpperCase()
}

function optionName(list, id) {
  const found = list.find(x => Number(x.id) === Number(id))
  return found ? found.name : formatHex(id)
}

function attributeOption(id) {
  return options.attributes.find(x => Number(x.id) === Number(id))
}

function maxValue(id) {
  const opt = attributeOption(id)
  return opt ? Number(opt.maxValue || 0) : 0
}

function applyMaxValue(index) {
  edits[index].value = maxValue(edits[index].attribute)
}

function applyAllMaxValues() {
  edits.forEach((edit, index) => { edit.value = maxValue(edit.attribute); edits[index].level = options.levels.at(-1)?.id ?? edit.level })
}

function applyStatus(next) {
  Object.assign(status, next || { found: false, hooked: false, address: 0, rva: 0, selectedAddr: 0, currentBytes: '', slots: [] })
  ;(status.slots || []).forEach((slot, i) => {
    if (i < edits.length) {
      edits[i].attribute = Number(slot.attribute || 0)
      edits[i].level = Number(slot.level || 0)
      edits[i].value = Number(slot.value || maxValue(slot.attribute) || 0)
    }
  })
}

function run(action, success) {
  const epoch = ++lifecycleEpoch
  loading.value = true
  return action()
    .then((res) => {
      if (!isCurrent(epoch)) return res
      applyStatus(res)
      if (success) emit('status', success, 'success')
      return res
    })
    .catch((err) => {
      if (isCurrent(epoch)) emit('status', String(err), 'error')
      return undefined
    })
    .finally(() => { if (isCurrent(epoch)) loading.value = false })
}

function scan() {
  run(() => OverLimitScan(), '上限突破特征定位成功')
}

function enable() {
  const epoch = ++lifecycleEpoch
  loading.value = true
  let acquiredOwnerToken = ''
  enableRequest = OverLimitAcquire(nextRuntimeAcquireRequestID())
    .then((next) => {
      acquiredOwnerToken = String(next?.ownerToken || '')
      if (!acquiredOwnerToken) throw new Error('后端未返回上限突破读取所有权令牌')
      if (!isCurrent(epoch)) {
        queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquiredOwnerToken, OverLimitRelease)
        return next
      }
      hookOwnerToken = acquiredOwnerToken
      applyStatus(next)
      emit('status', '上限突破读取已开启，请在突破界面加载角色', 'success')
      return next
    })
    .catch((err) => {
      if (isCurrent(epoch)) emit('status', String(err), 'error')
      return undefined
    })
    .finally(() => { if (isCurrent(epoch)) loading.value = false })
}

async function disableReader() {
  const epoch = ++lifecycleEpoch
  loading.value = true
  try {
    await enableRequest
    if (!isCurrent(epoch)) return
    const ownerToken = hookOwnerToken
    const next = ownerToken
      ? await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, ownerToken, OverLimitRelease)
      : null
    if (!isCurrent(epoch)) return
    if (hookOwnerToken === ownerToken) hookOwnerToken = ''
    applyStatus(next)
    emit('status', '上限突破读取已关闭', 'success')
  } catch (err) {
    if (isCurrent(epoch)) emit('status', String(err), 'error')
  } finally {
    if (isCurrent(epoch)) loading.value = false
  }
}

function refresh() {
  const epoch = ++lifecycleEpoch
  loading.value = true
  OverLimitGetStatus()
    .then((res) => {
      if (!isCurrent(epoch)) return
      applyStatus(res)
      emit('status', res?.selectedAddr ? '上限突破角色数据已刷新' : '已刷新，尚未读取到角色数据；请在突破界面切换一次角色或重新打开突破界面', res?.selectedAddr ? 'success' : 'error')
    })
    .catch((err) => { if (isCurrent(epoch)) emit('status', String(err), 'error') })
    .finally(() => { if (isCurrent(epoch)) loading.value = false })
}

function writeSaveAll() {
	const expectedSelectedAddr = Number(status.selectedAddr || 0)
  run(
		async () => {
      const ownerToken = hookOwnerToken
      if (!ownerToken) throw new Error('当前页面不再持有上限突破读取所有权')
      return OverLimitSetAllOwned(ownerToken, edits.map((edit, index) => ({ index, expectedSelectedAddr, attribute: Number(edit.attribute), level: Number(edit.level), value: Number(edit.value) })))
    },
    '四个上限突破槽位已写入，请返回游戏确认保存'
  )
}
</script>

<template>
  <div class="overlimit-page ui-page is-wide ui-page-stack">
    <section class="overlimit-shell ui-card ui-panel">
      <header class="ui-split">
        <div class="title-copy">
          <h2 class="ui-section-title">上限突破</h2>
          <p class="ui-section-copy">读取当前突破界面的四个能力槽，并一次写入全部配置。</p>
        </div>
        <span class="ui-tag" :class="status.hooked ? 'is-ok' : 'is-warn'">{{ status.hooked ? '读取已开启' : '等待开启' }}</span>
      </header>

      <details class="ui-disclosure guide-disclosure">
        <summary>操作步骤</summary>
        <ol class="guide-list">
          <li>启动游戏后开启读取，确保角色至少完成过一次突破。</li>
          <li>进行一次 3 级突破，停在 “Over the limit!” 界面。</li>
          <li>点击刷新，配置四个能力并写入，回到游戏确认。</li>
          <li>再次进行 3 级突破，不替换此前结果并直接取消。</li>
          <li>完成以上操作后保存游戏存档。</li>
        </ol>
      </details>

      <section class="reader-card ui-card ui-panel is-compact">
        <div class="reader-head ui-split">
          <h3 class="ui-section-title">突破界面读取</h3>
          <span class="ui-tag" :class="status.selectedAddr ? 'is-ok' : 'is-info'">
            {{ status.selectedAddr ? '已载入角色' : '等待角色数据' }}
          </span>
        </div>
        <div class="reader-actions ui-actions">
          <button type="button" class="ui-btn is-primary" @click="enable" :disabled="loading || status.hooked">开启读取</button>
          <button type="button" class="ui-btn is-ghost" @click="disableReader" :disabled="loading || !status.hooked">关闭读取</button>
          <button type="button" class="ui-btn" @click="refresh" :disabled="loading">刷新</button>
          <button type="button" class="ui-btn is-ghost" @click="scan" :disabled="loading">重新扫描</button>
          <button type="button" class="ui-btn" @click="applyAllMaxValues" :disabled="loading || !status.selectedAddr">四项最大</button>
          <button type="button" class="ui-btn is-primary" @click="writeSaveAll" :disabled="loading || !status.selectedAddr">写入结果</button>
        </div>
        <details class="ui-disclosure diagnostics">
          <summary>定位诊断</summary>
          <dl class="diagnostic-grid">
            <div><dt>RVA</dt><dd>{{ formatHex(status.rva) }}</dd></div>
            <div><dt>角色数据</dt><dd>{{ formatHex(status.selectedAddr) }}</dd></div>
            <div class="bytes"><dt>当前字节</dt><dd>{{ status.currentBytes || '未定位' }}</dd></div>
          </dl>
        </details>
      </section>

      <p v-if="!status.selectedAddr" class="ui-empty">开启读取后，在游戏上限突破界面加载角色。</p>

      <div v-else class="slot-list ui-card-grid">
        <article v-for="slot in status.slots" :key="slot.index" class="slot-card ui-card ui-panel is-compact">
          <header class="slot-head ui-split">
            <h3 class="ui-section-title">能力 {{ slot.index + 1 }}</h3>
            <span class="ui-tag">{{ optionName(options.attributes, slot.attribute) }} · {{ optionName(options.levels, slot.level) }}</span>
          </header>
          <div class="slot-grid">
            <label class="ui-field">
              <span class="ui-field-label">词条</span>
              <select v-model.number="edits[slot.index].attribute" class="ui-select" @change="applyMaxValue(slot.index)">
                <option v-for="opt in options.attributes" :key="opt.id" :value="opt.id">{{ opt.name }} ({{ opt.hex }})</option>
              </select>
            </label>
            <label class="ui-field">
              <span class="ui-field-label">等级</span>
              <select v-model.number="edits[slot.index].level" class="ui-select">
                <option v-for="opt in options.levels" :key="opt.id" :value="opt.id">{{ opt.name }}</option>
              </select>
            </label>
            <label class="value-edit ui-field">
              <span class="ui-field-label">数值</span>
              <span class="value-combo ui-control-group">
                <input v-model.number="edits[slot.index].value" type="number" min="0" :max="maxValue(edits[slot.index].attribute)" step="1" class="ui-input" />
                <button type="button" class="ui-btn is-sm" @click="applyMaxValue(slot.index)">最大</button>
              </span>
            </label>
          </div>
        </article>
      </div>
    </section>
  </div>
</template>

<style scoped>
.overlimit-page { padding-bottom:var(--space-8); }
.title-copy { display:flex; min-width:0; flex-direction:column; gap:var(--space-2); }
.reader-card { background:var(--surface-sunken); }
.reader-actions { align-items:stretch; }
.reader-actions .ui-btn { flex:1 1 128px; }
.guide-list { padding-left:var(--space-6); color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-relaxed); }
.guide-list li + li { margin-top:var(--space-2); }
.diagnostic-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-3); }
.diagnostic-grid div { min-width:0; padding:var(--space-3); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.diagnostic-grid .bytes { grid-column:1 / -1; }
.diagnostic-grid dt { color:var(--text-muted); font-size:var(--fs-xs); }
.diagnostic-grid dd { margin:var(--space-1) 0 0; color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-sm); overflow-wrap:anywhere; }
.slot-list { --ui-grid-min:360px; }
.slot-card { background:var(--surface-card-pop); }
.slot-head .ui-tag { white-space:normal; text-align:right; }
.slot-grid { display:grid; grid-template-columns:minmax(0,1fr) 112px; gap:var(--space-4); align-items:end; }
.value-edit { grid-column:1 / -1; }
.value-combo > .ui-btn { flex:0 0 auto; }

@container ui-page (max-width:600px) {
  .slot-list { --ui-grid-min:100%; }
  .reader-actions .ui-btn { flex-basis:calc(50% - var(--space-3)); }
}
@container ui-page (max-width:420px) {
  .slot-grid,
  .diagnostic-grid { grid-template-columns:minmax(0,1fr); }
  .diagnostic-grid .bytes,
  .value-edit { grid-column:auto; }
  .reader-actions .ui-btn { flex-basis:100%; }
}
</style>
