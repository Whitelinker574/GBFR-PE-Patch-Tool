export const FORMULA_PHASES = Object.freeze(['A1', 'B1', 'A2', 'B2'])

const phaseCopy = Object.freeze({
  A1: Object.freeze({
    zh: { title: 'A1 · 记录基准', instruction: '保持当前配装稳定，不要改动任何项目，然后采集第一份基准。' },
    en: { title: 'A1 · Baseline', instruction: 'Keep the current build unchanged and capture the first baseline.' },
  }),
  B1: Object.freeze({
    zh: { title: 'B1 · 单项变更', instruction: '只改一项可逆变量，等待游戏面板稳定后采集。' },
    en: { title: 'B1 · One change', instruction: 'Change exactly one variable that can be reversed, wait for the panel to settle, then capture.' },
  }),
  A2: Object.freeze({
    zh: { title: 'A2 · 恢复基准', instruction: '把刚才唯一的变更恢复到 A1，确认面板复原后采集。' },
    en: { title: 'A2 · Restore', instruction: 'Restore the one change to the A1 state, confirm the panel is restored, then capture.' },
  }),
  B2: Object.freeze({
    zh: { title: 'B2 · 重复变更', instruction: '重复 B1 的同一项变更，确认面板再次一致后采集并完成证据闭环。' },
    en: { title: 'B2 · Repeat', instruction: 'Repeat the exact B1 change, confirm the same panel result, then capture to close the evidence loop.' },
  }),
})

function normalizedLanguage(value) {
  return value === 'en' ? 'en' : 'zh'
}

function finiteNumber(value, label, { integer = false } = {}) {
  const number = Number(value)
  if (!Number.isFinite(number) || (integer && !Number.isInteger(number))) {
    throw new TypeError(`formula panel ${label} must be a finite ${integer ? 'integer' : 'number'}`)
  }
  return number
}

function normalizePanel(panel) {
  if (!panel || typeof panel !== 'object') throw new TypeError('formula panel is missing')
  const characterHash = String(panel.characterHash ?? '').trim().replace(/^0x/i, '').toUpperCase()
  if (!/^[0-9A-F]{8}$/.test(characterHash)) throw new TypeError('formula panel characterHash must be eight hexadecimal digits')
	const runtimeId = String(panel.runtimeId ?? characterHash).trim().replace(/^0x/i, '').toUpperCase()
	if (!/^[0-9A-F]{8}$/.test(runtimeId)) throw new TypeError('formula panel runtimeId must be eight hexadecimal digits')
	const candidateObjectHash = String(panel.candidateObjectHash ?? '00000000').trim().replace(/^0x/i, '').toUpperCase()
	if (!/^[0-9A-F]{8}$/.test(candidateObjectHash)) throw new TypeError('formula panel candidateObjectHash must be eight hexadecimal digits')
	const normalizeField = (field, rawType, relativeOffset, displayScale) => Object.freeze({
		rawType: String(field?.rawType ?? rawType),
		relativeOffset: finiteNumber(field?.relativeOffset ?? relativeOffset, 'relativeOffset', { integer: true }),
		rawBits: String(field?.rawBits ?? ''),
		displayScale: finiteNumber(field?.displayScale ?? displayScale, 'displayScale'),
		stableReads: finiteNumber(field?.stableReads ?? 0, 'stableReads', { integer: true }),
	})
  return Object.freeze({
    characterHash,
		runtimeId,
		candidateObjectHash,
		identitySource: String(panel.identitySource ?? ''),
    hp: finiteNumber(panel.hp, 'hp', { integer: true }),
    attack: finiteNumber(panel.attack, 'attack', { integer: true }),
    stunPower: finiteNumber(panel.stunPower, 'stunPower'),
		rawStunPower: finiteNumber(panel.rawStunPower ?? panel.stunPower, 'rawStunPower'),
    critRate: finiteNumber(panel.critRate, 'critRate'),
		hpField: normalizeField(panel.hpField, 'i32', 0x04, 1),
		attackField: normalizeField(panel.attackField, 'i32', 0x08, 1),
		stunField: normalizeField(panel.stunField, 'f32', 0x10, 10),
		critField: normalizeField(panel.critField, 'f32', 0x14, 1),
    source: String(panel.source ?? ''),
    verification: String(panel.verification ?? ''),
    gameVersion: String(panel.gameVersion ?? ''),
    runtimeVerified: panel.runtimeVerified === true,
  })
}

export function normalizeFormulaPanel(panel) {
	return normalizePanel(panel)
}

export function formulaPhaseCopy(phase, selectedLanguage = 'zh') {
  const copy = phaseCopy[String(phase ?? '').toUpperCase()]
  if (!copy) throw new TypeError(`unknown formula phase: ${phase}`)
  return copy[normalizedLanguage(selectedLanguage)]
}

export function formulaNextPhase(events = []) {
  if (!Array.isArray(events)) throw new TypeError('formula sampler events must be an array')
  if (events.length > FORMULA_PHASES.length) throw new TypeError('formula sampler has too many events')
  for (let index = 0; index < events.length; index += 1) {
    if (String(events[index]?.phase ?? '').toUpperCase() !== FORMULA_PHASES[index]) {
      throw new TypeError(`formula sampler event ${index + 1} is out of order`)
    }
  }
  return FORMULA_PHASES[events.length] ?? null
}

export function normalizeFormulaSamplerStatus(value) {
  if (value == null) {
    return Object.freeze({ connected: false, complete: false, events: Object.freeze([]), nextPhase: 'A1', sessionToken: '', experimentType: '' })
  }
  if (typeof value !== 'object') throw new TypeError('formula sampler status must be an object')
  const events = Object.freeze((Array.isArray(value.events) ? value.events : []).map((event, index) => {
    const phase = String(event?.phase ?? '').toUpperCase()
    if (phase !== FORMULA_PHASES[index]) throw new TypeError(`formula sampler event ${index + 1} is out of order`)
    return Object.freeze({ phase, panel: normalizePanel(event.panel) })
  }))
  const nextPhase = formulaNextPhase(events)
  const sessionToken = String(value.sessionToken ?? '')
  const experimentType = String(value.experimentType ?? '')
  if (value.connected === true && !sessionToken) {
    throw new TypeError('connected formula sampler status requires an owner token')
  }
  return Object.freeze({
    connected: value.connected === true,
    complete: nextPhase === null,
    events,
    nextPhase,
    sessionToken,
    experimentType,
  })
}
