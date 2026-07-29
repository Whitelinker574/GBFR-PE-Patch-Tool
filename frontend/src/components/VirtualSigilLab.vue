<script setup>
import { computed, ref, watch } from 'vue'
import { DeployVirtualSigilMod, FindSaveFiles, GetLastSavePath, GetVirtualSigilWorkspace, RemoveVirtualSigilMod } from '../../wailsjs/go/backend/App'
import { SelectSigilInputSave } from '../../wailsjs/go/backend/SigilGen'
import { language } from '../i18n'
import { characterAssetIcon, traitAssetIcon } from '../gameAssetIcons.js'
import { runtimeCompanionMessage } from '../runtimeCompanionMessages.js'
import ConfirmDialog from './ConfirmDialog.vue'
import SaveSourcePicker from './SaveSourcePicker.vue'

const emit = defineEmits(['status'])
const tx = (zh, en) => language.value === 'zh' ? zh : en
const runtimeText = value => runtimeCompanionMessage(value, language.value)
const confirmDialog = ref(null)
const workspace = ref(null)
const savePath = ref('')
const saveFiles = ref([])
const slotCount = ref(4)
const characterSlots = ref({})
const presets = ref([])
const activeCharacter = ref('4D0A60C3')
const activeSlot = ref(0)
const search = ref('')
const page = ref(0)
const presetName = ref('')
const busy = ref(false)
const message = ref('')
const tone = ref('')
const pageSize = 60

const selectionKeys = ['slotId', 'gemId', 'trait1', 'trait1Level', 'trait2', 'trait2Level', 'sigilLevel']
const inventoryBySlot = computed(() => new Map((workspace.value?.inventory || []).map(item => [Number(item.slotId), item])))
const characters = computed(() => workspace.value?.characters || [])
const currentSlots = computed(() => normalizedSlots(characterSlots.value[activeCharacter.value]))
const usedSlots = computed(() => {
  const result = new Map()
  for (const [hash, slots] of Object.entries(characterSlots.value)) {
    for (const selection of slots || []) if (selection?.slotId) result.set(Number(selection.slotId), hash)
  }
  return result
})
const filteredInventory = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  if (!query) return workspace.value?.inventory || []
  return (workspace.value?.inventory || []).filter(item => [item.name, item.primaryTraitName, item.secondaryTraitName, item.hash, item.slotId].some(value => String(value || '').toLocaleLowerCase().includes(query)))
})
const pageCount = computed(() => Math.max(1, Math.ceil(filteredInventory.value.length / pageSize)))
const visibleInventory = computed(() => filteredInventory.value.slice(page.value * pageSize, (page.value + 1) * pageSize))
const activeCount = computed(() => Object.values(characterSlots.value).flat().filter(selection => selection?.slotId).length)
const invalidSelections = computed(() => Object.entries(characterSlots.value).flatMap(([characterHash, slots]) => (slots || [])
  .map((selection, index) => ({ characterHash, index, selection }))
  .filter(entry => entry.selection?.slotId && !inventoryBySlot.value.has(Number(entry.selection.slotId)))))
const canSave = computed(() => !busy.value && !workspace.value?.recoveryRequired && invalidSelections.value.length === 0 && Boolean(savePath.value) && Boolean(workspace.value?.gameRunning))
const traitTotals = computed(() => {
  const totals = new Map()
  for (const selection of currentSlots.value) {
    const item = inventoryBySlot.value.get(Number(selection?.slotId))
    if (!item) continue
    addTraitTotal(totals, item.primaryTraitHash, item.primaryTraitName, item.trait1Level, item.primaryTraitMaxLevel)
    if (item.secondaryTraitHash) addTraitTotal(totals, item.secondaryTraitHash, item.secondaryTraitName, item.trait2Level, item.secondaryTraitMaxLevel)
  }
  return [...totals.values()].sort((a, b) => b.raw - a.raw || a.name.localeCompare(b.name))
})

function report(text, nextTone = '') {
  message.value = text
  tone.value = nextTone
  emit('status', text, nextTone)
}

function normalizedSlots(values) {
  const result = Array.from({ length: slotCount.value }, () => ({}))
  for (let index = 0; index < Math.min(values?.length || 0, result.length); index++) result[index] = selectionOnly(values[index])
  return result
}

function selectionOnly(value) {
  const result = {}
  for (const key of selectionKeys) result[key] = Number(value?.[key] || 0)
  return result
}

function addTraitTotal(target, hash, name, level, maxLevel) {
  if (!hash || !level) return
  const key = String(hash).toUpperCase()
  const current = target.get(key) || { hash: key, name: name || key, raw: 0, max: Number(maxLevel || 0) }
  current.raw += Number(level || 0)
  current.max = Math.max(current.max, Number(maxLevel || 0))
  current.effective = current.max > 0 ? Math.min(current.raw, current.max) : current.raw
  current.overflow = Math.max(0, current.raw - current.effective)
  target.set(key, current)
}

function displayCharacter(character) { return language.value === 'zh' ? character.nameZh : character.nameEn }
function selectedItem(selection) { return inventoryBySlot.value.get(Number(selection?.slotId)) }
function traitIcon(item) { return traitAssetIcon({ hash: item?.primaryTraitHash, name: item?.primaryTraitName }) }
function applyWorkspace(value) {
  workspace.value = value
  const config = value?.config || {}
  slotCount.value = Number(config.slotCount || 4)
  characterSlots.value = Object.fromEntries(Object.entries(config.characters || {}).map(([hash, slots]) => [hash.toUpperCase(), normalizedSlots(slots)]))
  presets.value = (config.presets || []).map(preset => ({ ...preset, slots: (preset.slots || []).map(selectionOnly) }))
  if (!characters.value.some(character => character.hash === activeCharacter.value)) activeCharacter.value = characters.value[0]?.hash || ''
}

async function refresh() {
  busy.value = true
  try { applyWorkspace(await GetVirtualSigilWorkspace('', savePath.value)) }
  catch (error) { report(runtimeText(error), 'danger') }
  finally { busy.value = false }
}

async function chooseSave() {
  const selected = await SelectSigilInputSave()
  if (!selected) return
  savePath.value = selected
  await refresh()
}

async function selectKnownSave(path) {
  savePath.value = String(path || '')
  if (savePath.value) await refresh()
}

function selectCharacter(hash) {
  activeCharacter.value = hash
  activeSlot.value = 0
  if (!characterSlots.value[hash]) characterSlots.value[hash] = normalizedSlots([])
}

function resizeAllSlots() {
  const next = {}
  for (const [hash, slots] of Object.entries(characterSlots.value)) next[hash] = normalizedSlots(slots)
  characterSlots.value = next
  activeSlot.value = Math.min(activeSlot.value, slotCount.value - 1)
  for (const preset of presets.value) preset.slots = normalizedSlots(preset.slots)
}

function assignItem(item) {
  const slotId = Number(item.slotId)
  const previousOwner = usedSlots.value.get(slotId)
  const next = { ...characterSlots.value }
  for (const [hash, slots] of Object.entries(next)) next[hash] = normalizedSlots(slots).map(selection => Number(selection.slotId) === slotId ? {} : selection)
  next[activeCharacter.value] = normalizedSlots(next[activeCharacter.value])
  next[activeCharacter.value][activeSlot.value] = selectionOnly(item)
  characterSlots.value = next
  if (previousOwner && previousOwner !== activeCharacter.value) report(tx('该物理因子已从另一名角色的虚拟槽转移。', 'The physical sigil was transferred from another character virtual slot.'), 'ok')
}

function clearSlot(index) {
  const next = { ...characterSlots.value, [activeCharacter.value]: normalizedSlots(characterSlots.value[activeCharacter.value]) }
  next[activeCharacter.value][index] = {}
  characterSlots.value = next
}

function savePreset() {
  const name = presetName.value.trim()
  if (!name) return report(tx('请先填写预设名称。', 'Enter a preset name first.'), 'danger')
  presets.value = [...presets.value, { id: `preset-${Date.now().toString(36)}`, name, characterHash: activeCharacter.value, slots: currentSlots.value.map(selectionOnly) }]
  presetName.value = ''
}

function applyPreset(preset) {
  selectCharacter(preset.characterHash)
  const next = { ...characterSlots.value }
  for (const selection of preset.slots || []) {
    if (!selection?.slotId) continue
    for (const [hash, slots] of Object.entries(next)) next[hash] = normalizedSlots(slots).map(current => Number(current.slotId) === Number(selection.slotId) ? {} : current)
  }
  next[preset.characterHash] = normalizedSlots(preset.slots)
  characterSlots.value = next
}

function deletePreset(id) { presets.value = presets.value.filter(preset => preset.id !== id) }

function configPayload() {
  return {
    schemaVersion: 1,
    slotCount: Number(slotCount.value),
    characters: Object.fromEntries(Object.entries(characterSlots.value).map(([hash, slots]) => [hash, normalizedSlots(slots).map(selectionOnly)])),
    presets: presets.value.map(preset => ({ ...preset, slots: normalizedSlots(preset.slots).map(selectionOnly) })),
  }
}

async function save() {
  busy.value = true
  try {
    const result = await DeployVirtualSigilMod({ savePath: savePath.value, config: configPayload() })
    const followup = result?.restartRequired
      ? tx('槽位数量已改变，请重启游戏。', 'Slot count changed; restart the game.')
      : result?.refreshRequired ? tx('请切换一次装备、角色或场景，让游戏重建技能。', 'Switch equipment, character, or scene once so the game rebuilds traits.') : ''
    report(tx(`虚拟因子配置已保存，共 ${result?.activeSlots || 0} 个活动槽。`, `Virtual sigil configuration saved with ${result?.activeSlots || 0} active slots.`) + (followup ? ` ${followup}` : ''), 'ok')
    await refresh()
  } catch (error) { report(runtimeText(error), 'danger') }
  finally { busy.value = false }
}

async function remove() {
  const accepted = await confirmDialog.value?.ask({ title: tx('停用虚拟因子运行时', 'Disable virtual sigil runtime'), message: tx('会先恢复 getter、枚举路径和原生槽位上限，再停用内置运行时；不会修改游戏存档。', 'The getter, enumeration path, and native slot limits are restored before the built-in runtime stops. The save is never modified.'), confirmLabel: tx('确认停用', 'Disable'), cancelLabel: tx('取消', 'Cancel'), tone: 'danger' })
  if (!accepted) return
  busy.value = true
  try { await RemoveVirtualSigilMod(''); report(tx('虚拟因子运行时已停用，Hook 和原生槽位上限已恢复。', 'Virtual sigil runtime disabled; hooks and native slot limits restored.'), 'ok'); await refresh() }
  catch (error) { report(runtimeText(error), 'danger') }
  finally { busy.value = false }
}

watch(search, () => { page.value = 0 })
watch(pageCount, count => { page.value = Math.min(page.value, count - 1) })
watch(slotCount, resizeAllSlots)

async function initialize() {
  try {
    saveFiles.value = await FindSaveFiles()
    savePath.value = await GetLastSavePath() || saveFiles.value[0]?.path || ''
  } catch {}
  await refresh()
}
initialize()
</script>

<template>
  <div class="virtual-lab ui-page-stack">
    <section class="virtual-intro"><div><p class="virtual-kicker">RUNTIME TRAIT EXTENSION · BUILT-IN</p><h2>{{ tx('虚拟因子槽', 'Virtual Sigil Slots') }}</h2><p>{{ tx('引用存档中真实、未装备的因子实例，在运行时加入额外技能；不扩写存档的 12 个物理配装槽。', 'Reference real unequipped sigil instances and add their traits at runtime without extending the save\'s 12 physical loadout slots.') }}</p></div><div class="virtual-boundary"><b>{{ tx('应用内自有运行时', 'Tool-owned built-in runtime') }}</b><span>{{ tx('不写存档、不修改装备归属；换档内容不一致时拒绝注入', 'No save or ownership writes; cross-save mismatches fail closed') }}</span></div></section>

    <section class="source-setup" aria-label="第一步 · 选择来源存档">
      <SaveSourcePicker
        :slots="saveFiles"
        :model-value="savePath"
        :busy="busy"
        :loaded="Boolean(workspace)"
        :summary="workspace ? tx(`${workspace.inventory?.length || 0} 个可引用因子`, `${workspace.inventory?.length || 0} available sigils`) : ''"
        :helper="tx('来源存档决定角色、真实库存实例和运行时校验目标', 'The source save defines characters, inventory instances, and runtime validation')"
        @select="selectKnownSave"
        @browse="chooseSave"
      />
      <label class="slot-count"><span>{{ tx('虚拟槽数量', 'Virtual slot count') }}</span><input v-model.number="slotCount" class="ui-input" type="number" min="1" max="8" step="1" /><small>{{ tx('运行中修改会自动安全重载', 'Changes safely reload the built-in runtime') }}</small></label>
      <div class="ui-notice" :class="workspace?.recoveryRequired ? 'is-danger' : workspace?.gameRunning ? 'is-ok' : 'is-info'">{{ workspace?.recoveryRequired ? runtimeText(workspace?.detail) || tx('Hook 恢复未完成，请先在页面底部重试恢复。', 'Hook restoration is incomplete. Retry restoration at the bottom first.') : workspace?.gameRunning ? (workspace?.state === 'active' ? tx('游戏已连接，虚拟因子运行时正在工作。', 'Game connected; virtual sigil runtime active.') : tx('游戏已连接，保存后会直接开启内置运行时。', 'Game connected; saving enables the built-in runtime directly.')) : tx('请先启动游戏，再保存并开启虚拟因子。', 'Start the game before saving and enabling virtual sigils.') }}</div>
      <div class="ui-notice is-info">{{ tx('角色专属因子不做强制拦截；跨角色配置会保留，但属于实验用强制选择，游戏可能忽略不适用的效果。', 'Character-exclusive sigils are not hard-blocked. Cross-character choices are preserved as experimental overrides, but the game may ignore effects that do not apply.') }}</div>
    </section>

    <section class="character-step"><header><h3>{{ tx('第二步 · 选择角色并配置槽位', 'Step 2 · Choose a character and configure slots') }}</h3><p>{{ tx('下方角色、未装备因子和预设都绑定第一步选择的来源存档。', 'The characters, unequipped sigils, and presets below are bound to the source save selected in step 1.') }}</p></header><div class="character-strip" aria-label="Character selection"><button v-for="character in characters" :key="character.hash" type="button" :class="{ active: activeCharacter === character.hash }" @click="selectCharacter(character.hash)"><img v-if="characterAssetIcon(character.hash)" :src="characterAssetIcon(character.hash)" alt="" /><span>{{ displayCharacter(character) }}</span><small>{{ (characterSlots[character.hash] || []).filter(item => item?.slotId).length }}/{{ slotCount }}</small></button></div></section>

    <section class="virtual-workspace">
      <div class="slot-panel"><header><div><h3>{{ tx('当前角色虚拟槽', 'Current character slots') }}</h3><p>{{ tx('先点一个槽，再从右侧选择真实因子。', 'Select a slot, then choose a real sigil from the right.') }}</p></div><b>{{ currentSlots.filter(item => item?.slotId).length }}/{{ slotCount }}</b></header><div class="virtual-slots"><article v-for="(selection, index) in currentSlots" :key="index" :class="{ active: activeSlot === index, filled: selection?.slotId, invalid: selection?.slotId && !selectedItem(selection) }" @click="activeSlot = index"><span class="slot-index">{{ String(index + 1).padStart(2, '0') }}</span><img v-if="traitIcon(selectedItem(selection))" :src="traitIcon(selectedItem(selection))" alt="" /><span v-else class="slot-icon-placeholder" aria-hidden="true">{{ selection?.slotId ? '!' : '＋' }}</span><div v-if="selectedItem(selection)"><b>{{ selectedItem(selection).name }}</b><small>{{ selectedItem(selection).primaryTraitName }} Lv{{ selectedItem(selection).trait1Level }}<template v-if="selectedItem(selection).secondaryTraitName"> · {{ selectedItem(selection).secondaryTraitName }} Lv{{ selectedItem(selection).trait2Level }}</template></small></div><div v-else-if="selection?.slotId"><b>{{ tx('来源实例失效', 'Source instance unavailable') }}</b><small>{{ tx(`SlotID #${selection.slotId} 不在当前来源存档的未装备因子中，请替换或清空。`, `SlotID #${selection.slotId} is not an unequipped sigil in the selected source save. Replace or clear it.`) }}</small></div><div v-else><b>{{ tx('空槽', 'Empty slot') }}</b><small>{{ tx('等待选择因子', 'Choose a sigil') }}</small></div><button v-if="selection?.slotId" type="button" :title="tx('清空', 'Clear')" @click.stop="clearSlot(index)">×</button></article></div></div>

      <div class="inventory-panel"><header><div><h3>{{ tx('未装备因子', 'Unequipped sigils') }}</h3><p>{{ tx(`${filteredInventory.length} 个可用实例`, `${filteredInventory.length} available instances`) }}</p></div><input v-model="search" class="ui-input" type="search" :placeholder="tx('搜索因子、主副词条或哈希', 'Search sigil, trait, or hash')" /></header><div class="inventory-list"><button v-for="item in visibleInventory" :key="item.slotId" type="button" :class="{ used: usedSlots.has(Number(item.slotId)), selected: Number(currentSlots[activeSlot]?.slotId) === Number(item.slotId) }" @click="assignItem(item)"><img v-if="traitIcon(item)" :src="traitIcon(item)" alt="" /><span><b>{{ item.name }}</b><small>{{ item.primaryTraitName }} Lv{{ item.trait1Level }}<template v-if="item.secondaryTraitName"> · {{ item.secondaryTraitName }} Lv{{ item.trait2Level }}</template></small></span><em>{{ usedSlots.has(Number(item.slotId)) ? tx('已占用', 'Used') : `#${item.slotId}` }}</em></button></div><footer v-if="pageCount > 1"><button class="ui-btn is-sm" type="button" :disabled="page === 0" @click="page--">‹</button><span>{{ page + 1 }} / {{ pageCount }}</span><button class="ui-btn is-sm" type="button" :disabled="page + 1 >= pageCount" @click="page++">›</button></footer></div>
    </section>

    <section class="trait-ledger"><header><div><h3>{{ tx('虚拟槽技能等级', 'Virtual-slot trait levels') }}</h3><p>{{ tx('原始等级不裁剪；有效等级与溢出按游戏目录上限分开显示。', 'Raw levels are preserved; effective and overflow levels are shown separately using catalog caps.') }}</p></div></header><div v-if="traitTotals.length" class="trait-grid"><article v-for="trait in traitTotals" :key="trait.hash"><img v-if="traitAssetIcon({ hash: trait.hash, name: trait.name })" :src="traitAssetIcon({ hash: trait.hash, name: trait.name })" alt="" /><span><b>{{ trait.name }}</b><small>{{ trait.hash }}</small></span><dl><div><dt>{{ tx('原始', 'Raw') }}</dt><dd>Lv{{ trait.raw }}</dd></div><div><dt>{{ tx('有效', 'Effective') }}</dt><dd>Lv{{ trait.effective }}</dd></div><div :class="{ overflow: trait.overflow }"><dt>{{ tx('溢出', 'Overflow') }}</dt><dd>+{{ trait.overflow }}</dd></div></dl></article></div><p v-else class="empty-state">{{ tx('当前角色还没有配置虚拟因子。', 'No virtual sigils are configured for this character.') }}</p></section>

    <section class="preset-panel"><header><div><h3>{{ tx('角色预设', 'Character presets') }}</h3><p>{{ tx('保存当前角色的整组虚拟槽；预设仍会核对真实实例指纹。', 'Save the current character slot set; presets still verify exact physical instances.') }}</p></div><div><input v-model="presetName" class="ui-input" type="text" maxlength="48" :placeholder="tx('预设名称', 'Preset name')" /><button class="ui-btn is-sm" type="button" @click="savePreset">{{ tx('保存预设', 'Save preset') }}</button></div></header><div v-if="presets.length" class="preset-list"><article v-for="preset in presets" :key="preset.id"><div><b>{{ preset.name }}</b><small>{{ preset.characterHash }} · {{ (preset.slots || []).filter(item => item?.slotId).length }}/{{ slotCount }}</small></div><button class="ui-btn is-sm" type="button" @click="applyPreset(preset)">{{ tx('应用', 'Apply') }}</button><button class="preset-delete" type="button" :title="tx('删除预设', 'Delete preset')" @click="deletePreset(preset.id)">×</button></article></div><p v-else class="empty-state">{{ tx('还没有保存预设。', 'No presets saved yet.') }}</p></section>

    <div v-if="invalidSelections.length" class="ui-notice is-danger"><b>{{ tx(`有 ${invalidSelections.length} 个槽位引用了另一份存档或已不存在的因子实例。`, `${invalidSelections.length} slots reference sigil instances from another save or no longer present.`) }}</b><span>{{ tx('这些槽位已经标红；替换或清空后才能保存，避免把无效实例写进组件配置。', 'The slots are highlighted. Replace or clear them before saving so invalid instances are not written to the component configuration.') }}</span></div>
    <section class="virtual-dock"><div><b>{{ tx('第三步 · 应用到当前游戏', 'Step 3 · Apply to the current game') }}</b><small>{{ workspace?.recoveryRequired ? tx('Hook 或原生槽位上限恢复未完成，只能先重试恢复。', 'Hook or native slot-limit restoration is incomplete; retry restoration first.') : tx(`${activeCount} 个已配置虚拟槽；实例始终按上方来源存档逐项校验`, `${activeCount} configured virtual slots; every instance is validated against the source save above`) }}</small></div><div class="dock-actions"><button v-if="workspace?.owned" class="ui-btn is-danger" type="button" :disabled="busy" @click="remove">{{ workspace?.recoveryRequired ? tx('重试恢复', 'Retry restoration') : tx('停用并恢复', 'Disable and restore') }}</button><button class="ui-btn is-primary" type="button" :disabled="!canSave" @click="save">{{ busy ? tx('处理中…', 'Working…') : workspace?.recoveryRequired ? tx('需先恢复', 'Restore first') : workspace?.state === 'active' ? tx('保存并热更新', 'Save and hot-update') : tx('开启虚拟因子', 'Enable virtual sigils') }}</button></div></section>
    <div v-if="message" class="ui-notice" :class="{ 'is-danger': tone === 'danger', 'is-ok': tone === 'ok' }" role="status">{{ message }}</div>
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<style scoped>
.virtual-lab { width:100%; max-width:100%; min-width:0; container:virtual-lab / inline-size; overflow-x:clip; padding-bottom:82px; }.virtual-intro,.source-setup,.character-step,.character-strip,.virtual-workspace,.trait-ledger,.preset-panel,.virtual-dock { width:100%; max-width:100%; min-width:0; }
.virtual-intro { display:grid; grid-template-columns:minmax(0,1fr) minmax(250px,330px); align-items:end; gap:var(--space-6); padding:var(--space-6) 0 var(--space-5); border-bottom:1px solid var(--border-default); }.virtual-kicker { margin:0 0 var(--space-2); color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); letter-spacing:.06em; }.virtual-intro h2,.source-setup h3,.slot-panel h3,.inventory-panel h3,.trait-ledger h3,.preset-panel h3 { margin:0; color:var(--text-primary); font-family:var(--font-display); }.virtual-intro h2 { font-size:var(--fs-2xl); }.virtual-intro p,.source-setup p,.slot-panel header p,.inventory-panel header p,.trait-ledger header p,.preset-panel header p { margin:var(--space-2) 0 0; color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }.virtual-boundary { padding:var(--space-4); border-left:3px solid var(--success); background:var(--success-bg); color:var(--success-ink); }.virtual-boundary b,.virtual-boundary span { display:block; }.virtual-boundary span { margin-top:4px; font-size:var(--fs-xs); }
.source-setup { display:grid; grid-template-columns:minmax(0,1fr) 180px; align-items:end; gap:var(--space-3); }.source-setup > .save-source-picker,.source-setup > .ui-notice { grid-column:1 / -1; }.character-step h3 { margin:0; color:var(--text-primary); font-size:var(--fs-lg); }.character-step p { margin:3px 0 0; color:var(--text-muted); font-size:var(--fs-xs); }.slot-count > span { display:block; margin-bottom:6px; color:var(--text-primary); font-size:var(--fs-xs); font-weight:var(--fw-bold); }.slot-count { grid-column:2; }.slot-count .ui-input { width:100%; }.slot-count small { display:block; margin-top:4px; color:var(--text-muted); font-size:10px; }
.character-step { display:grid; gap:var(--space-2); }
.character-strip { display:flex; gap:var(--space-2); padding:var(--space-2); overflow-x:auto; border-block:1px solid var(--border-default); scrollbar-width:thin; scrollbar-color:var(--border-strong) transparent; }.character-strip button { display:grid; grid-template-columns:36px minmax(68px,auto) auto; align-items:center; gap:var(--space-2); min-width:max-content; height:48px; padding:4px var(--space-3) 4px 5px; border:1px solid var(--border-default); border-radius:var(--radius-sm); color:var(--text-secondary); background:var(--surface-card); cursor:pointer; }.character-strip button.active { border-color:var(--accent-border); color:var(--text-primary); background:var(--selected-bg); box-shadow:3px 0 0 var(--accent) inset; }.character-strip img { width:36px; height:36px; border-radius:50%; object-fit:cover; }.character-strip span { font-size:var(--fs-xs); font-weight:var(--fw-semibold); }.character-strip small { color:var(--accent); font-family:var(--font-data); font-size:10px; }
.virtual-workspace { display:grid; grid-template-columns:minmax(320px,.92fr) minmax(380px,1.08fr); gap:var(--space-4); align-items:start; }.slot-panel,.inventory-panel { min-width:0; }.slot-panel > header,.inventory-panel > header,.trait-ledger > header,.preset-panel > header { display:flex; align-items:flex-start; justify-content:space-between; gap:var(--space-3); margin-bottom:var(--space-3); }.slot-panel > header > b { color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xl); }.virtual-slots { display:grid; gap:var(--space-2); }.virtual-slots article { display:grid; grid-template-columns:32px 38px minmax(0,1fr) 28px; align-items:center; gap:var(--space-2); min-width:0; min-height:66px; padding:var(--space-2); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-card); cursor:pointer; }.virtual-slots article.active { border-color:var(--accent-border); background:var(--selected-bg); box-shadow:3px 0 0 var(--accent) inset; }.virtual-slots article:not(.filled) img { opacity:.35; }.slot-index { color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); }.virtual-slots img,.inventory-list img,.trait-grid img { width:38px; height:38px; object-fit:contain; }.virtual-slots article > div,.inventory-list span,.trait-grid span { min-width:0; }.virtual-slots b,.virtual-slots small,.inventory-list b,.inventory-list small,.trait-grid b,.trait-grid small { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.virtual-slots b,.inventory-list b,.trait-grid b { color:var(--text-primary); font-size:var(--fs-xs); }.virtual-slots small,.inventory-list small,.trait-grid small { margin-top:4px; color:var(--text-muted); font-size:10px; }.virtual-slots article > button,.preset-delete { width:28px; height:28px; padding:0; border:1px solid var(--border-default); border-radius:50%; color:var(--danger); background:var(--surface-field); font-size:18px; cursor:pointer; }
.virtual-slots article.invalid { border-color:var(--danger); background:var(--danger-bg); box-shadow:3px 0 0 var(--danger) inset; }.slot-icon-placeholder { width:38px; height:38px; display:grid; place-items:center; border:1px dashed var(--border-strong); border-radius:var(--radius-sm); color:var(--text-muted); background:var(--surface-sunken); font-weight:var(--fw-bold); }.virtual-slots article.invalid .slot-icon-placeholder,.virtual-slots article.invalid b { color:var(--danger-ink); }.virtual-slots article.invalid small { white-space:normal; overflow-wrap:anywhere; }
.inventory-panel > header .ui-input { width:min(100%,300px); }.inventory-list { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-2); max-height:560px; overflow-y:auto; padding-right:3px; scrollbar-width:thin; scrollbar-color:var(--border-strong) transparent; }.inventory-list button { display:grid; grid-template-columns:36px minmax(0,1fr) auto; align-items:center; gap:var(--space-2); min-width:0; min-height:64px; padding:var(--space-2); border:1px solid var(--border-default); border-radius:var(--radius-sm); color:inherit; background:var(--surface-card); text-align:left; cursor:pointer; }.inventory-list button:hover,.inventory-list button.selected { border-color:var(--accent-border); background:var(--selected-bg); }.inventory-list button.used:not(.selected) { opacity:.68; }.inventory-list em { color:var(--text-muted); font-family:var(--font-data); font-size:9px; font-style:normal; }.inventory-panel footer { display:flex; align-items:center; justify-content:center; gap:var(--space-3); margin-top:var(--space-3); color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); }
.trait-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-2); }.trait-grid article { display:grid; grid-template-columns:38px minmax(0,1fr) auto; align-items:center; gap:var(--space-2); min-width:0; padding:var(--space-3); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-card); }.trait-grid dl { display:flex; gap:var(--space-3); margin:0; }.trait-grid dl div { text-align:right; }.trait-grid dt { color:var(--text-muted); font-size:9px; }.trait-grid dd { margin:2px 0 0; color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); }.trait-grid .overflow dd { color:var(--warning); }
.preset-panel > header > div:last-child { display:flex; gap:var(--space-2); width:min(100%,380px); }.preset-panel .ui-input { min-width:0; flex:1; }.preset-list { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:var(--space-2); }.preset-list article { display:grid; grid-template-columns:minmax(0,1fr) auto 28px; align-items:center; gap:var(--space-2); padding:var(--space-3); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-card); }.preset-list article b,.preset-list article small { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.preset-list article b { color:var(--text-primary); font-size:var(--fs-xs); }.preset-list article small { margin-top:3px; color:var(--text-muted); font-family:var(--font-data); font-size:9px; }.empty-state { margin:0; padding:var(--space-4); color:var(--text-muted); background:var(--surface-subtle); text-align:center; }
.virtual-dock { position:sticky; z-index:10; bottom:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-4); min-height:66px; padding:var(--space-3) var(--space-4); border:1px solid var(--border-strong); border-left:4px solid var(--accent); border-radius:var(--radius-md); background:var(--surface-card-pop); box-shadow:var(--shadow-2); }.virtual-dock b,.virtual-dock small { display:block; }.virtual-dock b { color:var(--text-primary); font-size:var(--fs-sm); }.virtual-dock small { margin-top:3px; color:var(--text-muted); font-size:var(--fs-xs); }.dock-actions { display:flex; gap:var(--space-2); }
@container virtual-lab (max-width:1000px) { .virtual-workspace { grid-template-columns:minmax(0,1fr); }.inventory-list { max-height:460px; }.preset-list { grid-template-columns:repeat(2,minmax(0,1fr)); } }
@container virtual-lab (max-width:720px) { .virtual-intro,.source-setup { grid-template-columns:minmax(0,1fr); }.slot-count { grid-column:auto; }.slot-panel > header,.inventory-panel > header,.trait-ledger > header,.preset-panel > header,.virtual-dock { align-items:stretch; flex-direction:column; }.inventory-panel > header .ui-input,.preset-panel > header > div:last-child,.dock-actions,.dock-actions .ui-btn { width:100%; }.trait-grid,.preset-list { grid-template-columns:minmax(0,1fr); } }
@container virtual-lab (max-width:500px) { .inventory-list { grid-template-columns:minmax(0,1fr); }.preset-panel > header > div:last-child { flex-direction:column; }.virtual-slots article { grid-template-columns:28px 34px minmax(0,1fr) 28px; }.trait-grid dl { gap:var(--space-2); } }
</style>
