import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
const ctCharactersBrief = JSON.parse(
  readFileSync(new URL('../../tools/portrait-briefs/ct-characters.json', import.meta.url), 'utf8'),
)

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

test('character mechanics owns a Vaseraga portrait and sticker instead of repeating Djeeta', () => {
  assert.equal(ctCharactersBrief.portrait.character, 'vaseraga')
  assert.equal(ctCharactersBrief.sticker.character, 'vaseraga')
  assert.match(shell, /ctCharacters:\s*\{[\s\S]*?speaker:\s*'巴萨拉卡'/)
  assert.match(shell, /note:\s*'冲突项不能同时开。先关掉亮着的那个，等状态回读后再切换。'/)
})

test('function portrait speakers stay aligned with their generated character identity', () => {
  assert.match(shell, /loadoutPresets:\s*\{[\s\S]*?speaker:\s*'古兰'[\s\S]*?note:\s*'先备份，再确认角色和目标槽；已有配装会被覆盖。'/)
  assert.match(shell, /wrightstoneMemory:\s*\{[\s\S]*?speaker:\s*'玛琪拉菲菈'[\s\S]*?note:\s*'写入后旧记录会失效。回到游戏里重新选中目标，再继续。'/)
})

test('every function portrait uses a large fixed top-anchored background crop so faces and props remain visible', () => {
  assert.match(
    shell,
    /\.tool-stage::before\s*\{[^}]*background-image:var\(--function-art\);[^}]*background-position:right var\(--art-x\) top var\(--art-y\);[^}]*background-size:auto var\(--art-scale\);/s,
  )
  assert.doesNotMatch(shell, /class="character-blend"|\.art-rail \.function-character img/)

  const portraitPages = [
    'progression', 'sigil', 'sigilMemory', 'loadout', 'loadoutPresets', 'wrightstone',
    'wrightstoneMemory', 'summon', 'overlimit', 'runtime', 'ctMonitor', 'ctCombat',
    'ctCharacters', 'ctQuest', 'chara', 'save', 'compatibility',
    'monster', 'patch', 'language',
  ]
  for (const page of portraitPages) {
    assert.match(shell, new RegExp(`\\.tool-stage\\[data-tool="${page}"\\][^\\{]*\\{[^}]*--art-scale:1[5-9][0-9]%`))
  }
  assert.match(shell, /\.tool-stage\[data-tool="sigilMemory"\] \{ --art-scale:160%; --art-x:calc\(-32\.55dvh \+ 43px\); --art-y:calc\(3dvh - 4px\); \}/)
  assert.match(shell, /\.tool-stage\[data-tool="save"\] \{ --art-scale:160%; --art-x:calc\(-43\.10dvh \+ 57px\); --art-y:calc\(3dvh - 4px\); \}/)
  assert.match(shell, /\.tool-stage\[data-tool="language"\] \{ --art-scale:178%; --art-x:calc\(-39\.06dvh \+ 52px\); --art-y:calc\(-17dvh \+ 22px\); \}/)
})
