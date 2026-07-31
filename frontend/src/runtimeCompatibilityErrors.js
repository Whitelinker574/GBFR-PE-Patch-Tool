const compatibilityCode = /^\[(GBFR_RUNTIME_GAME_203|GBFR_RUNTIME_UNKNOWN_EXE)\]\s*/u
const legacyGame203Boundary = /^(.*?)暂未支持游戏 2\.0\.3：静态目录、配装计算、分享与 Logs 数据已核对；离线存档的 2\.0\.3 游戏重启回读仍待验收，实时功能不会连接或写入$/u
const legacyUnknownBuildBoundary = /^(.*?)仅支持已验证的游戏 2\.0\.2 可执行文件；当前游戏版本不会连接或写入$/u

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

  let match = display.match(legacyGame203Boundary)
  if (coded?.[1] === 'GBFR_RUNTIME_GAME_203' || match) {
    return `${featureLabel(match?.[1])} does not yet support game 2.0.3. Static catalogs, loadout calculation, sharing, and Logs data have been checked. A save produced by 2.0.3 still needs game-restart readback validation. Live features will not connect or write.`
  }
  match = display.match(legacyUnknownBuildBoundary)
  if (coded?.[1] === 'GBFR_RUNTIME_UNKNOWN_EXE' || match) {
    return `${featureLabel(match?.[1])} supports only the verified game 2.0.2 executable. The current game build will not be connected to or modified.`
  }
  return display
}
