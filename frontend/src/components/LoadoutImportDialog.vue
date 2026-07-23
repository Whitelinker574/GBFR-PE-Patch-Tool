<script setup>
import { computed, ref, watch } from 'vue'

const props = defineProps({
  draft: { type: Object, default: null },
})
const emit = defineEmits(['cancel', 'apply'])

const choices = ref({})
const masteryProgress = ref(1)

const source = computed(() => props.draft?.applyPayload || {})
const caps = computed(() => props.draft?.capabilities || {})
const constructsWeapon = computed(() => !!source.value.constructedWeapon)
const targetFateLabel = computed(() => caps.value.targetFateDataAvailable
  ? `${Number(caps.value.targetCharacterLevel || 0) >= 100 ? '可解锁 11/11' : '解锁上限待定'} · 已完成 ${Number(caps.value.targetFateEpisodeCount || 0)}/11`
  : '未建立')
const has = computed(() => ({
  characterLevel: !!source.value.character?.characterBaseCaptured,
  factors: (props.draft?.constructedSigils || []).length > 0,
  skills: (props.draft?.skillHashes || []).length > 0,
  mastery: (props.draft?.masteryHashes || []).length > 0,
  masterProgress: !!source.value.character,
  weapon: Number(props.draft?.weaponSlotId || 0) > 0 || !!source.value.constructedWeapon,
  weaponEnhancement: !!source.value.weapon,
  wrightstone: !!source.value.weapon?.wrightstone,
  summons: (props.draft?.summonSlotIds || []).length === 4,
  overLimit: (source.value.overLimit || []).length > 0,
  characterGrowth: !!source.value.character,
  characterWeaponCollection: (source.value.character?.weapons || []).length > 0,
	characterWeaponWrightstones: !!source.value.character?.weaponWrightstonesCaptured && (source.value.character?.weapons || []).length > 0,
}))

const categoryKeys = ['characterLevel', 'factors', 'skills', 'mastery', 'masterProgress', 'weapon', 'weaponEnhancement', 'wrightstone', 'summons', 'overLimit', 'characterGrowth', 'characterWeaponCollection', 'characterWeaponWrightstones']
const targetNeedsLevel100 = computed(() => Number(caps.value.targetCharacterLevel || 0) < 100)
const needsLevel100Selection = computed(() => targetNeedsLevel100.value &&
  !!(choices.value.mastery || choices.value.masterProgress || choices.value.characterGrowth))
const levelPromotionUnavailable = computed(() => targetNeedsLevel100.value && !has.value.characterLevel)
const masteryBlocked = computed(() => !caps.value.targetMasterSystem || levelPromotionUnavailable.value)
const characterGrowthBlocked = computed(() => levelPromotionUnavailable.value)
const summonBlocked = computed(() => has.value.summons && !caps.value.targetSummonSystem)
const availableKeys = computed(() => categoryKeys.filter(key => has.value[key]
  && !(masteryBlocked.value && ['mastery', 'masterProgress'].includes(key))
  && !(characterGrowthBlocked.value && key === 'characterGrowth')
  && !(summonBlocked.value && key === 'summons')))
const allSelected = computed(() => availableKeys.value.length > 0 && availableKeys.value.every(key => choices.value[key]))
const selectedCount = computed(() => availableKeys.value.filter(key => choices.value[key]).length)
const selectedMissing = computed(() => {
  const byScope = props.draft?.missingByScope || {}
  const scopes = []
  if (choices.value.weapon || choices.value.weaponEnhancement) scopes.push('weapon')
  if (choices.value.wrightstone) scopes.push('wrightstone')
  if (choices.value.skills) scopes.push('skills')
  if (choices.value.mastery) scopes.push('mastery')
  if (choices.value.characterGrowth) scopes.push('characterGrowth')
  if (choices.value.summons) scopes.push('summons')
  return [...new Set(scopes.flatMap(scope => byScope[scope] || []))]
})
const cannotApply = computed(() => selectedCount.value === 0 || selectedMissing.value.length > 0 ||
  ((choices.value.mastery || choices.value.masterProgress) && masteryBlocked.value) ||
  (choices.value.characterGrowth && characterGrowthBlocked.value) ||
  (choices.value.summons && summonBlocked.value))

function reset() {
  const next = {
    characterLevel: false,
    factors: has.value.factors,
    skills: has.value.skills,
    mastery: has.value.mastery && !masteryBlocked.value,
    masterProgress: has.value.masterProgress && !masteryBlocked.value,
    weapon: has.value.weapon,
    weaponEnhancement: has.value.weaponEnhancement && constructsWeapon.value,
    wrightstone: has.value.wrightstone,
    summons: has.value.summons && !summonBlocked.value,
    overLimit: has.value.overLimit,
    characterGrowth: false,
    characterWeaponCollection: false,
		characterWeaponWrightstones: false,
  }
  if (targetNeedsLevel100.value && (next.mastery || next.masterProgress || next.characterGrowth)) next.characterLevel = true
  choices.value = next
  masteryProgress.value = Math.min(55, Math.max(1, Number(caps.value.sourceMasterProgressIndex || 1)))
}

watch(() => props.draft, reset, { immediate: true })

function toggleAll() {
  const selected = !allSelected.value
  const next = { ...choices.value }
  for (const key of availableKeys.value) next[key] = selected
  if (masteryBlocked.value) {
    next.mastery = false
    next.masterProgress = false
  }
  if (characterGrowthBlocked.value) next.characterGrowth = false
  if (targetNeedsLevel100.value && (next.mastery || next.masterProgress || next.characterGrowth)) next.characterLevel = true
  if (summonBlocked.value) next.summons = false
  choices.value = next
}

function toggle(key) {
  if (key === 'weaponEnhancement' && constructsWeapon.value && choices.value.weapon) return
  if (key === 'characterLevel' && needsLevel100Selection.value) return
  choices.value = { ...choices.value, [key]: !choices.value[key] }
  if (targetNeedsLevel100.value && ['mastery', 'masterProgress', 'characterGrowth'].includes(key) && choices.value[key]) {
    choices.value.characterLevel = true
  }
  if (['weaponEnhancement', 'wrightstone'].includes(key) && choices.value[key]) choices.value.weapon = true
	if (key === 'characterWeaponWrightstones' && choices.value[key]) choices.value.characterWeaponCollection = true
  if (key === 'weapon' && !choices.value.weapon) {
    choices.value.weaponEnhancement = false
    choices.value.wrightstone = false
  }
  if (key === 'weapon' && choices.value.weapon && constructsWeapon.value) choices.value.weaponEnhancement = true
	if (key === 'characterWeaponCollection' && !choices.value.characterWeaponCollection) choices.value.characterWeaponWrightstones = false
}

function masteryStars(value) {
  return '★'.repeat(Math.max(0, Math.min(5, Number(value) - 50)))
}

function submit() {
  if (cannotApply.value) return
  emit('apply', { ...choices.value, masterProgressIndex: Number(masteryProgress.value) })
}
</script>

<template>
  <Teleport to="body">
    <div v-if="draft" class="import-backdrop" role="presentation" @click.self="emit('cancel')">
      <section class="import-dialog" role="dialog" aria-modal="true" aria-labelledby="loadout-import-title">
        <header class="import-hero">
          <div><small>单套配装 · 分项导入</small><h2 id="loadout-import-title">选择要带入当前存档的内容</h2><p>每项只在勾选后写入；命运篇章始终保留目标存档。</p></div>
          <button class="dialog-close" type="button" aria-label="关闭" @click="emit('cancel')">×</button>
        </header>

        <div class="import-source-strip">
          <span><small>配装名称</small><b>{{ draft.name || '未命名配装' }}</b></span>
          <span class="import-fate-summary"><small>来源 → 目标角色</small><b>Lv{{ caps.sourceCharacterLevel || 0 }} → Lv{{ caps.targetCharacterLevel || 0 }} · 命运篇章 {{ targetFateLabel }} · 专精进度 {{ caps.targetMasterProgressIndex || 1 }}</b></span>
          <button type="button" :class="{ on: allSelected }" @click="toggleAll">{{ allSelected ? '取消全选' : '全部导入' }}</button>
        </div>

        <div class="import-grid">
          <button v-if="has.characterLevel" type="button" class="import-choice risk" :class="{ on: choices.characterLevel, locked: needsLevel100Selection }" @click="toggle('characterLevel')">
            <i class="import-choice-icon" aria-hidden="true">↥</i><span><b>角色等级</b><small>同步到 Lv{{ caps.sourceCharacterLevel || 0 }}，并覆盖对应的基础 HP、攻击、昏厥与暴击快照</small></span><em>{{ needsLevel100Selection ? '联动升至 Lv100' : choices.characterLevel ? '将覆盖' : '默认不改' }}</em>
          </button>
          <button v-if="has.factors" type="button" class="import-choice" :class="{ on: choices.factors }" @click="toggle('factors')">
            <i class="import-choice-icon" aria-hidden="true">◇</i><span><b>因子配置</b><small>创建 {{ draft.constructedSigils.length }} 枚独立因子，不复用来源实例</small></span><em>{{ choices.factors ? '已选择' : '保留目标' }}</em>
          </button>
          <button v-if="has.skills" type="button" class="import-choice" :class="{ on: choices.skills }" @click="toggle('skills')">
            <i class="import-choice-icon" aria-hidden="true">✦</i><span><b>主动技能</b><small>{{ draft.skillHashes.length }} 个角色技能</small></span><em>{{ choices.skills ? '已选择' : '保留目标' }}</em>
          </button>
          <button v-if="has.mastery" type="button" class="import-choice" :class="{ on: choices.mastery, blocked: masteryBlocked }" :disabled="masteryBlocked" @click="toggle('mastery')">
            <i class="import-choice-icon" aria-hidden="true">◎</i><span><b>专精配置</b><small>{{ draft.masteryHashes.length }} 个节点；按目标专精容量复核</small></span><em>{{ masteryBlocked ? '需角色 Lv100' : choices.mastery ? '已选择' : '保留目标' }}</em>
          </button>
          <article v-if="has.masterProgress" class="import-choice mastery-level" :class="{ on: choices.masterProgress, blocked: masteryBlocked }">
            <button type="button" :disabled="masteryBlocked" @click="toggle('masterProgress')"><i class="import-choice-icon" aria-hidden="true">★</i><span><b>专精等级</b><small>来源进度 {{ caps.sourceMasterProgressIndex || 1 }}；可单独调整</small></span><em>{{ masteryBlocked ? '需角色 Lv100' : choices.masterProgress ? '同步' : '保留目标' }}</em></button>
            <label v-if="choices.masterProgress && !masteryBlocked"><span>导入进度</span><input v-model.number="masteryProgress" type="range" min="1" max="55" /><strong>MLv{{ masteryProgress }} <i>{{ masteryStars(masteryProgress) }}</i></strong></label>
          </article>
          <button v-if="has.weapon" type="button" class="import-choice" :class="{ on: choices.weapon }" @click="toggle('weapon')">
            <i class="import-choice-icon" aria-hidden="true">⚔</i><span><b>装备武器</b><small v-if="source.constructedWeapon">目标存档缺少同款；导入时新增来源武器并绑定，不覆盖其他武器</small><small v-else>切换到目标存档已有的同类武器</small></span><em>{{ choices.weapon ? '已选择' : '保留目标' }}</em>
          </button>
          <button v-if="has.weaponEnhancement" type="button" class="import-choice risk" :class="{ on: choices.weaponEnhancement }" @click="toggle('weaponEnhancement')">
            <i class="import-choice-icon" aria-hidden="true">↗</i><span><b>同步武器强化</b><small v-if="constructsWeapon">新实例按来源初始化经验、等级、突破、幻晶、觉醒、超凡与五技能</small><small v-else>经验、等级、突破、幻晶、觉醒、超凡与五技能</small></span><em>{{ constructsWeapon ? '随新武器同步' : choices.weaponEnhancement ? '将覆盖' : '默认不改' }}</em>
          </button>
          <button v-if="has.wrightstone" type="button" class="import-choice nested" :class="{ on: choices.wrightstone }" @click="toggle('wrightstone')">
            <i class="import-choice-icon" aria-hidden="true">✧</i><span><b>只导入武器祝福</b><small>写入目标武器实际生效的祝福类型与三条附加技能；不改变武器等级</small></span><em>{{ choices.wrightstone ? '已选择' : '保留目标' }}</em>
          </button>
          <button v-if="has.summons" type="button" class="import-choice" :class="{ on: choices.summons, blocked: summonBlocked }" :disabled="summonBlocked" @click="toggle('summons')">
            <i class="import-choice-icon" aria-hidden="true">☾</i><span><b>召唤石配置</b><small>匹配已有实例；缺少时自动新增并登记</small></span><em>{{ summonBlocked ? '系统未开放' : choices.summons ? '已选择' : '保留目标' }}</em>
          </button>
          <button v-if="has.overLimit" type="button" class="import-choice" :class="{ on: choices.overLimit }" @click="toggle('overLimit')">
            <i class="import-choice-icon" aria-hidden="true">◆</i><span><b>上限突破</b><small>四槽属性与等级，可选择不覆盖</small></span><em>{{ choices.overLimit ? '已选择' : '保留目标' }}</em>
          </button>
          <button v-if="has.characterGrowth" type="button" class="import-choice risk" :class="{ on: choices.characterGrowth, blocked: characterGrowthBlocked }" :disabled="characterGrowthBlocked" @click="toggle('characterGrowth')">
            <i class="import-choice-icon" aria-hidden="true">▦</i><span><b>角色强化进度</b><small>同步攻击与 HP·抗性强化页；目标未满级时联动升至 Lv100，不改命运篇章或任何武器</small></span><em>{{ characterGrowthBlocked ? '缺少等级快照' : choices.characterGrowth ? '将覆盖' : '默认不改' }}</em>
          </button>
          <button v-if="has.characterWeaponCollection" type="button" class="import-choice risk" :class="{ on: choices.characterWeaponCollection }" @click="toggle('characterWeaponCollection')">
            <i class="import-choice-icon" aria-hidden="true">▤</i><span><b>整组角色武器收藏</b><small>同步该角色全部武器的等级、突破、幻晶、觉醒与超凡；会影响武器收集加成</small></span><em>{{ choices.characterWeaponCollection ? '将覆盖全部' : '默认不改' }}</em>
          </button>
			<button v-if="has.characterWeaponWrightstones" type="button" class="import-choice nested risk" :class="{ on: choices.characterWeaponWrightstones }" @click="toggle('characterWeaponWrightstones')">
				<i class="import-choice-icon" aria-hidden="true">✧</i><span><b>同步全部武器祝福</b><small>逐把复制祝福类型与实际生效的三条附加技能；未佩戴祝福的源武器会清空目标对应武器</small></span><em>{{ choices.characterWeaponWrightstones ? '将覆盖全部' : '默认不改' }}</em>
			</button>
        </div>

        <div v-if="selectedMissing.length" class="import-alert danger"><b>所选范围缺少目标资源</b><span>{{ selectedMissing.join('；') }}</span></div>
        <div v-else-if="needsLevel100Selection" class="import-alert"><b>将自动补足角色等级</b><span>目标角色为 Lv{{ caps.targetCharacterLevel || 0 }}；保存时会先写入来源的 Lv100 与四项基础快照，再导入所选专精或角色强化。命运篇章保持目标值。</span></div>
        <div v-else-if="masteryBlocked && (has.mastery || has.masterProgress)" class="import-alert"><b>专精暂不可导入</b><span v-if="!caps.targetMasterSystem">目标存档尚未建立该角色的专精字段；请先在游戏内开放专精系统，其他项目仍可单独导入。</span><span v-else>旧版分享没有角色等级基础快照，无法把当前 Lv{{ caps.targetCharacterLevel || 0 }} 的目标安全提升到 Lv100；请用新版从源存档重新分享。</span></div>
        <div v-if="characterGrowthBlocked && has.characterGrowth" class="import-alert"><b>角色强化暂不可导入</b><span>旧版分享没有 Lv100 基础快照，无法为未满级目标同步角色强化进度；其他项目仍可单独导入。</span></div>
        <div v-if="summonBlocked" class="import-alert"><b>召唤石系统未建立</b><span>目标存档需要先在游戏内开启对应 DLC 系统；本次可继续导入其他项目。</span></div>

        <footer><span>已选择 <b>{{ selectedCount }}</b> 项</span><button type="button" class="ui-btn" @click="emit('cancel')">取消</button><button type="button" class="ui-btn is-primary" :disabled="cannotApply" @click="submit">载入所选内容</button></footer>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.import-backdrop { position:fixed; z-index:10000; inset:0; display:grid; place-items:center; padding:24px; background:rgba(37,27,16,.56); backdrop-filter:blur(7px); }
.import-dialog { width:min(640px,calc(100vw - 24px)); max-height:min(860px,94vh); overflow:auto; border:1px solid rgba(132,94,45,.55); border-radius:18px; background:linear-gradient(155deg,#fffdf7 0%,#f7edd9 100%); box-shadow:0 28px 80px rgba(35,24,12,.34); color:#3b3021; }
.import-hero { position:relative; display:flex; justify-content:space-between; gap:20px; padding:24px 28px 20px; border-bottom:1px solid rgba(139,103,55,.2); background:radial-gradient(circle at 86% -30%,rgba(205,160,80,.28),transparent 48%); }
.import-hero small { color:#8b6737; font-weight:800; letter-spacing:.12em; }
.import-hero h2 { margin:5px 0 4px; font-family:var(--font-display); font-size:1.45rem; }
.import-hero p { margin:0; color:#756854; font-size:.88rem; }
.dialog-close { width:34px; height:34px; border:1px solid rgba(139,103,55,.25); border-radius:50%; background:rgba(255,255,255,.58); color:#6f5737; font-size:1.35rem; cursor:pointer; }
.import-source-strip { display:grid; grid-template-columns:minmax(0,1fr) minmax(0,1fr) auto; gap:12px; align-items:center; padding:14px 28px; border-bottom:1px solid rgba(139,103,55,.16); background:rgba(255,255,255,.42); }
.import-source-strip span { min-width:0; display:flex; flex-direction:column; }
.import-source-strip small { color:#897a66; font-size:.72rem; }
.import-source-strip b { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.import-source-strip .import-fate-summary b { overflow:visible; text-overflow:clip; white-space:normal; overflow-wrap:anywhere; line-height:1.35; }
.import-source-strip button { min-height:34px; padding:0 15px; border:1px solid #9a7440; border-radius:18px; background:transparent; color:#765126; font-weight:800; cursor:pointer; }
.import-source-strip button.on { background:#8b6737; color:#fff9e9; }
.import-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:10px; padding:20px 28px; }
.import-choice { min-width:0; display:grid; grid-template-columns:34px minmax(0,1fr) auto; gap:10px; align-items:center; padding:12px 13px; border:1px solid rgba(139,103,55,.2); border-radius:11px; background:rgba(255,255,255,.52); color:inherit; text-align:left; cursor:pointer; transition:transform .16s ease,border-color .16s ease,background .16s ease; }
button.import-choice:hover:not(:disabled) { transform:translateY(-1px); border-color:#9a7440; }
.import-choice.on { border-color:#6d8d6b; background:linear-gradient(135deg,rgba(102,142,105,.13),rgba(255,255,255,.65)); box-shadow:inset 3px 0 #628363; }
.import-choice.risk { border-style:dashed; }
.import-choice.risk.on { border-color:#b66b4a; background:rgba(182,107,74,.08); box-shadow:inset 3px 0 #b66b4a; }
.import-choice.nested { margin-left:24px; }
.import-choice.blocked { opacity:.48; cursor:not-allowed; }
.import-choice > i,.import-choice > button > i { width:30px; height:30px; display:grid; place-items:center; border-radius:50%; background:#eee0c5; color:#7e5b2f; font-style:normal; font-size:.72rem; font-weight:900; }
.import-choice-icon { font-family:var(--font-display); font-size:1rem !important; line-height:1; }
.import-choice > span,.import-choice > button > span { min-width:0; display:flex; flex-direction:column; gap:2px; }
.import-choice b { font-size:.92rem; }
.import-choice small { color:#80725f; font-size:.74rem; line-height:1.35; }
.import-choice em { color:#8b6737; font-size:.72rem; font-style:normal; font-weight:800; white-space:nowrap; }
.mastery-level { display:block; padding:0; }
.mastery-level > button { width:100%; display:grid; grid-template-columns:34px minmax(0,1fr) auto; gap:10px; align-items:center; padding:12px 13px; border:0; background:transparent; color:inherit; text-align:left; cursor:pointer; }
.mastery-level label { display:grid; grid-template-columns:auto minmax(120px,1fr) auto; gap:10px; align-items:center; padding:0 14px 12px 57px; color:#756854; font-size:.76rem; }
.mastery-level input { accent-color:#765126; }
.mastery-level strong { min-width:112px; color:#765126; font-variant-numeric:tabular-nums; }
.mastery-level strong i { color:#c38a31; font-style:normal; letter-spacing:1px; }
.import-alert { display:flex; flex-direction:column; gap:3px; margin:0 28px 10px; padding:10px 12px; border-left:3px solid #b88b4b; border-radius:5px; background:rgba(184,139,75,.1); color:#745c39; font-size:.78rem; }
.import-alert.danger { border-left-color:#b35b4d; background:rgba(179,91,77,.1); color:#8e4338; }
.import-dialog footer { position:sticky; bottom:0; display:flex; align-items:center; justify-content:flex-end; gap:9px; padding:14px 28px; border-top:1px solid rgba(139,103,55,.2); background:rgba(253,247,234,.96); backdrop-filter:blur(8px); }
.import-dialog footer > span { margin-right:auto; color:#786a57; font-size:.8rem; }
@media (max-width:720px) { .import-backdrop{padding:8px}.import-dialog{width:100%;max-height:96vh;border-radius:12px}.import-grid{grid-template-columns:1fr;padding:14px}.import-hero,.import-source-strip,.import-dialog footer{padding-left:16px;padding-right:16px}.import-source-strip{grid-template-columns:1fr auto}.import-source-strip span:nth-child(2){grid-column:1/-1;grid-row:2}.import-choice.nested{margin-left:12px} }
@media (max-width:1600px) and (min-width:721px) { .import-grid{grid-template-columns:1fr}.import-source-strip{grid-template-columns:1fr}.import-source-strip button{justify-self:start} }
</style>
