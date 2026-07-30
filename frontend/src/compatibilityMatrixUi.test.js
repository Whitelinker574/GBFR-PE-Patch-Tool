import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')

test('version compatibility reports current pages and runtime evidence', () => {
  assert.match(source, /30\s*个实际工具页\s*\+\s*1\s*个主页/)
  assert.match(source, /存档修改页面[\s\S]*?8\s*\/\s*8[\s\S]*?双存档对比复制/)
  assert.match(source, /游戏内即时编辑[\s\S]*?5\s*\/\s*5[\s\S]*?均需启动并连接游戏/)
  assert.match(source, /配装采集与复刻[\s\S]*?2\s*\/\s*2[\s\S]*?点击开启后持续后台运行/)
  assert.match(source, /单机运行时工具[\s\S]*?9 页接入[\s\S]*?角色语音、城镇镜头/)
  assert.match(source, /实时只读诊断[\s\S]*?2\s*\/\s*2[\s\S]*?不会修改进程数据或存档/)
  assert.match(source, /游戏文件与设置[\s\S]*?4 页已接入[\s\S]*?掉落与锻造规则、版本适配/)
  assert.match(source, /运行时补丁覆盖[\s\S]*?59\s*已接入\s*\/\s*4\s*待证据/)
  assert.match(source, /59\s*功能\s*\/\s*82\s*站点\s*\/\s*80\s*AOB/)
  assert.match(source, /DLC\s*2\.0\.2\s*增量审计[\s\S]*?现场修复/)
  assert.match(source, /真实游戏进程\s*E2E[\s\S]*?关键路径已验证[\s\S]*?自动完美格挡连招/)
  assert.match(source, /baselineVersion:\s*'DLC 2\.0\.2'/)
})

test('version compatibility exposes exact icon coverage', () => {
  for (const coverage of ['29 / 29', '261 / 262', '186 / 187', '159 / 163', '189 / 189', '301 / 312']) {
    const [mapped, total] = coverage.split(' / ')
    assert.match(source, new RegExp(`${mapped}\\s*\\/\\s*${total}`))
  }
})

test('version compatibility keeps long status badges readable', () => {
  assert.match(source, /\.matrix-row\s*\{[^}]*grid-template-columns:\s*minmax\(160px,1\.1fr\)\s+minmax\(96px,max-content\)\s+minmax\(180px,1\.4fr\)/is)
  assert.match(source, /\.matrix-row b\s*\{[^}]*white-space:\s*nowrap/is)
})
