export const COMBAT_TUNING_COOLDOWN_ID = 'combat-tuning-cooldown'
export const COMBAT_TUNING_CHARGE_ID = 'combat-tuning-charge'

const MULTIPLIER_MIN = 0.1
const MULTIPLIER_MAX = 100

function assertBoolean(value, label) {
  if (typeof value !== 'boolean') throw new Error(`${label} 必须是布尔值`)
  return value
}

function assertOptionalBoolean(value, label) {
  if (value === undefined) return false
  return assertBoolean(value, label)
}

function assertString(value, label, optional = false) {
  if (optional && value === undefined) return ''
  if (typeof value !== 'string') throw new Error(`${label} 必须是字符串`)
  return value
}

function normalizeFeatureStatus(value, kind) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error(`${kind}回读状态格式无效`)
  }
  const speedMultiplier = Number(value.speedMultiplier)
  if (!Number.isFinite(speedMultiplier) || speedMultiplier < MULTIPLIER_MIN || speedMultiplier > MULTIPLIER_MAX) {
    throw new Error(`${kind}回读倍率超出 0.1 到 100`)
  }
  if (!Array.isArray(value.rvas) || !Array.isArray(value.currentBytes) || value.rvas.length !== value.currentBytes.length) {
    throw new Error(`${kind}回读写入点格式无效`)
  }
  const rvas = value.rvas.map((rva, index) => {
    if (!Number.isSafeInteger(rva) || rva < 0) throw new Error(`${kind}回读 RVA[${index}] 无效`)
    return rva
  })
  const currentBytes = value.currentBytes.map((bytes, index) => {
    if (typeof bytes !== 'string' || (bytes !== '' && !/^(?:[0-9A-F]{2})(?: [0-9A-F]{2})*$/iu.test(bytes))) {
      throw new Error(`${kind}回读当前字节[${index}]无效`)
    }
    return bytes
  })
  return {
    available: assertBoolean(value.available, `${kind} available`),
    enabled: assertBoolean(value.enabled, `${kind} enabled`),
    candidate: assertBoolean(value.candidate, `${kind} candidate`),
    instant: assertOptionalBoolean(value.instant, `${kind} instant`),
    noCooldown: assertOptionalBoolean(value.noCooldown, `${kind} noCooldown`),
    applyWholeParty: assertOptionalBoolean(value.applyWholeParty, `${kind} applyWholeParty`),
    speedMultiplier,
    rvas,
    currentBytes,
    evidenceNote: assertString(value.evidenceNote, `${kind} evidenceNote`),
    error: assertString(value.error, `${kind} error`, true),
  }
}

export function emptyCombatTuningStatus() {
  const feature = () => ({
    available: false,
    enabled: false,
    candidate: true,
    instant: false,
    noCooldown: false,
    applyWholeParty: false,
    speedMultiplier: 2,
    rvas: [],
    currentBytes: [],
    evidenceNote: '',
    error: '',
  })
  return { cooldown: feature(), charge: feature() }
}

export function normalizeCombatTuningStatus(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('战斗参数回读状态格式无效')
  }
  return {
    cooldown: normalizeFeatureStatus(value.cooldown, '冷却'),
    charge: normalizeFeatureStatus(value.charge, '蓄力'),
  }
}

export function parseCombatTuningMultiplier(value, label) {
  const text = String(value ?? '').trim()
  const numeric = Number(text)
  if (!text || !Number.isFinite(numeric) || numeric < MULTIPLIER_MIN || numeric > MULTIPLIER_MAX) {
    throw new Error(`${label}请输入 0.1 到 100`)
  }
  return numeric
}

export function buildCooldownRequest({ enabled = true, mode, multiplier, scope }) {
  if (!enabled) return { enabled: false, noCooldown: false, speedMultiplier: 2, applyWholeParty: false }
  if (!['multiplier', 'instant'].includes(mode)) throw new Error('请选择冷却调整方式')
  if (!['self', 'party'].includes(scope)) throw new Error('请选择冷却作用范围')
  return {
    enabled: true,
    noCooldown: mode === 'instant',
    speedMultiplier: mode === 'instant' ? 2 : parseCombatTuningMultiplier(multiplier, '冷却速度倍率'),
    applyWholeParty: scope === 'party',
  }
}

export function buildChargeRequest({ enabled = true, mode, multiplier }) {
  if (!enabled) return { enabled: false, instant: false, speedMultiplier: 2 }
  if (!['multiplier', 'instant'].includes(mode)) throw new Error('请选择蓄力调整方式')
  return {
    enabled: true,
    instant: mode === 'instant',
    speedMultiplier: mode === 'instant' ? 2 : parseCombatTuningMultiplier(multiplier, '蓄力速度倍率'),
  }
}

export function combatTuningStatusMatchesRequest(status, request, kind) {
  if (!status || status.enabled !== request.enabled) return false
  if (!request.enabled) return true
  if (kind === 'cooldown') {
    return status.noCooldown === request.noCooldown
      && status.applyWholeParty === request.applyWholeParty
      && (request.noCooldown || status.speedMultiplier === request.speedMultiplier)
  }
  return status.instant === request.instant
    && (request.instant || status.speedMultiplier === request.speedMultiplier)
}
