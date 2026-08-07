const compatibilityCode = /^\[(GBFR_RUNTIME_UNKNOWN_EXE)\]\s*/u
const legacyUnknownBuildBoundary = /^(.*?)仅支持已识别的游戏 2\.0\.2 \/ 2\.0\.3(?: \/ 2\.0\.4)? 可执行文件；当前游戏版本不会连接或写入$/u

function featureLabel(value) {
  const source = String(value || '').trim()
  if (!source || source === '实时功能') return 'Live features'
  return 'This live feature'
}

export function translateRuntimeCompatibilityError(value, locale = 'zh') {
  const source = String(value ?? '').replace(/^Error:\s*/iu, '')
  const coded = source.match(compatibilityCode)
  const display = source.replace(compatibilityCode, '')
  if (locale !== 'en') return display

  const match = display.match(legacyUnknownBuildBoundary)
  if (coded?.[1] === 'GBFR_RUNTIME_UNKNOWN_EXE' || match) {
    return `${featureLabel(match?.[1])} supports the recognized game 2.0.2, 2.0.3, and 2.0.4 executables. This unknown game build will not be connected to or modified.`
  }
  return display
}
