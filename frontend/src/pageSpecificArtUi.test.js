import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
const assets = [
  ['loadoutLiveArt', './assets/gbfr/cutouts/loadout-live-official-edge-safe.webp'],
  ['loadoutPresetsArt', './assets/gbfr/cutouts/loadout-presets-official-edge-safe.webp'],
  ['wrightstoneMemoryArt', './assets/gbfr/cutouts/wrightstone-memory-official-edge-safe.webp'],
  ['summonSaveArt', './assets/gbfr/cutouts/summon-save-official-edge-safe.webp'],
  ['loadoutPresetsSticker', './assets/gbfr/stickers/loadout-presets.webp'],
  ['wrightstoneMemorySticker', './assets/gbfr/stickers/wrightstone-memory.webp'],
  ['summonSaveSticker', './assets/gbfr/stickers/summon-save.webp'],
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
  ['formulaSamplerArt', './assets/gbfr/cutouts/formula-sampler-official-edge-safe.webp'],
  ['formulaSamplerSticker', './assets/gbfr/stickers/formula-sampler.webp'],
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
  assert.match(shell, /summonSave:\s*summonSaveArt/)
  assert.match(shell, /loadoutPresets:\s*loadoutPresetsSticker/)
  assert.match(shell, /wrightstoneMemory:\s*wrightstoneMemorySticker/)
  assert.match(shell, /summonSave:\s*summonSaveSticker/)
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

test('character mechanics keeps its dedicated Vaseraga production assets and guidance', () => {
  assert.match(shell, /ctCharactersArt.*ct-characters-official-edge-safe\.webp/)
  assert.match(shell, /ctCharactersSticker.*ct-characters\.webp/)
  assert.match(shell, /ctCharacters:\s*\{[\s\S]*?speaker:\s*'巴萨拉卡'/)
  assert.match(shell, /note:\s*'冲突项不能同时开。先关掉亮着的那个，等状态回读后再切换。'/)
})

test('function portrait speakers stay aligned with their generated character identity', () => {
  assert.match(shell, /loadoutPresets:\s*\{[\s\S]*?speaker:\s*'古兰'[\s\S]*?note:\s*'先备份，再确认角色和目标槽；已有配装会被覆盖。'/)
  assert.match(shell, /wrightstoneMemory:\s*\{[\s\S]*?speaker:\s*'玛琪拉菲菈'[\s\S]*?note:\s*'写入后旧记录会失效。回到游戏里重新选中目标，再继续。'/)
  assert.match(shell, /summonSave:\s*\{[\s\S]*?speaker:\s*'圣德芬'[\s\S]*?note:\s*'系统没开放就先停手；种类、加护和副词条核对一致，再写入。'/)
})

test('offline summon save owns Sandalphon art instead of repeating the runtime summon guide', () => {
  const portrait = readFileSync(new URL('./assets/gbfr/cutouts/summon-save-official-edge-safe.webp', import.meta.url))
  const runtimePortrait = readFileSync(new URL('./assets/gbfr/cutouts/summon-official-edge-safe.webp', import.meta.url))
  const sticker = readFileSync(new URL('./assets/gbfr/stickers/summon-save.webp', import.meta.url))
  const runtimeSticker = readFileSync(new URL('./assets/gbfr/stickers/summon.webp', import.meta.url))
  assert.notEqual(createHash('sha256').update(portrait).digest('hex'), createHash('sha256').update(runtimePortrait).digest('hex'))
  assert.notEqual(createHash('sha256').update(sticker).digest('hex'), createHash('sha256').update(runtimeSticker).digest('hex'))
  assert.match(shell, /summonSave:\s*summonSaveArt/)
  assert.match(shell, /summonSave:\s*summonSaveSticker/)
})

test('formula sampler portrait caption matches Katalina', () => {
  assert.match(shell, /formulaSampler:\s*\{[\s\S]*?speaker:\s*'\u5361\u5854\u8389\u5a1c'/)
})

test('every function portrait stays top-anchored so tall windows keep faces and props visible', () => {
  assert.match(
    shell,
    /\.tool-stage::before\s*\{[^}]*background-image:var\(--function-art\);[^}]*background-position:right var\(--art-x\) top var\(--art-y\);[^}]*background-size:auto var\(--art-scale\);/s,
  )
  assert.doesNotMatch(shell, /class="character-blend"|\.art-rail \.function-character img/)

  const portraitPages = [
    'progression', 'sigil', 'sigilMemory', 'loadout', 'loadoutPresets', 'wrightstone',
    'wrightstoneMemory', 'summonSave', 'summon', 'overlimit', 'runtime', 'ctMonitor', 'formulaSampler', 'ctCombat',
    'ctCharacters', 'ctQuest', 'chara', 'save', 'compatibility',
    'monster', 'patch', 'language',
  ]
  for (const page of portraitPages) {
    assert.match(shell, new RegExp(`\\.tool-stage\\[data-tool="${page}"\\][^\\{]*\\{[^}]*--art-scale:1[5-9][0-9]%`))
  }
  assert.match(shell, /\.tool-stage\[data-tool="sigilMemory"\] \{ --art-scale:160%; --art-x:calc\(-32\.55dvh \+ 43px\); --art-y:calc\(3dvh - 4px\); \}/)
  assert.match(shell, /\.tool-stage\[data-tool="formulaSampler"\] \{ --art-scale:160%; --art-x:calc\(-9\.11dvh \+ 12px\); --art-y:calc\(3dvh - 4px\); \}/)
  assert.match(shell, /\.tool-stage\[data-tool="save"\] \{ --art-scale:160%; --art-x:calc\(-43\.10dvh \+ 57px\); --art-y:calc\(3dvh - 4px\); \}/)
  assert.match(shell, /\.tool-stage\[data-tool="language"\] \{ --art-scale:178%; --art-x:calc\(-39\.06dvh \+ 52px\); --art-y:calc\(-17dvh \+ 22px\); \}/)
})
