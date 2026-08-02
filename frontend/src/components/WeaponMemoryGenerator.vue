<script setup>
import { computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, reactive, ref, watch } from 'vue'
import {
  GetWeaponRuntimeSkillsWorkspace,
  WeaponMemoryAcquire,
  WeaponMemoryGetOptions,
  WeaponMemoryGetStatus,
  WeaponMemoryRelease,
  WeaponMemoryUpdateOwned,
  WeaponRuntimeSkillsDeploy,
  WeaponRuntimeSkillsRemove,
} from '../../wailsjs/go/backend/App'
import { traitAssetIcon } from '../gameAssetIcons'
import { language } from '../i18n.js'
import { backendLanguageReady } from '../backendLanguage.js'
import { nextRuntimeAcquireRequestID, queueRuntimeLeaseRelease, releaseRuntimeLease } from '../runtimeLeaseManager.js'
import CatalogSelect from './CatalogSelect.vue'
import ConfirmDialog from './ConfirmDialog.vue'

const EMPTY_HASH = 0x887AE0B0
const RUNTIME_LEASE_SCOPE = 'weapon-memory-generator'
const emit = defineEmits(['status', 'runtime-state'])
const confirmDialog = ref(null)
const loading = ref(false)
const writing = ref(false)
const extraWriting = ref(false)
const stale = ref(false)
const traits = ref([])
const liveMessage = ref('尚未启用读取。')
const status = reactive({ found: false, hooked: false, selectedAddr: 0, sourceVersion: '', weaponId: 0, weaponSlot: -1, weaponLevel: 0, skills: [] })
const slots = reactive(Array.from({ length: 5 }, (_, index) => ({ index, hash: 0, level: 0 })))
const extraSkills = reactive([])
const runtimeWorkspace = reactive({ installed: false, owned: false, recoveryRequired: false, state: '', detail: '', gameRunning: false, config: { weaponSlot: -1, weaponId: 0, skills: [] } })
let pollTimer = 0
let lastSelectedAddr = 0
let lifecycleEpoch = 0
let disposed = false
let hookOwnerToken = ''

function text(zh, en) { return language.value === 'en' ? en : zh }
function normaliseHash(value) {
  const hash = Number(value || 0) >>> 0
  return hash === EMPTY_HASH ? 0 : hash
}
function formatHex(value) { return normaliseHash(value) ? `0x${normaliseHash(value).toString(16).toUpperCase().padStart(8, '0')}` : '—' }
function isolatedError(error, englishFallback) {
  const raw = String(error || '')
  return language.value === 'en' && /[\u3400-\u9fff]/u.test(raw) ? englishFallback : raw
}
function currentSkill(index) {
  const skill = Array.isArray(status.skills) ? status.skills.find(item => Number(item?.index) === index) : null
  return skill || { index, hash: 0, name: '', level: 0 }
}
function optionFor(hash) { return traits.value.find(item => normaliseHash(item.hash) === normaliseHash(hash)) }
function optionName(hash) { return optionFor(hash)?.displayName || formatHex(hash) }
function skillIcon(hash, name = '') { return traitAssetIcon({ hash, name }) }
const catalogOptions = computed(() => traits.value.map(item => ({
  ...item,
  internalId: String(normaliseHash(item.hash) || ''),
  hashHex: formatHex(item.hash),
})).filter(item => item.internalId))

function syncDraft() {
  for (const slot of slots) {
    const current = currentSkill(slot.index)
    slot.hash = normaliseHash(current.hash)
    slot.level = Number(current.level || 0)
  }
}
function applyStatus(next, { sync = false } = {}) {
  Object.assign(status, { found: false, hooked: false, selectedAddr: 0, sourceVersion: '', weaponId: 0, weaponSlot: -1, weaponLevel: 0, skills: [], ...(next || {}) })
  const address = Number(status.selectedAddr || 0)
  if (!address) { lastSelectedAddr = 0; return }
  if (sync || address !== lastSelectedAddr) {
    lastSelectedAddr = address
    stale.value = false
    syncDraft()
  }
}
function addExtraSkill() { extraSkills.push({ hash: 0, level: 15 }) }
function removeExtraSkill(index) { extraSkills.splice(index, 1) }
function setExtraHash(skill, value) {
  skill.hash = normaliseHash(value)
  if (!skill.hash) skill.level = 0
  else if (Number(skill.level) <= 0) skill.level = Math.max(1, Number(optionFor(skill.hash)?.maxLevel || 15))
}
function normaliseExtraLevel(skill) {
  if (!skill.hash) { skill.level = 0; return }
  const value = Number(skill.level)
  skill.level = Number.isInteger(value) && value >= 1 ? Math.min(value, 2147483647) : 1
}
function applyRuntimeWorkspace(next) {
  Object.assign(runtimeWorkspace, { installed: false, owned: false, recoveryRequired: false, state: '', detail: '', gameRunning: false, config: { weaponSlot: -1, weaponId: 0, skills: [] }, ...(next || {}) })
  const configured = Array.isArray(runtimeWorkspace.config?.skills) ? runtimeWorkspace.config.skills : []
  extraSkills.splice(0, extraSkills.length, ...configured.map(skill => ({ hash: normaliseHash(skill.hash), level: Number(skill.level || 1) })))
}
async function loadRuntimeWorkspace() {
  const next = await GetWeaponRuntimeSkillsWorkspace()
  if (!disposed) applyRuntimeWorkspace(next)
}
function setSlotHash(slot, value) {
  const wasEmpty = !slot.hash
  slot.hash = normaliseHash(value)
  if (!slot.hash) slot.level = 0
  else if (wasEmpty && Number(slot.level) <= 0) slot.level = Math.max(1, Number(optionFor(slot.hash)?.maxLevel || 15))
}
function normaliseLevel(slot) {
  if (!slot.hash) { slot.level = 0; return }
  const value = Number(slot.level)
  slot.level = Number.isInteger(value) && value >= 0 ? Math.min(value, 2147483647) : 0
}
const changes = computed(() => slots.flatMap(slot => {
  const current = currentSkill(slot.index)
  const rows = []
  if (normaliseHash(slot.hash) !== normaliseHash(current.hash)) rows.push(text(
    `槽位 ${slot.index + 1}：${current.name || formatHex(current.hash)} → ${slot.hash ? optionName(slot.hash) : '留空'}`,
    `Slot ${slot.index + 1}: ${current.name || formatHex(current.hash)} → ${slot.hash ? optionName(slot.hash) : 'Empty'}`,
  ))
  if (Number(slot.level) !== Number(current.level || 0)) rows.push(text(
    `槽位 ${slot.index + 1}等级：${Number(current.level || 0)} → ${Number(slot.level)}`,
    `Slot ${slot.index + 1} level: ${Number(current.level || 0)} → ${Number(slot.level)}`,
  ))
  return rows
}))
const validationMessage = computed(() => {
  if (!status.hooked) return text('请先启用读取。', 'Enable capture first.')
  if (stale.value) return text('写入后的旧记录已失效，请在游戏里重新选择武器。', 'The previous record expired after writing. Select the weapon again in-game.')
  if (!status.selectedAddr) return text('请在游戏武器列表中高亮一把武器。', 'Highlight a weapon in the in-game weapon list.')
  for (const slot of slots) {
    if (!slot.hash && Number(slot.level) !== 0) return text(`槽位 ${slot.index + 1} 为空时等级必须为 0。`, `Slot ${slot.index + 1} must use level 0 when empty.`)
    if (slot.hash && (!Number.isInteger(Number(slot.level)) || Number(slot.level) < 1 || Number(slot.level) > 2147483647)) return text(`槽位 ${slot.index + 1} 等级必须是 1 到 2147483647 的整数。`, `Slot ${slot.index + 1} must be an integer from 1 to 2147483647.`)
  }
  if (!changes.value.length) return text('目标值和当前武器相同。', 'The draft matches the current weapon.')
  return ''
})
const canWrite = computed(() => !loading.value && !writing.value && !validationMessage.value)
const extraValidationMessage = computed(() => {
  if (!status.selectedAddr) return text('先在游戏内高亮目标武器，额外技能会绑定这把武器的槽位和 ID。', 'Highlight the target weapon in-game first. Extra skills bind to its slot and ID.')
  if (!extraSkills.length) return text('请添加至少一条第 6 条及以后的技能。', 'Add at least one skill beyond the five physical slots.')
  for (let index = 0; index < extraSkills.length; index++) {
    const skill = extraSkills[index]
    if (!normaliseHash(skill.hash)) return text(`额外技能 ${index + 1} 还没有选择技能。`, `Extra skill ${index + 1} has no selected skill.`)
    if (!Number.isInteger(Number(skill.level)) || Number(skill.level) < 1 || Number(skill.level) > 2147483647) return text(`额外技能 ${index + 1} 的等级必须是 1 到 2147483647 的整数。`, `Extra skill ${index + 1} must use an integer level from 1 to 2147483647.`)
  }
  return ''
})
const canDeployExtra = computed(() => !loading.value && !extraWriting.value && !extraValidationMessage.value)
const statusLabel = computed(() => stale.value ? text('需要重新选择', 'Reselect Required') : status.selectedAddr ? text('已读取当前武器', 'Current Weapon Captured') : status.hooked ? text('等待游戏内选择', 'Waiting for Selection') : text('默认关闭', 'Off by Default'))

function publishRuntimeState() {
  emit('runtime-state', {
    id: 'weaponMemory',
    active: (status.hooked === true && Boolean(hookOwnerToken)) || (runtimeWorkspace.installed === true && runtimeWorkspace.state === 'active' && runtimeWorkspace.recoveryRequired !== true),
    recoveryRequired: (status.hooked === true && !hookOwnerToken) || runtimeWorkspace.recoveryRequired === true,
  })
}
watch(() => status.hooked, publishRuntimeState, { immediate: true })
watch(() => runtimeWorkspace.installed, publishRuntimeState)
watch(() => runtimeWorkspace.state, publishRuntimeState)
watch(() => runtimeWorkspace.recoveryRequired, publishRuntimeState)
function stopPolling() { if (pollTimer) window.clearInterval(pollTimer); pollTimer = 0 }
function startPolling() { stopPolling(); pollTimer = window.setInterval(() => pollStatus(true), 700) }
async function pollStatus(silent = false) {
  const epoch = lifecycleEpoch
  const previous = Number(status.selectedAddr || 0)
  try {
    const next = await WeaponMemoryGetStatus()
    if (disposed || epoch !== lifecycleEpoch) return
    applyStatus(next)
    if (!status.hooked) stopPolling()
    if (!previous && status.selectedAddr) liveMessage.value = text('已读取当前高亮武器和五个技能槽。', 'Captured the highlighted weapon and all five skill slots.')
    else if (!silent) liveMessage.value = status.selectedAddr ? text('当前武器记录已刷新。', 'Current weapon record refreshed.') : text('请在游戏武器列表中高亮目标武器。', 'Highlight a target in the in-game weapon list.')
  } catch (error) {
    if (disposed || epoch !== lifecycleEpoch) return
    stopPolling(); applyStatus(null)
    liveMessage.value = text(`读取已停止：${String(error)}`, `Capture stopped: ${isolatedError(error, 'Could not read weapon capture status.')}`)
    if (!silent) emit('status', liveMessage.value, 'error')
  }
}
async function enable() {
  const epoch = ++lifecycleEpoch
  loading.value = true
  let acquired = ''
  try {
    const next = await WeaponMemoryAcquire(nextRuntimeAcquireRequestID())
    acquired = String(next?.ownerToken || '')
    if (!acquired) throw new Error(text('后端没有返回武器读取所有权。', 'The backend did not return a weapon-capture owner token.'))
    if (disposed || epoch !== lifecycleEpoch) { queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, acquired, WeaponMemoryRelease); return }
    hookOwnerToken = acquired
    applyStatus(next, { sync: true })
    liveMessage.value = status.selectedAddr ? text('读取已开启，并已捕获当前武器。', 'Capture enabled and the current weapon was found.') : text('读取已开启，请到游戏武器列表高亮目标。', 'Capture enabled. Highlight the target in the in-game weapon list.')
    emit('status', liveMessage.value, 'success'); startPolling()
  } catch (error) {
    if (!disposed && epoch === lifecycleEpoch) { liveMessage.value = isolatedError(error, 'Failed to enable weapon capture.'); emit('status', liveMessage.value, 'error') }
  } finally { if (!disposed && epoch === lifecycleEpoch) loading.value = false }
}
async function disable() {
  const epoch = ++lifecycleEpoch
  const owner = hookOwnerToken
  stopPolling()
  if (!owner) { applyStatus(null); return }
  loading.value = true
  try {
    const next = await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, owner, WeaponMemoryRelease)
    if (disposed || epoch !== lifecycleEpoch) return
    if (hookOwnerToken === owner) hookOwnerToken = ''
    applyStatus(next); stale.value = false
    liveMessage.value = text('读取已停止，游戏原始指令已恢复。', 'Capture stopped and the original game instruction was restored.')
    emit('status', liveMessage.value, 'success')
  } catch (error) {
    if (!disposed && epoch === lifecycleEpoch) { liveMessage.value = isolatedError(error, 'Failed to stop weapon capture.'); emit('status', liveMessage.value, 'error') }
  } finally { if (!disposed && epoch === lifecycleEpoch) loading.value = false }
}
async function write() {
  if (!canWrite.value) { liveMessage.value = validationMessage.value; emit('status', liveMessage.value, 'error'); return }
  const confirmed = await confirmDialog.value?.ask({
    title: text('写入当前武器技能', 'Write Current Weapon Skills'),
    message: text(`确认写入 ${changes.value.length} 项变更？`, `Write ${changes.value.length} changes?`),
    detail: `${changes.value.join('\n')}\n\n${text('当前游戏记录只有五个物理技能槽；这里会一次写回这五槽，不会修改武器等级、觉醒、祝福或角色配装。游戏可能规范化不常见组合。', 'The live weapon record has exactly five physical skill slots. This writes those five slots only; weapon level, awakening, wrightstone, and character loadout are untouched. The game may normalize unusual combinations.')}`,
    tone: 'warning', confirmLabel: text('确认写入五槽', 'Write Five Slots'),
  })
  if (!confirmed) return
  writing.value = true
  try {
    const ownerToken = hookOwnerToken
    if (!ownerToken) throw new Error(text('当前页面不再持有武器读取所有权。', 'This page no longer owns the weapon-capture lease.'))
    const result = await WeaponMemoryUpdateOwned(ownerToken, {
      expectedSelectedAddr: Number(status.selectedAddr || 0),
      slots: slots.map(slot => ({ hash: normaliseHash(slot.hash), level: Number(slot.level) })),
    })
    applyStatus(result); stale.value = true; stopPolling()
    const released = await releaseRuntimeLease(RUNTIME_LEASE_SCOPE, ownerToken, WeaponMemoryRelease)
    if (hookOwnerToken === ownerToken) hookOwnerToken = ''
    applyStatus(released); stale.value = false
    liveMessage.value = text('五个技能槽已保存并回读；读取已自动停止。继续修改前请重新启用并选择武器。', 'All five slots were saved and read back. Capture stopped automatically; enable it and select a weapon before another edit.')
    emit('status', liveMessage.value, 'success')
  } catch (error) {
    liveMessage.value = isolatedError(error, 'Failed to write the weapon skill record.'); emit('status', liveMessage.value, 'error')
  } finally { writing.value = false }
}

async function deployExtraSkills() {
  if (!canDeployExtra.value) { liveMessage.value = extraValidationMessage.value; emit('status', liveMessage.value, 'error'); return }
  const summary = extraSkills.map((skill, index) => `${index + 6}. ${optionName(skill.hash)} · Lv ${Number(skill.level)}`).join('\n')
  const confirmed = await confirmDialog.value?.ask({
    title: text('开启额外武器技能', 'Enable Extra Weapon Skills'),
    message: text(`把 ${extraSkills.length} 条额外技能绑定到当前武器？`, `Bind ${extraSkills.length} extra skills to the current weapon?`),
    detail: `${summary}\n\n${text('这些技能通过游戏原生武器技能聚合器生效，不会越界写存档。默认关闭；开启后会常驻，换场景会继续核对本机角色和目标武器，直到你手动关闭。', 'These skills use the native weapon trait aggregator and do not write beyond the save record. They are off by default and remain active across scenes until you stop them, while checking the local character and target weapon each time.')}`,
    tone: 'warning', confirmLabel: text('确认开启', 'Enable'),
  })
  if (!confirmed) return
  extraWriting.value = true
  try {
    await WeaponRuntimeSkillsDeploy({
      ownerToken: hookOwnerToken, expectedSelectedAddr: Number(status.selectedAddr || 0),
      weaponSlot: Number(status.weaponSlot), weaponId: normaliseHash(status.weaponId),
      skills: extraSkills.map(skill => ({ hash: normaliseHash(skill.hash), level: Number(skill.level) })),
    })
    await loadRuntimeWorkspace()
    liveMessage.value = text(`已开启 ${extraSkills.length} 条额外武器技能；它们会在原生状态重建时加入当前目标武器。`, `${extraSkills.length} extra weapon skills enabled. They are added when the native status rebuilds.`)
    emit('status', liveMessage.value, 'success'); publishRuntimeState()
  } catch (error) {
    liveMessage.value = isolatedError(error, 'Failed to enable extra weapon skills.'); emit('status', liveMessage.value, 'error')
  } finally { extraWriting.value = false }
}

async function removeExtraSkills() {
  extraWriting.value = true
  try {
    await WeaponRuntimeSkillsRemove(); await loadRuntimeWorkspace()
    liveMessage.value = runtimeWorkspace.state === 'inactive_pending_refresh'
      ? text('Hook 已恢复；请切换武器、角色或场景，让游戏重建一次状态并清除缓存效果。', 'The hook is restored. Switch weapon, character, or scene once so the game rebuilds status and clears the cached effect.')
      : text('额外武器技能已关闭，Hook 与缓存效果均已恢复。', 'Extra weapon skills are off; the hook and cached effect were both restored.')
    emit('status', liveMessage.value, 'success'); publishRuntimeState()
  } catch (error) {
    liveMessage.value = isolatedError(error, 'Failed to stop extra weapon skills.'); emit('status', liveMessage.value, 'error')
  } finally { extraWriting.value = false }
}

onMounted(async () => {
  loading.value = true
  try {
    await backendLanguageReady
    if (disposed) return
    const result = await WeaponMemoryGetOptions()
    traits.value = Array.isArray(result?.traits) ? result.traits : []
    await loadRuntimeWorkspace()
    await pollStatus(true)
    if (status.hooked) startPolling()
  } catch (error) { liveMessage.value = isolatedError(error, 'Failed to load the weapon skill catalog.') }
  finally { loading.value = false }
})
onDeactivated(stopPolling)
onActivated(() => { if (status.hooked) startPolling() })
onBeforeUnmount(() => {
  disposed = true; lifecycleEpoch++; stopPolling()
  const owner = hookOwnerToken; hookOwnerToken = ''
  if (owner) queueRuntimeLeaseRelease(RUNTIME_LEASE_SCOPE, owner, WeaponMemoryRelease)
  publishRuntimeState()
})
</script>

<template>
  <div class="weapon-memory-page ui-page is-wide ui-page-stack">
    <section class="workflow-card ui-card">
      <header class="ui-split">
        <div><h2 class="ui-section-title">{{ text('当前武器技能即时编辑', 'Live Weapon Skill Editor') }}</h2><p class="ui-section-copy">{{ text('开启读取后，在游戏武器列表高亮目标。上半区修改武器记录内的五个技能槽；下半区可通过游戏原生聚合器继续追加第六条及以后技能。', 'Enable capture, then highlight a target in the in-game weapon list. The upper section edits the five slots stored in the weapon record; the lower section adds a sixth skill and beyond through the game\'s native aggregator.') }}</p></div>
        <span class="ui-tag" :class="status.selectedAddr ? 'is-ok' : status.hooked ? 'is-info' : ''">{{ statusLabel }}</span>
      </header>
      <div class="ui-actions"><button type="button" class="ui-btn is-primary" :disabled="loading || Boolean(hookOwnerToken)" @click="enable">{{ status.hooked && !hookOwnerToken ? text('重新接管并恢复读取', 'Reclaim Weapon Capture') : text('启用当前武器读取', 'Enable Weapon Capture') }}</button><button type="button" class="ui-btn" :disabled="loading || !status.hooked" @click="pollStatus(false)">{{ text('刷新当前武器', 'Refresh Current Weapon') }}</button><button type="button" class="ui-btn is-ghost" :disabled="loading || !status.hooked || !hookOwnerToken" @click="disable">{{ text('停止并恢复游戏指令', 'Stop and Restore Game Instruction') }}</button></div>
      <p class="ui-notice" aria-live="polite">{{ liveMessage }}</p>
      <p class="ui-hint">{{ text('默认关闭。读取开启期间请停留在游戏武器列表；切页不会关闭，只有手动停止、F12、断开或退出才恢复。2.0.3 之外的游戏版本不会安装 Hook。', 'Off by default. Stay in the in-game weapon list while capture is active. Changing pages does not stop it; manual stop, F12, disconnect, or app exit restores it. The hook is not installed on game versions other than 2.0.3.') }}</p>
    </section>

    <section class="editor-card ui-card">
      <header class="ui-split"><div><h3 class="ui-section-title">{{ text('核对并修改五个存档物理槽', 'Review and Edit Five Physical Save Slots') }}</h3><p class="ui-section-copy">{{ text('这里写入武器记录中 A4–C8 的五个永久槽。第 6 条及以后不是塞进下一把武器，而是在下方通过游戏原生聚合器常驻生效。', 'This section writes the five persistent A4–C8 record slots. Skills 6 and beyond are not written into the next weapon; the section below applies them through the native aggregator.') }}</p></div><code>{{ status.weaponId ? `Weapon ${status.weaponId} · Slot ${status.weaponSlot} · Lv ${status.weaponLevel}` : text('尚未选择武器', 'No Weapon Selected') }}</code></header>
      <div class="weapon-skill-grid" :aria-disabled="!status.selectedAddr || stale">
        <article v-for="slot in slots" :key="slot.index" class="weapon-skill-card ui-card is-flat">
          <header><strong>{{ text(`技能槽 ${slot.index + 1}`, `Skill Slot ${slot.index + 1}`) }}</strong><span>{{ currentSkill(slot.index).name || formatHex(currentSkill(slot.index).hash) }} · Lv {{ Number(currentSkill(slot.index).level || 0) }}</span></header>
          <div class="current-skill"><span class="skill-icon"><img v-if="skillIcon(currentSkill(slot.index).hash, currentSkill(slot.index).name)" :src="skillIcon(currentSkill(slot.index).hash, currentSkill(slot.index).name)" alt="" /><i v-else>—</i></span><code>{{ formatHex(currentSkill(slot.index).hash) }}</code></div>
          <label class="ui-field"><span class="ui-field-label">{{ text('准备写入的技能', 'Pending Skill') }}</span><CatalogSelect :model-value="slot.hash ? String(slot.hash) : ''" :options="catalogOptions" :icon-resolver="item => skillIcon(item.hash, item.displayName)" detail-key="hashHex" optional :disabled="!status.selectedAddr || stale" :placeholder="text('留空', 'Empty')" :search-placeholder="text('搜索技能名称或 Hash', 'Search skill names or hashes')" @update:model-value="setSlotHash(slot, $event)" /></label>
          <label class="ui-field"><span class="ui-field-label">{{ text('准备写入的等级', 'Pending Level') }}</span><input v-model.number="slot.level" class="ui-input" type="number" min="0" max="2147483647" step="1" :disabled="!status.selectedAddr || stale || !slot.hash" @blur="normaliseLevel(slot)" /></label>
        </article>
      </div>
      <div class="write-bar"><span>{{ validationMessage || text(`已有 ${changes.length} 项待写入变更。`, `${changes.length} pending changes.`) }}</span><button type="button" class="ui-btn is-primary" :disabled="!canWrite" @click="write">{{ writing ? text('正在备份、写入并回读…', 'Backing Up, Writing, and Reading Back…') : text('预览并写入当前武器', 'Preview and Write Current Weapon') }}</button></div>
    </section>

    <section class="editor-card extra-skills-card ui-card">
      <header class="ui-split">
        <div><h3 class="ui-section-title">{{ text('第 6 条及以后 · 额外武器技能', 'Skill 6 and Beyond · Extra Weapon Skills') }}</h3><p class="ui-section-copy">{{ text('想加多少条就逐条添加，不使用“五条能力上限”。它们走游戏自己的武器技能聚合器，只对本机角色和这把武器生效；不会越界破坏下一条武器记录。', 'Add as many rows as needed; there is no five-skill capability ceiling here. They use the game’s native weapon trait aggregator and bind to the local character and this weapon without overwriting the next record.') }}</p></div>
        <span class="ui-tag" :class="runtimeWorkspace.recoveryRequired ? 'is-warning' : runtimeWorkspace.state === 'active' ? 'is-ok' : ''">{{ runtimeWorkspace.recoveryRequired ? text('需要恢复', 'Recovery Required') : runtimeWorkspace.state === 'active' ? text('常驻已开启', 'Persistent Runtime On') : text('默认关闭', 'Off by Default') }}</span>
      </header>
      <div class="extra-skill-list">
        <article v-for="(skill, index) in extraSkills" :key="index" class="extra-skill-row ui-card is-flat">
          <strong>{{ text(`额外技能 ${index + 6}`, `Extra Skill ${index + 6}`) }}</strong>
          <CatalogSelect :model-value="skill.hash ? String(skill.hash) : ''" :options="catalogOptions" :icon-resolver="item => skillIcon(item.hash, item.displayName)" detail-key="hashHex" :placeholder="text('选择技能', 'Choose a skill')" :search-placeholder="text('搜索技能名称或 Hash', 'Search skill names or hashes')" @update:model-value="setExtraHash(skill, $event)" />
          <label class="ui-field compact-level"><span class="ui-field-label">Lv</span><input v-model.number="skill.level" class="ui-input" type="number" min="1" max="2147483647" step="1" :disabled="!skill.hash" @blur="normaliseExtraLevel(skill)" /></label>
          <button type="button" class="ui-btn is-ghost" @click="removeExtraSkill(index)">{{ text('移除', 'Remove') }}</button>
        </article>
        <button type="button" class="ui-btn add-extra" @click="addExtraSkill">{{ text('＋ 添加下一条武器技能', '+ Add Another Weapon Skill') }}</button>
      </div>
      <p class="ui-hint">{{ text('当前目标：', 'Current target:') }} {{ status.weaponId ? `Weapon ${status.weaponId} · Slot ${status.weaponSlot}` : text('请先启用读取并在游戏内高亮武器', 'Enable capture and highlight a weapon in-game') }}。{{ runtimeWorkspace.detail }}</p>
      <div class="write-bar"><span>{{ extraValidationMessage || text(`准备让 ${extraSkills.length} 条额外技能常驻生效。`, `${extraSkills.length} extra skills are ready for the persistent runtime.`) }}</span><div class="ui-actions"><button type="button" class="ui-btn is-ghost" :disabled="extraWriting || !runtimeWorkspace.installed" @click="removeExtraSkills">{{ text('关闭额外技能', 'Stop Extra Skills') }}</button><button type="button" class="ui-btn is-primary" :disabled="!canDeployExtra" @click="deployExtraSkills">{{ extraWriting ? text('正在应用…', 'Applying…') : text('预览并开启额外技能', 'Preview and Enable Extra Skills') }}</button></div></div>
    </section>
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<style scoped>
.weapon-memory-page { display:grid; gap:var(--space-4); min-width:0; }
.workflow-card,.editor-card { display:grid; gap:var(--space-4); padding:var(--space-4); }
.weapon-skill-grid { display:grid; grid-template-columns:repeat(5,minmax(0,1fr)); gap:var(--space-3); min-width:0; }
.weapon-skill-card { display:grid; align-content:start; gap:var(--space-3); min-width:0; padding:var(--space-3); }
.weapon-skill-card > header { display:grid; gap:2px; min-width:0; }
.weapon-skill-card > header strong { color:var(--text-primary); font-size:var(--fs-md); }
.weapon-skill-card > header span { color:var(--text-muted); font-size:var(--fs-xs); overflow-wrap:anywhere; }
.current-skill { display:flex; align-items:center; justify-content:space-between; gap:var(--space-2); min-width:0; padding:var(--space-2); border:1px solid var(--border-soft); background:var(--surface-sunken); }
.skill-icon { width:36px; height:36px; display:grid; place-items:center; flex:0 0 auto; border:1px solid var(--border-soft); background:var(--surface-card-pop); }
.skill-icon img { width:28px; height:28px; object-fit:contain; }
.current-skill code { min-width:0; color:var(--text-muted); font-size:var(--fs-xs); overflow-wrap:anywhere; }
.write-bar { position:sticky; bottom:0; z-index:2; display:flex; align-items:center; justify-content:space-between; gap:var(--space-3); padding:var(--space-3); border:1px solid var(--accent-border); background:color-mix(in srgb,var(--surface-card) 92%,transparent); backdrop-filter:blur(8px); }
.write-bar span { color:var(--text-secondary); font-size:var(--fs-sm); }
.extra-skill-list { display:grid; gap:var(--space-2); }
.extra-skill-row { display:grid; grid-template-columns:minmax(110px,.35fr) minmax(260px,1fr) minmax(120px,.25fr) auto; align-items:end; gap:var(--space-3); padding:var(--space-3); }
.compact-level { margin:0; }
.add-extra { justify-self:start; }
@container tool-panel (max-width:1180px) { .weapon-skill-grid { grid-template-columns:repeat(3,minmax(0,1fr)); } }
@container tool-panel (max-width:760px) { .weapon-skill-grid { grid-template-columns:repeat(2,minmax(0,1fr)); }.extra-skill-row { grid-template-columns:minmax(0,1fr); align-items:stretch; }.write-bar { align-items:stretch; flex-direction:column; }.write-bar .ui-btn { width:100%; } }
@container tool-panel (max-width:480px) { .weapon-skill-grid { grid-template-columns:minmax(0,1fr); } }
</style>
