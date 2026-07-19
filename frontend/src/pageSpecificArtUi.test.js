import assert from 'node:assert/strict'
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
