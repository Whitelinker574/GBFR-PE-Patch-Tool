<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { FindSaveFiles, ProgressionGetCatalog, ProgressionLoad, ProgressionApply, SelectProgressionSave } from '../../wailsjs/go/main/App'
import { language } from '../i18n'
import { backendLanguageReady } from '../backendLanguage'
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

const collator = computed(() => new Intl.Collator(language.value === 'zh' ? 'zh-CN' : 'en', { numeric: true, sensitivity: 'base' }))
const ownerNames = {
  PL0000:'古兰', PL0100:'姬塔', PL0200:'卡塔莉娜', PL0300:'拉卡姆', PL0400:'伊欧', PL0500:'欧根',
  PL0600:'萝赛塔', PL0700:'菲莉', PL0800:'兰斯洛特', PL0900:'巴恩', PL1000:'珀西瓦尔', PL1100:'齐格飞',
  PL1200:'夏洛特', PL1300:'尤达拉哈', PL1400:'娜露梅', PL1500:'冈达葛萨', PL1600:'塞达', PL1700:'巴萨拉卡',
  PL1800:'卡莉奥丝特罗', PL1900:'伊德', PL2100:'圣德芬', PL2200:'希耶提', PL2300:'索恩', PL2400:'伽兰查',
  PL2500:'玛琪拉菲菈', PL2600:'贝阿朵丽丝', PL2700:'尤斯提斯', PL2800:'芙劳', PL2900:'菲迪埃尔',
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
    <section class="save-card">
      <div class="save-title">
        <div><strong>2.0.2 养成编辑（离线）</strong><small>用于添加具体物品、素材和武器；请完全退出游戏后修改存档</small></div>
        <span v-if="inventory" class="capacity">物品空位 {{ inventory.emptyItems }} · 武器空位 {{ inventory.emptyWeapons }}</span>
      </div>
      <div class="slots">
        <button v-for="slot in slots" :key="slot.index" class="slot-btn" :class="{ on: savePath === slot.path }" @click="load(slot.path)">{{ saveSlotLabel(slot) }}</button>
        <button class="plain-btn" @click="browse">浏览…</button>
        <button class="plain-btn" :disabled="!savePath" @click="load(savePath)">刷新</button>
      </div>
      <div v-if="savePath" class="path" :title="savePath">{{ savePath }}</div>
    </section>

    <div class="section-tabs">
      <button :class="{ on: section === 'items' }" @click="switchSection('items')">物品</button>
      <button :class="{ on: section === 'weapons' }" @click="switchSection('weapons')">武器养成</button>
      <button :class="{ on: section === 'resources' }" @click="switchSection('resources')">资源（离线备用）</button>
    </div>

    <div v-if="loading" class="empty">正在解析存档…</div>
    <div v-else-if="!inventory" class="empty">先选择一个存档。修改前请完全退出游戏。</div>

    <section v-else-if="section === 'resources'" class="editor-card resource-grid">
      <p class="offline-note">游戏运行中请优先使用“养成工坊 → 旅途实时助手”；这里只用于游戏已退出时的存档补救。</p>
      <label><span>金币</span><span class="number-combo"><input v-model.number="resources.rupees" type="number" min="0" max="999999999"><button @click="resources.rupees=999999999">最大</button></span></label>
      <label><span>强化点数 MSP</span><span class="number-combo"><input v-model.number="resources.mastery" type="number" min="0" max="999999999"><button @click="resources.mastery=999999999">最大</button></span></label>
      <label><span>达成章</span><span class="number-combo"><input v-model.number="resources.commendations" type="number" min="0" max="999999999"><button @click="resources.commendations=999999999">最大</button></span></label>
      <button class="plain-btn wide subtle" @click="maxResources">三项全部设为最大</button>
      <button class="primary-btn wide" :disabled="applying" @click="saveResources">{{ applying ? '写入并验证中…' : '保存三项资源' }}</button>
    </section>

    <template v-else-if="section === 'items'">
      <section class="editor-card toolbar">
        <input v-model="search" class="search" placeholder="搜索中文、英文、Hash 或分类">
        <select v-model="itemCategory" aria-label="物品分类"><option v-for="category in itemCategories" :key="category">{{ category }}</option></select>
        <select v-model="itemSort" aria-label="物品排序"><option value="name">按名称排序</option><option value="category">按分类排序</option><option value="quantity">按持有数量</option></select>
        <label class="owned-toggle"><input v-model="ownedOnly" type="checkbox"> 仅已拥有</label>
        <label class="experimental"><input v-model="showExperimental" type="checkbox"> 显示 8 个疑似坏档条目</label>
        <span class="result-count">显示 {{ itemResults.length }} 项</span>
      </section>
      <div class="workspace">
        <div class="catalog-list">
          <button v-for="item in itemResults" :key="item.hash" class="catalog-row" :class="{ on: selectedItem?.hash === item.hash, danger: item.dangerous }" @click="pickItem(item)">
            <span><strong>{{ itemDisplayName(item) }}</strong><small>{{ item.hash }}</small></span>
            <em v-if="inventory.items.some(i => i.hash === item.hash)">已有 {{ inventory.items.find(i => i.hash === item.hash).quantity }}</em>
          </button>
          <div v-if="!itemResults.length" class="empty small">没有匹配的安全条目</div>
        </div>
        <aside class="detail-panel">
          <template v-if="selectedItem">
            <h3>{{ itemDisplayName(selectedItem) }}</h3>
            <p>{{ itemCategoryName(selectedItem.category) }}</p><code>{{ selectedItem.hash }}</code>
            <label class="field"><span>操作</span><select v-model="itemMode"><option value="add">在现有数量上增加</option><option value="set">设置为指定数量</option></select></label>
            <label class="field"><span>数量（0–999）</span><span class="number-combo"><input v-model.number="itemQuantity" type="number" min="0" max="999"><button @click="itemQuantity=999">最大</button></span></label>
            <p v-if="selectedOwnedQuantity !== null" class="current">当前数量：{{ selectedOwnedQuantity }}</p>
            <label v-if="selectedItem.dangerous" class="danger-confirm"><input v-model="allowDangerous" type="checkbox"> 我知道此条目来源表标记为可能坏档，仍要实验写入</label>
            <button class="primary-btn" :disabled="applying || (selectedItem.dangerous && !allowDangerous)" @click="saveItem">{{ applying ? '写入并验证中…' : '应用物品修改' }}</button>
          </template>
          <div v-else class="empty small">从左侧选择物品</div>
        </aside>
      </div>
    </template>

    <template v-else-if="section === 'weapons'">
      <section class="editor-card toolbar">
        <div class="mini-tabs"><button :class="{ on: weaponView === 'owned' }" @click="switchWeaponView('owned')">修改已有</button><button :class="{ on: weaponView === 'catalog' }" @click="switchWeaponView('catalog')">添加武器</button></div>
        <input v-model="search" class="search" placeholder="搜索中文角色、英文武器名、编号或 Hash">
        <select v-model="weaponOwner" aria-label="所属角色"><option v-for="owner in weaponOwners" :key="owner">{{ owner }}</option></select>
        <select v-model="weaponSort" aria-label="武器排序"><option value="owner">按角色与编号</option><option value="name">按名称排序</option><option v-if="weaponView==='owned'" value="level">按等级从高到低</option></select>
        <span class="result-count">显示 {{ weaponResults.length }} 把</span>
      </section>
      <div class="workspace">
        <div class="catalog-list">
          <button v-for="weapon in weaponResults" :key="weaponView === 'owned' ? weapon.unitId : weapon.hash" class="catalog-row" :class="{ on: selectedWeapon === weapon, owned: weaponView === 'catalog' && ownedWeaponBases.has(weapon.baseHash || weapon.hash) }" :disabled="weaponView === 'catalog' && ownedWeaponBases.has(weapon.baseHash || weapon.hash)" @click="pickWeapon(weapon)">
            <span><span class="weapon-name-line"><strong>{{ weapon.displayName }}</strong><i class="weapon-type" :data-type="weapon.weaponType">{{ weaponTypeLabel(weapon) }}</i></span><small>{{ weapon.hash }}<template v-if="weapon.internalId"> · {{ weapon.internalId }}</template></small></span>
            <em v-if="weaponView === 'owned'" class="weapon-progress">Lv.{{ weapon.level }}<small v-if="weapon.canAwaken">觉醒 {{ weapon.awakening }}</small><small>超凡 {{ weapon.transcendence || 0 }}</small></em>
            <em v-else-if="ownedWeaponBases.has(weapon.baseHash || weapon.hash)">已拥有</em>
            <em v-else>{{ weapon.ownerName }}</em>
          </button>
        </div>
        <aside class="detail-panel">
          <template v-if="selectedWeapon">
            <h3>{{ selectedWeapon.displayName }}</h3><code>{{ selectedWeapon.hash }}</code>
            <div class="weapon-kind"><b>{{ weaponKind.label }}</b><span>{{ weaponKind.detail }}</span></div>
            <div class="weapon-fields">
              <label class="field"><span>等级</span><select v-model.number="weaponLevel"><option v-for="value in weaponLevelOptions" :key="value" :value="value">{{ value }}</option></select></label>
              <label class="field"><span>等级上限解锁</span><select v-model.number="weaponUncap"><option v-for="value in weaponUncapOptions" :key="value" :value="value">{{ value }} / 6{{ value === 6 ? '（可升至 150）' : '' }}</option></select></label>
              <label class="field"><span>幻晶</span><select v-model.number="weaponMirage"><option v-for="value in weaponMirageOptions" :key="value" :value="value">{{ value }}</option></select></label>
              <label class="field"><span>武器觉醒</span><select v-model.number="weaponAwakening" :disabled="!selectedWeapon.canAwaken"><option v-for="value in weaponAwakeningOptions" :key="value" :value="value">{{ value }} / 10</option></select></label>
              <label class="field"><span>DLC 超凡突破</span><select v-model.number="weaponTranscendence"><option v-for="value in weaponTranscendenceOptions" :key="value" :value="value">{{ value }} / 7</option></select></label>
              <label v-if="weaponTranscendence >= 7" class="field"><span>第 7 阶超凡效果</span><select v-model="weaponTranscendenceSkill"><option v-for="skill in weaponTranscendenceSkills" :key="skill.hash" :value="skill.hash">{{ skill.name }}</option></select></label>
            </div>
            <div class="weapon-actions"><button class="plain-btn subtle" @click="maxWeapon">等级与幻晶最大</button><button class="plain-btn subtle" @click="maxWeaponProgression">全部养成最大</button></div>
            <p class="current">“等级上限解锁 6”“武器觉醒 10”“DLC 超凡突破 7”是三个独立字段。觉醒跨阶段时会同步切换 2.0.2 对应武器 Hash；超凡 7 阶效果会写入独立效果槽。</p>
            <LegalityIndicator :status="weaponLegality.status" :message="weaponLegality.message" />
            <label v-if="weaponUnlockRisk" class="unlock-risk"><input v-model="allowWeaponUnlockRisk" type="checkbox"> 我已确认游戏内任务/觉醒解锁状态，仍按当前数值写入</label>
            <button class="primary-btn" :disabled="applying || !weaponLegality.writable || (!!weaponUnlockRisk && !allowWeaponUnlockRisk)" @click="saveWeapon">{{ applying ? '写入并验证中…' : weaponView === 'owned' ? '保存武器养成' : '添加这把武器' }}</button>
          </template>
          <div v-else class="empty small">从左侧选择武器</div>
        </aside>
      </div>
    </template>
  </div>
  <ConfirmDialog ref="confirmDialog" />
</template>

<style scoped>
.root { width:100%; max-width:940px; display:flex; flex-direction:column; gap:11px; color:rgba(255,255,255,.7); container-type:inline-size; }
.save-card,.editor-card,.detail-panel,.catalog-list { border:1px solid rgba(255,255,255,.075); background:rgba(255,255,255,.026); border-radius:12px; }
.save-card { padding:14px 16px; display:flex; flex-direction:column; gap:9px; }
.save-title { display:flex; justify-content:space-between; gap:12px; }
.save-title strong { display:block; font-size:.9rem; letter-spacing:.5px; }.save-title small { display:block; margin-top:3px; color:rgba(255,255,255,.3); font-size:.66rem; }
.capacity { font-size:.68rem; color:rgba(74,222,128,.7); white-space:nowrap; }
.slots,.toolbar { display:flex; gap:8px; align-items:center; flex-wrap:wrap; }.toolbar { padding:10px; }
.slot-btn,.plain-btn,.section-tabs button,.mini-tabs button { padding:7px 12px; border-radius:7px; border:1px solid rgba(255,255,255,.11); background:rgba(255,255,255,.045); color:rgba(255,255,255,.5); font:inherit; font-size:.72rem; cursor:pointer; }
.slot-btn.on,.section-tabs button.on,.mini-tabs button.on { color:#67e8f9; border-color:rgba(103,232,249,.4); background:rgba(103,232,249,.11); }.path { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font:750 .64rem var(--font-data); font-variant-numeric:tabular-nums lining-nums; color:rgba(255,255,255,.24); }
.section-tabs,.mini-tabs { display:flex; gap:5px; }.section-tabs { justify-content:center; }.section-tabs button { min-width:105px; }
.search,input,select { box-sizing:border-box; padding:8px 10px; border-radius:7px; border:1px solid rgba(255,255,255,.12); background:rgba(255,255,255,.055); color:#e5f7fa; outline:none; font:inherit; font-size:.74rem; }.search { flex:1; min-width:220px; }.search:focus,input:focus,select:focus { border-color:rgba(103,232,249,.45); }select option { background:#152132; }
.experimental { font-size:.67rem; color:rgba(251,191,36,.7); }.workspace { display:grid; grid-template-columns:minmax(0,1.55fr) minmax(250px,.8fr); gap:11px; min-height:360px; }.catalog-list { max-height:430px; overflow-y:auto; padding:5px; }.catalog-row { width:100%; min-height:48px; padding:7px 10px; display:flex; align-items:center; justify-content:space-between; gap:12px; border:1px solid transparent; border-bottom-color:rgba(255,255,255,.035); border-radius:7px; background:transparent; color:inherit; text-align:left; cursor:pointer; }.catalog-row:hover { background:rgba(255,255,255,.035); }.catalog-row.on { border-color:rgba(103,232,249,.35); background:rgba(103,232,249,.09); }.catalog-row.danger { color:#fbbf24; }.catalog-row strong { display:block; font-size:.76rem; font-weight:600; }.catalog-row small { display:block; margin-top:3px; color:rgba(255,255,255,.25); font:750 .62rem var(--font-data); font-variant-numeric:tabular-nums lining-nums; }.catalog-row em { font-style:normal; font-size:.67rem; color:#67e8f9; white-space:nowrap; }
.detail-panel { padding:16px; display:flex; flex-direction:column; gap:11px; align-self:start; position:sticky; top:0; }.detail-panel h3 { margin:0; font-size:.92rem; color:rgba(255,255,255,.78); }.detail-panel p { margin:0; font-size:.68rem; color:rgba(255,255,255,.33); line-height:1.55; }.detail-panel code { color:#67e8f9; font-size:.7rem; }.field { display:flex; flex-direction:column; gap:5px; font-size:.68rem; color:rgba(255,255,255,.4); }.field input,.field select { width:100%; }.weapon-fields { display:grid; grid-template-columns:1fr 1fr; gap:8px; }.current { color:rgba(103,232,249,.57)!important; }.danger-confirm { padding:8px; border-radius:7px; background:rgba(245,158,11,.08); color:#fbbf24; font-size:.67rem; line-height:1.4; }
.primary-btn { padding:9px 14px; border-radius:8px; border:1px solid rgba(74,222,128,.3); background:rgba(34,197,94,.12); color:#4ade80; font:inherit; font-size:.76rem; font-weight:600; cursor:pointer; }.primary-btn:hover:not(:disabled) { background:rgba(34,197,94,.19); }.primary-btn:disabled,button:disabled { opacity:.35; cursor:not-allowed; }.resource-grid { padding:16px; display:grid; grid-template-columns:repeat(3,1fr); gap:12px; }.resource-grid label { display:flex; flex-direction:column; gap:6px; font-size:.7rem; color:rgba(255,255,255,.42); }.offline-note { grid-column:1/-1; margin:0; padding:8px 10px; border:1px solid rgba(251,191,36,.18); border-radius:7px; background:rgba(245,158,11,.06); color:rgba(251,191,36,.7); font-size:.66rem; line-height:1.45; }.wide { grid-column:1/-1; }
.empty { padding:34px 12px; text-align:center; color:rgba(255,255,255,.3); font-size:.76rem; }.empty.small { padding:24px 8px; }input[type=checkbox] { accent-color:#67e8f9; }
@media (max-width:720px) { .workspace { grid-template-columns:1fr; }.detail-panel { position:static; }.resource-grid { grid-template-columns:1fr; }.wide { grid-column:auto; } }

/* Field notes catalog layout */
.root { gap:13px;color:var(--text-secondary) }.save-card,.editor-card,.detail-panel,.catalog-list { border-color:rgba(154,202,224,.14);background:rgba(8,31,53,.7);border-radius:4px 12px 4px 12px }.save-card { padding:14px 17px;border-top-color:rgba(218,187,115,.27) }.capacity { color:var(--green) }.slots { gap:7px }.slot-btn,.plain-btn,.section-tabs button,.mini-tabs button { border-color:rgba(154,202,224,.16);border-radius:3px 8px 3px 8px;background:rgba(16,52,79,.58);color:#91a9b5 }.slot-btn.on,.section-tabs button.on,.mini-tabs button.on { color:#f0d99d;border-color:rgba(218,187,115,.38);background:rgba(218,187,115,.09) }.path { color:#5f7b89 }.section-tabs { justify-content:flex-start;padding:3px;border-bottom:1px solid rgba(218,187,115,.12) }.section-tabs button { min-width:120px;border-color:transparent;background:transparent }.section-tabs button.on { box-shadow:inset 0 -1px var(--gold) }
.toolbar { display:grid;grid-template-columns:minmax(220px,1fr) auto auto auto;gap:8px;padding:11px 12px }.toolbar .mini-tabs { grid-column:auto }.toolbar:has(.mini-tabs) { grid-template-columns:auto minmax(220px,1fr) auto auto auto }.search,input,select { border-color:rgba(154,202,224,.18);border-radius:4px;background:rgba(12,43,68,.78);color:#edf4f6 }.toolbar select { min-width:122px }.owned-toggle,.experimental { display:flex;align-items:center;gap:5px;white-space:nowrap;font-size:9px;color:#7f9aa7 }.experimental { color:#c9a964 }.result-count { grid-column:1/-1;color:#648291;font-size:9px;text-align:right }
.workspace { grid-template-columns:minmax(0,1.55fr) minmax(280px,.8fr);gap:12px }.catalog-list { max-height:470px;padding:6px }.catalog-row { min-height:57px;padding:9px 11px;border-bottom-color:rgba(154,202,224,.065);border-radius:3px 8px 3px 8px }.catalog-row:hover { background:rgba(84,166,201,.07) }.catalog-row.on { border-color:rgba(218,187,115,.29);background:linear-gradient(90deg,rgba(218,187,115,.09),rgba(87,171,205,.05));box-shadow:inset 2px 0 var(--gold) }.catalog-row strong { color:#dce8ec;font-size:12px }.catalog-row small { color:#607c8a;font-size:9px }.catalog-row em { color:#8bdcf3 }.detail-panel { padding:18px;border-top-color:rgba(218,187,115,.25);box-shadow:0 14px 36px rgba(0,7,17,.17) }.detail-panel h3 { color:#f0e8d7;font:700 16px var(--font-ui);letter-spacing:.04em }.detail-panel code { color:#8bdcf3 }.weapon-fields { gap:10px }.field { color:#8099a6 }.number-combo { display:grid;grid-template-columns:minmax(0,1fr) auto;gap:5px }.number-combo input { min-width:0 }.number-combo button { min-width:45px;padding:0 9px;border:1px solid rgba(218,187,115,.27);border-radius:3px 7px 3px 7px;background:rgba(218,187,115,.07);color:#d9bd7c;font:inherit;font-size:9px;cursor:pointer }.number-combo button:hover { border-color:var(--gold);background:rgba(218,187,115,.14) }.plain-btn.subtle { color:#cdb574;border-color:rgba(218,187,115,.21);background:rgba(218,187,115,.045) }.primary-btn { border-color:rgba(218,187,115,.38);background:linear-gradient(135deg,rgba(184,142,61,.24),rgba(91,65,24,.15));color:#f0d99d }.resource-grid { border-top-color:rgba(218,187,115,.24) }.offline-note { border-color:rgba(218,187,115,.22);background:rgba(218,187,115,.055);color:#c4ad76 }
.catalog-row.owned { opacity:.58;cursor:not-allowed }.catalog-row.owned em { color:#a36f4c }.weapon-kind { display:flex;align-items:flex-start;gap:8px;margin:10px 0 2px;padding:8px 10px;border:1px solid rgba(218,187,115,.22);background:rgba(218,187,115,.06) }.weapon-kind b { flex:0 0 auto;color:#d2b36c;font-size:10px }.weapon-kind span { color:#8fa5ae;font-size:9px;line-height:1.5 }.unlock-risk { display:flex;align-items:flex-start;gap:7px;padding:9px 10px;border:1px solid rgba(207,139,76,.28);background:rgba(207,139,76,.08);color:#c5a676;font-size:9px;line-height:1.55 }
.weapon-name-line{display:flex;align-items:center;gap:7px;min-width:0}.weapon-name-line strong{min-width:0}.weapon-type{flex:0 0 auto;padding:2px 6px;border:1px solid rgba(188,151,78,.28);border-radius:999px;background:rgba(188,151,78,.07);color:#cdb574;font-style:normal;font-size:8px;line-height:1.2}.weapon-type[data-type="terminus"]{border-color:rgba(170,112,71,.35);color:#d3a06f}.weapon-type[data-type="ascension"],.weapon-type[data-type="special"]{border-color:rgba(197,162,90,.38);color:#e0c27f}.weapon-progress{display:flex;flex-direction:column;align-items:flex-end;gap:2px;color:#d9bd7c!important;font-family:var(--font-data);font-variant-numeric:tabular-nums}.weapon-progress small{font-size:8px;color:#77909b}.weapon-actions{display:grid;grid-template-columns:1fr 1fr;gap:7px}.weapon-actions button{width:100%}
.capacity{display:inline!important;align-self:center!important;padding:0!important;border:0!important;border-radius:0!important;color:#75654d!important;background:transparent!important;box-shadow:none!important;font-size:9px!important;font-weight:650!important}
.weapon-kind{display:block!important;margin:9px 0 2px!important;padding:8px 9px!important;border:1px solid rgba(133,96,44,.26)!important;border-left:3px solid #8b6737!important;border-radius:0!important;background:#edddba!important;box-shadow:none!important}.weapon-kind b{display:block!important;color:#6d4f27!important;font-size:10px!important;font-weight:750!important}.weapon-kind span{display:block!important;margin-top:3px!important;color:#6d604e!important;font-size:9px!important;line-height:1.5!important}
@container (max-width:680px){
  .toolbar,.toolbar:has(.mini-tabs){grid-template-columns:minmax(0,1fr) minmax(0,1fr);align-items:stretch}
  .toolbar>*{min-width:0;max-width:100%}
  .toolbar .mini-tabs,.toolbar .search,.toolbar .result-count{grid-column:1/-1}
  .toolbar .mini-tabs{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr)}
  .toolbar .mini-tabs button,.toolbar select{width:100%;min-width:0}
  .owned-toggle,.experimental{min-height:31px;white-space:normal}
  .section-tabs{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));width:100%}
  .section-tabs button{width:100%;min-width:0;padding-left:6px;padding-right:6px;white-space:normal}
}
@media(max-width:900px){.toolbar,.toolbar:has(.mini-tabs){grid-template-columns:1fr 1fr}.toolbar .search{grid-column:1/-1}.result-count{grid-column:1/-1}.workspace{grid-template-columns:1fr}.detail-panel{position:static}}
</style>
