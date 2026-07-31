import assert from 'node:assert/strict'
import test from 'node:test'

import { translateRuntimeCompatibilityError } from './runtimeCompatibilityErrors.js'

const game203 = '实时功能暂未支持游戏 2.0.3：静态目录、配装计算、分享与 Logs 数据已核对；离线存档的 2.0.3 游戏重启回读仍待验收，实时功能不会连接或写入'
const unknown = '实时功能仅支持已验证的游戏 2.0.2 可执行文件；当前游戏版本不会连接或写入'

test('runtime compatibility boundaries remain Chinese in Chinese mode', () => {
  assert.equal(translateRuntimeCompatibilityError(game203, 'zh'), game203)
  assert.equal(translateRuntimeCompatibilityError(unknown, 'zh'), unknown)
})

test('runtime compatibility boundaries are complete and isolated in English mode', () => {
  for (const source of [game203, unknown]) {
    const translated = translateRuntimeCompatibilityError(`Error: ${source}`, 'en')
    assert.doesNotMatch(translated, /[\u3400-\u9fff]/u)
    assert.match(translated, /will not/u)
  }
  assert.match(translateRuntimeCompatibilityError(game203, 'en'), /game 2\.0\.3/u)
  assert.match(translateRuntimeCompatibilityError(game203, 'en'), /restart readback validation/u)
  assert.match(translateRuntimeCompatibilityError(unknown, 'en'), /verified game 2\.0\.2 executable/u)
})
