import assert from 'node:assert/strict'
import test from 'node:test'
import { uiTranslations } from './i18n-ui.js'
import { explainRuntimeSigilWriteError, invalidSecondaryTraitMessage } from './utils/runtimeSigilErrors.js'

const expectedSecondaryMessage = '这个副词条不能和当前因子组成有效组合，在游戏中不会生效，并会被游戏自动替换成其他词条。请清空副词条，不用写入它。'

test('invalid secondary traits explain the in-game result and the next action in plain language', () => {
  assert.equal(invalidSecondaryTraitMessage(), expectedSecondaryMessage)
})

test('unknown runtime sigil hashes no longer expose catalog jargon to users', () => {
  const backendError = new Error('因子写入参数无效: 未知因子哈希 0xCE6C62CE；四个编辑入口只接受统一目录')
  assert.equal(explainRuntimeSigilWriteError(backendError, { hasSecondaryTrait: true }), expectedSecondaryMessage)
})

test('the plain-language guidance has an exact English-mode translation', () => {
  assert.equal(
    uiTranslations[expectedSecondaryMessage],
    'This secondary trait cannot form a valid combination with the current sigil. It will not take effect in-game and the game will replace it with another trait. Clear the secondary trait instead of writing it.',
  )
})

test('unrelated write failures retain their actionable backend detail', () => {
  assert.equal(
    explainRuntimeSigilWriteError(new Error('目标因子记录已变化，请重新选择目标记录')),
    '目标因子记录已变化，请重新选择目标记录',
  )
})
