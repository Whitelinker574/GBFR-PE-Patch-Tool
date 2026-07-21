import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')

test('version compatibility reports current pages and runtime evidence', () => {
  assert.match(source, /22\s*个实际工具页\s*\+\s*1\s*个主页/)
  assert.match(source, /存档修改页面[\s\S]*?7\s*\/\s*7[\s\S]*?配装预设、因子、物品与武器、祝福、召唤石存档、角色次数、任务与称号记录/)
  assert.match(source, /内存注入页面[\s\S]*?10\s*页接入[\s\S]*?战斗规则、角色机制、任务便利、怪物实验/)
  assert.match(source, /只读监测页面[\s\S]*?2\s*\/\s*2[\s\S]*?不安装\s*Hook、不写进程或存档/)
  assert.match(source, /工具设置页面[\s\S]*?3\s*\/\s*3/)
  assert.match(source, /运行时补丁覆盖[\s\S]*?60\s*\/\s*64/)
  assert.match(source, /58\s*功能\s*\/\s*81\s*站点\s*\/\s*79\s*AOB/)
  assert.match(source, /DLC\s*2\.0\.2\s*增量审计[\s\S]*?现场修复/)
  assert.match(source, /真实游戏进程\s*E2E[\s\S]*?关键路径已验证[\s\S]*?自动完美格挡连招/)
  assert.match(source, /baselineVersion:\s*'DLC 2\.0\.2'/)
})

test('version compatibility exposes exact icon coverage', () => {
  for (const coverage of ['29 / 29', '261 / 262', '183 / 184', '159 / 163', '189 / 189', '301 / 312']) {
    const [mapped, total] = coverage.split(' / ')
    assert.match(source, new RegExp(`${mapped}\\s*\\/\\s*${total}`))
  }
})

test('version compatibility keeps long status badges readable', () => {
  assert.match(source, /\.matrix-row\s*\{[^}]*grid-template-columns:\s*minmax\(160px,1\.1fr\)\s+minmax\(96px,max-content\)\s+minmax\(180px,1\.4fr\)/is)
  assert.match(source, /\.matrix-row b\s*\{[^}]*white-space:\s*nowrap/is)
})
