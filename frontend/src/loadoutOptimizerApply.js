export function optimizerEquipmentDraft(result) {
  const equipment = result?.applyPayload?.equipment || {}
  const first = key => Array.isArray(equipment[key]) ? equipment[key][0] : null
  const weapon = first('weapon')
  const mastery = first('mastery')
  const summons = Array.isArray(equipment.summons) ? equipment.summons : []
  const weaponSlotId = Number(weapon?.weaponSlotId || 0)
  const weaponSkillHashes = Array.isArray(weapon?.weaponSkillHashes) ? weapon.weaponSkillHashes.map(String).filter(Boolean).slice(0, 5) : []
  const summonSlotIds = summons.map(item => Number(item?.slotId || 0)).filter(Boolean).slice(0, 4)
  const summonEdits = summons.filter(item => item?.mainTraitHash && Number(item?.slotId || 0) > 0).map(item => ({
    slotId: Number(item.slotId),
    expectUnitId: Number(item.expectUnitId || 0),
    expectTypeHash: String(item.expectTypeHash || ''),
    expectMainTraitHash: String(item.expectMainTraitHash || ''),
    expectMainTraitLevel: Number(item.expectMainTraitLevel || 0),
    expectSubParamHash: String(item.expectSubParamHash || ''),
    expectSubParamLevel: Number(item.expectSubParamLevel || 0),
    expectRank: Number(item.expectRank || 0),
    mainTraitHash: String(item.mainTraitHash || ''),
    mainTraitLevel: Number(item.mainTraitLevel || 0),
    subParamHash: String(item.subParamHash || ''),
    subParamLevel: Number(item.subParamLevel || 0),
    rank: Number(item.rank || 0),
  }))
  const masteryHashes = Array.isArray(mastery?.nodeHashes) ? mastery.nodeHashes.map(String).filter(Boolean) : []
  return {
    changed: weaponSlotId > 0 || weaponSkillHashes.length > 0 || summonSlotIds.length > 0 || summonEdits.length > 0 || masteryHashes.length > 0,
    weaponSlotId,
    weaponSkillHashes,
    summonSlotIds,
    summonEdits,
    masteryUnitId: Number(mastery?.unitId || 0),
    masteryHashes,
  }
}
