function text(value) {
  return String(value ?? '').trim()
}

function identifier(value) {
  return String(value ?? '')
}

const featureSearchAliases = Object.freeze({
  'runtime-patch-038': ['闪避次数', '防御'],
  'runtime-patch-039': ['格挡次数', '防御'],
  'runtime-patch-040': ['完美防御', '自动格挡'],
  'runtime-patch-041': ['link', '连锁时间', '连锁槽'],
  'runtime-patch-042': ['战斗道具', '药水', '道具不减'],
  'runtime-patch-045': ['召唤石', '召唤限制'],
  'runtime-patch-046': ['召唤石', '任务限制'],
  'runtime-patch-048': ['隐藏宝箱', '开箱'],
  'runtime-patch-050': ['宝箱', '自动拾取'],
  'runtime-patch-051': ['战斗辅助', '全辅助', '无视限制'],
  'runtime-patch-052': ['无视钥匙', '直接开箱', '体验优化'],
  'runtime-patch-053': ['结算', '跳过动画'],
  'runtime-patch-054': ['msp', '境界点数', '点数不减'],
  'runtime-patch-055': ['因子合成', '强制最高等级', '炼成'],
  'runtime-patch-056': ['cp', '混沌点数', 'dlc点数'],
  'runtime-patch-057': ['专精技能上限', '突破专精', '专精上限'],
  'runtime-patch-058': ['支线目标', '额外奖励'],
  'runtime-patch-059': ['怒发冲冠', '刀上舞', '眩晕'],
  'runtime-patch-060': ['link time', '无限link', '持续不减'],
})

const modeGroupOrder = Object.freeze({
  combat: ['战斗功能'],
  quest: ['体验优化', '任务修改'],
  characters: [
    '角色修改', '古兰', '伊欧', '拉卡姆', '欧根', '卡塔莉娜', '罗塞塔', '菲莉',
    '娜露梅', '夏洛特', '尤达哈拉', '巴萨拉卡', '泽塔', '冈达葛萨', '巴恩', '伊德',
    '圣德芬', '希耶提', '索恩', '贝阿朵丽丝', '尤斯提斯', '伽兰查', '玛琪拉菲菈',
    '菲迪埃尔', '芙劳',
  ],
})

const modeFeatureOrder = Object.freeze({
  combat: [
    'runtime-patch-038', 'runtime-patch-039', 'runtime-patch-040', 'runtime-patch-042',
    'runtime-patch-041', 'runtime-patch-060', 'runtime-patch-043', 'runtime-patch-044',
    'runtime-patch-045', 'runtime-patch-046', 'runtime-patch-059',
  ],
  quest: [
    'runtime-patch-052', 'runtime-patch-051', 'runtime-patch-055', 'runtime-patch-054',
    'runtime-patch-056', 'runtime-patch-057', 'runtime-patch-050', 'runtime-patch-048',
    'runtime-patch-049', 'runtime-patch-047', 'runtime-patch-058', 'runtime-patch-053',
  ],
})

function orderedIndex(values, value) {
  const index = (values || []).indexOf(value)
  return index < 0 ? Number.MAX_SAFE_INTEGER : index
}

function catalogPatchBytes(value) {
  if (Array.isArray(value)) {
    return value.every(byte => Number.isInteger(byte) && byte >= 0 && byte <= 0xFF) ? value : null
  }
  if (typeof value !== 'string' || !/^(?:[a-z0-9+/]{4})*(?:[a-z0-9+/]{2}==|[a-z0-9+/]{3}=)?$/iu.test(value)) return null
  try {
    const decoded = globalThis.atob(value)
    return Array.from(decoded, character => character.charCodeAt(0))
  } catch {
    return null
  }
}

function searchHaystack(feature, featureLabel, groupLabel, aliases) {
  return [
    feature?.name,
    feature?.displayName,
    feature?.character,
    feature?.group,
    ...(Array.isArray(feature?.groupPath) ? feature.groupPath : []),
    featureLabel(feature),
    groupLabel(feature?.character || feature?.group),
    ...(Array.isArray(feature?.aliases) ? feature.aliases : []),
    ...(Array.isArray(aliases) ? aliases : []),
  ].map(text).join('\u0000').toLocaleLowerCase()
}

export function buildRuntimePatchGroups(features, mode, query = '', options = {}) {
  const wantedMode = text(mode)
  const needle = text(query).toLocaleLowerCase()
  const featureLabel = typeof options?.featureLabel === 'function' ? options.featureLabel : feature => feature?.displayName || feature?.name
  const groupLabel = typeof options?.groupLabel === 'function' ? options.groupLabel : value => value
  const aliasesFor = typeof options?.aliasesFor === 'function'
    ? options.aliasesFor
    : feature => featureSearchAliases[identifier(feature?.id)] || []
  const groups = new Map()

  for (const feature of Array.isArray(features) ? features : []) {
    if (feature?.mode !== wantedMode) continue
    if (needle && !searchHaystack(feature, featureLabel, groupLabel, aliasesFor(feature)).includes(needle)) continue
    const key = text(wantedMode === 'characters' ? feature.character || feature.group : feature.group) || '其他'
    if (!groups.has(key)) groups.set(key, [])
    groups.get(key).push(feature)
  }

  const featureOrder = modeFeatureOrder[wantedMode] || []
  const groupOrder = modeGroupOrder[wantedMode] || []
  return [...groups]
    .map(([key, groupedFeatures]) => ({
      key,
      label: text(groupLabel(key)) || key,
      features: [...groupedFeatures].sort((left, right) => {
        const priority = orderedIndex(featureOrder, identifier(left?.id)) - orderedIndex(featureOrder, identifier(right?.id))
        if (priority) return priority
        return Number(left?.catalogId || 0) - Number(right?.catalogId || 0)
      }),
    }))
    .sort((left, right) => {
      const priority = orderedIndex(groupOrder, left.key) - orderedIndex(groupOrder, right.key)
      if (priority) return priority
      return left.label.localeCompare(right.label, 'zh-Hans-CN')
    })
}

export function filterRuntimePatchGroups(groups, scope, statusIndex) {
  const source = Array.isArray(groups) ? groups : []
  if (scope === 'all') return source
  if (scope === 'active') {
    return source.map(group => ({
      ...group,
      features: group.features.filter(feature => {
        const status = statusIndex?.get(identifier(feature?.id))
        return status?.enabled || (Array.isArray(status?.rvas) && status.rvas.length > 0)
      }),
    })).filter(group => group.features.length > 0)
  }
  return source.filter(group => group.key === scope)
}

export function buildRuntimePatchStatusIndex(statuses) {
  return new Map((Array.isArray(statuses) ? statuses : [])
    .filter(status => text(status?.id))
    .map(status => [text(status.id), status]))
}

export function validateRuntimePatchStatusSet(features, statuses) {
  if (!Array.isArray(features) || !Array.isArray(statuses)) {
    throw new TypeError('实时功能目录与回读状态必须是数组')
  }

  const expectedIDs = features.map(feature => identifier(feature?.id))
  if (expectedIDs.some(id => !id.trim()) || new Set(expectedIDs).size !== expectedIDs.length) {
    throw new Error('实时功能目录 ID 必须非空且唯一')
  }
  if (statuses.length !== expectedIDs.length) {
    throw new Error('实时补丁回读状态数量与目录不一致')
  }

  const returnedIDs = statuses.map(status => identifier(status?.id))
  if (returnedIDs.some(id => !id.trim())) throw new Error('实时补丁回读状态 ID 不能为空')
  if (new Set(returnedIDs).size !== returnedIDs.length) {
    const duplicateID = returnedIDs.find((id, index) => returnedIDs.indexOf(id) !== index)
    throw new Error(`实时补丁回读状态 ID 重复：${duplicateID}`)
  }

  const expectedSet = new Set(expectedIDs)
  const unexpectedID = returnedIDs.find(id => !expectedSet.has(id))
  if (unexpectedID) throw new Error(`实时补丁回读状态包含目录外 ID：${unexpectedID}`)
  const featuresByID = new Map(features.map(feature => [identifier(feature?.id), feature]))
  for (const status of statuses) {
    const statusID = identifier(status?.id)
    if (typeof status?.enabled !== 'boolean') {
      throw new TypeError(`实时补丁回读状态 ${statusID} 的 enabled 必须是布尔值`)
    }
    if (typeof status.available !== 'boolean') {
      throw new TypeError(`实时补丁回读状态 ${statusID} 的 available 必须是布尔值`)
    }
    if (typeof status.error !== 'string') {
      throw new TypeError(`实时补丁回读状态 ${statusID} 的 error 必须是字符串`)
    }
    if (!Array.isArray(status.rvas)) {
      throw new TypeError(`实时补丁回读状态 ${statusID} 的 rvas 必须是数组`)
    }
    if (!Array.isArray(status.currentBytes)) {
      throw new TypeError(`实时补丁回读状态 ${statusID} 的 currentBytes 必须是数组`)
    }
    if (status.rvas.length !== status.currentBytes.length) {
      throw new Error(`实时补丁回读状态 ${statusID} 的 rvas 与 currentBytes 长度必须一致`)
    }

    const feature = featuresByID.get(statusID)
    const sites = Array.isArray(feature?.sites) ? feature.sites : []
    if (status.rvas.length > 0 && status.rvas.length !== sites.length) {
      throw new Error(`实时补丁回读状态 ${statusID} 的写入点数量与目录不一致`)
    }
    status.rvas.forEach((rva, index) => {
      if (!Number.isSafeInteger(rva) || rva < 0) {
        throw new TypeError(`实时补丁回读状态 ${statusID} 的 RVA[${index}] 必须是非负安全整数`)
      }
    })
    status.currentBytes.forEach((currentBytes, index) => {
      if (typeof currentBytes !== 'string' || (currentBytes !== '' && !/^(?:[0-9a-f]{2})(?: [0-9a-f]{2})*$/iu.test(currentBytes))) {
        throw new TypeError(`实时补丁回读状态 ${statusID} 的当前字节[${index}] 必须是空值或空格分隔的十六进制字节`)
      }
      const enableBytes = catalogPatchBytes(sites[index]?.enableBytes)
      if (!enableBytes) {
        throw new Error(`实时功能目录 ${statusID} 的补丁字节无效`)
      }
      if (currentBytes && currentBytes.split(' ').length !== enableBytes.length) {
        throw new Error(`实时补丁回读状态 ${statusID} 的当前字节[${index}] 长度与目录补丁不一致`)
      }
    })

    if (status.enabled && status.rvas.length === 0) {
      throw new Error(`实时补丁回读状态 ${statusID} 已开启却没有持有写入点`)
    }
    if (status.enabled && !status.available) {
      throw new Error(`实时补丁回读状态 ${statusID} 已开启时 available 必须为 true`)
    }
    if (status.enabled && status.error !== '') {
      throw new Error(`实时补丁回读状态 ${statusID} 已开启时 error 必须为空`)
    }
    if (status.enabled) {
      status.currentBytes.forEach((currentBytes, index) => {
        const expected = catalogPatchBytes(sites[index].enableBytes).map(byte => byte.toString(16).padStart(2, '0')).join(' ')
        if (currentBytes.toLocaleLowerCase() !== expected) {
          throw new Error(`实时补丁回读状态 ${statusID} 已开启，但当前字节[${index}] 与目录补丁不一致`)
        }
      })
    }
  }
  return statuses
}

export function findActiveRuntimePatchConflict(feature, statusIndex, features) {
  const byID = new Map((Array.isArray(features) ? features : [])
    .filter(item => text(item?.id))
    .map(item => [text(item.id), item]))
  for (const conflictID of Array.isArray(feature?.conflicts) ? feature.conflicts : []) {
    const status = statusIndex?.get(text(conflictID))
    if (status?.enabled || (Array.isArray(status?.rvas) && status.rvas.length > 0)) {
      return byID.get(text(conflictID)) || null
    }
  }
  return null
}

export function replaceRuntimePatchFeatureIDs(value, features) {
  let output = String(value ?? '')
  for (const feature of Array.isArray(features) ? features : []) {
    const id = text(feature?.id)
    const name = text(feature?.displayName || feature?.name)
    if (id && name) output = output.split(id).join(`「${name}」`)
  }
  return output
}
