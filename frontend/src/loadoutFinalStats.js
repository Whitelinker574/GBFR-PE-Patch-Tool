export function formatFinalStat(value, unit = '') {
  if (value === null || value === undefined || !Number.isFinite(Number(value))) return '—'
  const numeric = Number(value)
  const sign = unit === 'signedPct' && numeric > 0 ? '+' : unit === 'signedPct' && numeric < 0 ? '−' : ''
  const formatted = Math.abs(numeric).toLocaleString('zh-CN', { maximumFractionDigits: 2 })
  return `${sign}${formatted}${unit === 'pct' || unit === 'signedPct' ? '%' : ''}`
}

export function formatWeaponSkillLevel(skill) {
  const value = skill?.level ?? skill?.effectiveLevel
  if (value === null || value === undefined || !Number.isFinite(Number(value))) return 'Lv—'
  return `Lv${Number(value).toLocaleString('zh-CN', { maximumFractionDigits: 2 })}`
}

export function groupEffectTotals(totals = []) {
  const groups = new Map()
  for (const total of totals) {
    const label = total?.label || total?.key || '未命名加成'
    let group = groups.get(label)
    if (!group) {
      group = {
        key: `display|${label}`,
        label,
        catLabel: total?.catLabel || '',
        sources: [],
        parts: [],
      }
      groups.set(label, group)
    }
    for (const source of total?.sources || []) {
      if (source && !group.sources.includes(source)) group.sources.push(source)
    }
    const unit = total?.unit || 'flat'
    let part = group.parts.find(item => item.unit === unit)
    if (!part) {
      part = { unit, value: 0 }
      group.parts.push(part)
    }
    part.value += Number(total?.value || 0)
  }
  return [...groups.values()]
}

export function summarizeTraitLevels(bonuses = []) {
  return bonuses.reduce((summary, bonus) => {
    const effective = Math.max(0, Number(bonus?.level) || 0)
    const invested = Math.max(effective, Number(bonus?.rawLevel) || 0)
    const overflow = Math.max(0, invested - effective)
    summary.effective += effective
    summary.invested += invested
    summary.overflow += overflow
    if (overflow > 0 || bonus?.capped) summary.cappedCount += 1
    return summary
  }, { effective: 0, invested: 0, overflow: 0, cappedCount: 0 })
}
