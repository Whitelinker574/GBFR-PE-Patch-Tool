export const LOADOUT_ROUTE_BRANCH_VERSION = '2.0.2-graduation-branches-1'

const item = (traitId, targetLevel, reasonZh, reasonEn, extra = {}) => Object.freeze({
  traitId,
  targetLevel,
  reasonZh,
  reasonEn,
  ...extra,
})

const branch = (id, nameZh, nameEn, summaryZh, summaryEn, additions, extra = {}) => Object.freeze({
  id,
  nameZh,
  nameEn,
  summaryZh,
  summaryEn,
  additions: Object.freeze(additions),
  ...extra,
})

export const LOADOUT_ROUTE_BRANCHES = Object.freeze([
  branch(
    'graduation',
    '原版毕业',
    'Verified Graduation',
    '完整保留视频逐帧核对的 12 个因子槽，不做方向性替换。',
    'Keeps all twelve frame-verified sigil slots without directional replacements.',
    [],
    { actionType: '' },
  ),
  branch(
    'celestial-high-hp',
    '天星攻限 · 高血',
    'Celestial Offense · High HP',
    '保留角色专属核心，用高血天星、无条件上限与毕业任务增伤替换弹性槽。天星之界会降低最大 HP，天星之止息会阻止同伴为你恢复 HP。',
    'Keeps character-specific cores and replaces flexible slots with high-HP Celestial, unconditional cap, and endgame-task damage traits. Celestial Terra lowers max HP and Celestial Ventus blocks companion healing.',
    [
      item('MEMORY_TRAIT_A7726190', 15, '天星之煌在 HP≥75% 时提高攻击与伤害上限。', 'Celestial Lumen raises attack and damage cap at 75% HP or higher.', { condition: 'high-hp-75' }),
      item('MEMORY_TRAIT_9232DC17', 15, '天星之界以最大 HP 降低 30% 为代价提供无条件伤害上限。', 'Celestial Terra grants unconditional damage cap at the cost of 30% max HP.'),
      item('MEMORY_TRAIT_73220725', 15, '天星之止息提高伤害，但会阻止同伴为自己恢复 HP。', 'Celestial Ventus raises damage but blocks companion healing.'),
      item('MEMORY_TRAIT_D029FE08', 15, '浪迹天涯在无尽黄昏等毕业任务中提供攻击、上限和防御。', 'Fatebreaker grants attack, cap, and defense in endgame quests such as Endless Twilight.', { condition: 'endgame-quest' }),
    ],
    {
      actionType: 'normal',
      conflicts: Object.freeze(['MEMORY_TRAIT_0DE887A0']),
      keepTraitIds: Object.freeze(['SKILL_020_00', 'SKILL_146_00', 'SKILL_151_00', 'SKILL_233_00', 'SKILL_234_00']),
    },
  ),
  branch(
    'celestial-low-hp',
    '天星攻限 · 低血',
    'Celestial Offense · Low HP',
    '保留角色专属核心，把高血触发改为低血触发；适合能稳定压低 HP 的路线，不会把高血与低血天星同时算作常驻。',
    'Keeps character-specific cores and swaps the high-HP trigger for the low-HP version. Intended for routes that reliably stay at low HP; high- and low-HP Celestial effects are never treated as simultaneously active.',
    [
      item('MEMORY_TRAIT_0DE887A0', 15, '天星之炼在 HP≤25% 时提高攻击与伤害上限。', 'Celestial Nyx raises attack and damage cap at 25% HP or lower.', { condition: 'low-hp-25' }),
      item('MEMORY_TRAIT_9232DC17', 15, '天星之界以最大 HP 降低 30% 为代价提供无条件伤害上限。', 'Celestial Terra grants unconditional damage cap at the cost of 30% max HP.'),
      item('MEMORY_TRAIT_73220725', 15, '天星之止息提高伤害，但会阻止同伴为自己恢复 HP。', 'Celestial Ventus raises damage but blocks companion healing.'),
      item('MEMORY_TRAIT_D029FE08', 15, '浪迹天涯在无尽黄昏等毕业任务中提供攻击、上限和防御。', 'Fatebreaker grants attack, cap, and defense in endgame quests such as Endless Twilight.', { condition: 'endgame-quest' }),
    ],
    {
      actionType: 'normal',
      conflicts: Object.freeze(['MEMORY_TRAIT_A7726190', 'SKILL_006_00', 'SKILL_144_00']),
      keepTraitIds: Object.freeze(['SKILL_020_00', 'SKILL_146_00', 'SKILL_151_00', 'SKILL_233_00', 'SKILL_234_00', 'SKILL_036_00']),
    },
  ),
  branch(
    'defense',
    '防御特化',
    'Defense Focus',
    '金刚与体力抬高生命，刚健覆盖满血、坚守覆盖低血，坚持负责霸体承伤；不会只堆一个数值后把所有场景都叫“坦克”。',
    'Greater Aegis and HP raise health, Stronghold covers full HP, Garrison covers low HP, and Steel Nerves reduces damage during Stout Heart. It does not call a single inflated number a universal tank build.',
    [
      item('SKILL_166_00', 15, '金刚提高最大 HP。', 'Greater Aegis raises max HP.'),
      item('SKILL_001_00', 15, '体力补充基础 HP。', 'HP raises the base health pool.'),
      item('SKILL_144_00', 15, '刚健覆盖满血阶段的防御。', 'Stronghold covers the full-HP defense window.', { condition: 'high-hp' }),
      item('SKILL_036_00', 15, '坚守在失血后接管防御区间。', 'Garrison takes over the defense window after losing HP.', { condition: 'low-hp' }),
      item('SKILL_096_00', 15, '坚持降低霸体期间受到的伤害。', 'Steel Nerves reduces damage taken during Stout Heart.', { condition: 'stout-heart' }),
    ],
    {
      actionType: 'survival',
      keepTraitIds: Object.freeze(['SKILL_085_00', 'SKILL_087_00', 'SKILL_060_00', 'SKILL_141_00', 'BF78FBFC', '46EE3116']),
    },
  ),
  branch(
    'stun',
    '昏厥特化',
    'Stun Focus',
    '把 3 个弹性槽固定为昏厥 Lv45，其他槽仍沿用该角色毕业核心；适合需要更快打入昏厥窗口的队伍。',
    'Fixes three flexible slots to Stun Power Lv45 while keeping the character graduation core for teams that want faster stun windows.',
    [
      item('SKILL_004_00', 45, '3 枚昏厥 V+ 合计达到 Lv45。', 'Three Stun Power V+ sigils reach Lv45.', { slotCount: 3 }),
    ],
    {
      actionType: 'stun',
      keepTraitIds: Object.freeze(['SKILL_020_00', 'SKILL_146_00', 'SKILL_151_00']),
    },
  ),
  branch(
    'sustain',
    '续航生存',
    'Sustain',
    'HP 吸收负责持续回血，分歧提供恢复与异常净化，明镜止水和躲避性能把精准闪避转成循环与容错。',
    'Drain supplies ongoing healing, Divergence adds regeneration and cleansing, and Nimble Onslaught plus Improved Dodge turn precise dodges into uptime and safety.',
    [
      item('SKILL_067_00', 30, '2 枚 HP 吸收 V+ 提供持续回血。', 'Two Drain V+ sigils provide sustained healing.', { slotCount: 2 }),
      item('MEMORY_TRAIT_F26BAEA5', 15, '分歧提供持续恢复，并有概率解除弱化效果。', 'Divergence provides regeneration and a chance to cleanse debuffs.'),
      item('SKILL_106_00', 15, '明镜止水强化精准闪避后的冷却与奥义循环。', 'Nimble Onslaught improves cooldown and SBA cycling after a perfect dodge.'),
      item('SKILL_063_00', 15, '躲避性能扩大闪避窗口并增加连续躲避次数。', 'Improved Dodge widens the dodge window and adds consecutive dodges.'),
    ],
    {
      actionType: 'survival',
      keepTraitIds: Object.freeze(['SKILL_001_00', 'SKILL_085_00', 'SKILL_166_00', 'SKILL_036_00', 'SKILL_144_00']),
    },
  ),
  branch(
    'revive-potions',
    '药剂复活',
    'Potions & Revive',
    '豪胆、自动复活和药水携带数构成倒地保险；万能药用于处理弱化。这个方向优先保命，不会伪装成最高伤害路线。',
    'Guts, Autorevive, and Potion Hoarder form a recovery safety net, while Potent Greens handles debuffs. This is a survival branch, not a maximum-damage claim.',
    [
      item('SKILL_045_00', 15, '豪胆保留一次致死伤害后的生存机会。', 'Guts preserves a survival chance after lethal damage.'),
      item('SKILL_068_00', 15, '自动复活缩短倒地后的回场时间。', 'Autorevive returns the character after being downed.'),
      item('SKILL_073_00', 15, '药水携带数增加可用药剂。', 'Potion Hoarder increases available potions.'),
      item('SKILL_023_00', 15, '万能药缩短弱化持续时间；天然单技能壳不会虚构副词条。', 'Potent Greens shortens debuff duration; its natural single-trait shell does not invent a secondary.'),
    ],
    {
      actionType: 'survival',
      keepTraitIds: Object.freeze(['SKILL_001_00', 'SKILL_085_00', 'SKILL_166_00', 'SKILL_096_00']),
    },
  ),
  branch(
    'dodge-loop',
    '闪避循环',
    'Dodge Loop',
    '围绕精准闪避后的明镜止水、躲避性能和迅捷能力重组；摇曳步只作为明确牺牲攻击换容错的槽位，不会冒充无代价增益。',
    'Rebuilds around Nimble Onslaught, Improved Dodge, and Quick Cooldown after perfect dodges. Flight over Fight is explicitly treated as an offense-for-safety trade, never as a free gain.',
    [
      item('SKILL_106_00', 15, '明镜止水把精准闪避转化为冷却和奥义收益。', 'Nimble Onslaught converts perfect dodges into cooldown and SBA gains.'),
      item('SKILL_063_00', 15, '躲避性能扩大窗口并增加连续躲避次数。', 'Improved Dodge widens the window and adds consecutive dodges.'),
      item('SKILL_069_00', 15, '迅捷能力继续压缩技能循环空档。', 'Quick Cooldown further shortens ability downtime.'),
      item('SKILL_159_00', 15, '摇曳步以显著攻击代价换取自动精准闪避，应用前必须确认取舍。', 'Flight over Fight trades substantial offense for automatic perfect dodges and must be accepted as a deliberate tradeoff.'),
    ],
    {
      actionType: 'cooldown',
      keepTraitIds: Object.freeze(['SKILL_070_00', 'SKILL_096_00']),
    },
  ),
  branch(
    'black-crab',
    '漆黑钳蟹组合',
    'Dread Black Pincer Crab Set',
    '同时装入可怕的漆黑钳蟹因子+与漆黑之谊+，满足两件套条件；两枚合法壳都固定带“螃蟹的报恩”，不会只塞一枚就宣称套装生效。',
    'Equips both Dread Black Pincer Crab Sigil+ and Blackened Bond+ to satisfy the two-piece condition. Both legal shells include Crabvestment Returns; a single piece is never presented as an active set.',
    [
      item('BF78FBFC', 20, '漆黑钳蟹套装的第一项，并固定带螃蟹的报恩。', 'The first crab-set trait with fixed Crabvestment Returns.', { sigilId: '66CB28BA', secondaryTraitId: 'SKILL_141_00' }),
      item('46EE3116', 15, '漆黑钳蟹套装的第二项，并固定带螃蟹的报恩。', 'The second crab-set trait with fixed Crabvestment Returns.', { sigilId: '76786869', secondaryTraitId: 'SKILL_141_00' }),
    ],
    {
      actionType: 'normal',
      keepTraitIds: Object.freeze(['SKILL_020_00', 'SKILL_146_00', 'SKILL_151_00', 'SKILL_166_00', 'SKILL_144_00', 'SKILL_096_00']),
    },
  ),
])

const slotCount = item => Math.max(1, Math.floor(Number(item?.slotCount || 1)))

function isCharacterCore(item, atlas) {
  const trait = (atlas?.traits || []).find(entry => entry.internalId === item?.traitId)
  if (trait?.category === 'character_trait') return true
  const match = String(item?.traitId || '').match(/^SKILL_(\d{3})_(?:00|01|02)$/)
  const index = Number(match?.[1] || 0)
  return (index >= 114 && index <= 139) || (index >= 172 && index <= 178)
}

function expandItem(item, atlas, origin, sourceIndex) {
  const count = slotCount(item)
  const level = Math.max(1, Math.ceil(Number(item?.targetLevel || 1) / count))
  const exactSigilIds = item?.exactSigilIds || []
  const exactSecondaryTraitIds = item?.exactSecondaryTraitIds || []
  return Array.from({ length: count }, (_, index) => ({
    ...item,
    targetLevel: level,
    slotCount: 1,
    sigilId: exactSigilIds[index] || item?.sigilId || '',
    secondaryTraitId: exactSecondaryTraitIds[index] || item?.secondaryTraitId || '',
    exactSigilIds: undefined,
    exactSecondaryTraitIds: undefined,
    origin,
    sourceIndex,
    characterCore: isCharacterCore(item, atlas),
  }))
}

function exactSlotMatch(slot, wanted) {
  if (slot.traitId !== wanted.traitId) return false
  if (wanted.sigilId && slot.sigilId !== wanted.sigilId) return false
  if (wanted.secondaryTraitId && slot.secondaryTraitId !== wanted.secondaryTraitId) return false
  return Number(slot.targetLevel || 0) >= Number(wanted.targetLevel || 0)
}

function aggregateSlots(slots) {
  const rows = []
  const byKey = new Map()
  for (const slot of slots) {
    const key = [
      slot.traitId,
      slot.sigilId || '',
      slot.secondaryTraitId || '',
      slot.reasonZh || '',
      slot.reasonEn || '',
      slot.origin || '',
    ].join('|')
    const current = byKey.get(key)
    if (current) {
      current.slotCount += 1
      current.targetLevel += Number(slot.targetLevel || 0)
      continue
    }
    const row = {
      ...slot,
      targetLevel: Number(slot.targetLevel || 0),
      slotCount: 1,
    }
    delete row.sourceIndex
    delete row.characterCore
    if (!row.sigilId) delete row.sigilId
    if (!row.secondaryTraitId) delete row.secondaryTraitId
    byKey.set(key, row)
    rows.push(row)
  }
  return rows
}

function countTraits(slots) {
  const counts = new Map()
  for (const slot of slots) counts.set(slot.traitId, (counts.get(slot.traitId) || 0) + 1)
  return counts
}

function replacementRows(baseSlots, selectedSlots, atlas) {
  const traitById = new Map((atlas?.traits || []).map(item => [item.internalId, item]))
  const before = countTraits(baseSlots)
  const after = countTraits(selectedSlots)
  const ids = new Set([...before.keys(), ...after.keys()])
  return [...ids].map(traitId => {
    const removed = Math.max(0, Number(before.get(traitId) || 0) - Number(after.get(traitId) || 0))
    const added = Math.max(0, Number(after.get(traitId) || 0) - Number(before.get(traitId) || 0))
    return {
      traitId,
      name: traitById.get(traitId)?.displayName || traitId,
      removed,
      added,
    }
  }).filter(item => item.removed || item.added)
}

function buildBranchRoute(route, branchSpec, atlas) {
  const baseSlots = (route?.required || []).flatMap((entry, index) => expandItem(entry, atlas, 'base', index))
  if (branchSpec.id === 'graduation') {
    return {
      ...route,
      branchId: branchSpec.id,
      branchNameZh: branchSpec.nameZh,
      branchNameEn: branchSpec.nameEn,
      branchSummaryZh: branchSpec.summaryZh,
      branchSummaryEn: branchSpec.summaryEn,
      baseRouteId: route.id,
      derived: false,
      preservedSlots: baseSlots.length,
      replacedSlots: 0,
      replacements: [],
    }
  }

  const desiredSlots = branchSpec.additions.flatMap((entry, index) => expandItem(entry, atlas, 'branch', index))
  const selected = []
  const usedBase = new Set()

  for (let index = 0; index < baseSlots.length; index++) {
    if (!baseSlots[index].characterCore) continue
    selected.push(baseSlots[index])
    usedBase.add(index)
  }

  for (const wanted of desiredSlots) {
    const matchingBaseIndex = baseSlots.findIndex((slot, index) => !usedBase.has(index) && exactSlotMatch(slot, wanted))
    if (matchingBaseIndex >= 0) {
      selected.push(baseSlots[matchingBaseIndex])
      usedBase.add(matchingBaseIndex)
    } else if (selected.length < 12) {
      selected.push(wanted)
    }
  }

  const conflicts = new Set(branchSpec.conflicts || [])
  const keep = new Set(branchSpec.keepTraitIds || [])
  const remaining = baseSlots
    .map((slot, index) => ({ slot, index }))
    .filter(item => !usedBase.has(item.index) && !conflicts.has(item.slot.traitId))
    .sort((left, right) => Number(keep.has(right.slot.traitId)) - Number(keep.has(left.slot.traitId))
      || Number(right.slot.characterCore) - Number(left.slot.characterCore)
      || left.slot.sourceIndex - right.slot.sourceIndex
      || left.index - right.index)

  for (const candidate of remaining) {
    if (selected.length >= 12) break
    selected.push(candidate.slot)
    usedBase.add(candidate.index)
  }
  for (let index = 0; selected.length < 12 && index < baseSlots.length; index++) {
    if (usedBase.has(index)) continue
    selected.push(baseSlots[index])
    usedBase.add(index)
  }

  const trimmed = selected.slice(0, 12)
  const preservedSlots = trimmed.filter(slot => slot.origin === 'base').length
  return {
    ...route,
    id: `${route.id}::${branchSpec.id}`,
    baseRouteId: route.id,
    branchId: branchSpec.id,
    branchNameZh: branchSpec.nameZh,
    branchNameEn: branchSpec.nameEn,
    branchSummaryZh: branchSpec.summaryZh,
    branchSummaryEn: branchSpec.summaryEn,
    nameZh: `${route.nameZh} · ${branchSpec.nameZh}`,
    nameEn: `${route.nameEn} · ${branchSpec.nameEn}`,
    summaryZh: branchSpec.summaryZh,
    summaryEn: branchSpec.summaryEn,
    actionType: branchSpec.actionType || route.actionType,
    required: aggregateSlots(trimmed),
    derived: true,
    preservedSlots,
    replacedSlots: Math.max(0, 12 - preservedSlots),
    replacements: replacementRows(baseSlots, trimmed, atlas),
    evidence: `${route.evidence}+derived-local-2.0.2-trait-effects`,
  }
}

export function graduationRouteBranches(route, atlas) {
  if (!route) return []
  return LOADOUT_ROUTE_BRANCHES.map(branchSpec => buildBranchRoute(route, branchSpec, atlas))
}
