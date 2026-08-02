import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const page = readFileSync(new URL('./components/WeaponMemoryGenerator.vue', import.meta.url), 'utf8')
const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')

test('live weapon editor separates five persisted slots from expandable runtime skills', () => {
  assert.match(page, /Array\.from\(\{ length: 5 \}/)
  assert.match(page, /WeaponMemoryAcquire\(nextRuntimeAcquireRequestID\(\)\)/)
  assert.match(page, /WeaponMemoryUpdateOwned\(ownerToken, \{/)
  assert.match(page, /slots: slots\.map\(slot => \(\{ hash: normaliseHash\(slot\.hash\), level: Number\(slot\.level\) \}\)\)/)
  assert.match(page, /五个物理技能槽/)
  assert.match(page, /WeaponRuntimeSkillsDeploy/)
  assert.match(page, /ownerToken: hookOwnerToken, expectedSelectedAddr: Number\(status\.selectedAddr \|\| 0\)/)
  assert.match(page, /WeaponRuntimeSkillsRemove/)
  assert.match(page, /extraSkills\.push/)
  assert.match(page, /v-for="\(skill, index\) in extraSkills"/)
  assert.match(page, /第 6 条及以后/)
  assert.doesNotMatch(page, /不会虚构第六槽/)
  assert.match(page, /游戏可能规范化不常见组合/)
  assert.match(page, /queueRuntimeLeaseRelease\(RUNTIME_LEASE_SCOPE, owner, WeaponMemoryRelease\)/)
})

test('weapon editor is a first-class live-editing page with official Lilith art', () => {
  assert.match(shell, /weaponMemory: \(\) => import\('\.\/WeaponMemoryGenerator\.vue'\)/)
  assert.match(shell, /items: \['sigilMemory', 'wrightstoneMemory', 'weaponMemory', 'summon', 'overlimit', 'runtime'\]/)
  assert.match(shell, /weaponMemoryLilithArt from '\.\.\/assets\/gbfr\/cutouts\/weapon-memory-lilith-generated\.webp'/)
  assert.match(shell, /functionArt\.weaponMemory = weaponMemoryLilithArt/)
  assert.match(shell, /speaker: '莉莉丝'/)
})

test('weapon runtime recovery state is not presented as active green success', () => {
  assert.match(page, /runtimeWorkspace\.recoveryRequired\s*\?\s*text\('需要恢复'/)
  assert.match(page, /runtimeWorkspace\.state === 'active'/)
  assert.doesNotMatch(page, /active:\s*status\.hooked === true \|\| runtimeWorkspace\.installed === true/)
})

test('an orphaned weapon capture can be reclaimed instead of locking the enable button', () => {
  assert.match(page, /:disabled="loading \|\| Boolean\(hookOwnerToken\)"/)
  assert.match(page, /status\.hooked && !hookOwnerToken \? text\('重新接管并恢复读取'/)
  assert.match(page, /recoveryRequired: \(status\.hooked === true && !hookOwnerToken\)/)
  assert.match(page, /!status\.hooked \|\| !hookOwnerToken/)
})

test('weapon skill dropdown consumes backend search aliases and hash spellings', () => {
  const catalog = readFileSync(new URL('./components/CatalogSelect.vue', import.meta.url), 'utf8')
  assert.match(page, /\.\.\.item,\s*internalId:/)
  assert.match(page, /hashHex:\s*formatHex\(item\.hash\)/)
  assert.equal((page.match(/detail-key="hashHex"/g) || []).length, 2)
  assert.equal((page.match(/搜索技能名称或 Hash/g) || []).length, 2)
  assert.match(catalog, /Array\.isArray\(option\.searchTerms\)\s*\?\s*option\.searchTerms\s*:\s*\[\]/)
  assert.match(catalog, /\.some\(value => matchText\(value, q\)\)/)
})
