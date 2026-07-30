import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const read = path => readFileSync(new URL(path, import.meta.url), 'utf8')
const editor = read('./components/LoadoutEditor.vue')
const viewer = read('./components/LoadoutViewer.vue')
const detector = read('./components/RuntimeLoadoutDetector.vue')
const workshop = read('./components/LoadoutShareWorkshop.vue')
const shareDialog = read('./components/LoadoutShareCodeDialog.vue')
const publishDialog = read('./components/LoadoutPublishDialog.vue')
const picker = read('./components/SaveSourcePicker.vue')
const progression = read('./components/ProgressionEditor.vue')
const sigilGenerator = read('./components/SigilGenerator.vue')
const wrightstoneGenerator = read('./components/WrightstoneGenerator.vue')
const charaStats = read('./components/CharaStats.vue')
const saveEditor = read('./components/SaveEditor.vue')
const virtualSigils = read('./components/VirtualSigilLab.vue')

test('save, Logs, and runtime publishing register one reusable share-session shape', () => {
  assert.match(editor, /rememberPublishedLoadoutShare\(loadoutShareSessionKey\(\{[\s\S]*?savePath: props\.savePath[\s\S]*?unitId/u)
  assert.match(viewer, /const sessionKey = loadoutShareSessionKey\(\{ compatibilityCode: cacheKey \}\)/u)
  assert.match(viewer, /rememberPublishedLoadoutShare\(sessionKey, published\)/u)
  assert.match(detector, /const sessionKey = loadoutShareSessionKey\(\{ source: 'runtime', recordId: record\.id, role: member\.role \}\)/u)
  assert.match(detector, /publishedLoadoutShare\((?:sessionKey|target\.sessionKey)\)[\s\S]*?rememberPublishedLoadoutShare\(target\.sessionKey, published\)/u)
  assert.match(viewer, /:published="publishedShareFor\(lo\)"/u)
})

test('all share entry points reuse the shared clipboard and publication session', () => {
  assert.match(shareDialog, /copyShareText/u)
  assert.doesNotMatch(shareDialog, /document\.execCommand|navigator\.clipboard\?\.writeText/u)
  assert.doesNotMatch(viewer, /logsPublishedShares/u)
  assert.match(viewer, /publishedLoadoutShare\(sessionKey\)/u)
  assert.match(viewer, /import LoadoutPublishDialog from '\.\/LoadoutPublishDialog\.vue'/u)
  assert.match(detector, /import LoadoutPublishDialog from '\.\/LoadoutPublishDialog\.vue'/u)
  assert.match(publishDialog, /@click\.self="emit\('close'\)"[\s\S]*?@keydown\.esc="emit\('close'\)"/u)
})

test('loadout presets and virtual sigils reuse the shared save-source picker', () => {
  for (const source of [viewer, virtualSigils]) {
    assert.match(source, /import SaveSourcePicker from '\.\/SaveSourcePicker\.vue'/u)
    assert.match(source, /<SaveSourcePicker/u)
    assert.doesNotMatch(source, /class="save-row"|class="source-field"/u)
  }
  assert.match(viewer, /@select="load"[\s\S]*?@browse="browse"/u)
  assert.match(virtualSigils, /@select="selectKnownSave"[\s\S]*?@browse="chooseSave"/u)
})

test('virtual sigil slot count shares the save-source detail row instead of occupying its own setup row', () => {
  assert.match(picker, /class="source-details"[\s\S]*?<slot name="details" \/>/u)
  assert.match(virtualSigils, /<SaveSourcePicker[\s\S]*?<template #details>[\s\S]*?class="slot-count"[\s\S]*?<\/SaveSourcePicker>/u)
  assert.match(virtualSigils, /\.slot-count\s*\{[\s\S]*?display:grid[\s\S]*?grid-template-columns:minmax\(148px,1fr\) 72px/u)
  assert.doesNotMatch(virtualSigils, /\.slot-count\s*\{[^}]*grid-column/u)
})

test('share-image workshop reuses a published short link while retaining manual fallback', () => {
  assert.match(workshop, /published: \{ type: Object, default: null \}/u)
  assert.match(workshop, /watch\(\(\) => props\.published\?\.url/u)
  assert.match(workshop, /HTTPS 分享链接（自动复用）/u)
  assert.match(workshop, /v-model="shareUrl"/u)
})

test('share-image rendering is single-flight and disables both export entry points', () => {
  assert.match(workshop, /async function download\(\) \{\s*if \(exportBusy\.value\) return/u)
  assert.match(workshop, /async function copyPNG\(\) \{\s*if \(exportBusy\.value\) return/u)
  assert.match(workshop, /:disabled="exportBusy \|\| !selected" @click="copyPNG"/u)
  assert.match(workshop, /:disabled="exportBusy \|\| !selected" @click="download"/u)
})

test('share-image identity, equipment, and QR all stay bound to the selected loadout', () => {
  assert.match(viewer, /function shareGroup\(loadout\)[^{]*\{[^}]*loadouts:\s*\[loadout\]/u)
  assert.match(viewer, /:group="shareGroup\(lo\)"/u)
  assert.match(viewer, /:published="publishedShareFor\(lo\)"/u)
  assert.match(workshop, /characterSharePortraitProfile\(props\.group\?\.charaHash\)/u)
  assert.match(workshop, /characterAssetIcon\(props\.group\?\.charaHash\)/u)
  assert.match(workshop, /weaponAssetIcon\(\{ hash: selected\.value\?\.weaponHash \}\)/u)
  assert.match(workshop, /for \(const sigil of selected\.value\?\.sigils \|\| \[\]\)/u)
  assert.match(workshop, /watch\(\(\) => props\.published\?\.url/u)
  assert.match(workshop, /QRCode\.toDataURL\(value,[\s\S]*errorCorrectionLevel:\s*'M'/u)
})

test('progression editor uses the shared save-source picker with isolated language copy', () => {
  assert.match(progression, /import SaveSourcePicker from '\.\/SaveSourcePicker\.vue'/u)
  assert.match(progression, /const tx = \(zh, en\) => language\.value === 'en' \? en : zh/u)
  assert.match(progression, /<SaveSourcePicker[\s\S]*?@select="load"[\s\S]*?@browse="browse"/u)
  assert.match(picker, /import \{ language \} from '\.\.\/i18n\.js'/u)
  assert.match(picker, /tx\('选择存档槽', 'Choose Save Slot'\)/u)
  assert.match(picker, /tx\('尚未选择存档', 'No Save Selected'\)/u)
})

test('offline factor and wrightstone generators share the same save-source control', () => {
  for (const source of [sigilGenerator, wrightstoneGenerator]) {
    assert.match(source, /import SaveSourcePicker from '\.\/SaveSourcePicker\.vue'/u)
    assert.match(source, /<SaveSourcePicker/u)
    assert.doesNotMatch(source, /class="save-slots"|function saveSlotLabel/u)
  }
})

test('character statistics and quest records reuse the save-source picker without changing refresh behavior', () => {
  for (const source of [charaStats, saveEditor]) {
    assert.match(source, /import SaveSourcePicker from '\.\/SaveSourcePicker\.vue'/u)
    assert.match(source, /<SaveSourcePicker/u)
    assert.match(source, /action-label="刷新存档列表"/u)
    assert.doesNotMatch(source, /function saveSlotLabel|class="slots"/u)
  }
  assert.match(charaStats, /@select="load"[\s\S]*?@browse="refresh"/u)
  assert.match(saveEditor, /@select="load"[\s\S]*?@browse="scanSaves"/u)
  assert.match(picker, /actionLabel \|\| tx\('选择其他存档', 'Choose Another Save'\)/u)
})
