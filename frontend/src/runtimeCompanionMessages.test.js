import assert from 'node:assert/strict'
import test from 'node:test'

import { runtimeCompanionMessage } from './runtimeCompanionMessages.js'

test('built-in runtime details stay Chinese in the Chinese interface', () => {
  assert.equal(
    runtimeCompanionMessage('audio hook restored after active callbacks drained', 'zh'),
    '音频回调已安全结束，Wwise Hook 已恢复',
  )
  assert.equal(
    runtimeCompanionMessage('内置运行时启动失败: camera hook installation failed', 'zh'),
    '内置运行时启动失败: 镜头 Hook 安装失败',
  )
})

test('English interface keeps native diagnostic details', () => {
  const detail = 'hooks and native loop limits restored'
  assert.equal(runtimeCompanionMessage(detail, 'en'), detail)
})
