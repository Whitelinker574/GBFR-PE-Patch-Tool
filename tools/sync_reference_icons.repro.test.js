import assert from 'node:assert/strict'
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'
import test from 'node:test'

const toolsRoot = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.resolve(toolsRoot, '..')
const referenceZip = process.env.GBFR_REFERENCE_ZIP || 'D:\\gbf\\GBFR-UI-Reference-Library-2.0.2.zip'
const gameTableZip = process.env.GBFR_GAME_TABLE_ZIP || 'D:\\gbf\\GBFR-DLC-shuju-20260716-184413.zip'
const dataFiles = ['skill_names.json', 'traits.json', 'weapons.json', 'summons.json', 'items.json']

function readJSON(filename) {
  return JSON.parse(readFileSync(filename, 'utf8'))
}

test('default full sync and skills-only sync rebuild the same exact ability map without stale generated keys', { timeout: 300_000 }, () => {
  assert.ok(existsSync(referenceZip), `reference archive is required: ${referenceZip}`)
  assert.ok(existsSync(gameTableZip), `game-table archive is required: ${gameTableZip}`)

  const sandboxRoot = mkdtempSync(path.join(tmpdir(), 'gbfr-icon-sync-'))
  try {
    const sandboxTools = path.join(sandboxRoot, 'tools')
    const sandboxData = path.join(sandboxRoot, 'data')
    const sandboxSource = path.join(sandboxRoot, 'frontend', 'src')
    mkdirSync(sandboxTools, { recursive: true })
    mkdirSync(sandboxData, { recursive: true })
    mkdirSync(sandboxSource, { recursive: true })

    copyFileSync(path.join(toolsRoot, 'sync_reference_icons.ps1'), path.join(sandboxTools, 'sync_reference_icons.ps1'))
    for (const filename of dataFiles) {
      copyFileSync(path.join(repoRoot, 'data', filename), path.join(sandboxData, filename))
    }

    const checkedInSkills = readJSON(path.join(repoRoot, 'frontend', 'src', 'loadoutSkillIcons.json'))
    const staleKey = 'AB_STALE_GENERATOR_REGRESSION'
    writeFileSync(
      path.join(sandboxSource, 'loadoutSkillIcons.json'),
      `${JSON.stringify({ ...checkedInSkills, [staleKey]: 'cmn_icablt_pl0000_01.png' }, null, 2)}\n`,
      'utf8',
    )

    const result = spawnSync('powershell.exe', [
      '-NoProfile',
      '-ExecutionPolicy', 'Bypass',
      '-File', path.join(sandboxTools, 'sync_reference_icons.ps1'),
      '-ReferenceZip', referenceZip,
      '-GameTableZip', gameTableZip,
    ], {
      cwd: sandboxRoot,
      encoding: 'utf8',
      maxBuffer: 16 * 1024 * 1024,
      timeout: 280_000,
      windowsHide: true,
    })
    assert.equal(result.status, 0, `full icon sync failed\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`)

    const regeneratedSkills = readJSON(path.join(sandboxSource, 'loadoutSkillIcons.json'))
    const skillCatalog = readJSON(path.join(sandboxData, 'skill_names.json')).skills
    const expectedKeys = Object.values(skillCatalog)
      .filter(skill => /^PL\d{4}$/.test(skill.char) && skill.key !== 'AB_PL2400_09')
      .map(skill => skill.key)
      .sort()

    assert.equal(regeneratedSkills[staleKey], undefined, 'full sync must not retain keys from the previous generated JSON')
    assert.deepEqual(Object.keys(regeneratedSkills).sort(), expectedKeys)
    assert.equal(expectedKeys.length, 261)
    assert.equal(Object.values(skillCatalog).filter(skill => /^PL\d{4}$/.test(skill.char)).length, 262)

    assert.deepEqual(
      readJSON(path.join(sandboxSource, 'gameAssetIcons.json')),
      readJSON(path.join(repoRoot, 'frontend', 'src', 'gameAssetIcons.json')),
      'reusing the exact ability generator must not change other generated icon catalogs',
    )
    assert.deepEqual(
      readJSON(path.join(sandboxSource, 'loadoutTraitIcons.json')),
      readJSON(path.join(repoRoot, 'frontend', 'src', 'loadoutTraitIcons.json')),
      'legacy trait-name output must remain reproducible',
    )

    writeFileSync(
      path.join(sandboxSource, 'loadoutSkillIcons.json'),
      `${JSON.stringify({ ...regeneratedSkills, [staleKey]: 'cmn_icablt_pl0000_01.png' }, null, 2)}\n`,
      'utf8',
    )
    const skillsOnlyResult = spawnSync('powershell.exe', [
      '-NoProfile',
      '-ExecutionPolicy', 'Bypass',
      '-File', path.join(sandboxTools, 'sync_reference_icons.ps1'),
      '-ReferenceZip', referenceZip,
      '-GameTableZip', gameTableZip,
      '-SkillsOnly',
    ], {
      cwd: sandboxRoot,
      encoding: 'utf8',
      maxBuffer: 16 * 1024 * 1024,
      timeout: 280_000,
      windowsHide: true,
    })
    assert.equal(skillsOnlyResult.status, 0, `skills-only icon sync failed\nstdout:\n${skillsOnlyResult.stdout}\nstderr:\n${skillsOnlyResult.stderr}`)
    assert.deepEqual(readJSON(path.join(sandboxSource, 'loadoutSkillIcons.json')), regeneratedSkills)
    assert.deepEqual(JSON.parse(skillsOnlyResult.stdout.trim().split(/\r?\n/).at(-1)), {
      playable: 262,
      mapped: 261,
      missing: ['AB_PL2400_09'],
    })
  } finally {
    rmSync(sandboxRoot, { recursive: true, force: true })
  }
})
