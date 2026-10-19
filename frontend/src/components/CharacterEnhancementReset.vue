<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { CharacterEnhancementInspect, FindSaveFiles, ResetCharacterEnhancement, SelectSaveDiffFile } from '../../wailsjs/go/backend/App'
import { characterIdentityByHash } from '../characterRoster.js'
import { language } from '../i18n.js'
import SaveSourcePicker from './SaveSourcePicker.vue'
import CatalogSelect from './CatalogSelect.vue'

const emit = defineEmits(['status'])
const tx = (zh, en) => language.value === 'en' ? en : zh
const slots = ref([])
const path = ref('')
const snapshot = ref(null)
const selectedHash = ref('')
const mode = ref('all')
const confirmed = ref(false)
const busy = ref(false)
const lastBackup = ref('')
const selected = computed(() => snapshot.value?.characters?.find(row => row.hash === selectedHash.value))
const characterOptions = computed(() => (snapshot.value?.characters || []).map(row => ({ internalId: row.hash, displayName: name(row.hash), searchTerms: [characterIdentityByHash(row.hash)?.nameZh, characterIdentityByHash(row.hash)?.nameEn] })))
const hasChanges = computed(() => mode.value === 'all' ? selected.value?.learned > 0 : selected.value?.changedFromGranted > 0)
function name(hash) {
  const identity = characterIdentityByHash(hash)
  return identity ? (language.value === 'en' ? identity.nameEn : identity.nameZh) : hash
}
watch([selectedHash, mode], () => { confirmed.value = false })
async function load(target = path.value) {
  if (!target || busy.value) return
  busy.value = true
  confirmed.value = false
  path.value = target
  snapshot.value = null
  try {
    snapshot.value = await CharacterEnhancementInspect(target)
    if (!snapshot.value.characters?.some(row => row.hash === selectedHash.value)) selectedHash.value = snapshot.value.characters?.[0]?.hash || ''
  } catch (err) { emit('status', String(err), 'error') }
  finally { busy.value = false }
}
async function browse() {
  try { const target = await SelectSaveDiffFile(); if (target) await load(target) }
  catch (err) { emit('status', String(err), 'error') }
}
async function apply() {
  if (!snapshot.value || !confirmed.value || !hasChanges.value || busy.value) return
  busy.value = true
  try {
    const result = await ResetCharacterEnhancement({ path: path.value, expectedRevision: snapshot.value.revision, characterHash: selectedHash.value, mode: mode.value })
    lastBackup.value = result.backupPath || ''
    emit('status', tx(`已修改 ${result.changed} 组强化记录，回读核对 ${result.verified} 组。`, `Changed ${result.changed} enhancement groups and verified ${result.verified}.`), 'success')
    confirmed.value = false
    snapshot.value = null
  } catch (err) { confirmed.value = false; emit('status', String(err), 'error') }
  finally { busy.value = false }
  await load()
}
onMounted(async () => {
  try { slots.value = await FindSaveFiles() || [] }
  catch (err) { emit('status', String(err), 'error') }
})
</script>

<template>
  <section class="enhancement-reset ui-page-stack">
    <div class="ui-notice is-info">
      <strong>{{ tx('重置角色强化', 'Reset Character Enhancement') }}</strong>
      <p>{{ tx('让选定角色重新学习强化树，或只撤销入队后自己购买的强化。先完全退出游戏，再选存档和角色；只修改该角色的强化节点，不降低角色等级，不删除装备，也不退还 MSP。', 'Unlearn the selected character’s enhancement tree, or undo purchases made after recruitment. Fully exit the game first, then select a save and character. Only this character’s enhancement nodes are changed: levels and equipment remain, and MSP is not refunded.') }}</p>
    </div>
    <SaveSourcePicker :slots="slots" :model-value="path" :busy="busy" :loaded="!!snapshot" :title="tx('第一步 · 选择存档', '1 · Choose a Save')" @select="load" @browse="browse" />
    <section v-if="snapshot" class="ui-card reset-card">
      <h3>{{ tx('第二步 · 选择要重置的角色', '2 · Choose a Character') }}</h3>
      <CatalogSelect v-model="selectedHash" :options="characterOptions" :disabled="busy" :placeholder="tx('选择角色', 'Choose a Character')" :search-placeholder="tx('输入角色名筛选', 'Search Characters')" />
      <template v-if="selected">
        <p class="reset-note">{{ tx(`已学习 ${selected.learned} 项 · 自动解锁记录 ${selected.granted} 项`, `${selected.learned} learned · ${selected.granted} recorded grants`) }}</p>
        <h3>{{ tx('第三步 · 选择重置方式', '3 · Choose What to Reset') }}</h3>
        <div class="ui-seg reset-modes">
          <button type="button" class="ui-seg-btn" :class="{ 'is-on': mode === 'all' }" :disabled="busy" @click="mode = 'all'">{{ tx('全部取消学习', 'Unlearn Everything') }}</button>
          <button type="button" class="ui-seg-btn" :class="{ 'is-on': mode === 'granted' }" :disabled="busy" @click="mode = 'granted'">{{ tx('恢复入队自动解锁', 'Restore Recruitment Grants') }}</button>
        </div>
        <p class="reset-preview">{{ mode === 'all'
          ? tx(`${name(selected.hash)}：${selected.learned} 项已学习强化 → 0 项。包括入队自带的强化；攻击、防御、武器收藏及超凡页的强化效果一起取消。`, `${name(selected.hash)}: ${selected.learned} learned enhancements → 0, including recruitment grants. Enhancement effects from attack, defense, weapon collection, and transcendence trees are removed.`)
          : tx(`${name(selected.hash)}：${selected.learned} 项 → ${selected.granted} 项。恢复存档中保留的自动解锁记录；后期入队角色可能仍有较高习得率，并非全部归零。`, `${name(selected.hash)}: ${selected.learned} → ${selected.granted} enhancements. Restores grant records retained in this save; late-game recruits may still have substantial mastery. This is not a full reset.`) }}</p>
        <p class="ui-notice is-warning">{{ tx('取消强化也会取消对应的技能、因子栏位及属性加成，现有配装可能需要在游戏内重新整理。若想撤销，请到存档保护中恢复此次自动备份；不要用“恢复自动解锁”代替完整备份恢复。', 'Unlearning also removes the associated abilities, sigil-slot unlocks, and stat bonuses. You may need to rearrange your loadout in-game. To undo this operation, restore its automatic backup in Save Protection; restoring recruitment grants does not restore paid nodes.') }}</p>
        <label class="reset-confirm"><input v-model="confirmed" type="checkbox" :disabled="busy || !hasChanges">{{ tx('我已退出游戏，确认只重置上方选定角色，并理解不返还 MSP。', 'The game is closed. Reset only the selected character, without an MSP refund.') }}</label>
        <button type="button" class="ui-btn is-primary" :disabled="busy || !confirmed || !hasChanges" @click="apply">{{ busy ? tx('正在备份、写入并回读…', 'Backing Up, Writing, and Verifying…') : tx(`确认重置${name(selected.hash)}`, `Reset ${name(selected.hash)}`) }}</button>
        <small v-if="!hasChanges" class="reset-note">{{ tx('该角色已符合所选状态，无需重复写入。', 'This character already matches the selected state. No write is needed.') }}</small>
      </template>
    </section>
    <p v-if="lastBackup" class="reset-backup">{{ tx('此次操作的备份：', 'Backup for this operation: ') }}{{ lastBackup }}</p>
  </section>
</template>

<style scoped>
.reset-card { padding:var(--space-5); min-width:0; }
.reset-card h3 { margin:0 0 var(--space-3); font-size:var(--fs-md); }
.reset-note + h3 { margin-top:var(--space-5); }
.reset-modes { display:flex; flex-wrap:wrap; }
.reset-preview { line-height:var(--lh-normal); margin:var(--space-4) 0; color:var(--text-primary); }
.reset-confirm { display:flex; align-items:flex-start; gap:var(--space-2); margin:var(--space-4) 0; line-height:var(--lh-normal); }
.reset-confirm input { margin-top:4px; flex-shrink:0; }
.reset-note { display:block; margin-top:var(--space-2); color:var(--text-muted); }
.reset-backup { overflow-wrap:anywhere; color:var(--text-secondary); font-size:var(--fs-xs); }
</style>
