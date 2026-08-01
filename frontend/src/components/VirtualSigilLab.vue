<script setup>
import { computed, ref, watch } from 'vue'
import { DeployVirtualSigilMod, FindSaveFiles, GetLastSavePath, GetVirtualSigilWorkspace, RemoveVirtualSigilMod } from '../../wailsjs/go/backend/App'
import { CreateVirtualSigilSource, GetCompatibleSecondaryTraits, GetSigilList, GetTraitList } from '../../wailsjs/go/backend/SigilGen'
import { SelectSigilInputSave } from '../../wailsjs/go/backend/SigilGen'
import { language } from '../i18n'
import { characterAssetIcon, traitAssetIcon } from '../gameAssetIcons.js'
import { runtimeCompanionMessage } from '../runtimeCompanionMessages.js'
import ConfirmDialog from './ConfirmDialog.vue'
import CatalogSelect from './CatalogSelect.vue'
import SaveSourcePicker from './SaveSourcePicker.vue'

const emit = defineEmits(['status', 'runtime-state'])
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
const sourceMode = ref('inventory')
const constructorSigils = ref([])
const constructorTraits = ref([])
const constructorSecondaryTraits = ref([])
const constructorSigilId = ref('')
const constructorSecondaryTraitId = ref('')
const constructorSigilLevel = ref(15)
const constructorPrimaryLevel = ref(15)
const constructorSecondaryLevel = ref(15)
const constructorLoading = ref(false)
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
const canSave = computed(() => workspace.value?.available === true && !busy.value && !workspace.value?.recoveryRequired && invalidSelections.value.length === 0 && Boolean(savePath.value) && Boolean(workspace.value?.gameRunning))
const selectedConstructorSigil = computed(() => constructorSigils.value.find(item => item.internalId === constructorSigilId.value) || null)
const selectedConstructorPrimary = computed(() => constructorTraits.value.find(item => item.internalId === selectedConstructorSigil.value?.primaryTraitId) || null)
const selectedConstructorSecondary = computed(() => constructorTraits.value.find(item => item.internalId === constructorSecondaryTraitId.value) || null)
const constructorSigilMax = computed(() => highestLevel(selectedConstructorSigil.value?.allowedSigilLevels, selectedConstructorSigil.value?.defaultSigilLevel || 15))
const constructorPrimaryMax = computed(() => highestLevel(selectedConstructorPrimary.value?.allowedLevels, selectedConstructorPrimary.value?.maxLevel || 15))
const constructorSecondaryMax = computed(() => highestLevel(selectedConstructorSecondary.value?.allowedLevels, selectedConstructorSecondary.value?.maxLevel || 15))
const canCreateSource = computed(() => !busy.value && !constructorLoading.value && Boolean(savePath.value) && Boolean(selectedConstructorSigil.value) && workspace.value?.gameRunning !== true)
const runtimeActive = computed(() => workspace.value?.installed && workspace.value?.owned && workspace.value?.state === 'active')
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
function traitOptionIcon(item) { return traitAssetIcon({ hash: item?.hash, name: item?.displayName, internalId: item?.internalId }) }
function sigilOptionIcon(item) {
  const primary = constructorTraits.value.find(trait => trait.internalId === item?.primaryTraitId)
  return traitAssetIcon({ hash: primary?.hash, name: primary?.displayName, internalId: primary?.internalId })
}
function applyWorkspace(value) {
  workspace.value = value
  emit('runtime-state', {
    id: 'virtualSigils',
    active: value?.installed === true && value?.state === 'active' && value?.owned === true,
    recoveryRequired: value?.recoveryRequired === true,
  })
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

function highestLevel(values, fallback = 15) {
  const levels = (values || []).map(Number).filter(value => Number.isFinite(value) && value > 0)
  return levels.length ? Math.max(...levels) : fallback
}

function clampConstructorLevel(value, maximum) {
  return Math.max(1, Math.min(2147483647, Math.round(Number(value) || 1)))
}

async function selectConstructorSigil(id) {
  constructorSigilId.value = id
  constructorSecondaryTraitId.value = ''
  constructorSecondaryTraits.value = []
  const sigil = constructorSigils.value.find(item => item.internalId === id)
  if (!sigil) return
  constructorSigilLevel.value = highestLevel(sigil.allowedSigilLevels, sigil.defaultSigilLevel || 15)
  constructorPrimaryLevel.value = highestLevel(sigil.allowedFirstTraitLevels, sigil.firstTraitMaxLevel || 15)
  constructorLoading.value = true
  try {
    constructorSecondaryTraits.value = await GetCompatibleSecondaryTraits(id) || []
  } catch (error) {
    report(runtimeText(error), 'danger')
  } finally {
    constructorLoading.value = false
  }
}

async function createAndAssignSource() {
  if (!canCreateSource.value) {
    if (workspace.value?.gameRunning) report(tx('游戏运行时不能改写默认存档；请完全退出游戏后再制造因子。', 'The managed save cannot be changed while the game is running. Fully exit the game before creating the sigil.'), 'danger')
    return
  }
  const sigil = selectedConstructorSigil.value
  const primary = selectedConstructorPrimary.value
  const secondary = selectedConstructorSecondary.value
  const accepted = await confirmDialog.value?.ask({
    title: tx('制造并放入当前虚拟槽', 'Create and Assign to This Virtual Slot'),
    message: tx(`${sigil.displayName}会作为一颗新的未装备因子写入来源存档。`, `${sigil.displayName} will be written to the source save as a new unequipped sigil.`),
    detail: tx('会先自动备份、原子写入并重新打开回读；验证通过后自动放入当前虚拟槽。', 'The save is backed up, atomically written, reopened, and verified before the new instance is assigned to this virtual slot.'),
    confirmLabel: tx('备份并制造', 'Back Up and Create'),
    cancelLabel: tx('取消', 'Cancel'),
    tone: 'warning',
  })
  if (!accepted) return
  busy.value = true
  try {
    const draft = configPayload()
    const result = await CreateVirtualSigilSource(savePath.value, {
      sigilId: sigil.internalId,
      sigilName: '',
      level: Number(constructorSigilLevel.value),
      primaryTraitId: primary?.internalId || sigil.primaryTraitId,
      primaryTraitName: '',
      primaryLevel: Number(constructorPrimaryLevel.value),
      secondaryTraitId: secondary?.internalId || '',
      secondaryTraitName: '',
      secondaryLevel: secondary ? Number(constructorSecondaryLevel.value) : 0,
      quantity: 1,
    })
    await refresh()
    slotCount.value = Number(draft.slotCount || slotCount.value)
    characterSlots.value = Object.fromEntries(Object.entries(draft.characters || {}).map(([hash, slots]) => [hash, normalizedSlots(slots)]))
    presets.value = (draft.presets || []).map(preset => ({ ...preset, slots: normalizedSlots(preset.slots) }))
    const createdSlotId = Number(result?.slotIds?.[0] || 0)
    const created = inventoryBySlot.value.get(createdSlotId)
    if (!created) throw new Error(tx('新因子已写入并回读，但没有出现在未装备因子列表中。请检查是否被游戏或其他配装占用。', 'The new sigil was written and verified but did not appear in the unequipped inventory. Check whether the game or another loadout equipped it.'))
    assignItem(created)
    sourceMode.value = 'inventory'
    report(tx(`已制造 ${created.name} 并放入虚拟槽 ${activeSlot.value + 1}。重新启动游戏后即可开启运行时。`, `Created ${created.name} and assigned it to virtual slot ${activeSlot.value + 1}. Restart the game, then enable the runtime.`), 'ok')
  } catch (error) {
    report(runtimeText(error), 'danger')
  } finally {
    busy.value = false
  }
}

async function save() {
  busy.value = true
  try {
    const result = await DeployVirtualSigilMod({ savePath: savePath.value, config: configPayload() })
    const followup = result?.restartRequired
      ? tx('槽位数量已改变，请重启游戏。', 'Slot count changed; restart the game.')
      : result?.refreshRequired ? tx('请切换一次装备、角色或场景，让游戏重建技能。', 'Switch equipment, character, or scene once so the game rebuilds traits.') : ''
    report(tx(`已把 ${result?.activeSlots || 0} 个虚拟槽应用到当前游戏。`, `${result?.activeSlots || 0} virtual slots are now active in the current game.`) + (followup ? ` ${followup}` : ''), 'ok')
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
watch(constructorSigilId, id => { void selectConstructorSigil(id) })
watch(constructorSecondaryTraitId, id => {
  const trait = constructorTraits.value.find(item => item.internalId === id)
  constructorSecondaryLevel.value = trait ? Math.min(15, highestLevel(trait.allowedLevels, trait.maxLevel || 15)) : 0
})

async function initialize() {
  try {
    ;[saveFiles.value, constructorSigils.value, constructorTraits.value] = await Promise.all([FindSaveFiles(), GetSigilList(), GetTraitList()])
    savePath.value = await GetLastSavePath() || saveFiles.value[0]?.path || ''
    const first = constructorSigils.value.find(item => item.constructible !== false) || constructorSigils.value[0]
    if (first) constructorSigilId.value = first.internalId
  } catch {}
  await refresh()
}
initialize()
</script>

<template>
  <div class="virtual-lab ui-page-stack">
    <section class="virtual-intro"><div><p class="virtual-kicker">RUNTIME TRAIT EXTENSION · BUILT-IN</p><h2>{{ tx('额外技能槽（虚拟因子）', 'Extra Trait Slots (Virtual Sigils)') }}</h2><p>{{ tx('让角色在运行时额外读取 1 至 8 颗真实库存因子的技能。它不会把存档里的 12 个配装槽扩成更多槽位。', 'Let a character read traits from 1–8 additional real inventory sigils at runtime. This does not expand the save’s 12 loadout slots.') }}</p></div><div class="virtual-boundary"><b>{{ tx('运行时配置与存档写入分开', 'Runtime Configuration Is Separate from Save Writing') }}</b><span>{{ tx('从背包选择只会引用现有实例；只有主动选择“制造新因子”时，才会在游戏退出后备份并写入来源存档', 'Choosing from inventory only references existing instances. The source save is backed up and written only when you explicitly choose Create New Sigil after exiting the game') }}</span></div></section>

    <section class="source-setup" :aria-label="tx('第一步 · 选择因子来源', 'Step 1 · Choose the Sigil Source')">
      <SaveSourcePicker
        :slots="saveFiles"
        :model-value="savePath"
        :busy="busy"
        :loaded="Boolean(workspace)"
        :summary="workspace ? tx(`${workspace.inventory?.length || 0} 个可引用因子`, `${workspace.inventory?.length || 0} available sigils`) : ''"
        :helper="tx('这里只读取角色与未装备因子；从背包选择不会修改这份存档', 'This reads characters and unequipped sigils only; choosing an inventory sigil does not modify this save')"
        @select="selectKnownSave"
        @browse="chooseSave"
      >
        <template #details>
          <label class="slot-count">
            <span>{{ tx('每名角色的额外槽数', 'Extra Slots per Character') }}<small>{{ tx('1–8 个运行时槽位', '1–8 runtime slots') }}</small></span>
            <input v-model.number="slotCount" class="ui-input" type="number" min="1" max="8" step="1" />
          </label>
        </template>
      </SaveSourcePicker>
      <div class="ui-notice" :class="workspace?.recoveryRequired ? 'is-danger' : workspace?.available === false ? 'is-warn' : workspace?.gameRunning ? 'is-ok' : 'is-info'">{{ workspace?.recoveryRequired ? runtimeText(workspace?.detail) || tx('Hook 恢复未完成，请先在页面底部重试恢复。', 'Hook restoration is incomplete. Retry restoration at the bottom first.') : workspace?.available === false ? tx(workspace?.unavailableReason || '当前游戏版本或运行时身份未通过检查，请重新检测。', workspace?.unavailableReason || 'The current game version or runtime identity did not pass validation. Check again.') : workspace?.gameRunning ? (runtimeActive ? tx('游戏已连接，虚拟因子运行时正在工作。', 'Game connected; virtual sigil runtime active.') : tx('游戏已连接，保存后会直接开启内置运行时。', 'Game connected; saving enables the built-in runtime directly.')) : tx('请先启动游戏，再保存并开启虚拟因子。', 'Start the game before saving and enabling virtual sigils.') }}</div>
      <div class="ui-notice is-info">{{ tx('角色专属因子不做强制拦截；跨角色配置会保留，但属于实验用强制选择，游戏可能忽略不适用的效果。', 'Character-exclusive sigils are not hard-blocked. Cross-character choices are preserved as experimental overrides, but the game may ignore effects that do not apply.') }}</div>
    </section>

    <section class="character-step"><header><h3>{{ tx('第二步 · 给角色选择额外因子', 'Step 2 · Choose Extra Sigils for a Character') }}</h3><p>{{ tx('先选角色和左侧槽位，再从右侧背包点一颗未装备因子；同一实例不能重复占用。', 'Choose a character and a slot on the left, then select an unequipped inventory sigil on the right. One instance cannot fill multiple slots.') }}</p></header><div class="character-strip" :aria-label="tx('选择角色', 'Choose a Character')"><button v-for="character in characters" :key="character.hash" type="button" :class="{ active: activeCharacter === character.hash }" @click="selectCharacter(character.hash)"><img v-if="characterAssetIcon(character.hash)" :src="characterAssetIcon(character.hash)" alt="" /><span>{{ displayCharacter(character) }}</span><small>{{ (characterSlots[character.hash] || []).filter(item => item?.slotId).length }}/{{ slotCount }}</small></button></div></section>

    <section class="virtual-workspace">
      <div class="slot-panel"><header><div><h3>{{ tx('当前角色虚拟槽', 'Current character slots') }}</h3><p>{{ tx('先点一个槽，再从右侧选择真实因子。', 'Select a slot, then choose a real sigil from the right.') }}</p></div><b>{{ currentSlots.filter(item => item?.slotId).length }}/{{ slotCount }}</b></header><div class="virtual-slots"><article v-for="(selection, index) in currentSlots" :key="index" :class="{ active: activeSlot === index, filled: selection?.slotId, invalid: selection?.slotId && !selectedItem(selection) }" @click="activeSlot = index"><span class="slot-index">{{ String(index + 1).padStart(2, '0') }}</span><img v-if="traitIcon(selectedItem(selection))" :src="traitIcon(selectedItem(selection))" alt="" /><span v-else class="slot-icon-placeholder" aria-hidden="true">{{ selection?.slotId ? '!' : '＋' }}</span><div v-if="selectedItem(selection)"><b>{{ selectedItem(selection).name }}</b><small>{{ selectedItem(selection).primaryTraitName }} Lv{{ selectedItem(selection).trait1Level }}<template v-if="selectedItem(selection).secondaryTraitName"> · {{ selectedItem(selection).secondaryTraitName }} Lv{{ selectedItem(selection).trait2Level }}</template></small></div><div v-else-if="selection?.slotId"><b>{{ tx('来源实例失效', 'Source instance unavailable') }}</b><small>{{ tx(`SlotID #${selection.slotId} 不在当前来源存档的未装备因子中，请替换或清空。`, `SlotID #${selection.slotId} is not an unequipped sigil in the selected source save. Replace or clear it.`) }}</small></div><div v-else><b>{{ tx('空槽', 'Empty slot') }}</b><small>{{ tx('等待选择因子', 'Choose a sigil') }}</small></div><button v-if="selection?.slotId" type="button" :title="tx('清空', 'Clear')" @click.stop="clearSlot(index)">×</button></article></div></div>

      <div class="inventory-panel">
        <header>
          <div>
            <h3>{{ tx('选择这个槽要读取的因子', 'Choose the Sigil for This Slot') }}</h3>
            <p>{{ tx(`正在配置额外槽 ${activeSlot + 1}`, `Configuring extra slot ${activeSlot + 1}`) }}</p>
          </div>
          <nav class="source-mode-tabs" role="tablist" :aria-label="tx('因子来源', 'Sigil source')">
            <button type="button" role="tab" :aria-selected="sourceMode === 'inventory'" :class="{ active: sourceMode === 'inventory' }" @click="sourceMode = 'inventory'">{{ tx('从背包选择', 'Choose from inventory') }}</button>
            <button type="button" role="tab" :aria-selected="sourceMode === 'construct'" :class="{ active: sourceMode === 'construct' }" @click="sourceMode = 'construct'">{{ tx('制造新因子', 'Create new sigil') }}</button>
          </nav>
        </header>

        <template v-if="sourceMode === 'inventory'">
          <input v-model="search" class="inventory-search ui-input" type="search" :placeholder="tx('搜索因子、主副词条或哈希', 'Search sigil, trait, or hash')" />
          <p class="inventory-count">{{ tx(`${filteredInventory.length} 个可用实例`, `${filteredInventory.length} available instances`) }}</p>
          <div class="inventory-list">
            <button v-for="item in visibleInventory" :key="item.slotId" type="button" :class="{ used: usedSlots.has(Number(item.slotId)), selected: Number(currentSlots[activeSlot]?.slotId) === Number(item.slotId) }" @click="assignItem(item)">
              <img v-if="traitIcon(item)" :src="traitIcon(item)" alt="" />
              <span><b>{{ item.name }}</b><small>{{ item.primaryTraitName }} Lv{{ item.trait1Level }}<template v-if="item.secondaryTraitName"> · {{ item.secondaryTraitName }} Lv{{ item.trait2Level }}</template></small></span>
              <em>{{ usedSlots.has(Number(item.slotId)) ? tx('已占用', 'Used') : `#${item.slotId}` }}</em>
            </button>
          </div>
          <footer v-if="pageCount > 1"><button class="ui-btn is-sm" type="button" :disabled="page === 0" @click="page--">‹</button><span>{{ page + 1 }} / {{ pageCount }}</span><button class="ui-btn is-sm" type="button" :disabled="page + 1 >= pageCount" @click="page++">›</button></footer>
        </template>

        <section v-else class="source-constructor">
          <div class="ui-notice is-info">
            <b>{{ tx('制造后自动放入当前虚拟槽', 'Created sigil is assigned to this virtual slot') }}</b>
            <span>{{ tx('这里会先备份并写入一颗真实、未装备的因子，再回读 SlotID；不是凭空扩展存档的 12 个物理槽。', 'This backs up the save, writes one real unequipped sigil, and reads back its SlotID. It does not expand the save’s 12 physical slots.') }}</span>
          </div>
          <label class="constructor-field constructor-wide">
            <span>{{ tx('因子', 'Sigil') }}</span>
            <CatalogSelect v-model="constructorSigilId" :options="constructorSigils" :disabled="constructorLoading" :icon-resolver="sigilOptionIcon" :placeholder="tx('选择要制造的因子', 'Choose a sigil')" :search-placeholder="tx('搜索因子', 'Search sigils')" />
          </label>
          <label class="constructor-field">
            <span>{{ tx('因子等级', 'Sigil level') }}</span>
            <input v-model.number="constructorSigilLevel" class="ui-input" type="number" min="1" max="2147483647" step="1" @change="constructorSigilLevel = clampConstructorLevel(constructorSigilLevel, 2147483647)" />
          </label>
          <label class="constructor-field">
            <span>{{ tx('主词条等级', 'Primary trait level') }}</span>
            <input v-model.number="constructorPrimaryLevel" class="ui-input" type="number" min="1" max="2147483647" step="1" @change="constructorPrimaryLevel = clampConstructorLevel(constructorPrimaryLevel, 2147483647)" />
          </label>
          <div class="constructor-primary constructor-wide">
            <span>{{ tx('主词条', 'Primary trait') }}</span>
            <b>{{ selectedConstructorPrimary?.displayName || tx('跟随因子目录', 'Defined by sigil catalog') }}</b>
          </div>
          <label class="constructor-field constructor-wide">
            <span>{{ tx('副词条（可选）', 'Secondary trait (optional)') }}</span>
            <CatalogSelect v-model="constructorSecondaryTraitId" :options="constructorSecondaryTraits" :disabled="constructorLoading || !constructorSecondaryTraits.length" :icon-resolver="traitOptionIcon" optional :placeholder="tx('不设置副词条', 'No secondary trait')" :search-placeholder="tx('搜索兼容副词条', 'Search compatible traits')" />
          </label>
          <label v-if="selectedConstructorSecondary" class="constructor-field constructor-wide">
            <span>{{ tx('副词条等级', 'Secondary trait level') }}</span>
            <input v-model.number="constructorSecondaryLevel" class="ui-input" type="number" min="1" max="2147483647" step="1" @change="constructorSecondaryLevel = clampConstructorLevel(constructorSecondaryLevel, 2147483647)" />
          </label>
          <div v-if="workspace?.gameRunning" class="ui-notice is-danger">{{ tx('游戏运行时不能改写默认存档；请完全退出游戏后再制造因子。', 'The managed save cannot be changed while the game is running. Fully exit the game before creating the sigil.') }}</div>
          <button class="ui-btn is-primary constructor-submit" type="button" :disabled="!canCreateSource" @click="createAndAssignSource">{{ constructorLoading || busy ? tx('处理中…', 'Working…') : tx(`制造并放入槽 ${activeSlot + 1}`, `Create and assign to slot ${activeSlot + 1}`) }}</button>
        </section>
      </div>
    </section>

    <section class="trait-ledger"><header><div><h3>{{ tx('这些额外因子会提供什么技能', 'Traits Provided by These Extra Sigils') }}</h3><p>{{ tx('“有效”等级是游戏实际上限内可生效的部分；超过上限的等级单独列在“溢出”。', 'Effective is the portion within the game cap. Levels above that cap are listed separately as Overflow.') }}</p></div></header><div v-if="traitTotals.length" class="trait-grid"><article v-for="trait in traitTotals" :key="trait.hash"><img v-if="traitAssetIcon({ hash: trait.hash, name: trait.name })" :src="traitAssetIcon({ hash: trait.hash, name: trait.name })" alt="" /><span><b>{{ trait.name }}</b><small>{{ trait.hash }}</small></span><dl><div><dt>{{ tx('合计', 'Total') }}</dt><dd>Lv{{ trait.raw }}</dd></div><div><dt>{{ tx('有效', 'Effective') }}</dt><dd>Lv{{ trait.effective }}</dd></div><div :class="{ overflow: trait.overflow }"><dt>{{ tx('溢出', 'Overflow') }}</dt><dd>+{{ trait.overflow }}</dd></div></dl></article></div><p v-else class="empty-state">{{ tx('当前角色还没有配置额外因子。先选择一个槽，再从背包选择因子。', 'No extra sigils are configured for this character. Choose a slot, then select a sigil from the inventory.') }}</p></section>

    <section class="preset-panel"><header><div><h3>{{ tx('角色预设', 'Character presets') }}</h3><p>{{ tx('保存当前角色的整组虚拟槽；预设仍会核对真实实例指纹。', 'Save the current character slot set; presets still verify exact physical instances.') }}</p></div><div><input v-model="presetName" class="ui-input" type="text" maxlength="48" :placeholder="tx('预设名称', 'Preset name')" /><button class="ui-btn is-sm" type="button" @click="savePreset">{{ tx('保存预设', 'Save preset') }}</button></div></header><div v-if="presets.length" class="preset-list"><article v-for="preset in presets" :key="preset.id"><div><b>{{ preset.name }}</b><small>{{ preset.characterHash }} · {{ (preset.slots || []).filter(item => item?.slotId).length }}/{{ slotCount }}</small></div><button class="ui-btn is-sm" type="button" @click="applyPreset(preset)">{{ tx('应用', 'Apply') }}</button><button class="preset-delete" type="button" :title="tx('删除预设', 'Delete preset')" @click="deletePreset(preset.id)">×</button></article></div><p v-else class="empty-state">{{ tx('还没有保存预设。', 'No presets saved yet.') }}</p></section>

    <div v-if="invalidSelections.length" class="ui-notice is-danger"><b>{{ tx(`有 ${invalidSelections.length} 个槽位引用了另一份存档或已不存在的因子实例。`, `${invalidSelections.length} slots reference sigil instances from another save or no longer present.`) }}</b><span>{{ tx('这些槽位已经标红；替换或清空后才能保存，避免把无效实例写进组件配置。', 'The slots are highlighted. Replace or clear them before saving so invalid instances are not written to the component configuration.') }}</span></div>
    <section class="virtual-dock"><div><b>{{ tx('第三步 · 应用到当前游戏', 'Step 3 · Apply to the Current Game') }}</b><small>{{ workspace?.recoveryRequired ? tx('上次恢复尚未完成，请先点“重试恢复”。', 'The previous restoration is incomplete. Select Retry Restoration first.') : workspace?.available === false ? tx('稳定版仅保留配置预览与既有会话恢复；新的运行时会话暂不开放。', 'The stable build keeps configuration preview and recovery for existing sessions; new runtime sessions are not enabled yet.') : tx(`已配置 ${activeCount} 个额外槽；开启前会逐项核对来源存档中的真实实例`, `${activeCount} extra slots configured; every real instance is checked against the source save before activation`) }}</small></div><div class="dock-actions"><button v-if="workspace?.owned" class="ui-btn is-danger" type="button" :disabled="busy" @click="remove">{{ workspace?.recoveryRequired ? tx('重试恢复', 'Retry Restoration') : tx('停用并恢复原技能读取', 'Disable and Restore Original Trait Reading') }}</button><button class="ui-btn is-primary" type="button" :disabled="!canSave" @click="save">{{ busy ? tx('处理中…', 'Working…') : workspace?.available === false ? tx('稳定版暂未开放', 'Not Enabled in Stable Build') : workspace?.recoveryRequired ? tx('需先恢复', 'Restore First') : runtimeActive ? tx('保存并热更新', 'Save and Hot-Update') : tx('开启额外技能槽', 'Enable Extra Trait Slots') }}</button></div></section>
    <div v-if="message" class="ui-notice" :class="{ 'is-danger': tone === 'danger', 'is-ok': tone === 'ok' }" role="status">{{ message }}</div>
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<style scoped>
.virtual-lab { width:100%; max-width:1600px; min-width:0; margin-inline:0 auto; container:virtual-lab / inline-size; overflow-x:clip; padding-bottom:82px; }.virtual-intro,.source-setup,.character-step,.character-strip,.virtual-workspace,.trait-ledger,.preset-panel,.virtual-dock { width:100%; max-width:100%; min-width:0; }
.virtual-intro { display:grid; grid-template-columns:minmax(0,1fr) minmax(250px,330px); align-items:end; gap:var(--space-6); padding:var(--space-6) 0 var(--space-5); border-bottom:1px solid var(--border-default); }.virtual-kicker { margin:0 0 var(--space-2); color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); letter-spacing:.06em; }.virtual-intro h2,.source-setup h3,.slot-panel h3,.inventory-panel h3,.trait-ledger h3,.preset-panel h3 { margin:0; color:var(--text-primary); font-family:var(--font-display); }.virtual-intro h2 { font-size:var(--fs-2xl); }.virtual-intro p,.source-setup p,.slot-panel header p,.inventory-panel header p,.trait-ledger header p,.preset-panel header p { margin:var(--space-2) 0 0; color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }.virtual-boundary { padding:var(--space-4); border-left:3px solid var(--success); background:var(--success-bg); color:var(--success-ink); }.virtual-boundary b,.virtual-boundary span { display:block; }.virtual-boundary span { margin-top:4px; font-size:var(--fs-xs); }
.source-setup { display:grid; grid-template-columns:minmax(0,1fr); gap:var(--space-3); }.character-step h3 { margin:0; color:var(--text-primary); font-size:var(--fs-lg); }.character-step p { margin:3px 0 0; color:var(--text-muted); font-size:var(--fs-xs); }.slot-count { display:grid; grid-template-columns:minmax(148px,1fr) 72px; min-width:250px; align-items:center; gap:var(--space-2); padding-left:var(--space-3); border-left:1px solid var(--border-default); }.slot-count > span { display:block; color:var(--text-primary); font-size:var(--fs-xs); font-weight:var(--fw-bold); line-height:var(--lh-tight); }.slot-count .ui-input { width:72px; text-align:center; }.slot-count small { display:block; margin-top:3px; color:var(--text-muted); font-size:10px; font-weight:var(--fw-normal); }
.character-step { display:grid; gap:var(--space-2); }
.character-strip { display:flex; gap:var(--space-2); padding:var(--space-2); overflow-x:auto; border-block:1px solid var(--border-default); scrollbar-width:thin; scrollbar-color:var(--border-strong) transparent; }.character-strip button { display:grid; grid-template-columns:36px minmax(68px,auto) auto; align-items:center; gap:var(--space-2); min-width:max-content; height:48px; padding:4px var(--space-3) 4px 5px; border:1px solid var(--border-default); border-radius:var(--radius-sm); color:var(--text-secondary); background:var(--surface-card); cursor:pointer; }.character-strip button.active { border-color:var(--accent-hover); color:var(--text-on-accent); background:var(--accent); box-shadow:3px 0 0 color-mix(in srgb,var(--text-on-accent) 72%,transparent) inset; }.character-strip button.active :is(span,small) { color:var(--text-on-accent); }.character-strip img { width:36px; height:36px; border-radius:50%; object-fit:cover; }.character-strip span { font-size:var(--fs-xs); font-weight:var(--fw-semibold); }.character-strip small { color:var(--accent); font-family:var(--font-data); font-size:10px; }
.virtual-workspace { display:grid; grid-template-columns:minmax(0,1fr); gap:var(--space-4); align-items:start; }.slot-panel,.inventory-panel { min-width:0; }.slot-panel > header,.inventory-panel > header,.trait-ledger > header,.preset-panel > header { display:flex; align-items:flex-start; justify-content:space-between; gap:var(--space-3); margin-bottom:var(--space-3); }.slot-panel > header > b { color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xl); }.virtual-slots { display:grid; gap:var(--space-2); }.virtual-slots article { display:grid; grid-template-columns:32px 38px minmax(0,1fr) 28px; align-items:center; gap:var(--space-2); min-width:0; min-height:66px; padding:var(--space-2); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-card); cursor:pointer; }.virtual-slots article.active { border-color:var(--accent-hover); color:var(--text-on-accent); background:var(--accent); box-shadow:3px 0 0 color-mix(in srgb,var(--text-on-accent) 72%,transparent) inset; }.virtual-slots article.active :is(.slot-index,b,small) { color:var(--text-on-accent); }.virtual-slots article:not(.filled) img { opacity:.35; }.slot-index { color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); }.virtual-slots img,.inventory-list img,.trait-grid img { width:38px; height:38px; object-fit:contain; }.virtual-slots article > div,.inventory-list span,.trait-grid span { min-width:0; }.virtual-slots b,.virtual-slots small,.inventory-list b,.inventory-list small,.trait-grid b,.trait-grid small { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.virtual-slots b,.inventory-list b,.trait-grid b { color:var(--text-primary); font-size:var(--fs-xs); }.virtual-slots small,.inventory-list small,.trait-grid small { margin-top:4px; color:var(--text-muted); font-size:10px; }.virtual-slots article > button,.preset-delete { width:28px; height:28px; padding:0; border:1px solid var(--border-default); border-radius:50%; color:var(--danger); background:var(--surface-field); font-size:18px; cursor:pointer; }
.virtual-slots article.invalid { border-color:var(--danger); background:var(--danger-bg); box-shadow:3px 0 0 var(--danger) inset; }.slot-icon-placeholder { width:38px; height:38px; display:grid; place-items:center; border:1px dashed var(--border-strong); border-radius:var(--radius-sm); color:var(--text-muted); background:var(--surface-sunken); font-weight:var(--fw-bold); }.virtual-slots article.invalid .slot-icon-placeholder,.virtual-slots article.invalid b { color:var(--danger-ink); }.virtual-slots article.invalid small { white-space:normal; overflow-wrap:anywhere; }
.source-mode-tabs { display:flex; flex:0 0 auto; gap:var(--space-3); border-bottom:1px solid var(--border-default); }.source-mode-tabs button { flex:0 0 auto; padding:4px 2px 7px; border:0; border-bottom:2px solid transparent; border-radius:0; color:var(--text-muted); background:transparent; box-shadow:none; font-size:var(--fs-xs); font-weight:var(--fw-bold); cursor:pointer; }.source-mode-tabs button.active { border-bottom-color:var(--accent); color:var(--text-primary); }.inventory-search { width:100%; margin-bottom:var(--space-2); }.inventory-count { margin:0 0 var(--space-2); color:var(--text-muted); font-size:var(--fs-xs); }.inventory-list { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-2); max-height:560px; overflow-y:auto; padding-right:3px; scrollbar-width:thin; scrollbar-color:var(--border-strong) transparent; }.inventory-list button { display:grid; grid-template-columns:36px minmax(0,1fr) auto; align-items:center; gap:var(--space-2); min-width:0; min-height:64px; padding:var(--space-2); border:1px solid var(--border-default); border-radius:var(--radius-sm); color:inherit; background:var(--surface-card); text-align:left; cursor:pointer; }.inventory-list button:hover { border-color:var(--accent-border); background:var(--selected-bg); }.inventory-list button.selected { border-color:var(--accent-hover); color:var(--text-on-accent); background:var(--accent); }.inventory-list button.selected :is(b,small,em) { color:var(--text-on-accent); }.inventory-list button.used:not(.selected) { opacity:.68; }.inventory-list em { color:var(--text-muted); font-family:var(--font-data); font-size:9px; font-style:normal; }.inventory-panel footer { display:flex; align-items:center; justify-content:center; gap:var(--space-3); margin-top:var(--space-3); color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); }
.source-constructor { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-3); padding:var(--space-3); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-subtle); }.source-constructor > .ui-notice,.constructor-wide,.constructor-submit { grid-column:1 / -1; }.constructor-field { display:flex; min-width:0; flex-direction:column; gap:6px; }.constructor-field > span,.constructor-primary > span { color:var(--text-secondary); font-size:var(--fs-xs); font-weight:var(--fw-bold); }.constructor-primary { display:flex; align-items:center; justify-content:space-between; gap:var(--space-3); padding:var(--space-3); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-card); }.constructor-primary b { color:var(--text-primary); font-size:var(--fs-sm); }.constructor-submit { min-height:var(--control-height-lg); }
.trait-ledger,.preset-panel { padding:var(--space-4); border:1px solid var(--border-default); border-radius:var(--radius-md); background:color-mix(in srgb,var(--surface-card) 86%,transparent); }.trait-ledger > header > div,.preset-panel > header > div:first-child { max-width:72ch; }.trait-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,420px),1fr)); gap:var(--space-2); }.trait-grid article { display:grid; grid-template-columns:42px minmax(150px,1fr) minmax(210px,.8fr); align-items:center; gap:var(--space-3); min-width:0; padding:var(--space-3); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-card); }.trait-grid article:only-child { grid-column:1/-1; }.trait-grid dl { display:grid; grid-template-columns:repeat(3,minmax(58px,1fr)); gap:var(--space-2); margin:0; }.trait-grid dl div { min-width:0; padding-left:var(--space-2); border-left:1px solid var(--border-default); text-align:left; }.trait-grid dt { color:var(--text-muted); font-size:10px; }.trait-grid dd { margin:3px 0 0; color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); }.trait-grid .overflow dd { color:var(--warning); }
.preset-panel > header > div:last-child { display:grid; grid-template-columns:minmax(180px,1fr) auto; align-items:center; gap:var(--space-2); width:min(100%,430px); padding:var(--space-2); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-subtle); }.preset-panel .ui-input { width:100%; min-width:0; }.preset-panel > header .ui-btn { min-height:var(--control-height); white-space:nowrap; }.preset-list { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,260px),1fr)); gap:var(--space-2); }.preset-list article { display:grid; grid-template-columns:minmax(0,1fr) auto 28px; align-items:center; gap:var(--space-2); padding:var(--space-3); border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-card); }.preset-list article b,.preset-list article small { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.preset-list article b { color:var(--text-primary); font-size:var(--fs-xs); }.preset-list article small { margin-top:3px; color:var(--text-muted); font-family:var(--font-data); font-size:9px; }.empty-state { margin:0; padding:var(--space-4); border:1px dashed var(--border-default); border-radius:var(--radius-sm); color:var(--text-muted); background:var(--surface-subtle); text-align:left; }
.virtual-dock { position:sticky; z-index:10; bottom:0; display:flex; align-items:stretch; flex-direction:column; gap:var(--space-3); min-height:66px; padding:var(--space-3) var(--space-4); border:1px solid var(--border-strong); border-left:4px solid var(--accent); border-radius:var(--radius-md); background:var(--surface-card-pop); box-shadow:var(--shadow-2); }.virtual-dock > div:first-child { min-width:0; }.virtual-dock b,.virtual-dock small { display:block; }.virtual-dock b { color:var(--text-primary); font-size:var(--fs-sm); }.virtual-dock small { margin-top:3px; color:var(--text-muted); font-size:var(--fs-xs); }.dock-actions { display:flex; width:100%; flex-wrap:wrap; justify-content:flex-start; gap:var(--space-2); min-width:0; }.dock-actions .ui-btn { max-width:100%; white-space:normal; }
@container virtual-lab (max-width:1600px) { .inventory-list { max-height:460px; }.preset-list { grid-template-columns:repeat(2,minmax(0,1fr)); } }
@container virtual-lab (max-width:720px) { .virtual-intro,.source-setup { grid-template-columns:minmax(0,1fr); }.slot-panel > header,.inventory-panel > header,.trait-ledger > header,.preset-panel > header,.virtual-dock { align-items:stretch; flex-direction:column; }.source-mode-tabs,.preset-panel > header > div:last-child,.dock-actions,.dock-actions .ui-btn { width:100%; }.trait-grid,.preset-list { grid-template-columns:minmax(0,1fr); }.trait-grid article { grid-template-columns:42px minmax(0,1fr); }.trait-grid dl { grid-column:1/-1; } }
@container virtual-lab (max-width:500px) { .slot-count { min-width:0; padding-left:0; border-left:0; }.inventory-list,.source-constructor { grid-template-columns:minmax(0,1fr); }.source-constructor > .ui-notice,.constructor-wide,.constructor-submit { grid-column:auto; }.preset-panel > header > div:last-child { grid-template-columns:minmax(0,1fr); }.virtual-slots article { grid-template-columns:28px 34px minmax(0,1fr) 28px; }.trait-grid dl { gap:var(--space-2); } }
</style>
