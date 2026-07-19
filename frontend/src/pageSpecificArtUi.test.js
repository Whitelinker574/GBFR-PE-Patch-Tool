import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')

const assets = [
  ['loadoutLiveArt', './assets/gbfr/cutouts/loadout-live-official-edge-safe.webp'],
  ['loadoutPresetsArt', './assets/gbfr/cutouts/loadout-presets-official-edge-safe.webp'],
  ['wrightstoneMemoryArt', './assets/gbfr/cutouts/wrightstone-memory-official-edge-safe.webp'],
  ['loadoutPresetsSticker', './assets/gbfr/stickers/loadout-presets.webp'],
  ['wrightstoneMemorySticker', './assets/gbfr/stickers/wrightstone-memory.webp'],
]

const ctAssets = [
  ['ctCombatArt', './assets/gbfr/cutouts/ct-combat-official-edge-safe.webp'],
  ['ctCharactersArt', './assets/gbfr/cutouts/ct-characters-official-edge-safe.webp'],
  ['ctQuestArt', './assets/gbfr/cutouts/ct-quest-official-edge-safe.webp'],
  ['ctMonitorArt', './assets/gbfr/cutouts/ct-monitor-official-edge-safe.webp'],
  ['ctCombatSticker', './assets/gbfr/stickers/ct-combat.webp'],
  ['ctCharactersSticker', './assets/gbfr/stickers/ct-characters.webp'],
  ['ctQuestSticker', './assets/gbfr/stickers/ct-quest.webp'],
  ['ctMonitorSticker', './assets/gbfr/stickers/ct-monitor.webp'],
]

test('pages that previously repeated portraits now own function-specific approved assets', () => {
  for (const [binding, path] of assets) {
    const relativePath = path.replace('./assets', '../assets')
    assert.match(shell, new RegExp(`import ${binding} from '${relativePath.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}'`))
    assert.ok(existsSync(new URL(path, import.meta.url)), `${path} must exist`)
  }

  assert.match(shell, /loadout:\s*loadoutLiveArt/)
  assert.match(shell, /loadoutPresets:\s*loadoutPresetsArt/)
  assert.match(shell, /wrightstoneMemory:\s*wrightstoneMemoryArt/)
  assert.match(shell, /loadoutPresets:\s*loadoutPresetsSticker/)
  assert.match(shell, /wrightstoneMemory:\s*wrightstoneMemorySticker/)
})

test('CT pages ship their approved function-specific assets without repeated binaries', () => {
  const hashes = new Map()
  for (const [binding, path] of ctAssets) {
    const relativePath = path.replace('./assets', '../assets')
    assert.match(
      shell,
      new RegExp(`const ${binding} = new URL\\('${relativePath.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}', import\\.meta\\.url\\)\\.href`),
    )
    const url = new URL(path, import.meta.url)
    assert.ok(existsSync(url), `${path} must exist`)
    const hash = createHash('sha256').update(readFileSync(url)).digest('hex')
    assert.equal(hashes.has(hash), false, `${path} repeats ${hashes.get(hash)}`)
    hashes.set(hash, path)
  }
})
