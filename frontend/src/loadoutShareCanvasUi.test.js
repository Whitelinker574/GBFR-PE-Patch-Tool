import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/LoadoutShareWorkshop.vue', import.meta.url), 'utf8')

test('share canvas keeps the three fixed export contracts', () => {
  assert.match(source, /landscape:\s*\{\s*width:\s*960,\s*height:\s*540,\s*pixelRatio:\s*2,\s*output:\s*'1920x1080'/u)
  assert.match(source, /portrait:\s*\{\s*width:\s*720,\s*height:\s*960,\s*pixelRatio:\s*2,\s*output:\s*'1440x1920'/u)
  assert.match(source, /square:\s*\{\s*width:\s*640,\s*height:\s*640,\s*pixelRatio:\s*2\.5,\s*output:\s*'1600x1600'/u)
  assert.match(source, /\.share-canvas\.is-landscape\s*\{\s*width:960px;\s*height:540px;/u)
  assert.match(source, /\.share-canvas\.is-portrait\s*\{\s*width:720px;\s*height:960px;/u)
  assert.match(source, /\.share-canvas\.is-square\s*\{\s*width:640px;\s*height:640px;/u)
})

test('full density exposes optional metadata and honest captured share details', () => {
  assert.match(source, /density === 'full'[\s\S]*?tx\('完整', 'Full'\)/u)
  for (const state of ['showGameVersion', 'showGeneratedDate', 'showProjectMark']) {
    assert.match(source, new RegExp(`v-model="${state}"`, 'u'))
  }
  assert.match(source, /class="canvas-full-detail"/u)
  assert.match(source, /class="canvas-summons"/u)
  assert.match(source, /召唤石未记录/u)
  assert.match(source, /class="canvas-wrightstone"/u)
  assert.match(source, /wrightstone\.traits\.slice\(0, 3\)/u)
  assert.match(source, /祝福名称未记录/u)
  assert.match(source, /祝福词条未记录/u)
  assert.match(source, /tx\('等级未记录', 'Level Not Captured'\)/u)
  assert.match(source, /levelLabel:\s*capturedLevelLabel\(trait\?\.level\)/u)
  assert.match(source, /\{\{\s*trait\.levelLabel\s*\}\}/u)
  assert.doesNotMatch(source, /level:\s*Number\(trait\?\.level\)\s*\|\|\s*0/u)
  assert.doesNotMatch(source, /canvas-wrightstone[\s\S]{0,600}Lv\{\{\s*trait\.level\s*\}\}/u)
  assert.match(source, /onlineShortCode \|\| tx\('未生成', 'Not Generated'\)/u)
  assert.match(source, /normalizedShareUrl \|\| tx\('未填写', 'Not Provided'\)/u)
  assert.match(source, /Array\.isArray\(selected\.value\?\.summons\)[\s\S]*?Array\.isArray\(preview\.value\?\.summons\)/u)
  assert.doesNotMatch(source, /summonRows[\s\S]{0,300}(?:fake|fallbackSummon|defaultSummon)/iu)
})

test('full-density canvases use independent layouts and square keeps three-column data grids', () => {
  assert.match(source, /\.share-canvas\.is-landscape\.density-full \.canvas-content\s*\{[^}]*bottom:52px;[^}]*grid-template-columns:minmax\(0,1\.35fr\)[^}]*grid-template-rows:76px minmax\(0,1fr\)/u)
  assert.match(source, /\.share-canvas\.is-portrait\.density-full \.canvas-content\s*\{[^}]*grid-template-columns:1fr;[^}]*grid-template-rows:82px minmax\(0,1fr\) auto auto/u)
  assert.match(source, /\.share-canvas\.is-square\.density-full \.canvas-content\s*\{[^}]*grid-template-columns:1fr;[^}]*grid-template-rows:64px 160px 120px 104px;[^}]*gap:5px;[^}]*padding:5px 0/u)
  assert.match(source, /\.share-canvas\.is-square\.density-full \.canvas-sigils > div\s*\{[^}]*grid-template-columns:repeat\(3,minmax\(0,1fr\)\)/u)
  assert.match(source, /\.share-canvas\.is-square\.density-full \.canvas-sigils\s*\{[^}]*display:grid;[^}]*grid-template-rows:auto minmax\(0,1fr\);[^}]*overflow:hidden;/u)
  assert.match(source, /\.share-canvas\.is-square\.density-full \.canvas-sigils article\s*\{[^}]*min-height:0;[^}]*padding:1px 4px/u)
  assert.match(source, /\.share-canvas\.is-square\.density-full \.canvas-levels\s*\{[^}]*grid-template-columns:repeat\(3,minmax\(0,1fr\)\)/u)
  assert.match(source, /\.share-canvas\.is-square\.density-full \.canvas-summary > div\s*\{[^}]*gap:2px;[^}]*padding:5px/u)
  assert.match(source, /\.share-canvas\.is-square\.density-full\.has-qr \.canvas-summary > div\s*\{\s*padding:3px;/u)
  assert.match(source, /\.share-canvas\.is-square\.density-full\.has-qr \.canvas-ledger-scope\s*\{\s*margin-bottom:0;/u)
  assert.match(source, /\.share-canvas\.is-square\.density-full\.has-qr \.canvas-levels span\s*\{\s*padding:1px 3px;/u)
  assert.match(source, /\.share-canvas\.is-square\.density-full\.has-qr \.canvas-full-detail > div\s*\{\s*padding:3px;/u)
  assert.match(source, /\.share-canvas\.is-square\.density-full\.has-qr \.canvas-wrightstone > div\s*\{[^}]*gap:0;[^}]*margin-top:0;/u)
  assert.match(source, /\.share-canvas\.is-square \.share-portrait\s*\{[^}]*width:118%;[^}]*height:132%/u)
  assert.doesNotMatch(source, /\.density-compact \.canvas-sigils article:nth-child/u)
})

test('long legal content is clamped and every dynamic fixed-canvas zone is checked before export', () => {
  assert.match(source, /v-model="title"[^>]*maxlength="48"/u)
  assert.match(source, /v-model="description"[^>]*maxlength="140"/u)
  assert.match(source, /\(selected\?\.sigils \|\| \[\]\)\.slice\(0, 12\)/u)
  assert.match(source, /\.canvas-title h3\s*\{[^}]*overflow:hidden;[^}]*-webkit-line-clamp:1/u)
  assert.match(source, /\.canvas-title p\s*\{[^}]*overflow:hidden;[^}]*-webkit-line-clamp:2/u)
  assert.match(source, /\.canvas-summons article b,\.canvas-summons article em\s*\{[^}]*overflow:hidden;[^}]*text-overflow:ellipsis/u)
  for (const selector of ['.canvas-content', '.canvas-sigils', '.canvas-summary', '.canvas-levels', '.canvas-full-detail', '.canvas-summons', '.canvas-wrightstone', '.canvas-online', '.canvas-footer']) {
    assert.ok(source.includes(`'${selector}'`), `${selector} is missing from the pre-export overflow guard`)
  }
  assert.match(source, /content\.bottom > footer\.top - 1/u)
  assert.match(source, /\.share-canvas\.has-qr \.canvas-content\s*\{\s*bottom:106px;/u)
  assert.match(source, /\.share-canvas\.is-landscape\.density-full\.has-qr \.canvas-content\s*\{[^}]*bottom:100px;[^}]*grid-template-rows:70px minmax\(0,1fr\);[^}]*gap:6px 10px;[^}]*padding:4px 0;/u)
  assert.match(source, /\.share-canvas\.is-landscape\.density-full\.has-qr \.canvas-summary > div\s*\{\s*padding:4px;/u)
  assert.match(source, /\.share-canvas\.is-square\.density-full\.has-qr \.canvas-content\s*\{[^}]*grid-template-rows:58px 144px 106px 76px/u)
  assert.match(source, /assertCanvasFits\(canvasRef\.value\)[\s\S]*?toPng\(canvasRef\.value/u)
})

test('PNG download uses the native save dialog after asynchronous rendering', () => {
  assert.match(source, /import\s*\{\s*SaveLoadoutSharePNG\s*\}\s*from\s*'\.\.\/\.\.\/wailsjs\/go\/backend\/App'/u)
  assert.match(source, /await SaveLoadoutSharePNG\(filename,\s*url\)/u)
  assert.doesNotMatch(source, /anchor\.click\(\)/u)
})
