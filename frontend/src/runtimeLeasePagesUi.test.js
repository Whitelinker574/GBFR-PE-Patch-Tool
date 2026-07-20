import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const contracts = {
  'SigilMemoryGenerator.vue': ['SigilMemoryAcquire', 'SigilMemoryRelease'],
  'SigilLoadoutRestore.vue': ['SigilMemoryAcquire', 'SigilMemoryRelease'],
  'SummonEditor.vue': ['CharaAcquire', 'CharaRelease'],
  'MiscTools.vue': ['CharaAcquire', 'CharaRelease'],
  'MonsterEnhance.vue': ['CharaAcquire', 'CharaRelease'],
  'WrightstoneMemoryGenerator.vue': ['WrightstoneMemoryAcquire', 'WrightstoneMemoryRelease'],
  'OverLimit.vue': ['OverLimitAcquire', 'OverLimitRelease'],
}

const sources = Object.fromEntries(Object.keys(contracts).map((name) => [
  name,
  readFileSync(new URL(`./components/${name}`, import.meta.url), 'utf8'),
]))

test('every owned runtime page shares request IDs and queues its exact token on unmount', () => {
  for (const [name, [acquire, release]] of Object.entries(contracts)) {
    const source = sources[name]
    assert.match(source, /from ['"]\.\.\/runtimeLeaseManager\.js['"]/, `${name}: shared lease manager import`)
    assert.match(source, new RegExp(`${acquire}\\(nextRuntimeAcquireRequestID\\(\\)\\)`), `${name}: acquire must use the global generation`)
    assert.match(
      source,
      new RegExp(`onBeforeUnmount\\(\\(\\) => \\{[\\s\\S]*?queueRuntimeLeaseRelease\\([^;]*?${release}[^;]*?\\)`, 'i'),
      `${name}: unmount must transfer its owned token and release callback to the retry registry`,
    )
    assert.doesNotMatch(source, new RegExp(`(?:await\\s+)?${release}\\(`), `${name}: releases must not bypass the lease manager`)
  }
})

test('stale acquire cleanup is queued instead of swallowing release failures', () => {
  for (const [name, [, release]] of Object.entries(contracts)) {
    const source = sources[name]
    assert.doesNotMatch(source, new RegExp(`${release}\\([^)]*\\)\\.catch\\(\\(\\) => \\{\\}\\)`), `${name}: release failure must never be swallowed`)
    assert.match(source, new RegExp(`queueRuntimeLeaseRelease\\([^;]*?${release}`), `${name}: stale/unmount cleanup must be retriable`)
  }
})

test('runtime pages no longer call unconditional compatibility lifecycle APIs', () => {
  assert.doesNotMatch(sources['MiscTools.vue'], /Chara(?:Attach|Detach)/)
  assert.doesNotMatch(sources['WrightstoneMemoryGenerator.vue'], /WrightstoneMemory(?:Enable|Disable)/)
  assert.doesNotMatch(sources['OverLimit.vue'], /OverLimit(?:Enable|Disable)/)
})

test('MiscTools keeps the owned stable runtime surface without retired experimental bindings', () => {
  const miscSource = sources['MiscTools.vue']
  for (const symbol of [
    'Countdown',
    'FaceAccessory',
    'UnlockAllTrophy',
    'OtherSkinPurpleRune',
    'DamageMeter',
    'DamageOverlay',
  ]) {
    assert.doesNotMatch(miscSource, new RegExp(symbol), `MiscTools.vue: ${symbol} is not part of the stable runtime lease surface`)
  }
})

test('live writes carry the current owner token and never call compatibility writes', () => {
  const writeContracts = {
    'SigilMemoryGenerator.vue': ['SigilMemoryUpdateOwned', 'SigilMemoryUpdate'],
    'SigilLoadoutRestore.vue': ['SigilMemoryUpdateOwned', 'SigilMemoryUpdate'],
    'SummonEditor.vue': ['SummonUpdateOwned', 'SummonUpdate'],
    'WrightstoneMemoryGenerator.vue': ['WrightstoneMemoryUpdateOwned', 'WrightstoneMemoryUpdate'],
    'OverLimit.vue': ['OverLimitSetAllOwned', 'OverLimitSetAll'],
    'MonsterEnhance.vue': ['MonsterEnhanceSetPatchValueEnabledOwned', 'MonsterEnhanceSetPatchValueEnabled'],
  }
  for (const [name, [ownedWrite, compatibilityWrite]] of Object.entries(writeContracts)) {
    const source = sources[name]
    assert.match(source, new RegExp(`${ownedWrite}\\(ownerToken,`), `${name}: owned write must receive the captured owner token first`)
    assert.match(source, /if\s*\(!ownerToken\)\s*throw new Error/, `${name}: missing ownership must fail closed`)
    assert.doesNotMatch(source, new RegExp(`\\b${compatibilityWrite}\\(`), `${name}: compatibility write must not be callable`)
  }

  const miscSource = sources['MiscTools.vue']
  for (const [ownedRead, compatibilityRead] of [
    ['CurrencyGetAllOwned', 'CurrencyGetAll'],
    ['PotionGetAllOwned', 'PotionGetAll'],
  ]) {
    assert.match(miscSource, new RegExp(`${ownedRead}\\(connectionOwnerToken\\)`), `MiscTools.vue: ${ownedRead} must receive the current owner token`)
    assert.doesNotMatch(miscSource, new RegExp(`\\b${compatibilityRead}\\(`), `MiscTools.vue: ${compatibilityRead} must not bypass ownership`)
  }
  for (const [ownedWrite, compatibilityWrite] of [
    ['CurrencySetOneOwned', 'CurrencySetOne'],
    ['PotionSetOneOwned', 'PotionSetOne'],
  ]) {
    assert.match(miscSource, new RegExp(`${ownedWrite}\\(connectionOwnerToken,`), `MiscTools.vue: ${ownedWrite} must receive the current owner token`)
    assert.doesNotMatch(miscSource, new RegExp(`\\b${compatibilityWrite}\\(`), `MiscTools.vue: ${compatibilityWrite} must not bypass ownership`)
  }
  for (const [ownedCall, compatibilityCall, suffix = ''] of [
    ['MaterialConsumeGetStatusOwned', 'MaterialConsumeGetStatus', 'connectionOwnerToken'],
    ['MaterialConsumeSetEnabledOwned', 'MaterialConsumeSetEnabled', 'connectionOwnerToken,'],
    ['CollectibleTaskCompleteOwned', 'CollectibleTaskComplete', 'connectionOwnerToken'],
    ['InfiniteChallengeGetStatusOwned', 'InfiniteChallengeGetStatus', 'connectionOwnerToken'],
    ['InfiniteChallengeSetEnabledOwned', 'InfiniteChallengeSetEnabled', 'connectionOwnerToken,'],
    ['TerminusDropGetStatusOwned', 'TerminusDropGetStatus', 'connectionOwnerToken'],
    ['TerminusDropScanOwned', 'TerminusDropScan', 'connectionOwnerToken'],
    ['TerminusDropSetEnabledOwned', 'TerminusDropSetEnabled', 'connectionOwnerToken,'],
  ]) {
    assert.ok(miscSource.includes(`${ownedCall}(${suffix}`), `MiscTools.vue: ${ownedCall} must carry the current owner token`)
    assert.doesNotMatch(miscSource, new RegExp(`\\b${compatibilityCall}\\(`), `MiscTools.vue: ${compatibilityCall} must not bypass ownership`)
  }
  assert.ok(miscSource.includes("MonsterEnhanceSetPatchValueEnabledOwned(connectionOwnerToken, 'inventory_set_45'"), 'MiscTools.vue: inventory patch must carry the current owner token')
  assert.ok(!miscSource.includes("MonsterEnhanceSetPatchValueEnabled('inventory_set_45'"), 'MiscTools.vue: inventory patch must not bypass ownership')
  assert.doesNotMatch(miscSource, /['"](?:monster_hp|crocodile_damage)['"]/, 'MiscTools.vue: damage-meter monster hooks must not remain on the stable page')
  assert.doesNotMatch(miscSource, /\bMonsterEnhanceSetPatchValueEnabled\(/, 'MiscTools.vue: monster hooks must not bypass ownership')
})
