import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')

test('version compatibility reports current pages and runtime evidence', () => {
  assert.match(source, /游戏 2\.0\.5 静态数据[\s\S]*?已核对[\s\S]*?仅 reward_point 与 skillboard_effect_action_parts 改变/)
  assert.match(source, /静态与离线流程[\s\S]*?数据已核对[\s\S]*?MSP 自然上限更新为 9,999,999/)
  assert.match(source, /游戏 2\.0\.5 实时功能[\s\S]*?已迁移 · 实验边界[\s\S]*?识别 2\.0\.2–2\.0\.5/)
  assert.match(source, /天然掉落 data\.i 部署[\s\S]*?2\.0\.5 可部署/)
  assert.match(source, /存档修改页面[\s\S]*?8\s*\/\s*8[\s\S]*?双存档对比复制/)
  assert.match(source, /游戏内即时编辑[\s\S]*?5\s*\/\s*5[\s\S]*?均需启动并连接游戏/)
  assert.match(source, /配装采集与复刻[\s\S]*?2\s*\/\s*2[\s\S]*?点击开启后持续后台运行/)
  assert.match(source, /单机运行时工具[\s\S]*?9 页接入[\s\S]*?角色语音、城镇镜头/)
  assert.match(source, /实时只读诊断[\s\S]*?2\s*\/\s*2[\s\S]*?不会修改进程数据或存档/)
  assert.match(source, /游戏文件与设置[\s\S]*?4 页已接入[\s\S]*?掉落与锻造规则、版本适配/)
  assert.match(source, /运行时补丁覆盖[\s\S]*?60\s*已接入\s*\/\s*4\s*待证据/)
  assert.match(source, /60\s*功能\s*\/\s*83\s*站点\s*\/\s*81\s*AOB/)
  assert.match(source, /DLC\s*2\.0\.2\s*增量审计[\s\S]*?现场修复/)
  assert.match(source, /真实游戏进程\s*E2E[\s\S]*?2\.0\.5 只读连接通过[\s\S]*?任务内镜头、音频、虚拟因子和空间移动/)
  assert.match(source, /baselineVersion:\s*'游戏 2\.0\.5（静态与运行时）'/)
  assert.match(source, /2\.0\.5 同时更新 EXE 与 data\.i[\s\S]*?不使用统一地址偏移/)
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
