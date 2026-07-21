import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const source = readFileSync(new URL('./components/SummonSaveEditor.vue', import.meta.url), 'utf8')
const saveSource = readFileSync(new URL('./components/SaveSourcePicker.vue', import.meta.url), 'utf8')
const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')
const binding = readFileSync(new URL('../wailsjs/go/main/SummonSaveGen.js', import.meta.url), 'utf8')

test('offline summon editor is separate from the upstream runtime memory page', () => {
  assert.match(shell, /summonSave:\s*\{[\s\S]*?group:\s*'save'/)
  assert.match(shell, /<SummonSaveEditor\s+v-else-if="activeTab === 'summonSave'"/)
  assert.match(shell, /summon:\s*\{[\s\S]*?group:\s*'memory'/)
  assert.doesNotMatch(source, /CharaAcquire|SummonUpdateOwned|process|PID/)
})

test('offline summon writes fail closed for DLC-locked saves and expose evidence-scoped natural rules', () => {
  assert.match(source, /!info\.inventory\?\.unlocked/)
  assert.match(source, /工具不会强行解锁/)
  assert.match(source, /1455=0、1454=0/)
  assert.match(source, /天然词池与等级已校验/)
  assert.match(source, /固定词条已证 · 等级待证/)
  assert.match(source, /allowedMainHashes/)
  assert.match(source, /naturalSubLevels/)
  assert.match(source, /种类、主加护、副词条、等级和 Rank 会作为一条完整记录写入/)
})

test('generated binding exposes load, create-or-update, and output selection', () => {
  for (const name of ['Apply', 'GetOptions', 'LoadSaveFile', 'SelectInputSave', 'SelectOutputSave']) {
    assert.match(binding, new RegExp(`export function ${name}\\(`))
  }
  assert.match(source, /operation:\s*mode\.value/)
  assert.match(source, /expected:\s*mode\.value === 'update' \? selected\.value : null/)
})

test('offline summon editor uses the shared offline save-source and write-mode composition', () => {
  assert.match(source, /import SaveSourcePicker from '\.\/SaveSourcePicker\.vue'/)
  assert.match(source, /<SaveSourcePicker[\s\S]*?:slots="saveSlots"[\s\S]*?@select="load"[\s\S]*?@browse="browseInput"/)
  assert.match(saveSource, /选择存档槽/)
  assert.match(saveSource, /saveSlotLabel/)
  assert.match(saveSource, /`存档 \$\{match\[1\]\}`/)
  assert.match(saveSource, /selected-save/)
  assert.match(saveSource, /repeat\(auto-fit,minmax\(118px,1fr\)\)/)
  assert.doesNotMatch(saveSource, />刷新</)
  assert.match(source, /class="output-mode"/)
  assert.match(source, /覆盖当前存档[\s\S]*?另存为新存档/)
  assert.doesNotMatch(source, /class="path-grid"/)
  assert.doesNotMatch(source, /<span class="ui-field-label">输入存档<\/span>/)
})
