function text(value) {
  return String(value ?? '').trim()
}

function identifier(value) {
  return String(value ?? '')
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

function searchHaystack(feature, featureLabel, groupLabel) {
  return [
    feature?.name,
    feature?.displayName,
    feature?.character,
    feature?.group,
    ...(Array.isArray(feature?.groupPath) ? feature.groupPath : []),
    featureLabel(feature),
    groupLabel(feature?.character || feature?.group),
  ].map(text).join('\u0000').toLocaleLowerCase()
}

export function buildCT084Groups(features, mode, query = '', options = {}) {
  const wantedMode = text(mode)
  const needle = text(query).toLocaleLowerCase()
  const featureLabel = typeof options?.featureLabel === 'function' ? options.featureLabel : feature => feature?.displayName || feature?.name
  const groupLabel = typeof options?.groupLabel === 'function' ? options.groupLabel : value => value
  const groups = new Map()

  for (const feature of Array.isArray(features) ? features : []) {
    if (feature?.mode !== wantedMode) continue
    if (needle && !searchHaystack(feature, featureLabel, groupLabel).includes(needle)) continue
    const key = text(wantedMode === 'characters' ? feature.character || feature.group : feature.group) || '其他'
    if (!groups.has(key)) groups.set(key, [])
    groups.get(key).push(feature)
  }

  return [...groups].map(([key, groupedFeatures]) => ({
    key,
    label: text(groupLabel(key)) || key,
    features: groupedFeatures,
  }))
}

export function buildCT084StatusIndex(statuses) {
  return new Map((Array.isArray(statuses) ? statuses : [])
    .filter(status => text(status?.id))
    .map(status => [text(status.id), status]))
}

export function validateCT084StatusSet(features, statuses) {
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

export function findActiveCT084Conflict(feature, statusIndex, features) {
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

export function replaceCT084FeatureIDs(value, features) {
  let output = String(value ?? '')
  for (const feature of Array.isArray(features) ? features : []) {
    const id = text(feature?.id)
    const name = text(feature?.displayName || feature?.name)
    if (id && name) output = output.split(id).join(`「${name}」`)
  }
  return output
}
