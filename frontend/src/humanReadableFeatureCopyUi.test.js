import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

import { uiTranslations } from './i18n-ui.js'

const read = name => readFileSync(new URL(`./components/${name}.vue`, import.meta.url), 'utf8')
const shell = read('PatchTool')
const home = read('HomeJournal')
const sigil = read('SigilGenerator')
const sigilMemory = read('SigilMemoryGenerator')
const wrightstone = read('WrightstoneGenerator')
const wrightstoneMemory = read('WrightstoneMemoryGenerator')
const saveDiff = read('SaveDiffLab')
const audio = read('AudioMixerLab')
const camera = read('CameraLab')
const virtualSigils = read('VirtualSigilLab')
const runtimeCopy = readFileSync(new URL('./runtimePatchMonitorView.js', import.meta.url), 'utf8')

test('tool entry copy tells users what changes, how to apply it, and how to recover', () => {
  assert.match(shell, /progression:\s*\{[\s\S]*?只改你在页面中确认的项目[\s\S]*?存档保护/)
  assert.match(shell, /sigil:\s*\{[\s\S]*?新增独立因子实例[\s\S]*?自动备份并回读[\s\S]*?恢复本次写入前的备份/)
  assert.match(shell, /sigilMemory:\s*\{[\s\S]*?当前高亮的那一颗因子[\s\S]*?写入的是当前游戏进程/)
  assert.match(shell, /wrightstone:\s*\{[\s\S]*?新增祝福石实例[\s\S]*?技能曲线[\s\S]*?存档保护/)
  assert.match(shell, /runtime:\s*\{[\s\S]*?title:\s*'货币、素材与任务掉落'[\s\S]*?当前游戏会话/)
  assert.match(shell, /saveDiff:\s*\{[\s\S]*?不需要跳转到其他编辑页[\s\S]*?自动备份、原子写入并逐条回读/)
})

test('embedded natural-drop catalogs are described as app-owned data, not a user-supplied folder', () => {
  const block = shell.match(/naturalDrop:\s*\{[\s\S]*?\n  \},/)?.[0] || ''
  assert.match(block, /应用内置的 2\.0\.2 目录/)
  assert.match(block, /十一张 2\.0\.2 精确表已随应用内嵌并校验/)
  assert.match(block, /普通物品会加入“无尽模式·锻造师奖励池”/)
  assert.doesNotMatch(block, /自行解包得到|选择.*system\/table/)
})

test('home destinations describe the normal user action instead of implementation jargon', () => {
  assert.match(home, /配装预设[\s\S]*?按技能目标配因子/)
  assert.match(home, /title:\s*'货币、素材与任务掉落'/)
  assert.match(home, /读取并修改游戏里当前高亮的因子/)
  assert.match(home, /运行时读取额外库存因子，不扩存档十二槽/)
  assert.doesNotMatch(home, /title:\s*'游戏便利运行时'/)
})

test('offline and live equipment editors present an explicit read, review, and write flow', () => {
  assert.match(sigil, /第二步 · 配置要新增的因子/)
  assert.match(sigil, /第三步 · 核对待写入清单/)
  assert.match(sigil, /第四步 · 保存到当前存档/)
  assert.match(sigilMemory, /第一步 · 启用读取/)
  assert.match(sigilMemory, /第二步 · 调整因子/)
  assert.match(sigilMemory, /第三步 · 写入当前因子/)
  assert.match(wrightstone, /第二步 · 配置要新增的祝福石/)
  assert.match(wrightstone, /第四步 · 保存到当前存档/)
  assert.match(wrightstoneMemory, /第一步 · 读取游戏中选中的祝福石/)
  assert.match(wrightstoneMemory, /第三步 · 写入当前祝福石/)
  assert.doesNotMatch(wrightstoneMemory, /detail:\s*`\$\{changeDetail\}\\n\\n重复/u)
})

test('save comparison and runtime pages state their write boundary and recovery action', () => {
  assert.match(saveDiff, /把需要的差异从一侧拖到另一侧/)
  assert.match(saveDiff, /默认存档写入要求游戏完全退出/)
  assert.match(saveDiff, /脱敏差分已导出到：\$\{path\}/)
  assert.match(runtimeCopy, /坐标移动与重力抑制是两个独立功能；穿墙仍未开放/)
  assert.match(runtimeCopy, /这里.*没有修改数量、Hash 或 Flags 的入口/)
  assert.match(audio, /停用并恢复原音/)
  assert.match(camera, /停用并恢复原镜头/)
  assert.match(virtualSigils, /不会把存档里的 12 个配装槽扩成更多槽位/)
  assert.match(virtualSigils, /停用并恢复原技能读取/)
})

test('new shell copy has exact English translations instead of mixed-language fallbacks', () => {
  for (const chinese of [
    '货币、素材与任务掉落',
    '给所选存档补充素材和养成资源，或调整武器等级与强化进度；只改你在页面中确认的项目。',
    '修改游戏实际读取的掉落与锻造表：可添加 Transmarvel 因子、召唤石、祝福石和普通物品，不会直接向存档背包添加物品。',
    '十一张 2.0.2 精确表已随应用内嵌并校验，不需要自行解包；普通物品会加入“无尽模式·锻造师奖励池”。发现同表冲突时会停止，避免覆盖其他模组。',
    '查看整套配装，手动编辑或按技能目标配因子',
    '运行时读取额外库存因子，不扩存档十二槽',
    '第二步 · 配置要新增的因子',
    '第四步 · 保存到当前存档',
    '第三步 · 写入当前因子',
  ]) {
    assert.ok(uiTranslations[chinese], `missing exact translation for: ${chinese}`)
    assert.doesNotMatch(uiTranslations[chinese], /[\u3400-\u9fff]/u)
  }
})
