const INVALID_SECONDARY_TRAIT_MESSAGE = '这个副词条不能和当前因子组成有效组合，在游戏中不会生效，并会被游戏自动替换成其他词条。请清空副词条，不用写入它。'
const INVALID_SIGIL_MESSAGE = '当前因子不是游戏 2.0.2 中可用的有效因子，写入后不会按画面中的内容生效。请从列表重新选择因子。'

export function invalidSecondaryTraitMessage() {
  return INVALID_SECONDARY_TRAIT_MESSAGE
}

export function invalidRuntimeSigilMessage() {
  return INVALID_SIGIL_MESSAGE
}

function cleanErrorMessage(error) {
  return String(error ?? '')
    .replace(/^Error:\s*/i, '')
    .replace(/^因子写入参数无效[:：]\s*/, '')
    .trim()
}

export function explainRuntimeSigilWriteError(error, { hasSecondaryTrait = false } = {}) {
  const detail = cleanErrorMessage(error)
  const invalidSecondary = /(?:未知因子哈希|未知副词条哈希|副词条.+不能用于因子|没有副词条槽|主词条与副词条重复|天然副词条池)/.test(detail)
  if (invalidSecondary && hasSecondaryTrait) return invalidSecondaryTraitMessage()
  if (/未知因子哈希/.test(detail)) return invalidRuntimeSigilMessage()
  return detail || '写入没有完成，请重新选择游戏中的目标因子后再试。'
}
