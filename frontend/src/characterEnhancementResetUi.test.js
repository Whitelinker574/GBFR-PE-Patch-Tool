import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/CharacterEnhancementReset.vue', import.meta.url), 'utf8')
const lab = readFileSync(new URL('./components/SaveDiffLab.vue', import.meta.url), 'utf8')

test('enhancement reset is directly reachable in save laboratory', () => {
  assert.match(lab, /角色强化重置/)
  assert.match(lab, /<CharacterEnhancementReset v-if="mode === 'enhancement'"/)
  assert.match(source, /SaveSourcePicker/)
  assert.match(source, /characterIdentityByHash/)
})

test('enhancement reset explicitly separates all-unlearned from recruitment grants', () => {
  assert.match(source, /const mode = ref\('all'\)/)
  for (const text of ['全部取消学习', '恢复入队自动解锁', '并非全部归零', '不退还 MSP', '不降低角色等级', '不删除装备']) assert.ok(source.includes(text), text)
  assert.match(source, /selected\.value\?\.changedFromGranted > 0/)
})

test('enhancement reset requires a revision-bound preview and fresh user confirmation', () => {
  assert.match(source, /expectedRevision: snapshot\.value\.revision/)
  assert.match(source, /characterHash: selectedHash\.value/)
  assert.match(source, /watch\(\[selectedHash, mode\], \(\) => \{ confirmed\.value = false \}\)/)
  assert.match(source, /:disabled="busy \|\| !confirmed \|\| !hasChanges"/)
  assert.match(source, /lastBackup\.value = result\.backupPath/)
  assert.match(source, /恢复此次自动备份/)
})
