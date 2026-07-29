<script setup>
import { computed, reactive, ref } from 'vue'
import {
  DeployNaturalDropMod,
  GetNaturalDropWorkspace,
  RestoreNaturalDropDefaults,
  SelectNaturalDropGameExecutable,
  SelectNaturalDropTableDirectory,
} from '../../wailsjs/go/backend/App'
import { language } from '../i18n'
import ConfirmDialog from './ConfirmDialog.vue'

const emit = defineEmits(['status'])

const sourceDir = ref('')
const gameExePath = ref('')
const workspace = ref(null)
const search = ref('')
const tier = ref('all')
const busy = ref(false)
const actionMessage = ref('')
const actionTone = ref('')
const confirmDialog = ref(null)
const selections = reactive({})
const wrightstoneSelections = reactive({})
const wrightstoneOnly = ref(true)

const tx = (zh, en) => language.value === 'en' ? en : zh
const displayName = item => language.value === 'en' ? item?.nameEn : item?.nameZh
const displayKind = item => language.value === 'en' ? item?.typeNameEn : item?.typeNameZh

const tableReady = computed(() => workspace.value?.summonTablesReady === true)
const wrightstoneTableReady = computed(() => workspace.value?.wrightstoneTablesReady === true)
const gameReady = computed(() => Boolean(workspace.value?.indexValid && gameExePath.value))
const selectedList = computed(() => Object.entries(selections)
  .filter(([, value]) => value?.enabled)
  .map(([typeHash, value]) => ({ typeHash, mainTrait: value.mainTrait, subParam: value.subParam })))
const selectedWrightstones = computed(() => Object.values(wrightstoneSelections).flatMap(value => (value?.variants || [])
  .filter(variant => variant.enabled)
  .map(variant => ({ mainTrait: value.mainTrait, subTrait1: variant.subTrait1, subTrait2: variant.subTrait2 }))))
const activeConflictScopes = computed(() => new Set([
  ...(selectedList.value.length ? ['summon'] : []),
  ...(selectedWrightstones.value.length ? ['wrightstone'] : []),
]))
const activeConflicts = computed(() => (workspace.value?.conflicts || []).filter(item => activeConflictScopes.value.has(item.scope)))
const hasConflicts = computed(() => activeConflicts.value.length > 0)
const tiers = computed(() => [...new Set((workspace.value?.summons || []).map(item => item.tier).filter(Boolean))].sort().reverse())
const visibleSummons = computed(() => {
  const needle = search.value.trim().toLocaleLowerCase(language.value === 'en' ? 'en' : 'zh-CN')
  return (workspace.value?.summons || []).filter(item => {
    if (tier.value !== 'all' && item.tier !== tier.value) return false
    if (!needle) return true
    const haystack = [item.nameZh, item.nameEn, item.typeHash, item.typeNameZh, item.typeNameEn,
      ...(item.mainTraits || []).flatMap(option => [option.nameZh, option.nameEn, option.hash]),
      ...(item.subParams || []).flatMap(option => [option.nameZh, option.nameEn, option.hash])]
      .join(' ').toLocaleLowerCase(language.value === 'en' ? 'en' : 'zh-CN')
    return haystack.includes(needle)
  })
})
const canDeploy = computed(() => gameReady.value && !hasConflicts.value && !busy.value &&
  ((selectedList.value.length > 0 && tableReady.value) || (selectedWrightstones.value.length > 0 && wrightstoneTableReady.value)) &&
  (selectedList.value.length === 0 || tableReady.value) && (selectedWrightstones.value.length === 0 || wrightstoneTableReady.value))
const canRestore = computed(() => Boolean(workspace.value?.owned && gameReady.value && !busy.value))

function ensureSelection(item) {
  if (selections[item.typeHash]) return selections[item.typeHash]
  selections[item.typeHash] = {
    enabled: false,
    mainTrait: item.mainTraits?.[0]?.hash || '',
    subParam: item.subParams?.[0]?.hash || '',
  }
  return selections[item.typeHash]
}

function syncSelections() {
  for (const item of workspace.value?.summons || []) ensureSelection(item)
  for (const family of workspace.value?.wrightstones || []) {
    if (wrightstoneSelections[family.mainTrait.hash]) continue
    const first = family.subTraits?.[0]?.hash || ''
    const second = family.subTraits?.[1]?.hash || first
    wrightstoneSelections[family.mainTrait.hash] = {
      mainTrait: family.mainTrait.hash,
      variants: Array.from({ length: family.maxVariants || 3 }, () => ({ enabled: false, subTrait1: first, subTrait2: second })),
    }
  }
}

function setMessage(message, tone = 'info') {
  actionMessage.value = message
  actionTone.value = tone
  emit('status', message, tone === 'danger' ? 'error' : tone === 'ok' ? 'success' : 'info')
}

async function refreshWorkspace() {
  if (!sourceDir.value && !gameExePath.value) return
  busy.value = true
  actionMessage.value = ''
  try {
    workspace.value = await GetNaturalDropWorkspace(sourceDir.value, gameExePath.value)
    sourceDir.value = workspace.value?.sourceDir || sourceDir.value
    gameExePath.value = workspace.value?.gameExePath || gameExePath.value
    syncSelections()
  } catch (error) {
    workspace.value = null
    setMessage(String(error), 'danger')
  } finally {
    busy.value = false
  }
}

async function chooseSource() {
  const path = await SelectNaturalDropTableDirectory()
  if (!path) return
  sourceDir.value = path
  await refreshWorkspace()
}

async function chooseGame() {
  const path = await SelectNaturalDropGameExecutable()
  if (!path) return
  gameExePath.value = path
  await refreshWorkspace()
}

function toggleSelection(item) {
  const selection = ensureSelection(item)
  selection.enabled = !selection.enabled
}

function selectVisible() {
  const shouldEnable = visibleSummons.value.some(item => !ensureSelection(item).enabled)
  for (const item of visibleSummons.value) ensureSelection(item).enabled = shouldEnable
}

async function deploy() {
  if (!canDeploy.value) return
  const accepted = await confirmDialog.value?.ask({
    title: tx('部署天然掉落模组', 'Deploy natural-drop mod'),
    message: tx(`将部署 ${selectedList.value.length} 颗召唤石与 ${selectedWrightstones.value.length} 个祝福石变体。`, `Deploy ${selectedList.value.length} summons and ${selectedWrightstones.value.length} wrightstone variants.`),
    detail: tx('应用会备份原始 data.i，再把所选功能的生成表登记为游戏原生外部文件。游戏必须完全退出；已有外部表会阻止覆盖。', 'The app backs up the original data.i and registers the selected generated tables as native external files. The game must be closed; existing external overrides block deployment.'),
    confirmLabel: tx('确认部署', 'Deploy'),
    cancelLabel: tx('取消', 'Cancel'),
    tone: 'warning',
  })
  if (!accepted) return
  busy.value = true
  try {
    const result = await DeployNaturalDropMod({ sourceDir: sourceDir.value, gameExePath: gameExePath.value, selections: selectedList.value, wrightstones: selectedWrightstones.value, wrightstoneOnly: wrightstoneOnly.value })
    setMessage(tx(`已部署 ${result.selectedSummons} 颗召唤石与 ${result.selectedWrightstones} 个祝福石变体。现在可正常启动游戏。`, `Deployed ${result.selectedSummons} summons and ${result.selectedWrightstones} wrightstone variants. The game can now be launched normally.`), 'ok')
    await refreshWorkspace()
  } catch (error) {
    setMessage(String(error), 'danger')
  } finally {
    busy.value = false
  }
}

async function restoreOrRemove() {
  if (!canRestore.value) return
  const accepted = await confirmDialog.value?.ask({
    title: tx('恢复游戏原始掉落', 'Restore original drops'),
    message: tx('将恢复部署前自动备份的 data.i，并移除本工具生成的掉落表。', 'Restore the automatically backed-up data.i and remove the drop tables generated by this tool.'),
    detail: tx('请先完全退出游戏。恢复只接受本工具清单和 SHA-256 校验都通过的备份。', 'Close the game first. Restoration only accepts a tool-owned backup whose manifest and SHA-256 both validate.'),
    confirmLabel: tx('确认恢复', 'Restore'), cancelLabel: tx('取消', 'Cancel'), tone: 'warning',
  })
  if (!accepted) return
  busy.value = true
  try {
    await RestoreNaturalDropDefaults({ gameExePath: gameExePath.value })
    setMessage(tx('原始 data.i 已恢复，本工具生成的掉落表和备份清单已清理。', 'The original data.i has been restored, and the generated tables and deployment manifest were removed.'), 'ok')
    await refreshWorkspace()
  } catch (error) {
    setMessage(String(error), 'danger')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="natural-drop-lab ui-page-stack">
    <section class="drop-intro">
      <div>
        <p class="drop-kicker">{{ tx('应用内直连 · DLC 2.0.2', 'NATIVE DEPLOYMENT · DLC 2.0.2') }}</p>
        <h2>{{ tx('天然掉落部署', 'Natural Drop Deployment') }}</h2>
        <p>{{ tx('从你自己的解包表生成召唤石掉落与 Transmarvel 天然祝福石。所有选择都来自 2.0.2 原表已有记录。', 'Build summon drops and legitimate Transmarvel wrightstones from your own extracted tables. Every choice comes from existing 2.0.2 records.') }}</p>
      </div>
      <div class="drop-safety">
        <strong>{{ tx('来源原表只读', 'Source tables are read-only') }}</strong>
        <span>{{ tx('部署前自动备份 data.i；恢复时校验备份与工具清单', 'data.i is backed up before deployment and verified against the tool manifest during restoration') }}</span>
      </div>
    </section>

    <section class="drop-setup" aria-labelledby="drop-setup-title">
      <div class="section-heading">
        <div><h3 id="drop-setup-title">{{ tx('一、确认数据与游戏目录', '1. Verify data and game folders') }}</h3><p>{{ tx('选择解包的 system/table 和游戏程序，应用会直接维护原生 data.i。', 'Choose the extracted system/table folder and game executable; the app maintains the native data.i directly.') }}</p></div>
        <span v-if="workspace?.owned" class="install-state owned">{{ workspace.installed ? tx('天然掉落已启用', 'Natural drops enabled') : tx('部署状态需恢复', 'Deployment needs recovery') }}</span>
      </div>
      <div class="path-rows">
        <div class="path-row">
          <span class="path-index">01</span>
          <div><b>{{ tx('2.0.2 解包表', '2.0.2 extracted tables') }}</b><code :title="sourceDir">{{ sourceDir || tx('尚未选择', 'Not selected') }}</code></div>
          <button class="ui-btn is-sm" type="button" :disabled="busy" @click="chooseSource">{{ tx('选择表目录', 'Choose tables') }}</button>
        </div>
        <div class="path-row">
          <span class="path-index">02</span>
          <div><b>{{ tx('游戏程序', 'Game executable') }}</b><code :title="gameExePath">{{ gameExePath || tx('尚未选择', 'Not selected') }}</code></div>
          <button class="ui-btn is-sm" type="button" :disabled="busy" @click="chooseGame">{{ tx('选择游戏', 'Choose game') }}</button>
        </div>
      </div>
      <div v-if="workspace?.tables?.length" class="table-ledger" aria-label="table validation">
        <div v-for="item in workspace.tables" :key="item.name" class="table-line" :class="{ valid: item.valid }">
          <span class="table-mark" aria-hidden="true">{{ item.valid ? '✓' : '!' }}</span><b>{{ item.name }}</b><span>{{ item.size.toLocaleString() }} B</span><code>{{ item.sha256.slice(0, 12) }}…</code>
        </div>
      </div>
      <div v-if="hasConflicts" class="ui-notice is-danger">
        <b>{{ tx('发现表覆盖冲突，部署已锁定。', 'Table override conflicts found; deployment is locked.') }}</b>
        <span v-for="conflict in activeConflicts" :key="`${conflict.modId}:${conflict.file}`">{{ conflict.modId }} · {{ conflict.file }}</span>
      </div>
      <div v-if="gameReady" class="ui-notice is-ok">
        <b>{{ tx('原生索引已验证', 'Native index verified') }}</b>
        <span>{{ workspace.indexSummary }}</span>
        <code>{{ workspace.indexPath }}</code>
      </div>
    </section>

    <section v-if="tableReady" class="drop-catalog" aria-labelledby="drop-catalog-title">
      <div class="catalog-toolbar">
        <div><h3 id="drop-catalog-title">{{ tx('二、选择召唤石与天然词条', '2. Choose summons and natural traits') }}</h3><p>{{ tx('每颗召唤石的两个词条池独立生成；筛选不会清空已经选择的项目。', 'Each summon gets independent trait pools; filtering does not clear existing selections.') }}</p></div>
        <div class="catalog-controls">
          <input v-model="search" class="ui-input" type="search" :placeholder="tx('搜索召唤石、词条或哈希', 'Search summons, traits or hashes')" />
          <select v-model="tier" class="ui-select" :aria-label="tx('稀有度筛选', 'Tier filter')"><option value="all">{{ tx('全部稀有度', 'All tiers') }}</option><option v-for="value in tiers" :key="value" :value="value">{{ value }}</option></select>
          <button class="ui-btn is-sm is-ghost" type="button" @click="selectVisible">{{ tx('全选/取消当前结果', 'Toggle visible') }}</button>
        </div>
      </div>
      <div class="catalog-count"><span>{{ tx(`显示 ${visibleSummons.length} 颗`, `${visibleSummons.length} shown`) }}</span><b>{{ tx(`已选 ${selectedList.length} 颗`, `${selectedList.length} selected`) }}</b></div>
      <div class="summon-grid">
        <article v-for="item in visibleSummons" :key="item.typeHash" class="summon-card" :class="{ selected: ensureSelection(item).enabled }">
          <header>
            <button class="summon-check" type="button" :aria-pressed="ensureSelection(item).enabled" :aria-label="tx(`选择${item.nameZh}`, `Select ${item.nameEn}`)" @click="toggleSelection(item)"><span>{{ ensureSelection(item).enabled ? '✓' : '' }}</span></button>
            <div><h4>{{ displayName(item) }}</h4><p>{{ item.tier }} · {{ displayKind(item) }} · {{ tx(`${item.rewardPools} 个奖励池`, `${item.rewardPools} reward pools`) }}</p></div>
            <code>{{ item.typeHash }}</code>
          </header>
          <div class="trait-fields">
            <label><span>{{ tx('主加护', 'Main trait') }}</span><select v-model="ensureSelection(item).mainTrait" class="ui-select" :disabled="!ensureSelection(item).enabled"><option v-for="option in item.mainTraits" :key="option.hash" :value="option.hash">{{ displayName(option) }}</option></select></label>
            <label><span>{{ tx('附加词条', 'Bonus trait') }}</span><select v-model="ensureSelection(item).subParam" class="ui-select" :disabled="!ensureSelection(item).enabled"><option v-for="option in item.subParams" :key="option.hash" :value="option.hash">{{ displayName(option) }}</option></select></label>
          </div>
        </article>
      </div>
      <p v-if="visibleSummons.length === 0" class="empty-result">{{ tx('没有匹配的天然掉落记录。', 'No matching natural-drop records.') }}</p>
    </section>

    <section v-if="wrightstoneTableReady" class="drop-catalog wrightstone-catalog" aria-labelledby="wrightstone-catalog-title">
      <div class="catalog-toolbar">
        <div><h3 id="wrightstone-catalog-title">{{ tx('三、配置天然祝福石', '3. Configure legitimate wrightstones') }}</h3><p>{{ tx('每种祝福最多三个变体，固定为原表最高品质的主 Lv20、副 Lv15、副 Lv10；两个副词条可以相同。', 'Each family supports up to three variants at the legitimate maximum shape: main Lv20, sub Lv15 and sub Lv10. The two sub traits may match.') }}</p></div>
        <label class="wrightstone-only"><input v-model="wrightstoneOnly" type="checkbox"><span><b>{{ tx('Transmarvel 只出祝福石', 'Wrightstones only from Transmarvel') }}</b><small>{{ tx('开启后从原本 25% 提高到 100%', 'Raises the original 25% chance to 100%') }}</small></span></label>
      </div>
      <div class="wrightstone-grid">
        <article v-for="family in workspace.wrightstones" :key="family.mainTrait.hash" class="wrightstone-card">
          <header><div><h4>{{ displayName(family) }}</h4><p>{{ displayName(family.mainTrait) }} Lv20 · {{ tx(`最多 ${family.maxVariants} 个变体`, `Up to ${family.maxVariants} variants`) }}</p></div><code>{{ family.mainTrait.hash }}</code></header>
          <div class="wrightstone-variants">
            <div v-for="(variant, index) in wrightstoneSelections[family.mainTrait.hash].variants" :key="index" class="wrightstone-variant" :class="{ enabled: variant.enabled }">
              <label class="variant-toggle"><input v-model="variant.enabled" type="checkbox"><span>{{ tx(`变体 ${index + 1}`, `Variant ${index + 1}`) }}</span></label>
              <label><span>{{ tx('副词条 Lv15', 'Sub trait Lv15') }}</span><select v-model="variant.subTrait1" class="ui-select" :disabled="!variant.enabled"><option v-for="option in family.subTraits" :key="option.hash" :value="option.hash">{{ displayName(option) }}</option></select></label>
              <label><span>{{ tx('副词条 Lv10', 'Sub trait Lv10') }}</span><select v-model="variant.subTrait2" class="ui-select" :disabled="!variant.enabled"><option v-for="option in family.subTraits" :key="option.hash" :value="option.hash">{{ displayName(option) }}</option></select></label>
            </div>
          </div>
        </article>
      </div>
    </section>
    <div v-else-if="sourceDir" class="ui-notice is-info">
      <b>{{ tx('天然祝福石尚未就绪', 'Wrightstone deployment is not ready') }}</b>
      <span>{{ tx('当前目录缺少四张已验证原表：item_pendulum、gacha_lot、gacha_rate_group、gacha。召唤石功能仍可单独部署。', 'The folder is missing the four verified tables: item_pendulum, gacha_lot, gacha_rate_group and gacha. Summon deployment remains available on its own.') }}</span>
    </div>

    <section class="deploy-dock" :class="{ ready: canDeploy }">
      <div><b>{{ tx('四、生成并部署', '4. Build and deploy') }}</b><span v-if="!sourceDir">{{ tx('等待 2.0.2 解包表', 'Waiting for 2.0.2 tables') }}</span><span v-else-if="hasConflicts">{{ tx('先处理冲突模组', 'Resolve conflicting mods first') }}</span><span v-else>{{ tx(`${selectedList.length} 颗召唤石 · ${selectedWrightstones.length} 个祝福石变体`, `${selectedList.length} summons · ${selectedWrightstones.length} wrightstone variants`) }}</span></div>
      <div class="dock-actions"><button v-if="workspace?.owned" class="ui-btn is-secondary" type="button" :disabled="!canRestore" @click="restoreOrRemove">{{ tx('恢复原始掉落', 'Restore original drops') }}</button><button class="ui-btn is-primary" type="button" :disabled="!canDeploy" @click="deploy">{{ busy ? tx('处理中…', 'Working…') : workspace?.installed ? tx('更新部署', 'Update deployment') : tx('生成并部署', 'Build and deploy') }}</button></div>
    </section>
    <div v-if="actionMessage" class="ui-notice" :class="{ 'is-danger': actionTone === 'danger', 'is-ok': actionTone === 'ok', 'is-info': actionTone === 'info' }" role="status">{{ actionMessage }}</div>
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<style scoped>
.natural-drop-lab { container: natural-drop / inline-size; padding-bottom:86px; }
.drop-intro { display:grid; grid-template-columns:minmax(0,1fr) minmax(230px,310px); gap:var(--space-6); align-items:end; padding:var(--space-6) 0 var(--space-5); border-bottom:1px solid var(--border-default); }
.drop-kicker { margin:0 0 var(--space-2); color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); letter-spacing:.06em; }
.drop-intro h2,.section-heading h3,.catalog-toolbar h3 { margin:0; color:var(--text-primary); font-family:var(--font-display); }
.drop-intro h2 { font-size:var(--fs-2xl); }
.drop-intro p,.section-heading p,.catalog-toolbar p { margin:var(--space-2) 0 0; color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }
.drop-safety { padding:var(--space-4); border-left:3px solid var(--success); background:var(--success-bg); color:var(--success-ink); }
.drop-safety strong,.drop-safety span { display:block; }
.drop-safety span { margin-top:var(--space-1); font-size:var(--fs-xs); }
.drop-setup,.drop-catalog { min-width:0; }
.section-heading,.catalog-toolbar { display:flex; align-items:flex-start; justify-content:space-between; gap:var(--space-5); }
.section-heading > div,.catalog-toolbar > div:first-child { min-width:0; }
.section-heading h3,.catalog-toolbar h3 { font-size:var(--fs-lg); }
.install-state { flex:0 0 auto; padding:5px 9px; border:1px solid var(--warning); border-radius:var(--radius-sm); color:var(--warning-ink); background:var(--warning-bg); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.install-state.owned { border-color:var(--success); color:var(--success-ink); background:var(--success-bg); }
.path-rows { margin-top:var(--space-4); border-top:1px solid var(--border-default); }
.path-row { display:grid; grid-template-columns:36px minmax(0,1fr) auto; align-items:center; gap:var(--space-3); min-height:64px; padding:var(--space-3) 0; border-bottom:1px solid var(--border-default); }
.path-index { display:grid; place-items:center; width:30px; height:30px; border:1px solid var(--accent-border); border-radius:50%; color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.path-row b,.path-row code { display:block; }
.path-row b { color:var(--text-primary); font-size:var(--fs-sm); }
.path-row code { max-width:100%; margin-top:3px; overflow:hidden; color:var(--text-muted); font-family:var(--font-data); font-size:var(--fs-xs); text-overflow:ellipsis; white-space:nowrap; }
.table-ledger { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:1px; margin-top:var(--space-4); overflow:hidden; border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--border-default); }
.table-line { display:grid; grid-template-columns:22px minmax(0,1fr) auto auto; align-items:center; gap:var(--space-2); min-width:0; padding:9px 11px; color:var(--danger-ink); background:var(--surface-card-pop); font-size:var(--fs-xs); }
.table-line.valid { color:var(--success-ink); }
.table-line b { overflow:hidden; color:var(--text-primary); text-overflow:ellipsis; white-space:nowrap; }
.table-line code { color:var(--text-muted); font-family:var(--font-data); }
.table-mark { display:grid; place-items:center; width:19px; height:19px; border:1px solid currentColor; border-radius:50%; font-weight:var(--fw-bold); }
.drop-setup .ui-notice { display:flex; flex-direction:column; gap:3px; margin-top:var(--space-4); }
.catalog-controls { display:grid; grid-template-columns:minmax(220px,1fr) minmax(130px,auto) auto; gap:var(--space-2); width:min(100%,680px); }
.catalog-count { display:flex; justify-content:space-between; gap:var(--space-3); margin:var(--space-4) 0 var(--space-3); color:var(--text-muted); font-size:var(--fs-xs); }
.catalog-count b { color:var(--accent); }
.summon-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-3); }
.summon-card { min-width:0; padding:var(--space-4); border:1px solid var(--border-default); border-radius:var(--radius-md); background:var(--surface-card); box-shadow:var(--shadow-1); transition:border-color var(--dur-fast) var(--ease-out),background var(--dur-fast) var(--ease-out); }
.summon-card.selected { border-color:var(--selected-border); background:var(--selected-bg); }
.summon-card header { display:grid; grid-template-columns:30px minmax(0,1fr) auto; align-items:start; gap:var(--space-3); }
.summon-check { width:28px; height:28px; display:grid; place-items:center; padding:0; border:1px solid var(--border-strong); border-radius:var(--radius-sm); color:var(--text-on-accent); background:var(--surface-field); cursor:pointer; }
.summon-check[aria-pressed="true"] { border-color:var(--accent-border); background:var(--accent); }
.summon-check span { font-size:var(--fs-sm); font-weight:var(--fw-bold); }
.summon-card h4 { margin:0; overflow-wrap:anywhere; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-md); }
.summon-card header p { margin:3px 0 0; color:var(--text-muted); font-size:var(--fs-xs); }
.summon-card header code { color:var(--text-muted); font-family:var(--font-data); font-size:10px; }
.trait-fields { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-3); margin-top:var(--space-4); }
.trait-fields label { min-width:0; }
.trait-fields label > span { display:block; margin-bottom:5px; color:var(--text-secondary); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.trait-fields .ui-select { width:100%; min-width:0; }
.wrightstone-only { display:flex; align-items:flex-start; gap:var(--space-2); max-width:280px; color:var(--text-secondary); cursor:pointer; }
.wrightstone-only input { margin-top:3px; accent-color:var(--accent); }
.wrightstone-only span,.wrightstone-only b,.wrightstone-only small { display:block; }
.wrightstone-only b { color:var(--text-primary); font-size:var(--fs-sm); }
.wrightstone-only small { margin-top:2px; color:var(--text-muted); font-size:var(--fs-xs); }
.wrightstone-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-3); margin-top:var(--space-4); }
.wrightstone-card { min-width:0; padding:var(--space-4); border:1px solid var(--border-default); border-radius:var(--radius-md); background:var(--surface-card); box-shadow:var(--shadow-1); }
.wrightstone-card > header { display:flex; align-items:flex-start; justify-content:space-between; gap:var(--space-3); }
.wrightstone-card h4 { margin:0; color:var(--text-primary); font-family:var(--font-display); font-size:var(--fs-md); }
.wrightstone-card header p { margin:3px 0 0; color:var(--text-muted); font-size:var(--fs-xs); }
.wrightstone-card header code { flex:0 0 auto; color:var(--text-muted); font-family:var(--font-data); font-size:10px; }
.wrightstone-variants { margin-top:var(--space-3); border-top:1px solid var(--border-default); }
.wrightstone-variant { display:grid; grid-template-columns:minmax(76px,.55fr) repeat(2,minmax(0,1fr)); align-items:end; gap:var(--space-2); padding:var(--space-3) 0; border-bottom:1px solid var(--border-default); opacity:.62; }
.wrightstone-variant.enabled { opacity:1; }
.wrightstone-variant label > span { display:block; margin-bottom:5px; color:var(--text-secondary); font-size:var(--fs-xs); font-weight:var(--fw-semibold); }
.wrightstone-variant .ui-select { width:100%; min-width:0; }
.variant-toggle { align-self:center; display:flex; align-items:center; gap:var(--space-2); cursor:pointer; }
.variant-toggle input { accent-color:var(--accent); }
.variant-toggle > span { margin:0; color:var(--text-primary); }
.empty-result { margin:0; padding:var(--space-7); border:1px dashed var(--border-default); color:var(--text-muted); text-align:center; }
.deploy-dock { position:sticky; z-index:10; bottom:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-4); min-height:64px; padding:var(--space-3) var(--space-4); border:1px solid var(--border-strong); border-left:4px solid var(--border-strong); border-radius:var(--radius-md); background:var(--surface-card-pop); box-shadow:var(--shadow-2); }
.deploy-dock.ready { border-left-color:var(--accent); }
.deploy-dock b,.deploy-dock span { display:block; }
.deploy-dock b { color:var(--text-primary); font-family:var(--font-display); }
.deploy-dock span { margin-top:2px; color:var(--text-muted); font-size:var(--fs-xs); }
.dock-actions { display:flex; gap:var(--space-2); flex:0 0 auto; }

@container natural-drop (max-width:860px) {
  .drop-intro { grid-template-columns:minmax(0,1fr); align-items:start; }
  .drop-safety { max-width:none; }
  .catalog-toolbar { flex-direction:column; }
  .catalog-controls { width:100%; }
  .summon-grid { grid-template-columns:minmax(0,1fr); }
  .wrightstone-grid { grid-template-columns:minmax(0,1fr); }
}
@container natural-drop (max-width:620px) {
  .section-heading { flex-direction:column; }
  .path-row { grid-template-columns:32px minmax(0,1fr); }
  .path-row .ui-btn { grid-column:1 / -1; width:100%; }
  .table-ledger { grid-template-columns:minmax(0,1fr); }
  .catalog-controls { grid-template-columns:minmax(0,1fr); }
  .table-line { grid-template-columns:22px minmax(0,1fr) auto; }
  .table-line code { display:none; }
  .trait-fields { grid-template-columns:minmax(0,1fr); }
  .wrightstone-variant { grid-template-columns:minmax(0,1fr); }
  .wrightstone-card > header { flex-direction:column; }
  .deploy-dock { position:static; align-items:stretch; flex-direction:column; }
  .dock-actions,.dock-actions .ui-btn { width:100%; }
  .dock-actions { flex-direction:column-reverse; }
}
@media (prefers-reduced-motion:reduce) { .summon-card { transition:none; } }
</style>
