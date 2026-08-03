import fs from 'node:fs'
import process from 'node:process'

import {
  buildCatalogCandidates,
  buildInventoryCandidates,
  buildOptimizerTargetCatalog,
  solveEquipmentAwareSuggestions,
} from '../src/loadoutOptimizer.js'

function hydrateAtlas(index) {
  const traits = Array.isArray(index?.traits) ? index.traits : []
  const sigils = (Array.isArray(index?.sigils) ? index.sigils : []).map(entry => ({
    ...entry,
    secondaryTraits: (entry.secondaryTraitIndexes || []).map((traitIndex, index) => ({
      ...traits[traitIndex],
      maxLevel: Number(entry.secondaryTraitMaxLevels?.[index] || 0),
    })),
  }))
  return {
    dataVersion: index?.dataVersion || '', traits, sigils,
    writableSecondaryTraits: Array.isArray(index?.writableSecondaryTraits) ? index.writableSecondaryTraits : [],
  }
}

function normalized(value) {
  return String(value || '').trim().toLocaleLowerCase('zh-CN')
}

function main() {
  const [, , bundlePath, resultPath] = process.argv
  if (!bundlePath || !resultPath) throw new Error('usage: qa_real_save_target_solver.mjs <bundle.json> <result.json>')
  const bundle = JSON.parse(fs.readFileSync(bundlePath, 'utf8'))
  const atlas = hydrateAtlas(bundle.atlas)
  const targetCatalog = buildOptimizerTargetCatalog(atlas, bundle.snapshot)
  const traitByName = new Map(targetCatalog.map(trait => [normalized(trait.displayName), trait]))
  const missingTargets = bundle.targets.filter(target => !traitByName.has(normalized(target.name))).map(target => target.name)
  if (missingTargets.length) throw new Error(`targets absent from the current atlas: ${missingTargets.join('、')}`)
  const targets = bundle.targets.map((target, index) => {
    const trait = traitByName.get(normalized(target.name))
    return {
      traitId: trait.internalId,
      name: trait.displayName,
      cap: Number(target.level),
      weight: Math.max(1, bundle.targets.length - index),
      screenshotName: target.name,
    }
  })
  const inventory = buildInventoryCandidates(bundle.context.sigils || [], targets, atlas)
  const inventoryFillers = buildInventoryCandidates(bundle.context.sigils || [], targets, atlas, { includeUnmatched: true })
  const catalog = buildCatalogCandidates(atlas, targets, bundle.context.ownerCode || '')
  const retainedSlotIDs = new Set((bundle.loadout.sigils || []).map(item => Number(item.slotId || 0)).filter(Boolean))
  const retained = inventory.filter(item => retainedSlotIDs.has(Number(item.slotId || 0))).map(item => ({ ...item, retained: true }))
  const fillerSigils = inventoryFillers.map(item => retainedSlotIDs.has(Number(item.slotId || 0)) ? { ...item, retained: true } : item)
  const candidates = []
  const seen = new Set()
  for (const item of [...retained, ...inventory, ...catalog]) {
    const key = String(item?.id || `${item?.source || ''}:${item?.slotId || 0}:${item?.hash || ''}`)
    if (seen.has(key)) continue
    seen.add(key)
    candidates.push(item)
  }

  const snapshot = structuredClone(bundle.snapshot)
  snapshot.domain = 'owned-first'
  const masteryBase = new Set((Array.isArray(snapshot.baseSelection?.mastery)
    ? snapshot.baseSelection.mastery
    : [snapshot.baseSelection?.mastery]).filter(Boolean).map(String))
  snapshot.stages = (snapshot.stages || []).map(stage => stage.key === 'mastery'
    ? { ...stage, options: (stage.options || []).filter(option => masteryBase.has(String(option.id))) }
    : stage).filter(stage => stage.key !== 'mastery' || stage.options.length)

  const results = solveEquipmentAwareSuggestions({
    snapshot,
    sigilCandidates: candidates,
    sigilSlotCount: 12,
    limit: 10,
    scenario: { mode: 'target', targets, domain: 'owned-first', baseSigils: retained, fillerSigils },
  })
  if (!results.length) throw new Error('the real-save solver returned no plans')
  const plan = results[0]
  const totals = new Map((plan.totals || []).map(item => [String(item.traitId), item]))
  const totalsByName = new Map((plan.totals || []).map(item => [normalized(item.name), item]))
  const targetResults = targets.map(target => {
    const total = totals.get(target.traitId) || totalsByName.get(normalized(target.name))
    return {
      traitId: total?.traitId || target.traitId,
      name: target.name,
      requested: target.cap,
      achieved: Number(total?.level || 0),
      effective: Number(total?.effective || 0),
      met: Number(total?.effective || 0) >= target.cap,
      sources: (plan.targetSources || []).find(item => item.traitId === target.traitId)?.sources || [],
    }
  })
  fs.writeFileSync(resultPath, JSON.stringify({
    plan,
    targetResults,
    inventoryCandidateCount: inventory.length,
    catalogCandidateCount: catalog.length,
    completedPrefix: targetResults.findIndex(item => !item.met) < 0
      ? targetResults.length
      : targetResults.findIndex(item => !item.met),
  }, null, 2))
  process.stdout.write(JSON.stringify({
    targets: targetResults.length,
    met: targetResults.filter(item => item.met).length,
    completedPrefix: targetResults.findIndex(item => !item.met) < 0 ? targetResults.length : targetResults.findIndex(item => !item.met),
    inventoryCandidates: inventory.length,
    catalogCandidates: catalog.length,
    picked: plan.picked?.length || 0,
    pickedPlan: (plan.picked || []).map(item => ({
      source: item.source,
      slotId: Number(item.slotId || 0),
      sigilId: item.sigilId || '',
      primary: item.primaryTraitId || item.traits?.[0]?.id || '',
      primaryHash: item.exactPrimaryTraitHash || '',
      secondary: item.secondaryTraitId || item.traits?.[1]?.id || '',
      secondaryHash: item.exactSecondaryTraitHash || '',
    })),
    equipment: plan.equipment?.map(item => `${item.stage}:${item.label || item.id}`) || [],
  }))
}

main()
