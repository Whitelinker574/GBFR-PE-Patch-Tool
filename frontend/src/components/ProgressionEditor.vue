<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { FindSaveFiles, ProgressionGetCatalog, ProgressionLoad, ProgressionApply, SelectProgressionSave } from '../../wailsjs/go/backend/App'
import { language } from '../i18n'
import { backendLanguageReady } from '../backendLanguage'
import { itemAssetIcon, weaponAssetIcon } from '../gameAssetIcons'
import LegalityIndicator from './LegalityIndicator.vue'
import ConfirmDialog from './ConfirmDialog.vue'

const emit = defineEmits(['status'])
const slots = ref([])
const catalog = ref({ items: [], weapons: [] })
const inventory = ref(null)
const savePath = ref('')
const section = ref('items')
const loading = ref(false)
const applying = ref(false)
const confirmDialog = ref(null)
const search = ref('')
const showExperimental = ref(false)
const itemCategory = ref('全部分类')
const itemSort = ref('name')
const ownedOnly = ref(false)
const selectedItem = ref(null)
const itemQuantity = ref(1)
const itemMode = ref('add')
const allowDangerous = ref(false)
const weaponView = ref('owned')
const weaponOwner = ref(window.localStorage.getItem('gbfr.progression.weaponOwner') || '全部角色')
const weaponSort = ref('owner')
const selectedWeapon = ref(null)
const weaponLevel = ref(150)
const weaponUncap = ref(6)
const weaponMirage = ref(99)
const weaponAwakening = ref(0)
const weaponTranscendence = ref(0)
const weaponTranscendenceSkill = ref('BBD77C33')
const allowWeaponUnlockRisk = ref(false)
const resources = ref({ rupees: 0, mastery: 0, commendations: 0 })
const weaponLevelOptions = Array.from({ length: 150 }, (_, i) => i + 1)
const weaponUncapOptions = [0, 1, 2, 3, 4, 5, 6]
const weaponMirageOptions = Array.from({ length: 100 }, (_, i) => i)
const weaponAwakeningOptions = Array.from({ length: 11 }, (_, i) => i)
const weaponTranscendenceOptions = Array.from({ length: 8 }, (_, i) => i)
const weaponTranscendenceSkills = [
  { hash: 'BBD77C33', name: '超凡强击' },
  { hash: '020DB733', name: '超凡技艺' },
  { hash: '3F682593', name: '超凡奥秘' },
  { hash: '79027FC8', name: '超凡破限' },
]

function itemIcon(item) { return itemAssetIcon(item) }
function weaponIcon(weapon) { return weaponAssetIcon(weapon) }

const collator = computed(() => new Intl.Collator(language.value === 'zh' ? 'zh-CN' : 'en', { numeric: true, sensitivity: 'base' }))
const ownerNames = {
  PL0000:'古兰', PL0100:'姬塔', PL0200:'卡塔莉娜', PL0300:'拉卡姆', PL0400:'伊欧', PL0500:'欧根',
  PL0600:'萝赛塔', PL0700:'菲莉', PL0800:'兰斯洛特', PL0900:'巴恩', PL1000:'珀西瓦尔', PL1100:'齐格飞',
  PL1200:'夏洛特', PL1300:'尤达拉哈', PL1400:'娜露梅', PL1500:'冈达葛萨', PL1600:'塞达', PL1700:'巴萨拉卡',
  PL1800:'卡莉奥丝特罗', PL1900:'伊德', PL2100:'圣德芬', PL2200:'希耶提', PL2300:'索恩', PL2400:'伽兰查',
  PL2500:'玛琪拉菲菈', PL2600:'贝阿朵丽丝', PL2700:'尤斯塔斯', PL2800:'芙劳', PL2900:'菲迪埃尔',
}

function itemCategoryName(category = '') {
  const [nameCN = '', nameEN = ''] = category.split(' / ')
  if (language.value === 'zh') return nameCN.trim() || '其他'
  return nameEN.trim() || 'Other'
}
function itemDisplayName(item = {}) {
  if (language.value === 'zh') return item.nameCn || `未收录物品 ${item.hash || ''}`.trim()
  return item.nameEn || `Uncatalogued Item ${item.hash || ''}`.trim()
}
function weaponIndex(weapon) {
  const matched = String(weapon.internalId || '').match(/_(\d{2})(?:_|$)/)
  return matched ? matched[1] : '特殊'
}
function enrichWeapon(weapon) {
  const catalogWeapon = catalog.value.weapons.find(row => row.hash === weapon.hash || row.hash === weapon.baseHash) || weapon
  const ownerCode = catalogWeapon.ownerCode || ''
  return {
    ...catalogWeapon, ...weapon, ownerCode,
    ownerName: ownerNames[ownerCode] || (ownerCode ? `角色 ${ownerCode.replace('PL', '')}` : '通用武器'),
    displayName: language.value === 'zh'
      ? (catalogWeapon.nameCn || (ownerCode ? `${ownerNames[ownerCode] || '角色武器'} · ${weaponIndex(catalogWeapon)}号武器` : `未收录武器 ${catalogWeapon.hash || ''}`.trim()))
      : (catalogWeapon.name || `Uncatalogued Weapon ${catalogWeapon.hash || ''}`.trim()),
    searchNameCN: catalogWeapon.nameCn || '',
    searchNameEN: catalogWeapon.name || '',
  }
}

const itemCategories = computed(() => ['全部分类', ...new Set(catalog.value.items.map(i => itemCategoryName(i.category)))])
const weaponOwners = computed(() => ['全部角色', ...new Set(catalog.value.weapons.filter(w => !w.catalogHidden).map(w => ownerNames[w.ownerCode] || (w.ownerCode ? `角色 ${w.ownerCode.replace('PL', '')}` : '通用武器')))])

const normalizedSearch = computed(() => search.value.trim().toLowerCase())
const itemResults = computed(() => {
  const needle = normalizedSearch.value
  const ownedHashes = new Set((inventory.value?.items || []).map(i => i.hash))
  return catalog.value.items
    .filter(i => (showExperimental.value || !i.dangerous)
      && (itemCategory.value === '全部分类' || itemCategoryName(i.category) === itemCategory.value)
      && (!ownedOnly.value || ownedHashes.has(i.hash))
      && (!needle || `${i.nameCn} ${i.nameEn} ${i.hash} ${i.category}`.toLowerCase().includes(needle)))
    .sort((a, b) => itemSort.value === 'quantity'
      ? ((inventory.value?.items.find(i => i.hash === b.hash)?.quantity || 0) - (inventory.value?.items.find(i => i.hash === a.hash)?.quantity || 0))
      : itemSort.value === 'category' ? collator.value.compare(itemCategoryName(a.category), itemCategoryName(b.category)) || collator.value.compare(itemDisplayName(a), itemDisplayName(b))
        : collator.value.compare(itemDisplayName(a), itemDisplayName(b)))
    .slice(0, 240)
})
const ownedItems = computed(() => {
  const needle = normalizedSearch.value
  return (inventory.value?.items || []).filter(i => !needle || `${i.name} ${i.hash}`.toLowerCase().includes(needle)).slice(0, 160)
})
const weaponResults = computed(() => {
  const rows = (weaponView.value === 'owned' ? (inventory.value?.weapons || []) : catalog.value.weapons.filter(w => !w.catalogHidden)).map(enrichWeapon)
  const needle = normalizedSearch.value
  return rows
    .filter(w => (weaponOwner.value === '全部角色' || w.ownerName === weaponOwner.value)
      && (!needle || `${w.displayName} ${w.searchNameCN} ${w.searchNameEN} ${w.hash} ${w.internalId || ''} ${w.ownerCode || ''} ${w.ownerName}`.toLowerCase().includes(needle)))
    .sort((a, b) => weaponSort.value === 'level' ? ((b.level || 0) - (a.level || 0))
      : weaponSort.value === 'name' ? collator.value.compare(a.displayName, b.displayName)
        : collator.value.compare(a.ownerName, b.ownerName) || collator.value.compare(weaponIndex(a), weaponIndex(b)))
    .slice(0, 240)
})
const selectedOwnedQuantity = computed(() => selectedItem.value ? inventory.value?.items.find(i => i.hash === selectedItem.value.hash)?.quantity : null)
const ownedWeaponBases = computed(() => new Set((inventory.value?.weapons || []).map(w => w.baseHash || w.hash)))
const selectedWeaponAlreadyOwned = computed(() => weaponView.value === 'catalog' && !!selectedWeapon.value && ownedWeaponBases.value.has(selectedWeapon.value.baseHash || selectedWeapon.value.hash))
const weaponKind = computed(() => {
  const type = selectedWeapon.value?.weaponType
  if (type === 'ascension') return { label: '觉醒武器', detail: '可在游戏内完成觉醒，觉醒等级最高为 10；写入前需要等级 150、上限解锁 6。' }
  if (type === 'terminus') return { label: '巴哈姆特武器', detail: '巴哈姆特武器可觉醒至 10；觉醒与 DLC 超凡突破是两套独立进度。' }
  if (type === 'special') return { label: '特殊觉醒武器', detail: '该武器具有独立觉醒阶段；觉醒阶段和超凡突破均可分别修改。' }
  return { label: '普通武器', detail: '普通武器没有觉醒等级，但仍可具有 DLC 超凡突破进度。' }
})
function weaponTypeLabel(weapon) {
  if (weapon.weaponType === 'ascension') return '觉醒'
  if (weapon.weaponType === 'terminus') return '巴哈姆特'
  if (weapon.weaponType === 'special') return '特殊觉醒'
  return '普通'
}
const weaponUnlockRisk = computed(() => {
  if (!selectedWeapon.value) return ''
  const originalAwakening = weaponView.value === 'owned' ? Number(selectedWeapon.value.awakening || 0) : 0
  const raisesAwakening = Number(weaponAwakening.value) > originalAwakening
  const originalTranscendence = weaponView.value === 'owned' ? Number(selectedWeapon.value.transcendence || 0) : 0
  const raisesTranscendence = Number(weaponTranscendence.value) > originalTranscendence
  if (!raisesAwakening && !raisesTranscendence) return ''
  return '正在提高游戏内尚未完成的武器进度。工具会按 2.0.2 结构同步觉醒 Hash 与超凡字段，但不会替你完成对应任务条件。'
})
const weaponLegality = computed(() => {
  if (!selectedWeapon.value) return { status: 'impossible', writable: false, message: '请先选择武器' }
  if (selectedWeaponAlreadyOwned.value) return { status: 'impossible', writable: false, message: '当前存档已经拥有这把武器，不能重复添加相同武器' }
  const values = [weaponLevel.value, weaponUncap.value, weaponMirage.value, weaponAwakening.value, weaponTranscendence.value].map(Number)
  if (values.some(v => !Number.isInteger(v))) return { status: 'impossible', writable: false, message: '武器养成字段必须是整数' }
  if (weaponLevel.value < 1 || weaponLevel.value > 150) return { status: 'impossible', writable: false, message: '等级必须为 1–150；更高等级没有对应的 2.0.2 经验表，无法可靠写入' }
  if (weaponUncap.value < 0 || weaponUncap.value > 6 || weaponMirage.value < 0 || weaponMirage.value > 99 || weaponAwakening.value < 0 || weaponAwakening.value > 10 || weaponTranscendence.value < 0 || weaponTranscendence.value > 7) {
    return { status: 'impossible', writable: false, message: '上限解锁、幻晶、觉醒或超凡突破超出 2.0.2 可验证范围' }
  }
  if (!selectedWeapon.value.canAwaken && weaponAwakening.value !== 0) return { status: 'impossible', writable: false, message: '普通武器没有觉醒阶段，觉醒等级必须为 0' }
  if (weaponAwakening.value > 0 && (weaponLevel.value !== 150 || weaponUncap.value !== 6)) return { status: 'impossible', writable: false, message: '武器觉醒要求等级 150 且等级上限解锁为 6' }
  if (weaponTranscendence.value > 0 && (weaponLevel.value !== 150 || weaponUncap.value !== 6)) return { status: 'impossible', writable: false, message: '超凡突破要求等级 150 且等级上限解锁为 6' }
  if (weaponUnlockRisk.value) return { status: 'forced', writable: true, message: weaponUnlockRisk.value }
  return { status: 'legal', writable: true, message: '等级、上限解锁、觉醒 Hash 与超凡突破字段均符合 DLC 2.0.2 存档结构' }
})

async function init() {
  try {
    await backendLanguageReady
    const [foundSlots, foundCatalog] = await Promise.all([FindSaveFiles(), ProgressionGetCatalog()])
    slots.value = foundSlots || []
    catalog.value = foundCatalog || { items: [], weapons: [] }
    if (!weaponOwners.value.includes(weaponOwner.value)) weaponOwner.value = '全部角色'
  } catch (err) { emit('status', String(err), 'error') }
}

async function browse() {
  try {
    const path = await SelectProgressionSave()
    if (path) await load(path)
  } catch (err) { emit('status', String(err), 'error') }
}

async function load(path) {
  loading.value = true
  savePath.value = path
  try {
    inventory.value = await ProgressionLoad(path)
    resources.value = { rupees: inventory.value.rupees, mastery: inventory.value.mastery, commendations: inventory.value.commendations }
    selectedItem.value = null
    selectedWeapon.value = null
    emit('status', `已加载 ${inventory.value.items.length} 种物品、${inventory.value.weapons.length} 把武器`, 'success')
  } catch (err) {
    inventory.value = null
    emit('status', String(err), 'error')
  } finally { loading.value = false }
}

function pickItem(item) {
  selectedItem.value = item
  allowDangerous.value = false
  const owned = inventory.value?.items.find(i => i.hash === item.hash)
  itemMode.value = owned ? 'set' : 'add'
  itemQuantity.value = owned?.quantity ?? 1
}

function pickWeapon(weapon) {
  if (weaponView.value === 'catalog' && ownedWeaponBases.value.has(weapon.baseHash || weapon.hash)) {
    emit('status', '当前存档已经拥有这把武器，不能重复添加', 'error')
    return
  }
  selectedWeapon.value = weapon
  const isOwned = weaponView.value === 'owned'
  weaponLevel.value = isOwned ? (weapon.level ?? 1) : 1
  weaponUncap.value = isOwned ? (weapon.uncap ?? 0) : 0
  weaponMirage.value = isOwned ? (weapon.mirage ?? 0) : 0
  weaponAwakening.value = isOwned ? (weapon.awakening ?? 0) : 0
  weaponTranscendence.value = isOwned ? (weapon.transcendence ?? 0) : 0
  weaponTranscendenceSkill.value = isOwned && weapon.transcendenceSkill ? weapon.transcendenceSkill : 'BBD77C33'
  if (!weapon.canAwaken) weaponAwakening.value = 0
  allowWeaponUnlockRisk.value = false
}

function maxResources() { resources.value = { rupees: 999999999, mastery: 999999999, commendations: 999999999 } }
function maxWeapon() { weaponLevel.value = 150; weaponMirage.value = 99 }
function maxWeaponProgression() {
  weaponLevel.value = 150
  weaponUncap.value = 6
  weaponMirage.value = 99
  weaponAwakening.value = selectedWeapon.value?.canAwaken ? 10 : 0
  weaponTranscendence.value = 7
  if (!weaponTranscendenceSkill.value) weaponTranscendenceSkill.value = 'BBD77C33'
}

function clamp(value, min, max) { return Math.max(min, Math.min(max, Number(value) || 0)) }

async function apply(resourceChanges, itemChanges, weaponChanges, message) {
  if (!savePath.value) return
  const confirmed = await confirmDialog.value?.ask({
    title: '写入存档前确认',
    message,
    detail: '请确认游戏已经完全退出。工具会先自动备份原存档，再写入并回读验证。',
    tone: 'warning',
    confirmLabel: '备份并写入',
  })
  if (!confirmed) return
  applying.value = true
  try {
    const result = await ProgressionApply(savePath.value, '', resourceChanges, itemChanges, weaponChanges)
    emit('status', `写入成功，已验证 ${result.verifiedChanges} 项并自动备份`, 'success')
    await load(savePath.value)
  } catch (err) { emit('status', String(err), 'error') }
  finally { applying.value = false }
}

function saveResources() {
  const changes = [
    { kind: 'rupees', value: clamp(resources.value.rupees, 0, 999999999) },
    { kind: 'mastery', value: clamp(resources.value.mastery, 0, 999999999) },
    { kind: 'commendations', value: clamp(resources.value.commendations, 0, 999999999) },
  ]
  apply(changes, [], [], '修改金币、强化点数与达成章')
}

function saveItem() {
  if (!selectedItem.value) return
  const qty = clamp(itemQuantity.value, 0, 999)
  apply([], [{ hash: selectedItem.value.hash, quantity: qty, mode: itemMode.value, allowDangerous: allowDangerous.value }], [], `${itemMode.value === 'add' ? '添加' : '设置'} ${itemDisplayName(selectedItem.value)} × ${qty}`)
}

function saveWeapon() {
  if (!selectedWeapon.value) return
  if (!weaponLegality.value.writable || (weaponUnlockRisk.value && !allowWeaponUnlockRisk.value)) {
    emit('status', weaponUnlockRisk.value || weaponLegality.value.message, 'error')
    return
  }
  const isOwned = weaponView.value === 'owned'
  const change = {
    action: isOwned ? 'update' : 'add',
    unitId: isOwned ? selectedWeapon.value.unitId : 0,
    hash: selectedWeapon.value.hash,
    level: Number(weaponLevel.value),
    uncap: Number(weaponUncap.value),
    mirage: Number(weaponMirage.value),
    awakening: Number(weaponAwakening.value),
    transcendence: Number(weaponTranscendence.value),
    transcendenceSkill: Number(weaponTranscendence.value) >= 7 ? weaponTranscendenceSkill.value : '',
  }
  apply([], [], [change], `${isOwned ? '修改' : '添加'}武器 ${selectedWeapon.value.displayName}`)
}

function switchSection(value) { section.value = value; search.value = ''; selectedItem.value = null; selectedWeapon.value = null }
function switchWeaponView(value) { weaponView.value = value; selectedWeapon.value = null; allowWeaponUnlockRisk.value = false }

function saveSlotLabel(slot) {
  const fileName = String(slot?.name || slot?.path || '').split(/[\\/]/).pop()
  const match = fileName.match(/SaveData\d+/i)
  return match ? match[0].replace(/^savedata/i, 'SaveData') : fileName.replace(/\.dat$/i, '')
}

onMounted(init)
watch(weaponOwner, value => window.localStorage.setItem('gbfr.progression.weaponOwner', value))
</script>

<template>
  <div class="root">
    <section class="save-card ui-card compact-save-bar">
      <div class="save-title">
        <div><strong>2.0.2 养成编辑（离线）</strong><small>用于添加具体物品、素材和武器；请完全退出游戏后修改存档</small></div>
        <span v-if="inventory" class="capacity">物品空位 {{ inventory.emptyItems }} · 武器空位 {{ inventory.emptyWeapons }}</span>
      </div>
      <div class="slots">
        <button v-for="slot in slots" :key="slot.index" class="slot-btn ui-btn is-sm" :class="{ on: savePath === slot.path }" @click="load(slot.path)">{{ saveSlotLabel(slot) }}</button>
        <button class="plain-btn ui-btn is-sm" @click="browse">浏览…</button>
        <button class="plain-btn ui-btn is-sm" :disabled="!savePath" @click="load(savePath)">刷新</button>
      </div>
      <div v-if="savePath" class="path" :title="savePath">{{ savePath }}</div>
    </section>

    <div class="section-tabs ui-seg">
      <button class="ui-seg-btn" :class="{ on: section === 'items', 'is-on': section === 'items' }" @click="switchSection('items')">物品</button>
      <button class="ui-seg-btn" :class="{ on: section === 'weapons', 'is-on': section === 'weapons' }" @click="switchSection('weapons')">武器养成</button>
      <button class="ui-seg-btn" :class="{ on: section === 'resources', 'is-on': section === 'resources' }" @click="switchSection('resources')">资源（离线备用）</button>
    </div>

    <div v-if="loading" class="empty ui-empty">正在解析存档…</div>
    <div v-else-if="!inventory" class="empty ui-empty">先选择一个存档。修改前请完全退出游戏。</div>

    <section v-else-if="section === 'resources'" class="editor-card resource-grid ui-card ui-panel">
      <p class="offline-note">游戏运行中请优先使用“养成工坊 → 旅途实时助手”；这里只用于游戏已退出时的存档补救。</p>
      <label><span>金币</span><span class="number-combo"><input v-model.number="resources.rupees" class="ui-input" type="number" min="0" max="999999999"><button class="ui-btn is-sm" @click="resources.rupees=999999999">最大</button></span></label>
      <label><span>强化点数 MSP</span><span class="number-combo"><input v-model.number="resources.mastery" class="ui-input" type="number" min="0" max="999999999"><button class="ui-btn is-sm" @click="resources.mastery=999999999">最大</button></span></label>
      <label><span>达成章</span><span class="number-combo"><input v-model.number="resources.commendations" class="ui-input" type="number" min="0" max="999999999"><button class="ui-btn is-sm" @click="resources.commendations=999999999">最大</button></span></label>
      <button class="plain-btn wide subtle ui-btn is-subtle" @click="maxResources">三项全部设为最大</button>
      <button class="primary-btn wide ui-btn is-primary" :disabled="applying" @click="saveResources">{{ applying ? '写入并验证中…' : '保存三项资源' }}</button>
    </section>

    <template v-else-if="section === 'items'">
      <section class="editor-card toolbar ui-card">
        <input v-model="search" class="search ui-input" placeholder="搜索中文、英文、Hash 或分类">
        <select v-model="itemCategory" class="ui-select" aria-label="物品分类"><option v-for="category in itemCategories" :key="category">{{ category }}</option></select>
        <select v-model="itemSort" class="ui-select" aria-label="物品排序"><option value="name">按名称排序</option><option value="category">按分类排序</option><option value="quantity">按持有数量</option></select>
        <label class="owned-toggle"><input v-model="ownedOnly" type="checkbox"> 仅已拥有</label>
        <label class="experimental"><input v-model="showExperimental" type="checkbox"> 显示 8 个疑似坏档条目</label>
        <span class="result-count">显示 {{ itemResults.length }} 项</span>
      </section>
      <div class="workspace">
        <div class="catalog-list ui-card">
          <button v-for="item in itemResults" :key="item.hash" class="catalog-row ui-row" :class="{ on: selectedItem?.hash === item.hash, danger: item.dangerous }" @click="pickItem(item)">
            <img v-if="itemIcon(item)" class="catalog-asset-icon item-icon" :src="itemIcon(item)" alt="" />
            <span><strong>{{ itemDisplayName(item) }}</strong><small>{{ item.hash }}</small></span>
            <em v-if="inventory.items.some(i => i.hash === item.hash)">已有 {{ inventory.items.find(i => i.hash === item.hash).quantity }}</em>
          </button>
          <div v-if="!itemResults.length" class="empty small ui-empty">没有匹配的安全条目</div>
        </div>
        <aside class="detail-panel ui-card ui-panel">
          <template v-if="selectedItem">
            <img v-if="itemIcon(selectedItem)" class="detail-asset-icon item-icon" :src="itemIcon(selectedItem)" alt="" />
            <h3>{{ itemDisplayName(selectedItem) }}</h3>
            <p>{{ itemCategoryName(selectedItem.category) }}</p><code>{{ selectedItem.hash }}</code>
            <label class="field ui-field"><span class="ui-field-label">操作</span><select v-model="itemMode" class="ui-select"><option value="add">在现有数量上增加</option><option value="set">设置为指定数量</option></select></label>
            <label class="field ui-field"><span class="ui-field-label">数量（0–999）</span><span class="number-combo"><input v-model.number="itemQuantity" class="ui-input" type="number" min="0" max="999"><button class="ui-btn is-sm" @click="itemQuantity=999">最大</button></span></label>
            <p v-if="selectedOwnedQuantity !== null" class="current">当前数量：{{ selectedOwnedQuantity }}</p>
            <label v-if="selectedItem.dangerous" class="danger-confirm"><input v-model="allowDangerous" type="checkbox"> 我知道此条目来源表标记为可能坏档，仍要实验写入</label>
            <button class="primary-btn ui-btn is-primary" :disabled="applying || (selectedItem.dangerous && !allowDangerous)" @click="saveItem">{{ applying ? '写入并验证中…' : '应用物品修改' }}</button>
          </template>
          <div v-else class="empty small ui-empty">从左侧选择物品</div>
        </aside>
      </div>
    </template>

    <template v-else-if="section === 'weapons'">
      <section class="editor-card toolbar ui-card">
        <div class="mini-tabs ui-seg"><button class="ui-seg-btn" :class="{ on: weaponView === 'owned', 'is-on': weaponView === 'owned' }" @click="switchWeaponView('owned')">修改已有</button><button class="ui-seg-btn" :class="{ on: weaponView === 'catalog', 'is-on': weaponView === 'catalog' }" @click="switchWeaponView('catalog')">添加武器</button></div>
        <input v-model="search" class="search ui-input" placeholder="搜索中文角色、英文武器名、编号或 Hash">
        <select v-model="weaponOwner" class="ui-select" aria-label="所属角色"><option v-for="owner in weaponOwners" :key="owner">{{ owner }}</option></select>
        <select v-model="weaponSort" class="ui-select" aria-label="武器排序"><option value="owner">按角色与编号</option><option value="name">按名称排序</option><option v-if="weaponView==='owned'" value="level">按等级从高到低</option></select>
        <span class="result-count">显示 {{ weaponResults.length }} 把</span>
      </section>
      <div class="workspace">
        <div class="catalog-list ui-card">
          <button v-for="weapon in weaponResults" :key="weaponView === 'owned' ? weapon.unitId : weapon.hash" class="catalog-row ui-row" :class="{ on: selectedWeapon === weapon, owned: weaponView === 'catalog' && ownedWeaponBases.has(weapon.baseHash || weapon.hash) }" :disabled="weaponView === 'catalog' && ownedWeaponBases.has(weapon.baseHash || weapon.hash)" @click="pickWeapon(weapon)">
            <img v-if="weaponIcon(weapon)" class="catalog-asset-icon weapon-icon" :src="weaponIcon(weapon)" alt="" />
            <span><span class="weapon-name-line"><strong>{{ weapon.displayName }}</strong><i class="weapon-type" :data-type="weapon.weaponType">{{ weaponTypeLabel(weapon) }}</i></span><small>{{ weapon.hash }}<template v-if="weapon.internalId"> · {{ weapon.internalId }}</template></small></span>
            <em v-if="weaponView === 'owned'" class="weapon-progress">Lv.{{ weapon.level }}<small v-if="weapon.canAwaken">觉醒 {{ weapon.awakening }}</small><small>超凡 {{ weapon.transcendence || 0 }}</small></em>
            <em v-else-if="ownedWeaponBases.has(weapon.baseHash || weapon.hash)">已拥有</em>
            <em v-else>{{ weapon.ownerName }}</em>
          </button>
        </div>
        <aside class="detail-panel ui-card ui-panel">
          <template v-if="selectedWeapon">
            <img v-if="weaponIcon(selectedWeapon)" class="detail-asset-icon weapon-icon" :src="weaponIcon(selectedWeapon)" alt="" />
            <h3>{{ selectedWeapon.displayName }}</h3><code>{{ selectedWeapon.hash }}</code>
            <div class="weapon-kind"><b>{{ weaponKind.label }}</b><span>{{ weaponKind.detail }}</span></div>
            <div class="weapon-fields">
              <label class="field ui-field"><span class="ui-field-label">等级</span><select v-model.number="weaponLevel" class="ui-select"><option v-for="value in weaponLevelOptions" :key="value" :value="value">{{ value }}</option></select></label>
              <label class="field ui-field"><span class="ui-field-label">等级上限解锁</span><select v-model.number="weaponUncap" class="ui-select"><option v-for="value in weaponUncapOptions" :key="value" :value="value">{{ value }} / 6{{ value === 6 ? '（可升至 150）' : '' }}</option></select></label>
              <label class="field ui-field"><span class="ui-field-label">幻晶</span><select v-model.number="weaponMirage" class="ui-select"><option v-for="value in weaponMirageOptions" :key="value" :value="value">{{ value }}</option></select></label>
              <label class="field ui-field"><span class="ui-field-label">武器觉醒</span><select v-model.number="weaponAwakening" class="ui-select" :disabled="!selectedWeapon.canAwaken"><option v-for="value in weaponAwakeningOptions" :key="value" :value="value">{{ value }} / 10</option></select></label>
              <label class="field ui-field"><span class="ui-field-label">DLC 超凡突破</span><select v-model.number="weaponTranscendence" class="ui-select"><option v-for="value in weaponTranscendenceOptions" :key="value" :value="value">{{ value }} / 7</option></select></label>
              <label v-if="weaponTranscendence >= 7" class="field ui-field"><span class="ui-field-label">第 7 阶超凡效果</span><select v-model="weaponTranscendenceSkill" class="ui-select"><option v-for="skill in weaponTranscendenceSkills" :key="skill.hash" :value="skill.hash">{{ skill.name }}</option></select></label>
            </div>
            <div class="weapon-actions"><button class="plain-btn subtle ui-btn is-subtle" @click="maxWeapon">等级与幻晶最大</button><button class="plain-btn subtle ui-btn is-subtle" @click="maxWeaponProgression">全部养成最大</button></div>
            <p class="current">“等级上限解锁 6”“武器觉醒 10”“DLC 超凡突破 7”是三个独立字段。觉醒跨阶段时会同步切换 2.0.2 对应武器 Hash；超凡 7 阶效果会写入独立效果槽。</p>
            <LegalityIndicator :status="weaponLegality.status" :message="weaponLegality.message" />
            <label v-if="weaponUnlockRisk" class="unlock-risk"><input v-model="allowWeaponUnlockRisk" type="checkbox"> 我已确认游戏内任务/觉醒解锁状态，仍按当前数值写入</label>
            <button class="primary-btn ui-btn is-primary" :disabled="applying || !weaponLegality.writable || (!!weaponUnlockRisk && !allowWeaponUnlockRisk)" @click="saveWeapon">{{ applying ? '写入并验证中…' : weaponView === 'owned' ? '保存武器养成' : '添加这把武器' }}</button>
          </template>
          <div v-else class="empty small ui-empty">从左侧选择武器</div>
        </aside>
      </div>
    </template>
  </div>
  <ConfirmDialog ref="confirmDialog" />
</template>

<style scoped>
.root {
  width:100%;
  max-width:1000px;
  display:flex;
  flex-direction:column;
  gap:var(--space-5);
  color:var(--text-secondary);
  container-type:inline-size;
}

.compact-save-bar {
  display:flex;
  flex-direction:column;
  gap:var(--space-3);
  padding:var(--space-4) var(--space-5);
}
.save-title { display:flex; align-items:flex-start; justify-content:space-between; gap:var(--space-5); min-width:0; }
.save-title > div { min-width:0; }
.save-title strong { display:block; color:var(--text-primary); font-size:var(--fs-md); font-weight:var(--fw-semibold); }
.save-title small { display:block; margin-top:var(--space-1); color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }
.capacity { flex:0 0 auto; color:var(--success-ink); font-size:var(--fs-sm); white-space:nowrap; }
.slots { display:flex; flex-wrap:wrap; align-items:center; gap:var(--space-2); }
.slot-btn.on { border-color:var(--selected-border); background:var(--selected-bg); color:var(--selected-fg); }
.path {
  overflow:hidden;
  color:var(--text-secondary);
  font-family:var(--font-data);
  font-size:var(--fs-xs);
  font-variant-numeric:tabular-nums lining-nums;
  text-overflow:ellipsis;
  white-space:nowrap;
}

.section-tabs { align-self:flex-start; }
.editor-card { min-width:0; }
.toolbar {
  display:grid;
  grid-template-columns:minmax(220px,1fr) auto auto auto;
  align-items:center;
  gap:var(--space-3);
  padding:var(--space-4);
}
.toolbar:has(.mini-tabs) { grid-template-columns:auto minmax(220px,1fr) auto auto auto; }
.toolbar .search { min-width:0; width:100%; }
.toolbar .ui-select { min-width:128px; }
.owned-toggle,
.experimental {
  display:flex;
  align-items:center;
  gap:var(--space-2);
  min-height:var(--control-height-sm);
  color:var(--text-secondary);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
  white-space:nowrap;
}
.experimental { color:var(--warning-ink); }
.result-count {
  grid-column:1 / -1;
  color:var(--text-secondary);
  font-size:var(--fs-sm);
  text-align:right;
}

.workspace {
  display:grid;
  grid-template-columns:minmax(0,1.6fr) minmax(300px,.9fr);
  gap:var(--space-5);
  min-height:360px;
}
.catalog-list {
  min-width:0;
  max-height:500px;
  overflow-y:auto;
  padding:var(--space-2);
}
.catalog-row {
  width:100%;
  min-height:54px;
  justify-content:space-between;
  border-color:transparent;
  border-bottom-color:var(--border-soft);
  background:transparent;
  color:var(--text-secondary);
  text-align:left;
  cursor:pointer;
}
.catalog-row.on {
  border-color:var(--selected-border);
  background:var(--surface-row);
  box-shadow:3px 0 0 var(--selected-bar) inset;
}
.catalog-row.danger { color:var(--warning-ink); }
.catalog-row.owned { opacity:var(--state-disabled-opacity); cursor:not-allowed; }
.catalog-asset-icon { width:44px; height:44px; flex:0 0 44px; border:1px solid var(--line-soft); border-radius:7px; background:var(--surface-field); object-fit:contain; }
.catalog-asset-icon.weapon-icon { width:64px; flex-basis:64px; }
.catalog-row strong { display:block; color:var(--text-primary); font-size:var(--fs-md); font-weight:var(--fw-semibold); }
.catalog-row small {
  display:block;
  margin-top:var(--space-1);
  color:var(--text-secondary);
  font-family:var(--font-data);
  font-size:var(--fs-xs);
  font-variant-numeric:tabular-nums lining-nums;
}
.catalog-row em {
  flex:0 0 auto;
  color:var(--info-ink);
  font-size:var(--fs-sm);
  font-style:normal;
  white-space:nowrap;
}

.detail-panel {
  position:sticky;
  top:0;
  align-self:start;
  min-width:0;
  gap:var(--space-4);
  padding:var(--space-6);
}
.detail-asset-icon { width:112px; height:84px; float:right; margin:0 0 var(--space-3) var(--space-3); border:1px solid var(--line-soft); border-radius:9px; background:var(--surface-field); object-fit:contain; }
.detail-asset-icon.item-icon { width:72px; height:72px; }
.detail-panel h3 { margin:0; color:var(--text-primary); font-size:var(--fs-lg); }
.detail-panel p { margin:0; color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }
.detail-panel code { color:var(--info-ink); font-family:var(--font-data); font-size:var(--fs-sm); overflow-wrap:anywhere; }
.field { min-width:0; }
.field .ui-input,
.field .ui-select { width:100%; }
.weapon-fields { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-3); }
.current { color:var(--info-ink); }
.danger-confirm,
.unlock-risk {
  display:flex;
  align-items:flex-start;
  gap:var(--space-2);
  padding:var(--space-3);
  border:1px solid var(--warning);
  border-radius:var(--radius-sm);
  background:var(--warning-bg);
  color:var(--warning-ink);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
}

.number-combo { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:var(--space-2); }
.number-combo .ui-input { min-width:0; }
.resource-grid {
  display:grid;
  grid-template-columns:repeat(3,minmax(0,1fr));
  gap:var(--space-5);
  padding:var(--space-6);
}
.resource-grid label { display:flex; min-width:0; flex-direction:column; gap:var(--space-2); color:var(--text-secondary); font-size:var(--fs-sm); }
.offline-note {
  grid-column:1 / -1;
  margin:0;
  padding:var(--space-3) var(--space-4);
  border-left:3px solid var(--warning);
  border-radius:var(--radius-sm);
  background:var(--warning-bg);
  color:var(--warning-ink);
  font-size:var(--fs-sm);
  line-height:var(--lh-normal);
}
.wide { grid-column:1 / -1; width:100%; }

.weapon-kind {
  display:flex;
  flex-direction:column;
  gap:var(--space-1);
  padding:var(--space-3) var(--space-4);
  border-left:3px solid var(--accent);
  border-radius:var(--radius-sm);
  background:var(--accent-soft);
}
.weapon-kind b { color:var(--accent-hover); font-size:var(--fs-sm); }
.weapon-kind span { color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }
.weapon-name-line { display:flex; min-width:0; align-items:center; gap:var(--space-2); }
.weapon-name-line strong { min-width:0; }
.weapon-type {
  flex:0 0 auto;
  padding:1px var(--space-2);
  border:1px solid var(--border-strong);
  border-radius:var(--radius-pill);
  background:var(--accent-soft);
  color:var(--accent-hover);
  font-size:var(--fs-xs);
  font-style:normal;
  line-height:var(--lh-tight);
}
.weapon-progress {
  display:flex;
  flex-direction:column;
  align-items:flex-end;
  gap:var(--space-1);
  color:var(--accent-hover);
  font-family:var(--font-data);
  font-variant-numeric:tabular-nums;
}
.weapon-progress small { color:var(--text-secondary); font-size:var(--fs-xs); }
.weapon-actions { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-2); }
.weapon-actions button { width:100%; }

.empty.small { padding:var(--space-7) var(--space-3); }

@container (max-width:760px) {
  .workspace { grid-template-columns:1fr; }
  .detail-panel { position:static; }
  .resource-grid { grid-template-columns:1fr; }
  .wide { grid-column:auto; }
}

@container (max-width:680px) {
  .save-title { flex-direction:column; gap:var(--space-2); }
  .capacity { white-space:normal; }
  .section-tabs { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); width:100%; }
  .section-tabs .ui-seg-btn { min-width:0; padding-inline:var(--space-2); white-space:normal; }
  .toolbar,
  .toolbar:has(.mini-tabs) { grid-template-columns:repeat(2,minmax(0,1fr)); align-items:stretch; }
  .toolbar > * { min-width:0; max-width:100%; }
  .toolbar .mini-tabs,
  .toolbar .search,
  .toolbar .result-count { grid-column:1 / -1; }
  .toolbar .mini-tabs { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); }
  .toolbar .ui-select { width:100%; min-width:0; }
  .owned-toggle,
  .experimental { white-space:normal; }
}

@container (max-width:460px) {
  .compact-save-bar,
  .resource-grid,
  .detail-panel { padding:var(--space-4); }
  .section-tabs { grid-template-columns:1fr; }
  .toolbar,
  .toolbar:has(.mini-tabs) { grid-template-columns:1fr; }
  .toolbar > *,
  .toolbar .mini-tabs,
  .toolbar .search,
  .toolbar .result-count { grid-column:1; }
  .weapon-fields,
  .weapon-actions { grid-template-columns:1fr; }
  .catalog-row { align-items:flex-start; }
}
</style>
