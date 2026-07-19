function text(value) {
  return String(value ?? '').trim()
}

function identifier(value) {
  return String(value ?? '')
}

function searchHaystack(feature) {
  return [
    feature?.name,
    feature?.displayName,
    feature?.character,
    feature?.group,
    ...(Array.isArray(feature?.groupPath) ? feature.groupPath : []),
  ].map(text).join('\u0000').toLocaleLowerCase()
}

export function buildCT084Groups(features, mode, query = '') {
  const wantedMode = text(mode)
  const needle = text(query).toLocaleLowerCase()
  const groups = new Map()

  for (const feature of Array.isArray(features) ? features : []) {
    if (feature?.mode !== wantedMode) continue
    if (needle && !searchHaystack(feature).includes(needle)) continue
    const key = text(wantedMode === 'characters' ? feature.character || feature.group : feature.group) || '其他'
    if (!groups.has(key)) groups.set(key, [])
    groups.get(key).push(feature)
  }

  return [...groups].map(([key, groupedFeatures]) => ({
    key,
    label: key,
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
    throw new TypeError('CT 0.8.4 目录与回读状态必须是数组')
  }

  const expectedIDs = features.map(feature => identifier(feature?.id))
  if (expectedIDs.some(id => !id.trim()) || new Set(expectedIDs).size !== expectedIDs.length) {
    throw new Error('CT 0.8.4 功能目录 ID 必须非空且唯一')
  }
  if (statuses.length !== expectedIDs.length) {
    throw new Error('CT 0.8.4 回读状态数量与目录不一致')
  }

  const returnedIDs = statuses.map(status => identifier(status?.id))
  if (returnedIDs.some(id => !id.trim())) throw new Error('CT 0.8.4 回读状态 ID 不能为空')
  if (new Set(returnedIDs).size !== returnedIDs.length) {
    const duplicateID = returnedIDs.find((id, index) => returnedIDs.indexOf(id) !== index)
    throw new Error(`CT 0.8.4 回读状态 ID 重复：${duplicateID}`)
  }

  const expectedSet = new Set(expectedIDs)
  const unexpectedID = returnedIDs.find(id => !expectedSet.has(id))
  if (unexpectedID) throw new Error(`CT 0.8.4 回读状态包含目录外 ID：${unexpectedID}`)
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
