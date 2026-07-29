<script setup>
import { computed } from 'vue'
import { characterAssetIcon, summonAssetIcon, traitAssetIcon, weaponAssetIcon } from '../gameAssetIcons'
import { language } from '../i18n.js'

const props = defineProps({
  loadout: { type: Object, required: true },
  sourceLabel: { type: String, default: '' },
})

const characterIcon = computed(() => characterAssetIcon(props.loadout?.characterHash))
const weaponIcon = computed(() => weaponAssetIcon({ hash: props.loadout?.weapon?.hashHex || props.loadout?.weapon?.hash }))
const masteryRanks = computed(() => props.loadout?.masterySummary?.ranks || [])
const tx = (zh, en) => language.value === 'en' ? en : zh
const sigilCaptureLabel = computed(() => {
  const count = (props.loadout?.sigils || []).length
  return count ? tx(`${count}/12 因子`, `${count}/12 Sigils`) : tx('因子未记录', 'Sigils Not Recorded')
})
const summonCaptureLabel = computed(() => {
  const count = (props.loadout?.summons || []).length
  return count ? tx(`${count}/4 召唤石`, `${count}/4 Summons`) : tx('召唤石未记录', 'Summons Not Recorded')
})
const masteryCaptureLabel = computed(() => props.loadout?.masteryAvailable
  ? `MLv${Number(props.loadout?.masterLevel || 0)}`
  : tx('专精未记录', 'Mastery Not Recorded'))
const overLimitRecorded = computed(() => (props.loadout?.overLimit || []).length > 0)
const visibleStats = computed(() => {
  const stats = props.loadout?.stats || {}
  return [
    [tx('等级', 'Level'), stats.level, ''],
    ['HP', stats.totalHp, 'integer'],
    [tx('攻击力', 'Attack'), stats.totalAttack, 'integer'],
    [tx('暴击率', 'Critical Rate'), stats.criticalRate, 'percent'],
    [tx('昏厥值', 'Stun Power'), stats.stunPower, 'decimal'],
    [tx('战力', 'Power'), stats.totalPower, 'integer'],
  ].filter(([, value]) => Number(value) > 0)
})

function formatValue(value, kind) {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return '—'
  const formatted = numeric.toLocaleString('zh-CN', { maximumFractionDigits: kind === 'decimal' || kind === 'percent' ? 1 : 0 })
  return kind === 'percent' ? `${formatted}%` : formatted
}

function traitIcon(trait) {
  return traitAssetIcon({ hash: trait?.hashHex || trait?.hash, name: trait?.name })
}

function sigilIcon(sigil) {
  return traitAssetIcon({ hash: sigil?.primaryTraitHashHex || sigil?.primaryTraitHash, name: sigil?.primaryTraitName })
}

function summonIcon(summon) {
  return summonAssetIcon({ typeHash: summon?.typeHashHex || summon?.typeHash })
}

function combinedIcon(skill) {
  return traitAssetIcon({ internalId: skill?.traitId, name: skill?.name })
}
</script>

<template>
  <article class="captured-loadout-preview" :aria-label="tx('捕获配装预览', 'Captured Loadout Preview')">
    <header class="preview-identity">
      <div class="preview-character-icon">
        <img v-if="characterIcon" :src="characterIcon" alt="" />
        <span v-else aria-hidden="true">◇</span>
      </div>
      <div class="preview-title">
        <small>{{ sourceLabel || tx('配装预览', 'Loadout Preview') }}</small>
        <h3>{{ loadout.characterName || tx('未识别角色', 'Unknown Character') }}</h3>
        <p>{{ loadout.weapon?.name || tx('未记录武器', 'Weapon Not Recorded') }} · {{ sigilCaptureLabel }} · {{ summonCaptureLabel }} · {{ masteryCaptureLabel }}</p>
      </div>
      <div class="preview-actions"><slot name="actions" /></div>
    </header>

    <dl v-if="visibleStats.length" class="preview-stats">
      <div v-for="([label, value, kind]) in visibleStats" :key="label"><dt>{{ label }}</dt><dd>{{ formatValue(value, kind) }}</dd></div>
    </dl>

    <div class="preview-columns preview-main-columns">
      <section class="preview-section preview-weapon">
        <header>
          <img v-if="weaponIcon" :src="weaponIcon" alt="" />
          <div><small>{{ tx('装备武器', 'Equipped Weapon') }}</small><strong>{{ loadout.weapon?.name || tx('未记录武器', 'Not Recorded') }}</strong></div>
        </header>
        <div v-if="loadout.weapon?.name" class="weapon-levels">
          <span>Lv{{ loadout.weapon.level || '—' }}</span>
          <span>{{ tx('觉醒', 'Awakening') }} {{ loadout.weapon.awakeningLevel || 0 }}</span>
          <span>{{ tx('强化', 'Plus Marks') }} +{{ loadout.weapon.plusMarks || 0 }}</span>
        </div>
        <div class="trait-group">
          <h4>{{ tx('武器技能', 'Weapon Skills') }}</h4>
          <p v-if="!(loadout.weapon?.skills || []).length" class="empty-line">{{ tx('未记录', 'Not Recorded') }}</p>
          <div v-for="skill in loadout.weapon?.skills || []" :key="`skill-${skill.hashHex || skill.hash}`" class="trait-row">
            <img v-if="traitIcon(skill)" :src="traitIcon(skill)" alt="" />
            <span>{{ skill.name || skill.hashHex }}</span><b>Lv{{ skill.level }}</b>
          </div>
        </div>
        <div class="trait-group">
          <h4>{{ tx('武器祝福', 'Wrightstone Traits') }}</h4>
          <p v-if="!(loadout.weapon?.traits || []).length" class="empty-line">{{ tx('未记录', 'Not Recorded') }}</p>
          <div v-for="trait in loadout.weapon?.traits || []" :key="`trait-${trait.hashHex || trait.hash}`" class="trait-row">
            <img v-if="traitIcon(trait)" :src="traitIcon(trait)" alt="" />
            <span>{{ trait.name || trait.hashHex }}</span><b>Lv{{ trait.level }}</b>
          </div>
        </div>
      </section>

      <section class="preview-section preview-equipment-detail">
        <header><div><small>{{ tx('角色配装', 'Character Loadout') }}</small><strong>{{ tx('技能、召唤石与专精', 'Abilities, Summons and Mastery') }}</strong></div></header>
        <div class="trait-group">
          <h4>{{ tx('角色技能', 'Character Abilities') }}</h4>
          <p v-if="!(loadout.abilities || []).length" class="empty-line">{{ tx('未记录', 'Not Recorded') }}</p>
          <div v-for="(ability, index) in loadout.abilities || []" :key="ability.hashHex || ability.hash" class="trait-row ability-row">
            <i>{{ index + 1 }}</i><span>{{ ability.name || ability.hashHex }}</span><b>{{ ability.key || '' }}</b>
          </div>
        </div>
        <div class="trait-group">
          <h4>{{ tx('召唤石', 'Summons') }}</h4>
          <p v-if="!(loadout.summons || []).length" class="empty-line">{{ tx('未记录', 'Not Recorded') }}</p>
          <article v-for="summon in loadout.summons || []" :key="`${summon.index}-${summon.typeHashHex}`" class="summon-row">
            <img v-if="summonIcon(summon)" :src="summonIcon(summon)" alt="" />
            <i v-else>{{ summon.index + 1 }}</i>
            <div><strong>{{ summon.name || summon.typeHashHex }}</strong><span>{{ summon.mainTraitName }} Lv{{ summon.mainTraitLevel }} · {{ summon.subParamName }} Lv{{ summon.subParamLevel }}</span></div>
          </article>
        </div>
        <div class="trait-group">
          <h4>{{ tx('角色上限突破', 'Overmastery') }}</h4>
          <p v-if="!overLimitRecorded" class="empty-line">{{ tx('未记录', 'Not Recorded') }}</p>
          <div v-else class="overlimit-grid">
            <span v-for="slot in loadout.overLimit || []" :key="slot.index" :class="{ empty: !slot.name }">
              <i>{{ slot.index + 1 }}</i><b>{{ slot.name || tx('空槽', 'Empty') }}</b><em v-if="slot.level">Lv{{ slot.level }}</em>
            </span>
          </div>
        </div>
        <div class="mastery-block">
          <div class="mastery-heading">
            <div><small>{{ tx('专精配置', 'Mastery') }}</small><strong>{{ loadout.masteryAvailable ? (loadout.masterySummary?.primaryLabel || tx('未判定方向', 'Direction Not Resolved')) : tx('未记录', 'Not Recorded') }}</strong></div>
            <b>{{ masteryCaptureLabel }}</b>
          </div>
          <p v-if="!loadout.masteryAvailable" class="mastery-warning">{{ loadout.masteryUnavailableReason || tx('当前记录无法解析专精', 'Mastery unavailable for this record') }}</p>
          <template v-else>
            <div class="mastery-ranks">
              <span v-for="rank in masteryRanks" :key="rank.rank"><b>{{ rank.rank }}</b><em>{{ rank.count }}/{{ rank.cap }}</em></span>
            </div>
            <details v-if="(loadout.mastery || []).length" class="mastery-nodes">
              <summary>{{ tx('查看全部专精节点', 'View all mastery nodes') }} <b>{{ (loadout.mastery || []).length }}</b></summary>
              <div><article v-for="node in loadout.mastery || []" :key="node.hash"><i>{{ node.rank }}</i><span><strong>{{ node.name || node.rankLabel }}</strong><small>{{ node.desc }}</small></span></article></div>
            </details>
          </template>
        </div>
      </section>

      <section class="preview-section preview-combined">
        <header><div><small>{{ tx('合并技能等级', 'Combined Skill Levels') }}</small><strong>{{ (loadout.combinedSkills || []).length }} {{ tx('项已生效技能', 'active skills') }}</strong></div></header>
        <p v-if="!(loadout.combinedSkills || []).length" class="empty-line combined-empty">{{ tx('没有可合并的已记录技能', 'No recorded skills can be merged') }}</p>
        <article v-for="skill in loadout.combinedSkills || []" :key="skill.traitId" class="combined-row">
          <img v-if="combinedIcon(skill)" :src="combinedIcon(skill)" alt="" />
          <i v-else aria-hidden="true">◇</i>
          <div><strong>{{ skill.name || skill.traitId }}</strong><small v-if="skill.effect">{{ skill.effect }}</small><small v-else-if="skill.warning">{{ skill.warning }}</small></div>
          <span><b>Lv{{ skill.level }}</b><em v-if="skill.rawLevel > skill.level">/{{ skill.rawLevel }}</em></span>
        </article>
      </section>
    </div>

    <section class="preview-section preview-sigils">
      <header><div><small>{{ tx('装备因子', 'Equipped Sigils') }}</small><strong>{{ (loadout.sigils || []).length ? `${(loadout.sigils || []).length} / 12` : tx('未记录', 'Not Recorded') }}</strong></div></header>
      <p v-if="!(loadout.sigils || []).length" class="empty-line">{{ tx('未记录', 'Not Recorded') }}</p>
      <div class="sigil-grid">
        <article v-for="sigil in loadout.sigils || []" :key="`${sigil.index}-${sigil.hashHex || sigil.hash}`" class="sigil-card">
          <img v-if="sigilIcon(sigil)" :src="sigilIcon(sigil)" alt="" />
          <span class="sigil-slot">{{ sigil.index + 1 }}</span>
          <div class="sigil-copy">
            <strong>{{ sigil.name || sigil.hashHex }}</strong>
            <span>{{ sigil.primaryTraitName }} <b>Lv{{ sigil.primaryTraitLevel }}</b></span>
            <span v-if="sigil.secondaryTraitName">{{ sigil.secondaryTraitName }} <b>Lv{{ sigil.secondaryTraitLevel }}</b></span>
          </div>
          <em>Lv{{ sigil.level }}</em>
        </article>
      </div>
    </section>
  </article>
</template>

<style scoped>
.captured-loadout-preview { width:100%; min-width:0; display:grid; gap:var(--space-4); container:captured-preview / inline-size; color:var(--text-primary); }
.preview-identity { min-width:0; display:grid; grid-template-columns:56px minmax(0,1fr) auto; gap:var(--space-3); align-items:center; padding:var(--space-4); border:1px solid var(--border-strong); border-left:4px solid var(--accent); border-radius:var(--radius-md); background:var(--surface-card-pop); box-shadow:var(--shadow-2); }
.preview-character-icon { width:56px; height:56px; display:grid; place-items:center; overflow:hidden; border:1px solid var(--border-strong); border-radius:var(--radius-sm); background:var(--surface-sunken); color:var(--accent); font-size:1.4rem; }
.preview-character-icon img { width:100%; height:100%; object-fit:cover; }
.preview-title { min-width:0; }
.preview-title small { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.preview-title h3 { margin:2px 0; overflow-wrap:anywhere; font-family:var(--font-display); font-size:var(--fs-xl); }
.preview-title p { margin:0; color:var(--text-muted); font-size:var(--fs-sm); overflow-wrap:anywhere; }
.preview-actions { min-width:0; display:flex; flex-wrap:wrap; justify-content:flex-end; gap:var(--space-2); }
.preview-actions:empty { display:none; }
.preview-stats { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,120px),1fr)); gap:var(--space-2); margin:0; }
.preview-stats > div { min-width:0; padding:var(--space-3); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.preview-stats dt { color:var(--text-muted); font-size:var(--fs-xs); }
.preview-stats dd { min-width:0; margin:2px 0 0; overflow-wrap:anywhere; color:var(--text-primary); font-family:var(--font-data); font-size:var(--fs-md); font-weight:var(--fw-bold); }
.preview-columns { min-width:0; display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,280px),1fr)); gap:var(--space-4); align-items:start; }
.preview-main-columns { grid-template-columns:minmax(250px,.92fr) minmax(290px,1.08fr) minmax(260px,1fr); }
.preview-section { min-width:0; padding:var(--space-4); border:1px solid var(--border-soft); border-radius:var(--radius-md); background:var(--surface-card-pop); }
.preview-section > header { min-width:0; display:flex; align-items:center; gap:var(--space-3); padding-bottom:var(--space-3); border-bottom:1px solid var(--border-soft); }
.preview-section > header img { width:64px; height:44px; flex:0 0 auto; object-fit:contain; }
.preview-section > header div { min-width:0; }
.preview-section > header small,.preview-section > header strong { display:block; min-width:0; overflow-wrap:anywhere; }
.preview-section > header small { color:var(--text-muted); font-size:var(--fs-xs); }
.preview-section > header strong { margin-top:2px; }
.weapon-levels { display:flex; flex-wrap:wrap; gap:var(--space-2); margin-top:var(--space-3); }
.weapon-levels span { padding:3px 8px; border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); color:var(--text-secondary); font-size:var(--fs-xs); }
.trait-group { min-width:0; margin-top:var(--space-3); }
.trait-group h4 { margin:0 0 var(--space-2); color:var(--text-muted); font-size:var(--fs-xs); }
.trait-row { min-width:0; display:grid; grid-template-columns:28px minmax(0,1fr) auto; gap:var(--space-2); align-items:center; padding:5px 0; border-top:1px solid var(--border-soft); }
.trait-row img { width:28px; height:28px; border-radius:6px; object-fit:cover; }
.trait-row span { min-width:0; overflow-wrap:anywhere; color:var(--text-secondary); font-size:var(--fs-sm); }
.trait-row b { color:var(--accent); font-size:var(--fs-xs); white-space:nowrap; }
.ability-row { grid-template-columns:28px minmax(0,1fr); }
.ability-row i { width:25px; height:25px; display:grid; place-items:center; border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--accent-soft); color:var(--accent-hover); font-size:var(--fs-xs); font-style:normal; font-weight:var(--fw-bold); }
.ability-row b { display:none; }
.empty-line { margin:0; color:var(--text-muted); font-size:var(--fs-xs); }
.summon-row { min-width:0; display:grid; grid-template-columns:42px minmax(0,1fr); gap:var(--space-2); align-items:center; padding:7px 0; border-top:1px solid var(--border-soft); }
.summon-row > img,.summon-row > i { width:42px; height:42px; display:grid; place-items:center; border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); object-fit:cover; color:var(--accent); font-style:normal; }
.summon-row div { min-width:0; }
.summon-row strong,.summon-row span { display:block; min-width:0; overflow-wrap:anywhere; }
.summon-row strong { font-size:var(--fs-xs); }
.summon-row span { margin-top:2px; color:var(--text-muted); font-size:var(--fs-2xs); line-height:1.4; }
.overlimit-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-2); margin-top:var(--space-3); }
.overlimit-grid span { min-width:0; display:grid; grid-template-columns:22px minmax(0,1fr) auto; align-items:center; gap:var(--space-2); padding:var(--space-2); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.overlimit-grid span.empty { opacity:.58; }
.overlimit-grid i { width:22px; height:22px; display:grid; place-items:center; border-radius:50%; background:var(--accent-soft); color:var(--accent-hover); font-size:var(--fs-xs); font-style:normal; }
.overlimit-grid b { min-width:0; overflow-wrap:anywhere; font-size:var(--fs-xs); }
.overlimit-grid em { color:var(--accent); font-size:var(--fs-xs); font-style:normal; white-space:nowrap; }
.mastery-block { min-width:0; margin-top:var(--space-4); padding-top:var(--space-3); border-top:1px solid var(--border-soft); }
.mastery-heading { min-width:0; display:flex; align-items:center; justify-content:space-between; gap:var(--space-3); }
.mastery-heading div { min-width:0; }
.mastery-heading small,.mastery-heading strong { display:block; min-width:0; overflow-wrap:anywhere; }
.mastery-heading small { color:var(--text-muted); font-size:var(--fs-xs); }
.mastery-heading strong { margin-top:2px; font-size:var(--fs-sm); }
.mastery-heading > b { flex:0 0 auto; color:var(--accent); font-family:var(--font-data); font-size:var(--fs-sm); }
.mastery-warning { margin:var(--space-2) 0 0; padding:var(--space-2); border:1px solid var(--border-warning); border-radius:var(--radius-sm); color:var(--text-secondary); font-size:var(--fs-xs); overflow-wrap:anywhere; }
.mastery-ranks { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:var(--space-2); margin-top:var(--space-3); }
.mastery-ranks span { min-width:0; display:grid; justify-items:center; gap:1px; padding:6px 3px; border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.mastery-ranks b { color:var(--accent); font-size:var(--fs-xs); }
.mastery-ranks em { color:var(--text-secondary); font-family:var(--font-data); font-size:var(--fs-2xs); font-style:normal; }
.mastery-nodes { margin-top:var(--space-3); }
.mastery-nodes summary { display:flex; align-items:center; justify-content:space-between; gap:var(--space-2); padding:7px 9px; border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); color:var(--text-secondary); font-size:var(--fs-xs); cursor:pointer; }
.mastery-nodes summary b { color:var(--accent); }
.mastery-nodes > div { max-height:280px; overflow:auto; margin-top:var(--space-2); padding-right:3px; scrollbar-width:thin; }
.mastery-nodes article { min-width:0; display:grid; grid-template-columns:28px minmax(0,1fr); gap:var(--space-2); padding:6px 0; border-bottom:1px solid var(--border-soft); }
.mastery-nodes article > i { color:var(--accent); font-size:var(--fs-2xs); font-style:normal; font-weight:var(--fw-bold); }
.mastery-nodes article span,.mastery-nodes article strong,.mastery-nodes article small { display:block; min-width:0; overflow-wrap:anywhere; }
.mastery-nodes article strong { font-size:var(--fs-xs); }
.mastery-nodes article small { margin-top:2px; color:var(--text-muted); font-size:var(--fs-2xs); line-height:1.45; }
.preview-combined { align-self:stretch; }
.combined-empty { margin-top:var(--space-3); }
.combined-row { min-width:0; display:grid; grid-template-columns:34px minmax(0,1fr) auto; gap:var(--space-2); align-items:start; padding:8px 0; border-bottom:1px solid var(--border-soft); }
.combined-row > img,.combined-row > i { width:34px; height:34px; border-radius:var(--radius-sm); object-fit:cover; }
.combined-row > i { display:grid; place-items:center; border:1px solid var(--border-soft); background:var(--surface-sunken); color:var(--accent); font-style:normal; }
.combined-row > div { min-width:0; }
.combined-row strong,.combined-row small { display:block; min-width:0; overflow-wrap:anywhere; }
.combined-row strong { font-size:var(--fs-sm); }
.combined-row small { margin-top:2px; color:var(--text-muted); font-size:var(--fs-2xs); line-height:1.4; }
.combined-row > span { display:flex; align-items:baseline; color:var(--accent); font-family:var(--font-data); font-size:var(--fs-xs); white-space:nowrap; }
.combined-row > span em { color:var(--text-muted); font-style:normal; }
.preview-sigils > header { justify-content:space-between; }
.sigil-grid { min-width:0; display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,280px),1fr)); gap:var(--space-2); margin-top:var(--space-3); }
.sigil-card { min-width:0; display:grid; grid-template-columns:36px 20px minmax(0,1fr) auto; align-items:start; gap:var(--space-2); padding:var(--space-3); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.sigil-card > img { width:36px; height:36px; border-radius:7px; object-fit:cover; }
.sigil-slot { width:20px; height:20px; display:grid; place-items:center; border-radius:50%; background:var(--accent-soft); color:var(--accent-hover); font-size:var(--fs-2xs); }
.sigil-copy { min-width:0; display:grid; gap:2px; }
.sigil-copy strong,.sigil-copy span { min-width:0; overflow-wrap:anywhere; }
.sigil-copy strong { font-size:var(--fs-sm); }
.sigil-copy span { color:var(--text-secondary); font-size:var(--fs-xs); }
.sigil-copy b { color:var(--accent); }
.sigil-card > em { color:var(--text-muted); font-size:var(--fs-xs); font-style:normal; white-space:nowrap; }
@container captured-preview (max-width:720px) {
  .preview-identity { grid-template-columns:48px minmax(0,1fr); }
  .preview-character-icon { width:48px; height:48px; }
  .preview-actions { grid-column:1/-1; justify-content:stretch; }
  .preview-actions :deep(.ui-btn) { flex:1 1 140px; }
  .overlimit-grid { grid-template-columns:minmax(0,1fr); }
}
@container captured-preview (max-width:860px) {
  .preview-main-columns { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .preview-combined { grid-column:1/-1; }
}
@container captured-preview (max-width:720px) {
  .preview-main-columns { grid-template-columns:minmax(0,1fr); }
  .preview-combined { grid-column:auto; }
}
@container captured-preview (max-width:420px) {
  .sigil-card { grid-template-columns:32px minmax(0,1fr) auto; }
  .sigil-card > img { width:32px; height:32px; }
  .sigil-slot { display:none; }
}
</style>
