import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const lab = readFileSync(new URL('./components/RuntimeQOLLab.vue', import.meta.url), 'utf8')
const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')

test('convenience runtime has one themed persistent workspace', () => {
  assert.match(shell, /runtimeQOL:\s*\(\) => import\('\.\/RuntimeQOLLab\.vue'\)/)
  assert.match(shell, /items: \['runtimeQOL', 'virtualSigils', 'audioMixer', 'camera'/)
  assert.match(shell, /runtimeQOL:\s*\{[\s\S]*?title:\s*'显示与房间工具'/)
  assert.match(lab, /DeployRuntimeQOL/)
  assert.match(lab, /RemoveRuntimeQOL/)
  assert.match(lab, /GetRuntimeQOLWorkspace/)
  assert.match(shell, /EventsOn\('runtime-qol-session'/)
  assert.match(shell, /ClipboardSetText\(sessionId\)/)
  assert.match(lab, /damageCapPercentage/)
  assert.match(lab, /detailedEnemyHp/)
  assert.match(lab, /detailedSba/)
  assert.match(lab, /sessionCapture/)
	assert.match(lab, /normalQuestLevelSync/)
	assert.match(lab, /returnWrightstone/)
	assert.match(lab, /normalQuestLevelSync" type="checkbox"/)
	assert.doesNotMatch(lab, /normalQuestLevelSync" type="checkbox" disabled/)
	assert.match(lab, /returnWrightstone" type="checkbox"/)
	assert.doesNotMatch(lab, /returnWrightstone" type="checkbox" disabled/)
	assert.match(lab, /普通任务等级同步 · 实验/)
	assert.match(lab, /重镶返还原祝福石 · 实验/)
	assert.doesNotMatch(lab, /本构建不会安装该 Hook/)
	assert.match(lab, /freeCaptain/)
  assert.match(lab, /ClipboardSetText/)
})

test('convenience runtime layout remains bounded on narrow containers', () => {
  assert.match(lab, /container:qol-lab \/ inline-size/)
  assert.match(lab, /@container qol-lab \(max-width:520px\)/)
  assert.match(lab, /grid-template-columns:minmax\(0,1fr\)/)
  assert.doesNotMatch(lab, /background:\s*(?:#fff|white)/i)
})

test('pages without a generated sticker use the full sidebar note width', () => {
  assert.match(shell, /:class="\{ 'has-sticker': currentSticker \}"/)
  assert.match(shell, /\.sidebar-mascot\.has-sticker\s*\{\s*grid-template-columns:46px minmax\(0,1fr\)/)
  assert.match(shell, /\.sidebar-mascot:not\(\.has-sticker\)\s*\{\s*display:none/)
})
