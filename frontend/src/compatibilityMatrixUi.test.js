import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')

test('version compatibility reports the current product and audited feature coverage', () => {
  assert.match(source, /22\s*个实际工具页\s*\+\s*1\s*个主页/)
  assert.match(source, /存档修改页面[\s\S]*?7\s*\/\s*7[\s\S]*?配装预设、因子、物品与武器、祝福、召唤石存档、角色次数、任务与称号记录/)
  assert.match(source, /Save editing pages[\s\S]*?7\s*\/\s*7[\s\S]*?summon saves[\s\S]*?quest and title records/)
  assert.match(source, /内存注入页面[\s\S]*?10\s*页接入[\s\S]*?综合实时、即时因子、即时祝福、实时配装、召唤石、上限突破、CT\s*战斗、CT\s*角色、CT\s*任务、怪物实验/)
  assert.match(source, /只读监测页面[\s\S]*?2\s*\/\s*2[\s\S]*?运行监测与角色公式采样[\s\S]*?不安装\s*Hook、不写进程或存档/)
  assert.match(source, /工具设置页面[\s\S]*?3\s*\/\s*3[\s\S]*?版本适配、语言与显示、游戏文件维护/)
  assert.match(source, /CT\s*安全直接覆盖[\s\S]*?60\s*\/\s*64/)
  assert.match(source, /58\s*个新增功能\s*\+\s*2\s*个已有安全实现/)
  assert.match(source, /58\s*功能\s*\/\s*81\s*站点\s*\/\s*79\s*AOB/)
	assert.match(source, /上游\s*v1\.8\.5\s*增量[\s\S]*?2\s*\/\s*2\s*已提炼[\s\S]*?称号搜索支持拼音[\s\S]*?三字节补丁/)
	assert.match(source, /Upstream\s*v1\.8\.5\s*delta[\s\S]*?2\s*\/\s*2\s*integrated/)
	assert.match(source, /CT\s*0\.8\.5\s*增量审计[\s\S]*?58\s*稳定项零变化\s*\+\s*1[\s\S]*?23\s*字节守卫/)
	assert.match(source, /CT\s*0\.8\.5\s*delta audit[\s\S]*?58\s*stable sites unchanged\s*\+\s*1[\s\S]*?23-byte guard/)
})

test('version compatibility exposes exact icon coverage instead of claiming completeness', () => {
  for (const coverage of ['29 / 29', '261 / 262', '183 / 184', '159 / 163', '189 / 189', '301 / 312']) {
    const [mapped, total] = coverage.split(' / ')
    assert.match(source, new RegExp(`${mapped}\\s*\\/\\s*${total}`))
  }
  assert.match(source, /角色图标[\s\S]*?29\s*\/\s*29/)
  assert.match(source, /主动技能[\s\S]*?261\s*\/\s*262/)
  assert.match(source, /因子图标[\s\S]*?183\s*\/\s*184/)
  assert.match(source, /武器图标[\s\S]*?159\s*\/\s*163/)
  assert.match(source, /召唤石图标[\s\S]*?189\s*\/\s*189/)
  assert.match(source, /物品图标[\s\S]*?301\s*\/\s*312/)
})

test('version compatibility keeps long status badges readable in the desktop matrix', () => {
  assert.match(source, /\.matrix-row\s*\{[^}]*grid-template-columns:\s*minmax\(160px,1\.1fr\)\s+minmax\(96px,max-content\)\s+minmax\(180px,1\.4fr\)/is)
  assert.match(source, /\.matrix-row b\s*\{[^}]*white-space:\s*nowrap/is)
})

test('version compatibility states the unverified runtime boundary honestly', () => {
  assert.match(source, /真实游戏进程\s*E2E[\s\S]*?待实机验证/)
  assert.match(source, /本轮未连接正在运行的目标游戏/)
  assert.match(source, /baselineVersion:\s*'DLC 2\.0\.2'/)
  assert.doesNotMatch(source, /baselineVersion:\s*'DLC 2\.0\.2\s*·\s*CT 0\.8\.4'/)
  assert.doesNotMatch(source, /核心功能已按当前版本校验。/)
  assert.doesNotMatch(source, /实时货币与素材指令已按当前版本特征校验。/)
  assert.doesNotMatch(source, /可在旧版文件补丁页手动选择/)
})
