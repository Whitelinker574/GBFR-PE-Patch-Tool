const PARTY_ROLES = Object.freeze(['player', 'party1', 'party2', 'party3', 'companion'])
const SELECTED_KINDS = Object.freeze(['material', 'keyItem'])
const SELECTED_RVAS = Object.freeze({ material: 0x3F4BAC3, keyItem: 0x3F2061C })

const COPY = Object.freeze({
  memoryMonitoring: ['内存监测', 'Memory Monitoring'],
  sourceLabel: ['游戏 2.0.2', 'Game 2.0.2'],
  pageTitle: ['角色配装检测', 'Character Loadout Detection'],
  pageSummary: ['后台自动记录每场任务的队伍配装，并保留选中物品只读工具。', 'Automatically archive party loadouts for every quest while retaining the read-only selected-item tool.'],
  readOnly: ['只读', 'Read Only'],
  notConnected: ['未连接', 'Not Connected'],
  connected: ['已连接', 'Connected'],
  releasing: ['等待安全释放', 'Waiting for Safe Release'],
  connect: ['连接游戏进程', 'Connect to Game Process'],
  disconnect: ['安全断开', 'Disconnect Safely'],
  retryDisconnect: ['重试安全断开', 'Retry Safe Disconnect'],
  refresh: ['刷新', 'Refresh'],
  refreshing: ['刷新中…', 'Refreshing…'],
  working: ['处理中…', 'Working…'],
  tabParty: ['任务配装记录', 'Quest Loadout History'],
  tabItems: ['选中物品', 'Selected Item'],
  partyTitle: ['队伍数值与配装捕获', 'Party Values & Loadout Capture'],
  partySummary: ['点击“读取队伍与配装”后连续读取三次；稳定时会在每个角色卡片底部出现配装操作。', 'Select Read Party & Loadouts to take three consecutive reads. Stable captures expose loadout actions at the bottom of each character card.'],
  readPartyLoadouts: ['读取队伍与配装', 'Read Party & Loadouts'],
  readingPartyLoadouts: ['正在读取队伍与配装…', 'Reading Party & Loadouts…'],
  loadoutGuideTitle: ['配装捕获步骤', 'Loadout Capture Steps'],
  loadoutGuideConnect: ['连接游戏进程', 'Connect to the game'],
  loadoutGuideRead: ['进入稳定场景后读取队伍与配装', 'Enter a stable scene and read party loadouts'],
  loadoutGuideOpen: ['展开角色卡片中的配装操作', 'Open the loadout actions in a character card'],
  partyEmptyTitle: ['尚无队伍快照', 'No Party Snapshot Yet'],
  partyEmptyCopy: ['连接游戏并进入稳定场景后刷新；切换场景时请稍候再试。', 'Connect to the game, enter a stable scene, then refresh. Wait briefly after changing scenes.'],
  partyReadyTitle: ['三快照拓扑验证通过', 'Three-Snapshot Topology Verified'],
  partyReadyCopy: ['数值来自最后一次稳定快照，不会自动伪造或补零。', 'Values come from the final stable snapshot and are never fabricated or zero-filled.'],
  verifiedSnapshot: ['运行时已验证', 'Runtime Verified'],
  snapshotCount: ['连续快照', 'Consecutive Snapshots'],
  gameVersion: ['游戏版本', 'Game Version'],
  processId: ['进程 PID', 'Process PID'],
  hp: ['HP', 'HP'],
  sba: ['奥义槽', 'SBA Gauge'],
  dodge: ['闪避次数', 'Dodge Count'],
  position: ['位置', 'Position'],
  directPosition: ['直接坐标', 'Direct Position'],
  entityAddress: ['实体地址', 'Entity Address'],
  loadout: ['队友配装', 'Party Loadout'],
  loadoutCandidate: ['稳定候选', 'Stable Candidate'],
  loadoutUnavailable: ['本次未定位', 'Unavailable This Read'],
  loadoutEvidence: ['证据边界', 'Evidence Boundary'],
  loadoutCandidateCopy: ['武器与因子连续三次内容一致，但仍需 2.0.2 实机逐项对照后才能升级为已验证布局。', 'Weapon and sigil content matched across three reads, but the layout remains a candidate until field-by-field 2.0.2 live comparison.'],
  character: ['角色', 'Character'],
  level: ['等级', 'Level'],
  attack: ['攻击力', 'Attack'],
  stunPower: ['昏厥值', 'Stun Power'],
  criticalRate: ['暴击率', 'Critical Rate'],
  totalPower: ['战力', 'Power'],
  weapon: ['武器', 'Weapon'],
  weaponLevel: ['武器等级', 'Weapon Level'],
  awakeningLevel: ['觉醒', 'Awakening'],
  plusMarks: ['强化', 'Plus Marks'],
  weaponTraits: ['武器祝福附加技能', 'Wrightstone Traits'],
  weaponSkills: ['武器可替换技能', 'Replaceable Weapon Skills'],
  overLimit: ['上限突破', 'Overmastery'],
  emptySlot: ['空槽', 'Empty'],
  loadoutTitle: ['分享名称', 'Share Title'],
  runtimeCapture: ['实时捕获配装', 'Runtime Capture'],
  runtimePreviewTitle: ['运行时完整配装预览', 'Full Runtime Loadout Preview'],
  previewLoadout: ['预览配装', 'Preview Loadout'],
  backToRuntimeMonitor: ['返回运行监测', 'Back to Runtime Monitor'],
  loadoutOpenActions: ['展开配装操作', 'Open Loadout Actions'],
  loadoutNotCaptured: ['尚未捕获配装', 'Loadout Not Captured Yet'],
  loadoutNotCapturedCopy: ['连接后点击上方“读取队伍与配装”；稳定读取成功后，这里会出现复制、导出、上传和部署按钮。', 'After connecting, select Read Party & Loadouts above. A stable capture will expose copy, export, upload, and deploy actions here.'],
  copyLoadoutCode: ['复制配装码', 'Copy Loadout Code'],
  exportLoadout: ['导出 JSON', 'Export JSON'],
  uploadLoadout: ['上传并复制链接', 'Upload and Copy Link'],
  deployLoadout: ['部署到存档', 'Deploy to Save'],
  loadoutCodeCopied: ['已复制完整配装码', 'Full loadout code copied'],
  loadoutExported: ['已导出实时配装：{output}', 'Runtime loadout exported: {output}'],
  loadoutPublished: ['已上传配装并复制链接：{code}', 'Loadout uploaded and link copied: {code}'],
  loadoutDeployReady: ['已转到配装预设；选择目标存档后会打开分项导入。', 'Opened loadout presets; select a target save to continue selective import.'],
  sigils: ['因子', 'Sigils'],
  noWeaponTraits: ['未读取到武器技能', 'No weapon traits read'],
  noSigils: ['未装备因子', 'No sigils equipped'],
  runtimeLayout: ['相对访问链', 'Relative Access Path'],
  fieldUnavailable: ['此实体无该字段', 'This entity does not have this field'],
  player: ['玩家', 'Player'],
  party1: ['队伍成员 1', 'Party Member 1'],
  party2: ['队伍成员 2', 'Party Member 2'],
  party3: ['队伍成员 3', 'Party Member 3'],
  companion: ['碧的小红龙', 'Vyrn'],
  selectedTitle: ['当前选中素材 / 关键物品', 'Currently Selected Material / Key Item'],
  selectedSummary: ['分别捕获游戏内素材列表与关键物品列表当前高亮记录。', 'Capture the currently highlighted record from the in-game material or key-item list.'],
  readOnlyBanner: ['只读，不会写物品/存档', 'Read only — never writes items or save data'],
  neverWritesSave: ['捕获器只记录选中地址；一次读取会核对完整 0x0C 记录并清除该地址，页面没有数量、Hash 或 Flags 写入入口。', 'The capture records only a selected address. A one-shot read revalidates the complete 0x0C record and clears that address; this page has no quantity, hash, or flags writer.'],
  hookTechnical: ['启用时会临时安装两个只读地址捕获 Hook；安全断开会恢复原字节。', 'Enabling temporarily installs two read-only address-capture hooks. Safe disconnect restores the original bytes.'],
  enableCapture: ['启用只读捕获', 'Enable Read-Only Capture'],
  disableCapture: ['停用并恢复原字节', 'Disable and Restore Original Bytes'],
  refreshCapture: ['刷新捕获状态', 'Refresh Capture Status'],
  captureDisabled: ['捕获未启用', 'Capture Disabled'],
  captureAwaiting: ['等待在游戏中选中', 'Waiting for In-Game Selection'],
  captureReady: ['已捕获，可读取一次', 'Captured — Ready for One Read'],
  needsReselection: ['本次已读取，请重新选择', 'Read Consumed — Select Again'],
  material: ['素材', 'Material'],
  keyItem: ['关键物品', 'Key Item'],
  selectedAddress: ['本次选中地址', 'Selected Address'],
  readOnce: ['读取一次', 'Read Once'],
  reading: ['读取中…', 'Reading…'],
  lastRead: ['最近一次真实读取', 'Most Recent Real Read'],
  noRecord: ['尚未读取记录', 'No Record Read Yet'],
  catalogName: ['目录名', 'Catalog Name'],
  category: ['目录分类', 'Catalog Category'],
  hash: ['Hash', 'Hash'],
  quantity: ['数量', 'Quantity'],
  flags: ['Flags', 'Flags'],
  unknownCategory: ['本地目录未命名', 'Not Named in Local Catalog'],
  stepConnect: ['连接游戏', 'Connect to the game'],
  stepEnable: ['启用只读捕获', 'Enable read-only capture'],
  stepSelect: ['在对应游戏列表中选中目标，再刷新状态', 'Highlight the target in the matching in-game list, then refresh'],
  stepRead: ['地址出现后读取一次；下一次必须重新选择', 'Read once after an address appears; select again for the next read'],
  statusConnect: ['连接游戏后可读取真实运行时数据。', 'Connect to read real runtime data.'],
  statusConnected: ['已连接游戏进程 PID {pid}。', 'Connected to game process PID {pid}.'],
  statusDisconnected: ['尚未连接游戏进程。', 'No game process is connected.'],
  statusReleaseFailed: ['安全断开尚未完成，恢复任务会在后台重试：{error}', 'Safe disconnect is incomplete. Restoration will retry in the background: {error}'],
  statusPartyRead: ['已读取并验证当前队伍快照。', 'Read and verified the current party snapshot.'],
  statusPartyFailed: ['队伍快照读取失败：{error}', 'Party snapshot failed: {error}'],
  statusCaptureEnabled: ['两个只读捕获器已启用。', 'Both read-only captures are enabled.'],
  statusCaptureDisabled: ['捕获器已停用，原字节已恢复。', 'Captures are disabled and original bytes restored.'],
  statusCaptureRefreshed: ['捕获状态已刷新。', 'Capture status refreshed.'],
  statusItemRead: ['已只读核验 {name}；下一次请重新选择。', 'Read-only verification complete for {name}; select again for the next read.'],
  statusReadRefreshFailed: ['记录已安全读取，但捕获状态刷新失败：{error}', 'The record was read safely, but capture status refresh failed: {error}'],
  statusActionFailed: ['操作失败：{error}', 'Operation failed: {error}'],
  statusReleaseComplete: ['捕获 Hook 已恢复，并已断开游戏进程。', 'Capture hooks were restored and the game process was disconnected.'],
  selectAgain: ['需重新选择', 'Selection Required Again'],
  captureAddress: ['Hook 地址', 'Hook Address'],
  hookRva: ['Hook RVA', 'Hook RVA'],
  validData: ['已验证数据', 'Verified Data'],
  notInParty: ['未编入', 'Not in Party'],
  emptySlotCopy: ['当前槽位没有运行时实体，未生成任何数值。', 'This slot has no runtime entity; no values were fabricated.'],
  readOnlyChip: ['不写入记录', 'Record Writes Disabled'],
  errorMissingOwner: ['后端未返回运行时连接所有权令牌', 'The backend did not return a runtime connection owner token'],
  errorInvalidPid: ['后端返回的游戏进程 ID 无效', 'The backend returned an invalid game process ID'],
  errorInvalidModule: ['后端返回的游戏模块基址无效', 'The backend returned an invalid game module base'],
  errorCaptureEnableVerification: ['只读捕获启用后的回读状态不一致', 'Read-back state did not confirm that read-only capture was enabled'],
  errorCaptureDisableVerification: ['恢复原字节后，捕获状态仍显示为启用', 'Capture still appeared enabled after restoring the original bytes'],
})

function deepFreeze(value) {
  if (!value || typeof value !== 'object' || Object.isFrozen(value)) return value
  for (const child of Object.values(value)) deepFreeze(child)
  return Object.freeze(value)
}

function objectValue(value, label) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new TypeError(`${label} must be an object`)
  return value
}

function stringValue(value, label, exact = '') {
  if (typeof value !== 'string' || !value.trim()) throw new TypeError(`${label} must be a non-empty string`)
  if (exact && value !== exact) throw new TypeError(`${label} must equal ${exact}`)
  return value
}

function booleanValue(value, label) {
  if (typeof value !== 'boolean') throw new TypeError(`${label} must be a boolean`)
  return value
}

function unsignedInteger(value, label, maximum = Number.MAX_SAFE_INTEGER, allowZero = true) {
  if (!Number.isSafeInteger(value) || value < 0 || value > maximum || (!allowZero && value === 0)) {
    throw new TypeError(`${label} must be a ${allowZero ? 'non-negative' : 'positive'} safe integer`)
  }
  return value
}

function finiteNumber(value, label) {
  if (typeof value !== 'number' || !Number.isFinite(value)) throw new TypeError(`${label} must be finite`)
  return value
}

function normalizePosition(value, label) {
  const position = objectValue(value, label)
  return {
    x: finiteNumber(position.x, `${label}.x`),
    y: finiteNumber(position.y, `${label}.y`),
    z: finiteNumber(position.z, `${label}.z`),
  }
}

function isPresent(value) {
  return value !== undefined && value !== null
}

function normalizePartyEntity(value, expectedRole) {
  const entity = objectValue(value, `party entity ${expectedRole}`)
  if (entity.role !== expectedRole) throw new TypeError(`party entity role must be ${expectedRole}`)
  const present = booleanValue(entity.present, `${expectedRole} present`)
  const capabilities = objectValue(entity.capabilities, `${expectedRole} capabilities`)
  const normalizedCapabilities = {
    dodge: booleanValue(capabilities.dodge, `${expectedRole} dodge capability`),
    sba: booleanValue(capabilities.sba, `${expectedRole} SBA capability`),
    directPosition: booleanValue(capabilities.directPosition, `${expectedRole} direct-position capability`),
    loadout: booleanValue(capabilities.loadout, `${expectedRole} loadout capability`),
  }
  const hasDodge = isPresent(entity.dodgeCount)
  const hasSBA = isPresent(entity.sba) || isPresent(entity.maxSba)
  const hasCompleteSBA = isPresent(entity.sba) && isPresent(entity.maxSba)
  const hasDirectPosition = isPresent(entity.directPosition)
  const hasLoadout = isPresent(entity.loadout)
  if (hasDodge !== normalizedCapabilities.dodge) throw new TypeError(`${expectedRole} dodge is unavailable but a value was supplied`)
  if (hasSBA !== normalizedCapabilities.sba || (normalizedCapabilities.sba && !hasCompleteSBA)) {
    throw new TypeError(`${expectedRole} SBA availability does not match its capability`)
  }
  if (hasDirectPosition !== normalizedCapabilities.directPosition) {
    throw new TypeError(`${expectedRole} direct position availability does not match its capability`)
  }
  if (hasLoadout) {
    const available = booleanValue(objectValue(entity.loadout, `${expectedRole} loadout`).available, `${expectedRole} loadout available`)
    if (available !== normalizedCapabilities.loadout) throw new TypeError(`${expectedRole} loadout availability does not match its capability`)
  } else if (normalizedCapabilities.loadout) {
    throw new TypeError(`${expectedRole} loadout capability requires a loadout record`)
  }

  if (!present) {
    const position = normalizePosition(entity.position, `${expectedRole} position`)
    const hasRuntimeData = entity.address !== 0
      || entity.hp !== 0
      || entity.maxHp !== 0
      || position.x !== 0
      || position.y !== 0
      || position.z !== 0
      || normalizedCapabilities.dodge
      || normalizedCapabilities.sba
      || normalizedCapabilities.directPosition
      || hasDodge
      || hasSBA
      || hasDirectPosition
      || hasLoadout
    if (hasRuntimeData) throw new TypeError(`${expectedRole} absent slot must not contain runtime entity data`)
    return {
      role: expectedRole,
      present: false,
      displayName: stringValue(entity.displayName, `${expectedRole} display name`),
      address: 0,
      hp: 0,
      maxHp: 0,
      position,
      capabilities: normalizedCapabilities,
    }
  }

  const hp = unsignedInteger(entity.hp, `${expectedRole} HP`, 1_000_000_000)
  const maxHp = unsignedInteger(entity.maxHp, `${expectedRole} max HP`, 1_000_000_000, false)
  if (hp > maxHp) throw new TypeError(`${expectedRole} HP exceeds max HP`)

  const normalized = {
    role: expectedRole,
    present: true,
    displayName: stringValue(entity.displayName, `${expectedRole} display name`),
    address: unsignedInteger(entity.address, `${expectedRole} address`, Number.MAX_SAFE_INTEGER, false),
    hp,
    maxHp,
    position: normalizePosition(entity.position, `${expectedRole} position`),
    capabilities: normalizedCapabilities,
  }
  if (normalizedCapabilities.dodge) normalized.dodgeCount = unsignedInteger(entity.dodgeCount, `${expectedRole} dodge count`, 0xFFFFFFFF)
  if (normalizedCapabilities.sba) {
    normalized.sba = finiteNumber(entity.sba, `${expectedRole} SBA`)
    normalized.maxSba = finiteNumber(entity.maxSba, `${expectedRole} max SBA`)
    if (normalized.maxSba <= 0 || normalized.sba < 0 || normalized.sba > normalized.maxSba) {
      throw new TypeError(`${expectedRole} SBA range is invalid`)
    }
  }
  if (normalizedCapabilities.directPosition) {
    normalized.directPosition = normalizePosition(entity.directPosition, `${expectedRole} direct position`)
  }
  if (hasLoadout) normalized.loadout = normalizePartyLoadout(entity.loadout, expectedRole)
  return normalized
}

function normalizeRuntimeTrait(value, label) {
  const trait = objectValue(value, label)
  const hash = unsignedInteger(trait.hash, `${label} hash`, 0xFFFFFFFF, false)
  const hashHex = stringValue(trait.hashHex, `${label} hash hex`)
  if (hashHex !== hash.toString(16).toUpperCase().padStart(8, '0')) throw new TypeError(`${label} hash hex does not match hash`)
  return {
    hash,
    hashHex,
    name: stringValue(trait.name, `${label} name`),
    level: unsignedInteger(trait.level, `${label} level`, 999),
  }
}

function normalizePartyLoadout(value, role) {
  const loadout = objectValue(value, `${role} loadout`)
  const available = booleanValue(loadout.available, `${role} loadout available`)
  const evidence = stringValue(loadout.evidence, `${role} loadout evidence`)
  const verification = stringValue(loadout.verification, `${role} loadout verification`)
  if (!available) {
    return {
      available: false,
      stable: false,
      snapshotCount: 0,
      verification,
      evidence,
      unavailableReason: stringValue(loadout.unavailableReason, `${role} loadout unavailable reason`),
    }
  }
  if (loadout.stable !== true || loadout.snapshotCount !== 3 || verification !== 'candidate') {
    throw new TypeError(`${role} loadout must be a stable three-snapshot candidate`)
  }
  const stats = objectValue(loadout.stats, `${role} loadout stats`)
  const normalizedStats = {
    level: unsignedInteger(stats.level, `${role} level`, 999, false),
    totalHp: unsignedInteger(stats.totalHp, `${role} total HP`, 1_000_000_000, false),
    totalAttack: unsignedInteger(stats.totalAttack, `${role} total attack`, 1_000_000_000),
    stunPower: finiteNumber(stats.stunPower, `${role} stun power`),
    criticalRate: finiteNumber(stats.criticalRate, `${role} critical rate`),
    totalPower: unsignedInteger(stats.totalPower, `${role} total power`, 100_000_000, false),
  }
  const weapon = objectValue(loadout.weapon, `${role} weapon`)
  const weaponHash = unsignedInteger(weapon.hash, `${role} weapon hash`, 0xFFFFFFFF, false)
  const weaponHashHex = stringValue(weapon.hashHex, `${role} weapon hash hex`)
  if (weaponHashHex !== weaponHash.toString(16).toUpperCase().padStart(8, '0')) throw new TypeError(`${role} weapon hash hex does not match hash`)
  if (!Array.isArray(weapon.traits) || weapon.traits.length > 3) throw new TypeError(`${role} weapon traits are invalid`)
  const normalizedWeapon = {
    hash: weaponHash,
    hashHex: weaponHashHex,
    name: stringValue(weapon.name, `${role} weapon name`),
    level: unsignedInteger(weapon.level, `${role} weapon level`, 999, false),
    starLevel: unsignedInteger(weapon.starLevel, `${role} weapon star level`, 20),
    plusMarks: unsignedInteger(weapon.plusMarks, `${role} weapon plus marks`, 9999),
    awakeningLevel: unsignedInteger(weapon.awakeningLevel, `${role} weapon awakening level`, 100),
    wrightstoneId: unsignedInteger(weapon.wrightstoneId, `${role} wrightstone ID`, 0xFFFFFFFF),
    hp: unsignedInteger(weapon.hp, `${role} weapon HP`, 100_000_000),
    attack: unsignedInteger(weapon.attack, `${role} weapon attack`, 100_000_000),
    traits: weapon.traits.map((trait, index) => normalizeRuntimeTrait(trait, `${role} weapon trait ${index + 1}`)),
    skills: Array.isArray(weapon.skills) ? weapon.skills.map((trait, index) => normalizeRuntimeTrait(trait, `${role} weapon skill ${index + 1}`)) : [],
  }
	if (normalizedWeapon.skills.length > 5) throw new TypeError(`${role} weapon skills are invalid`)
  if (!Array.isArray(loadout.sigils) || loadout.sigils.length > 12) throw new TypeError(`${role} sigils are invalid`)
  const seenIndices = new Set()
  const sigils = loadout.sigils.map((value, entryIndex) => {
    const sigil = objectValue(value, `${role} sigil ${entryIndex + 1}`)
    const index = unsignedInteger(sigil.index, `${role} sigil index`, 11)
    if (seenIndices.has(index)) throw new TypeError(`${role} sigil index is duplicated`)
    seenIndices.add(index)
    const hash = unsignedInteger(sigil.hash, `${role} sigil hash`, 0xFFFFFFFF, false)
    const hashHex = stringValue(sigil.hashHex, `${role} sigil hash hex`)
    if (hashHex !== hash.toString(16).toUpperCase().padStart(8, '0')) throw new TypeError(`${role} sigil hash hex does not match hash`)
    const primaryHash = unsignedInteger(sigil.primaryTraitHash, `${role} sigil primary hash`, 0xFFFFFFFF, false)
    const normalized = {
      index,
      hash,
      hashHex,
      name: stringValue(sigil.name, `${role} sigil name`),
      level: unsignedInteger(sigil.level, `${role} sigil level`, 999),
      primaryTraitHash: primaryHash,
      primaryTraitHashHex: stringValue(sigil.primaryTraitHashHex, `${role} sigil primary hash hex`),
      primaryTraitName: stringValue(sigil.primaryTraitName, `${role} sigil primary name`),
      primaryTraitLevel: unsignedInteger(sigil.primaryTraitLevel, `${role} sigil primary level`, 999),
    }
    if (normalized.primaryTraitHashHex !== primaryHash.toString(16).toUpperCase().padStart(8, '0')) throw new TypeError(`${role} sigil primary hash hex does not match hash`)
    if (sigil.secondaryTraitHash) {
      const secondaryHash = unsignedInteger(sigil.secondaryTraitHash, `${role} sigil secondary hash`, 0xFFFFFFFF, false)
      normalized.secondaryTraitHash = secondaryHash
      normalized.secondaryTraitHashHex = stringValue(sigil.secondaryTraitHashHex, `${role} sigil secondary hash hex`)
      normalized.secondaryTraitName = stringValue(sigil.secondaryTraitName, `${role} sigil secondary name`)
      normalized.secondaryTraitLevel = unsignedInteger(sigil.secondaryTraitLevel, `${role} sigil secondary level`, 999)
      if (normalized.secondaryTraitHashHex !== secondaryHash.toString(16).toUpperCase().padStart(8, '0')) throw new TypeError(`${role} sigil secondary hash hex does not match hash`)
    }
    return normalized
  })
	if (!Array.isArray(loadout.overLimit) || loadout.overLimit.length !== 4) throw new TypeError(`${role} overmastery slots are invalid`)
	const overLimit = loadout.overLimit.map((value, slotIndex) => {
	  const slot = objectValue(value, `${role} overmastery ${slotIndex + 1}`)
	  const index = unsignedInteger(slot.index, `${role} overmastery index`, 3)
	  if (index !== slotIndex) throw new TypeError(`${role} overmastery slots are out of order`)
	  const attributeHash = unsignedInteger(slot.attributeHash, `${role} overmastery hash`, 0xFFFFFFFF)
	  return {
	    index,
	    attributeHash,
	    hashHex: typeof slot.hashHex === 'string' ? slot.hashHex : '',
	    name: typeof slot.name === 'string' ? slot.name : '',
	    flags: unsignedInteger(slot.flags, `${role} overmastery flags`, 0xFFFFFFFF),
	    level: unsignedInteger(slot.level, `${role} overmastery level`, 10),
	    value: finiteNumber(slot.value, `${role} overmastery value`),
	  }
	})
  return {
    available: true,
    stable: true,
    snapshotCount: 3,
    verification,
    evidence,
    layout: stringValue(loadout.layout, `${role} loadout layout`),
    characterCode: stringValue(loadout.characterCode, `${role} character code`),
    characterHash: stringValue(loadout.characterHash, `${role} character hash`),
    characterName: stringValue(loadout.characterName, `${role} character name`),
    runtimeLabel: typeof loadout.runtimeLabel === 'string' ? loadout.runtimeLabel : '',
    online: booleanValue(loadout.online, `${role} online flag`),
    partyIndex: unsignedInteger(loadout.partyIndex, `${role} party index`, 0xFF),
    stats: normalizedStats,
    weapon: normalizedWeapon,
    sigils,
		overLimit,
  }
}

function verifyOwnerAndProcess(value, expectedOwnerToken, expectedPID, label) {
  stringValue(expectedOwnerToken, `expected ${label} owner`)
  const expectedProcess = unsignedInteger(expectedPID, `expected ${label} process`, 0xFFFFFFFF, false)
  if (value.ownerToken !== expectedOwnerToken) throw new TypeError(`${label} owner token is stale`)
  if (value.pid !== expectedProcess) throw new TypeError(`${label} process identity changed`)
}

export function normalizeRuntimePatchPartySnapshot(value, expectedOwnerToken, expectedPID) {
  const snapshot = objectValue(value, 'party snapshot')
  verifyOwnerAndProcess(snapshot, expectedOwnerToken, expectedPID, 'party snapshot')
  if (snapshot.runtimeVerified !== true) throw new TypeError('party snapshot is not runtime verified')
  if (snapshot.snapshotCount !== 3) throw new TypeError('party snapshot count must be three')
  stringValue(snapshot.gameVersion, 'party game version', '2.0.2')
  stringValue(snapshot.source, 'party source', 'game_runtime_patch_2.0.2')
  stringValue(snapshot.verification, 'party verification')
  if (!Array.isArray(snapshot.entities) || snapshot.entities.length !== PARTY_ROLES.length) {
    throw new TypeError('party entities must contain exactly five entries')
  }
  const normalized = {
    ownerToken: expectedOwnerToken,
    pid: expectedPID,
    processCreated: finiteNumber(snapshot.processCreated, 'party process creation identity'),
    rootAddress: unsignedInteger(snapshot.rootAddress, 'party root address', Number.MAX_SAFE_INTEGER, false),
    entities: PARTY_ROLES.map((role, index) => normalizePartyEntity(snapshot.entities[index], role)),
    source: snapshot.source,
    verification: snapshot.verification,
    gameVersion: snapshot.gameVersion,
    snapshotCount: snapshot.snapshotCount,
    runtimeVerified: true,
  }
  return deepFreeze(normalized)
}

function normalizeCapture(value, expectedKind) {
  const capture = objectValue(value, `selected ${expectedKind} capture`)
  if (capture.kind !== expectedKind) throw new TypeError(`selected capture kind must be ${expectedKind}`)
  const found = booleanValue(capture.found, `${expectedKind} found`)
  const hooked = booleanValue(capture.hooked, `${expectedKind} hooked`)
  const captured = booleanValue(capture.captured, `${expectedKind} captured`)
  const address = unsignedInteger(capture.address, `${expectedKind} hook address`, Number.MAX_SAFE_INTEGER, !found)
  const selectedAddr = unsignedInteger(capture.selectedAddr, `${expectedKind} selected address`)
  const rva = unsignedInteger(capture.rva, `${expectedKind} RVA`, 0xFFFFFFFF, false)
  if (rva !== SELECTED_RVAS[expectedKind]) throw new TypeError(`selected ${expectedKind} RVA does not match the verified runtime layout`)
  if (hooked && !found) throw new TypeError(`selected ${expectedKind} hook cannot exist without a found signature`)
  if (captured && (!hooked || selectedAddr === 0)) throw new TypeError(`selected ${expectedKind} captured state requires a hooked non-zero address`)
  if (!captured && selectedAddr !== 0) throw new TypeError(`selected ${expectedKind} address must be empty when not captured`)
  return {
    kind: expectedKind,
    displayName: stringValue(capture.displayName, `${expectedKind} display name`),
    found,
    hooked,
    address,
    rva,
    selectedAddr,
    captured,
  }
}

export function normalizeRuntimePatchSelectedStatus(value, expectedOwnerToken, expectedPID) {
  const status = objectValue(value, 'selected-item status')
  verifyOwnerAndProcess(status, expectedOwnerToken, expectedPID, 'selected-item status')
  if (status.readOnly !== true) throw new TypeError('selected-item status must be read-only')
  stringValue(status.gameVersion, 'selected-item game version', '2.0.2')
  stringValue(status.source, 'selected-item source', 'game_selected_item_read_only_2.0.2')
  const material = normalizeCapture(status.material, 'material')
  const keyItem = normalizeCapture(status.keyItem, 'keyItem')
  if (material.hooked !== keyItem.hooked) throw new TypeError('selected-item hooks must be enabled or disabled as a pair')
  if (booleanValue(status.enabled, 'selected-item enabled') !== (material.hooked && keyItem.hooked)) {
    throw new TypeError('selected-item enabled state does not match its capture pair')
  }
  return deepFreeze({
    ownerToken: expectedOwnerToken,
    pid: expectedPID,
    processCreated: finiteNumber(status.processCreated, 'selected-item process creation identity'),
    enabled: status.enabled,
    readOnly: true,
    gameVersion: status.gameVersion,
    source: status.source,
    material,
    keyItem,
  })
}

function uint32Hex(value) {
  return value.toString(16).toUpperCase().padStart(8, '0')
}

export function normalizeRuntimePatchSelectedRecord(value, expectedKind, expectedSelectedAddr) {
  if (!SELECTED_KINDS.includes(expectedKind)) throw new TypeError(`unknown selected-item kind: ${expectedKind}`)
  const record = objectValue(value, 'selected-item record')
  const expectedAddress = unsignedInteger(expectedSelectedAddr, 'ExpectedSelectedAddr', Number.MAX_SAFE_INTEGER, false)
  if (record.kind !== expectedKind) throw new TypeError(`selected-item record kind must be ${expectedKind}`)
  if (record.selectedAddr !== expectedAddress) throw new TypeError('selected address does not match ExpectedSelectedAddr')
  if (record.readOnly !== true) throw new TypeError('selected-item record must be read-only')
  stringValue(record.gameVersion, 'selected-item record game version', '2.0.2')
  const hash = unsignedInteger(record.hash, 'selected-item hash', 0xFFFFFFFF)
  const flags = unsignedInteger(record.flags, 'selected-item flags', 0xFFFFFFFF)
  const hashHex = stringValue(record.hashHex, 'selected-item hash hex')
  const flagsHex = stringValue(record.flagsHex, 'selected-item flags hex')
  if (hashHex !== uint32Hex(hash)) throw new TypeError('selected-item hash hex does not match hash')
  if (flagsHex !== uint32Hex(flags)) throw new TypeError('selected-item flags hex does not match flags')
  if (record.category !== undefined && typeof record.category !== 'string') throw new TypeError('selected-item category must be a string')
  return deepFreeze({
    kind: expectedKind,
    displayName: stringValue(record.displayName, 'selected-item display name'),
    selectedAddr: expectedAddress,
    hash,
    hashHex,
    name: stringValue(record.name, 'selected-item catalog name'),
    category: record.category || '',
    quantity: unsignedInteger(record.quantity, 'selected-item quantity', 0xFFFFFFFF),
    flags,
    flagsHex,
    readOnly: true,
    gameVersion: record.gameVersion,
  })
}

export function consumeRuntimePatchSelectedCapture(status, kind) {
  if (!SELECTED_KINDS.includes(kind)) throw new TypeError(`unknown selected-item kind: ${kind}`)
  const source = objectValue(status, 'selected-item status')
  return deepFreeze({
    ...source,
    material: { ...source.material },
    keyItem: { ...source.keyItem },
    [kind]: { ...source[kind], captured: false, selectedAddr: 0 },
  })
}

export function selectedCapturePhase(status, kind, consumed = false) {
  if (!SELECTED_KINDS.includes(kind)) throw new TypeError(`unknown selected-item kind: ${kind}`)
  if (!status?.enabled) return 'disabled'
  if (status[kind]?.captured) return 'ready'
  return consumed ? 'reselect' : 'awaiting'
}

export function partyOptionalMetric(entity, metric, selectedLanguage = 'zh') {
  const missing = { available: false, text: runtimeMonitorText('fieldUnavailable', selectedLanguage) }
  if (metric === 'dodge') {
    if (!entity?.capabilities?.dodge) return missing
    return { available: true, text: formatRuntimeInteger(entity.dodgeCount, selectedLanguage) }
  }
  if (metric === 'sba') {
    if (!entity?.capabilities?.sba) return missing
    const current = Number(entity.sba)
    const maximum = Number(entity.maxSba)
    return { available: true, text: `${current.toFixed(1)} / ${maximum.toFixed(1)} (${((current / maximum) * 100).toFixed(1)}%)` }
  }
  throw new TypeError(`unknown optional party metric: ${metric}`)
}

export function runtimeMonitorRoleName(role, selectedLanguage = 'zh') {
  if (!PARTY_ROLES.includes(role)) return String(role || '')
  return runtimeMonitorText(role, selectedLanguage)
}

export function runtimeMonitorCopyKeys() {
  return Object.freeze(Object.keys(COPY))
}

export function runtimeMonitorText(key, selectedLanguage = 'zh', parameters = {}) {
  const pair = COPY[key]
  if (!pair) throw new TypeError(`unknown runtime monitor copy key: ${key}`)
  let output = pair[selectedLanguage === 'en' ? 1 : 0]
  for (const [name, value] of Object.entries(parameters || {})) output = output.split(`{${name}}`).join(String(value))
  return output
}

export function formatRuntimeInteger(value, selectedLanguage = 'zh') {
  const normalized = unsignedInteger(value, 'runtime integer')
  return new Intl.NumberFormat(selectedLanguage === 'en' ? 'en-US' : 'zh-CN', { maximumFractionDigits: 0 }).format(normalized)
}

export function formatRuntimeAddress(value) {
  const normalized = unsignedInteger(value, 'runtime address')
  return `0x${normalized.toString(16).toUpperCase().padStart(16, '0')}`
}

export function formatRuntimeCoordinate(value) {
  return finiteNumber(value, 'runtime coordinate').toFixed(2)
}
