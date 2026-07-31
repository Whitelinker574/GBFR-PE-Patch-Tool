import assert from 'node:assert/strict'
import test from 'node:test'

import { translateRuntimeCompatibilityError } from './runtimeCompatibilityErrors.js'

const unknown = '实时功能仅支持已识别的游戏 2.0.2 / 2.0.3 可执行文件；当前游戏版本不会连接或写入'

test('runtime compatibility boundaries remain Chinese in Chinese mode', () => {
  assert.equal(translateRuntimeCompatibilityError(unknown, 'zh'), unknown)
})

test('runtime compatibility boundaries are complete and isolated in English mode', () => {
  const translated = translateRuntimeCompatibilityError(`Error: ${unknown}`, 'en')
  assert.doesNotMatch(translated, /[\u3400-\u9fff]/u)
  assert.match(translated, /game 2\.0\.2 and 2\.0\.3/u)
  assert.match(translated, /will not/u)
})

test('stable compatibility codes do not depend on exact Chinese backend copy', () => {
  const unknownCoded = '[GBFR_RUNTIME_UNKNOWN_EXE] 当前版本无法识别'
  assert.match(translateRuntimeCompatibilityError(unknownCoded, 'en'), /game 2\.0\.2 and 2\.0\.3/u)
})
