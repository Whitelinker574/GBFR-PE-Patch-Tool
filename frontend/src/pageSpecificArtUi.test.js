import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
const manifest = JSON.parse(readFileSync(new URL('../public/generated/function-assets/manifest.json', import.meta.url), 'utf8'))
const assets = [
  ['loadout', 'art', './assets/gbfr/cutouts/loadout-live-official-edge-safe.webp'],
  ['loadoutPresets', 'art', './assets/gbfr/cutouts/loadout-presets-official-edge-safe.webp'],
  ['wrightstoneMemory', 'art', './assets/gbfr/cutouts/wrightstone-memory-official-edge-safe.webp'],
  ['summonSave', 'art', './assets/gbfr/cutouts/summon-save-official-edge-safe.webp'],
  ['loadoutPresets', 'sticker', './assets/gbfr/stickers/loadout-presets.webp'],
  ['wrightstoneMemory', 'sticker', './assets/gbfr/stickers/wrightstone-memory.webp'],
  ['summonSave', 'sticker', './assets/gbfr/stickers/summon-save.webp'],
]

const ctAssets = [
  ['patchCombat', 'art', './assets/gbfr/cutouts/patch-combat-official-edge-safe.webp'],
  ['patchCharacters', 'art', './assets/gbfr/cutouts/patch-characters-official-edge-safe.webp'],
  ['patchQuest', 'art', './assets/gbfr/cutouts/patch-quest-official-edge-safe.webp'],
  ['runtimeMonitor', 'art', './assets/gbfr/cutouts/runtime-monitor-official-edge-safe.webp'],
  ['patchCombat', 'sticker', './assets/gbfr/stickers/patch-combat.webp'],
  ['patchCharacters', 'sticker', './assets/gbfr/stickers/patch-characters.webp'],
  ['patchQuest', 'sticker', './assets/gbfr/stickers/patch-quest.webp'],
  ['runtimeMonitor', 'sticker', './assets/gbfr/stickers/runtime-monitor.webp'],
  ['formulaSampler', 'art', './assets/gbfr/cutouts/formula-sampler-official-edge-safe.webp'],
  ['formulaSampler', 'sticker', './assets/gbfr/stickers/formula-sampler.webp'],
]

const experimentalPageAssets = [
  ['spatialTools', 'art', './assets/gbfr/cutouts/spatial-tools-official-edge-safe.webp'],
  ['spatialTools', 'sticker', './assets/gbfr/stickers/spatial-tools.webp'],
  ['selectedItemMonitor', 'art', './assets/gbfr/cutouts/selected-item-monitor-official-edge-safe.webp'],
  ['selectedItemMonitor', 'sticker', './assets/gbfr/stickers/selected-item-monitor.webp'],
  ['saveDiff', 'art', './assets/gbfr/cutouts/save-diff-official-edge-safe.webp'],
  ['saveDiff', 'sticker', './assets/gbfr/stickers/save-diff.webp'],
  ['naturalDrop', 'art', './assets/gbfr/cutouts/natural-drop-official-edge-safe.webp'],
  ['naturalDrop', 'sticker', './assets/gbfr/stickers/natural-drop.webp'],
  ['audioMixer', 'art', './assets/gbfr/cutouts/audio-mixer-official-edge-safe.webp'],
  ['audioMixer', 'sticker', './assets/gbfr/stickers/audio-mixer.webp'],
  ['camera', 'art', './assets/gbfr/cutouts/camera-official-edge-safe.webp'],
  ['camera', 'sticker', './assets/gbfr/stickers/camera.webp'],
  ['virtualSigils', 'art', './assets/gbfr/cutouts/virtual-sigils-official-edge-safe.webp'],
  ['virtualSigils', 'sticker', './assets/gbfr/stickers/virtual-sigils.webp'],
  ['runtimeQOL', 'art', './assets/gbfr/cutouts/runtime-qol-official-edge-safe.webp'],
  ['runtimeQOL', 'sticker', './assets/gbfr/stickers/runtime-qol.webp'],
]

function assertGeneratedDisplay(id, kind) {
  const entry = manifest.assets[id]?.[kind]
  assert.ok(entry, `${id}.${kind} must exist in the generated manifest`)
  assert.match(entry.sourceHash, /^[0-9a-f]{12}$/)
  assert.deepEqual(Object.keys(entry.variants), ['display'])
  const current = entry.variants.display
  assert.ok(current?.width > 0 && current?.height > 0, `${id}.${kind}.display dimensions`)
  assert.ok(existsSync(new URL(`../public${current.url}`, import.meta.url)), `${current.url} must be generated`)
}

test('pages that previously repeated portraits now own function-specific approved assets', () => {
  for (const [id, kind, path] of assets) {
    assert.ok(existsSync(new URL(path, import.meta.url)), `${path} must exist`)
    assertGeneratedDisplay(id, kind)
  }

  assert.equal(manifest.schemaVersion, 1)
  assert.match(shell, /import \{ functionAssetManifest \} from ['"]\.\.\/generated\/functionAssetManifest\.js['"]/)
  assert.match(shell, /asset\.art\.variants\.display\.url/)
  assert.match(shell, /asset\.sticker\.variants\.display\.url/)
})

test('runtime patch pages ship their approved function-specific assets without repeated binaries', () => {
  const hashes = new Map()
  for (const [id, kind, path] of ctAssets) {
    const url = new URL(path, import.meta.url)
    assert.ok(existsSync(url), `${path} must exist`)
    assertGeneratedDisplay(id, kind)
    const hash = createHash('sha256').update(readFileSync(url)).digest('hex')
    assert.equal(hashes.has(hash), false, `${path} repeats ${hashes.get(hash)}`)
    hashes.set(hash, path)
  }
})

test('experimental function pages ship approved function-specific portraits and stickers', () => {
  const hashes = new Map()
  for (const [id, kind, path] of experimentalPageAssets) {
    const url = new URL(path, import.meta.url)
    assert.ok(existsSync(url), `${path} must exist`)
    assertGeneratedDisplay(id, kind)
    const hash = createHash('sha256').update(readFileSync(url)).digest('hex')
    assert.equal(hashes.has(hash), false, `${path} repeats ${hashes.get(hash)}`)
    hashes.set(hash, path)
  }
})

test('character mechanics keeps its dedicated Vaseraga production assets and guidance', () => {
  assert.match(manifest.assets.patchCharacters.art.variants.display.url, /patch-characters-official-edge-safe\.display\./)
  assert.match(manifest.assets.patchCharacters.sticker.variants.display.url, /patch-characters\.display\./)
  assert.match(shell, /patchCharacters:\s*\{[\s\S]*?speaker:\s*'巴萨拉卡'/)
  assert.match(shell, /note:\s*'冲突项不能同时开。先关掉亮着的那个，等状态回读后再切换。'/)
})

test('function portrait speakers stay aligned with their assigned character identity', () => {
  assert.match(shell, /loadoutPresets:\s*\{[\s\S]*?speaker:\s*'古兰'[\s\S]*?note:\s*'先备份，再确认角色和目标槽；已有配装会被覆盖。'/)
  assert.match(shell, /wrightstoneMemory:\s*\{[\s\S]*?speaker:\s*'玛琪拉菲菈'[\s\S]*?note:\s*'写入后旧记录会失效。回到游戏里重新选中目标，再继续。'/)
  assert.match(shell, /summonSave:\s*\{[\s\S]*?speaker:\s*'圣德芬'[\s\S]*?note:\s*'系统未开放只会提示；种类、加护和副词条核对一致，再写入。'/)
  assert.match(shell, /spatialTools:\s*\{[\s\S]*?speaker:\s*'泽塔'[\s\S]*?note:\s*'先记下原点，再移动。没有碰撞证据的能力，不会冒充穿墙。'/)
  assert.match(shell, /selectedItemMonitor:\s*\{[\s\S]*?speaker:\s*'齐格飞'[\s\S]*?note:\s*'换了物品要重新选中再读取；这里只看，不会写入。'/)
  assert.match(shell, /camera:\s*\{[\s\S]*?speaker:\s*'索恩'[\s\S]*?note:\s*'先看准距离和高度；顶部显示常驻后，切页也不会停。'/)
  assert.match(shell, /runtimeQOL:\s*\{[\s\S]*?speaker:\s*'夏洛特'[\s\S]*?note:\s*'实验开关可以先试，但记得核对任务和背包；F12 可以恢复。'/)
})

test('offline summon save owns Sandalphon art instead of repeating the runtime summon guide', () => {
  const portrait = readFileSync(new URL('./assets/gbfr/cutouts/summon-save-official-edge-safe.webp', import.meta.url))
  const runtimePortrait = readFileSync(new URL('./assets/gbfr/cutouts/summon-official-edge-safe.webp', import.meta.url))
  const sticker = readFileSync(new URL('./assets/gbfr/stickers/summon-save.webp', import.meta.url))
  const runtimeSticker = readFileSync(new URL('./assets/gbfr/stickers/summon.webp', import.meta.url))
  assert.notEqual(createHash('sha256').update(portrait).digest('hex'), createHash('sha256').update(runtimePortrait).digest('hex'))
  assert.notEqual(createHash('sha256').update(sticker).digest('hex'), createHash('sha256').update(runtimeSticker).digest('hex'))
  assert.notEqual(manifest.assets.summonSave.art.sourceHash, manifest.assets.summon.art.sourceHash)
  assert.notEqual(manifest.assets.summonSave.sticker.sourceHash, manifest.assets.summon.sticker.sourceHash)
})

test('formula sampler portrait caption matches Katalina', () => {
  assert.match(shell, /formulaSampler:\s*\{[\s\S]*?speaker:\s*'\u5361\u5854\u8389\u5a1c'/)
})

test('pages without approved artwork use the full work area without a dead portrait rail', () => {
  assert.match(shell, /'art-collapsed': artCollapsed \|\| !currentArt/)
  assert.match(shell, /v-if="!isLoadoutWorkspace && currentArt" class="art-toggle"/)
  assert.match(shell, /v-if="!isLoadoutWorkspace && currentArt && !artCollapsed" class="art-caption"/)
})

test('every function portrait stays top-anchored so tall windows keep faces and props visible', () => {
  assert.match(
    shell,
    /\.tool-stage::before\s*\{[^}]*background-image:var\(--function-art\);[^}]*background-position:right var\(--art-x\) top var\(--art-y\);[^}]*background-size:auto var\(--art-scale\);/s,
  )
  assert.doesNotMatch(shell, /class="character-blend"|\.art-rail \.function-character img/)

  const portraitPages = [
    'progression', 'sigil', 'sigilMemory', 'loadout', 'loadoutPresets', 'wrightstone',
    'wrightstoneMemory', 'summonSave', 'summon', 'overlimit', 'runtime', 'runtimeMonitor', 'formulaSampler', 'patchCombat',
    'patchCharacters', 'patchQuest', 'chara', 'save', 'compatibility',
    'monster', 'patch', 'language', 'spatialTools', 'selectedItemMonitor',
    'saveDiff', 'naturalDrop', 'audioMixer', 'camera', 'virtualSigils', 'runtimeQOL',
  ]
  for (const page of portraitPages) {
    assert.match(shell, new RegExp(`\\.tool-stage\\[data-tool="${page}"\\][^\\{]*\\{[^}]*--art-scale:1[5-9][0-9]%`))
  }
  assert.match(shell, /\.tool-stage\[data-tool="sigilMemory"\] \{ --art-scale:160%; --art-x:calc\(-32\.55dvh \+ 43px\); --art-y:calc\(3dvh - 4px\); \}/)
  assert.match(shell, /\.tool-stage\[data-tool="formulaSampler"\] \{ --art-scale:160%; --art-x:calc\(-9\.11dvh \+ 12px\); --art-y:calc\(3dvh - 4px\); \}/)
  assert.match(shell, /\.tool-stage\[data-tool="save"\] \{ --art-scale:160%; --art-x:calc\(-43\.10dvh \+ 57px\); --art-y:calc\(3dvh - 4px\); \}/)
  assert.match(shell, /\.tool-stage\[data-tool="language"\] \{ --art-scale:178%; --art-x:calc\(-39\.06dvh \+ 52px\); --art-y:calc\(-17dvh \+ 22px\); \}/)
})
