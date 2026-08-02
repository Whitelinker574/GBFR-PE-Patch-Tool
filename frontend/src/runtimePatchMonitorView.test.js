import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'

const viewURL = new URL('./runtimePatchMonitorView.js', import.meta.url)
const view = existsSync(viewURL) ? await import(viewURL) : null
const component = readFileSync(new URL('./components/RuntimePatchMonitor.vue', import.meta.url), 'utf8')
const detector = readFileSync(new URL('./components/RuntimeLoadoutDetector.vue', import.meta.url), 'utf8')

const ownerToken = 'runtime-monitor-owner'
const processInfo = { pid: 2468 }

function combatEntity(role, index) {
  return {
    role,
    present: true,
    displayName: role,
    address: 0x140001000 + index * 0x1000,
    hp: 20000 + index,
    maxHp: 30000 + index,
    dodgeCount: index,
    sba: 25 + index,
    maxSba: 100,
    position: { x: 1.25 + index, y: -2.5, z: 3.75 },
    capabilities: { dodge: true, sba: true, directPosition: false, loadout: false },
  }
}

function validPartySnapshot(version = '2.0.2') {
  return {
    ownerToken,
    pid: processInfo.pid,
    processCreated: 1337000,
    rootAddress: 0x140000000,
    entities: [
      combatEntity('player', 0),
      combatEntity('party1', 1),
      combatEntity('party2', 2),
      combatEntity('party3', 3),
      {
        role: 'companion',
        present: true,
        displayName: 'Vyrn',
        address: 0x140009000,
        hp: 999,
        maxHp: 1000,
        position: { x: 8, y: 9, z: 10 },
        directPosition: { x: 8.5, y: 9.5, z: 10.5 },
        capabilities: { dodge: false, sba: false, directPosition: true, loadout: false },
      },
    ],
    source: `game_runtime_patch_${version}`,
    verification: 'three-snapshot topology verification',
    gameVersion: version,
    snapshotCount: 3,
    runtimeVerified: true,
  }
}

function capture(kind, selectedAddr = 0, version = '2.0.2') {
  const rvas = version === '2.0.3'
    ? { material: 0x3F479F3, keyItem: 0x3F1C54C }
    : { material: 0x3F4BAC3, keyItem: 0x3F2061C }
  return {
    kind,
    displayName: kind,
    found: true,
    hooked: true,
    address: kind === 'material' ? 0x140010000 : 0x140020000,
    rva: rvas[kind],
    selectedAddr,
    captured: selectedAddr > 0,
  }
}

function validSelectedStatus(version = '2.0.2') {
  return {
    ownerToken,
    pid: processInfo.pid,
    processCreated: 1337000,
    enabled: true,
    readOnly: true,
    gameVersion: version,
    source: `game_selected_item_read_only_${version}`,
    material: capture('material', 0x140030000, version),
    keyItem: capture('keyItem', 0, version),
  }
}

test('party monitoring exposes persistent stable-party history as the primary workflow', () => {
  assert.equal(view.runtimeMonitorText('tabParty', 'zh'), '队伍配装记录')
  assert.match(component, /<RuntimeLoadoutDetector/)
  assert.match(detector, /开启后台检测/)
  assert.match(detector, /EventsOn\(DETECTOR_STATUS_EVENT/)
  assert.doesNotMatch(detector, /setInterval|pollTimer/)
  for (const action of ['copy', 'export', 'deploy']) {
    assert.match(detector, new RegExp(`runAction\\(preview\\.record, preview\\.member, '${action}'\\)`))
  }
  assert.match(detector, /openRuntimePublish\(preview\.record, preview\.member\)/)
  assert.match(detector, /<LoadoutPublishDialog[\s\S]*@close="closeRuntimePublish"[\s\S]*@submit="publishRuntimeTarget"/)
})

test('party detector copy does not claim exact quest boundaries', () => {
  assert.doesNotMatch(detector, /每场任务|every quest|Quest Loadout History|本场已保存/)
  assert.match(detector, /稳定队伍|Stable Party/)
  assert.match(detector, /不承诺等同游戏任务边界/)
})

test('party snapshots accept only the verified five-entity 2.0.2 contract', () => {
  assert.ok(view, 'runtimePatchMonitorView.js must exist')
  const normalized = view.normalizeRuntimePatchPartySnapshot(validPartySnapshot(), ownerToken, processInfo.pid)

  assert.equal(normalized.runtimeVerified, true)
  assert.equal(normalized.snapshotCount, 3)
  assert.deepEqual(normalized.entities.map(entity => entity.role), ['player', 'party1', 'party2', 'party3', 'companion'])
  assert.equal(normalized.entities[4].dodgeCount, undefined)
  assert.equal(normalized.entities[4].sba, undefined)
  assert.equal(normalized.entities[4].maxSba, undefined)
})

test('solo training snapshots preserve empty slots without fabricating zero-valued entities', () => {
  assert.ok(view, 'runtimePatchMonitorView.js must exist')
  const snapshot = validPartySnapshot()
  for (let index = 1; index < 4; index += 1) {
    snapshot.entities[index] = {
      role: `party${index}`,
      present: false,
      displayName: `party${index}`,
      address: 0,
      hp: 0,
      maxHp: 0,
      position: { x: 0, y: 0, z: 0 },
      capabilities: { dodge: false, sba: false, directPosition: false, loadout: false },
    }
  }
  const normalized = view.normalizeRuntimePatchPartySnapshot(snapshot, ownerToken, processInfo.pid)
  assert.equal(normalized.entities[0].present, true)
  assert.deepEqual(normalized.entities.slice(1, 4).map(entity => entity.present), [false, false, false])
  assert.throws(
    () => view.normalizeRuntimePatchPartySnapshot({ ...snapshot, entities: snapshot.entities.map((entity, index) => index === 1 ? { ...entity, hp: 1 } : entity) }, ownerToken, processInfo.pid),
    /absent|empty|present/i,
  )
})

test('stable candidate teammate loadouts preserve weapon, two-trait sigils, and panel values', () => {
  assert.ok(view, 'runtimePatchMonitorView.js must exist')
  const snapshot = validPartySnapshot()
  snapshot.entities[1].capabilities.loadout = true
  snapshot.entities[1].loadout = {
    available: true,
    stable: true,
    snapshotCount: 3,
    verification: 'candidate',
    evidence: 'three matching snapshots pending live comparison',
    layout: 'entity+0x70 -> instance+{0x15030,0x15080,0x1AE90}',
    characterCode: 'PL1600',
    characterHash: '0D21B430',
    characterName: 'Zeta',
    runtimeLabel: 'PL1600',
    online: true,
    partyIndex: 1,
    stats: { level: 100, totalHp: 50000, totalAttack: 20000, stunPower: 250, criticalRate: 100, totalPower: 30000 },
    weapon: {
      hash: 0x02352554,
      hashHex: '02352554',
      name: 'Brionac',
      level: 150,
      starLevel: 5,
      plusMarks: 99,
      awakeningLevel: 10,
      wrightstoneId: 1234,
      hp: 500,
      attack: 3000,
      traits: [{ hash: 0x7EDD69D0, hashHex: '7EDD69D0', name: 'ATK', level: 15 }],
	  skills: [{ hash: 0xDC584F60, hashHex: 'DC584F60', name: 'Damage Cap', level: 15 }],
    },
    sigils: [{
      index: 0,
      hash: 0x2D7F2E70,
      hashHex: '2D7F2E70',
      name: 'Attack Power V+',
      level: 15,
      primaryTraitHash: 0x50079A1C,
      primaryTraitHashHex: '50079A1C',
      primaryTraitName: 'ATK',
      primaryTraitLevel: 15,
      secondaryTraitHash: 0xDC584F60,
      secondaryTraitHashHex: 'DC584F60',
      secondaryTraitName: 'Damage Cap',
      secondaryTraitLevel: 15,
    }],
	overLimit: [
	  { index: 0, attributeHash: 0x52A207B5, hashHex: '52A207B5', name: 'Attack', flags: 0x200, level: 10, value: 1000 },
	  { index: 1, attributeHash: 0, flags: 0, level: 0, value: 0 },
	  { index: 2, attributeHash: 0, flags: 0, level: 0, value: 0 },
	  { index: 3, attributeHash: 0, flags: 0, level: 0, value: 0 },
	],
  }
  const member = view.normalizeRuntimePatchPartySnapshot(snapshot, ownerToken, processInfo.pid).entities[1]
  assert.equal(member.loadout.characterCode, 'PL1600')
  assert.equal(member.loadout.weapon.hashHex, '02352554')
  assert.equal(member.loadout.sigils[0].secondaryTraitLevel, 15)
  assert.equal(member.loadout.stats.criticalRate, 100)
	assert.equal(member.loadout.weapon.skills[0].level, 15)
	assert.equal(member.loadout.overLimit[0].level, 10)
})

test('unavailable teammate loadouts remain explicit and cannot masquerade as candidates', () => {
  assert.ok(view, 'runtimePatchMonitorView.js must exist')
  const snapshot = validPartySnapshot()
  snapshot.entities[2].loadout = {
    available: false,
    stable: false,
    snapshotCount: 0,
    verification: 'unavailable',
    evidence: 'bounded validation failed',
    unavailableReason: 'weapon hash is unknown',
  }
  const member = view.normalizeRuntimePatchPartySnapshot(snapshot, ownerToken, processInfo.pid).entities[2]
  assert.equal(member.capabilities.loadout, false)
  assert.equal(member.loadout.available, false)
  assert.match(member.loadout.unavailableReason, /weapon/i)

  snapshot.entities[2].capabilities.loadout = true
  assert.throws(
    () => view.normalizeRuntimePatchPartySnapshot(snapshot, ownerToken, processInfo.pid),
    /loadout availability/i,
  )
})

test('party snapshots reject stale ownership, changed process identity, and incomplete verification', () => {
  assert.ok(view, 'runtimePatchMonitorView.js must exist')
  for (const mutate of [
    snapshot => { snapshot.ownerToken = 'stale-owner' },
    snapshot => { snapshot.pid = 9999 },
    snapshot => { snapshot.runtimeVerified = false },
    snapshot => { snapshot.snapshotCount = 2 },
    snapshot => { snapshot.entities.pop() },
    snapshot => { snapshot.entities[1].role = 'player' },
  ]) {
    const snapshot = validPartySnapshot()
    mutate(snapshot)
    assert.throws(
      () => view.normalizeRuntimePatchPartySnapshot(snapshot, ownerToken, processInfo.pid),
      /party|owner|process|verified|snapshot|entities|role/i,
    )
  }
})

test('spatial teleport results stay bound to the owner, process, and exact 2.0.2 contract', () => {
  const result = view.normalizeRuntimeSpatialTeleport({
    ownerToken,
    pid: processInfo.pid,
    processCreated: 1337000,
    before: { x: 1, y: 2, z: 3 },
    requested: { x: 4, y: 5, z: 6 },
    observed: { x: 4, y: 5, z: 6 },
    gameVersion: '2.0.2',
    source: 'game_runtime_spatial_2.0.2',
    snapshotCount: 3,
    runtimeVerified: true,
  }, ownerToken, processInfo.pid)
  assert.deepEqual(result.observed, { x: 4, y: 5, z: 6 })
  assert.throws(() => view.normalizeRuntimeSpatialTeleport({ ...result, ownerToken: 'stale' }, ownerToken, processInfo.pid), /owner/i)
  assert.throws(() => view.normalizeRuntimeSpatialTeleport({ ...result, runtimeVerified: false }, ownerToken, processInfo.pid), /verified/i)
  assert.doesNotThrow(() => view.normalizeRuntimeSpatialTeleport({ ...result, source: 'game_runtime_spatial_continuous_2.0.2' }, ownerToken, processInfo.pid))
})

test('gravity status accepts only the owned verified 2.0.2 entry and matching instruction bytes', () => {
  const status = view.normalizeRuntimeSpatialGravityStatus({
    ownerToken,
    enabled: true,
    available: true,
    owned: true,
    recoveryPending: false,
    address: 0x179DD964,
    rva: 0x39DD964,
    currentBytes: '90 90 90 90 90 90 90 90',
    pid: processInfo.pid,
    processCreated: 1337000,
    gameVersion: '2.0.2',
    source: 'game_runtime_gravity_patch_2.0.2',
    error: '',
  }, ownerToken, processInfo.pid)
  assert.equal(status.enabled, true)
  assert.throws(() => view.normalizeRuntimeSpatialGravityStatus({ ...status, rva: 1 }, ownerToken, processInfo.pid), /RVA/i)
  assert.throws(() => view.normalizeRuntimeSpatialGravityStatus({ ...status, currentBytes: 'CC CC CC CC CC CC CC CC' }, ownerToken, processInfo.pid), /instruction bytes/i)
})

test('spatial arrow-key status stays bound to the owner, process, foreground guard, and speed range', () => {
  const status = view.normalizeRuntimeSpatialHotkeyStatus({
    enabled: true,
    foregroundOnly: true,
    speed: 8,
    ownerLeaseId: ownerToken,
    pid: processInfo.pid,
    processCreated: '134147703140663100',
    gameVersion: '2.0.2',
    source: 'game_runtime_spatial_hotkeys_2.0.2',
    inputMode: 'native_wasd',
    flightEnabled: false,
    flightMode: 'virtual_ground',
    verticalInputMode: 'same_frame_height_hook',
    flightDiagnostics: {
      actionId: 1,
      currentHeight: 12.5,
      targetHeight: 12.5,
      grounded: true,
      anchored: true,
      lastFloorQueryHit: false,
      contactTemplateReady: true,
      floorQueries: '4500',
      acceptedContacts: '1200',
      snapshotSequence: '4500',
    },
    lastError: '',
  }, ownerToken, processInfo.pid)
  assert.equal(status.enabled, true)
  assert.equal(status.foregroundOnly, true)
  assert.equal(status.flightEnabled, false)
  assert.equal(status.flightMode, 'virtual_ground')
  assert.equal(status.verticalInputMode, 'same_frame_height_hook')
  assert.equal(status.flightDiagnostics.actionId, 1)
  assert.equal(status.flightDiagnostics.contactTemplateReady, true)
  assert.equal(status.flightDiagnostics.floorQueries, '4500')
  assert.equal(status.flightDiagnostics.acceptedContacts, '1200')
  assert.equal(status.processCreated, '134147703140663100')
  assert.throws(() => view.normalizeRuntimeSpatialHotkeyStatus({ ...status, foregroundOnly: false }, ownerToken, processInfo.pid), /foreground/i)
  assert.throws(() => view.normalizeRuntimeSpatialHotkeyStatus({ ...status, speed: 20.1 }, ownerToken, processInfo.pid), /speed/i)
  assert.throws(() => view.normalizeRuntimeSpatialHotkeyStatus({ ...status, ownerLeaseId: 'stale' }, ownerToken, processInfo.pid), /owner/i)
  assert.throws(() => view.normalizeRuntimeSpatialHotkeyStatus({ ...status, flightMode: 'unknown' }, ownerToken, processInfo.pid), /flight mode/i)
})

test('unavailable party capabilities cannot masquerade as real zero values', () => {
  assert.ok(view, 'runtimePatchMonitorView.js must exist')
  const snapshot = validPartySnapshot()
  snapshot.entities[4].dodgeCount = 0
  assert.throws(
    () => view.normalizeRuntimePatchPartySnapshot(snapshot, ownerToken, processInfo.pid),
    /dodge.*unavailable|unavailable.*dodge/i,
  )

  const companion = view.normalizeRuntimePatchPartySnapshot(validPartySnapshot(), ownerToken, processInfo.pid).entities[4]
  assert.deepEqual(view.partyOptionalMetric(companion, 'dodge', 'zh'), {
    available: false,
    text: '此实体无该字段',
  })
  assert.deepEqual(view.partyOptionalMetric(companion, 'sba', 'en'), {
    available: false,
    text: 'This entity does not have this field',
  })
})

test('available party capabilities preserve legitimate zero values', () => {
  assert.ok(view, 'runtimePatchMonitorView.js must exist')
  const snapshot = validPartySnapshot()
  snapshot.entities[0].dodgeCount = 0
  snapshot.entities[0].sba = 0
  const player = view.normalizeRuntimePatchPartySnapshot(snapshot, ownerToken, processInfo.pid).entities[0]

  assert.deepEqual(view.partyOptionalMetric(player, 'dodge', 'zh'), { available: true, text: '0' })
  assert.deepEqual(view.partyOptionalMetric(player, 'sba', 'en'), { available: true, text: '0.0 / 100.0 (0.0%)' })
})

test('selected-item status enforces a paired read-only capture contract', () => {
  assert.ok(view, 'runtimePatchMonitorView.js must exist')
  const status = view.normalizeRuntimePatchSelectedStatus(validSelectedStatus(), ownerToken, processInfo.pid)

  assert.equal(status.enabled, true)
  assert.equal(status.readOnly, true)
  assert.equal(status.material.selectedAddr, 0x140030000)
  assert.equal(status.keyItem.selectedAddr, 0)
})

test('selected-item status rejects write-capable, stale, or internally inconsistent responses', () => {
  assert.ok(view, 'runtimePatchMonitorView.js must exist')
  for (const mutate of [
    status => { status.ownerToken = 'stale-owner' },
    status => { status.pid = 9999 },
    status => { status.readOnly = false },
    status => { status.enabled = false },
    status => { status.material.captured = true; status.material.selectedAddr = 0 },
    status => { status.keyItem.kind = 'material' },
  ]) {
    const status = validSelectedStatus()
    mutate(status)
    assert.throws(
      () => view.normalizeRuntimePatchSelectedStatus(status, ownerToken, processInfo.pid),
      /selected|owner|process|read.only|enabled|captured|kind/i,
    )
  }
})

test('one-shot item records are bound to the expected kind and selected address', () => {
  assert.ok(view, 'runtimePatchMonitorView.js must exist')
  const expectedAddress = 0x140030000
  const record = view.normalizeRuntimePatchSelectedRecord({
    kind: 'material',
    displayName: 'Selected Material',
    selectedAddr: expectedAddress,
    hash: 0x89ABCDEF,
    hashHex: '89ABCDEF',
    name: 'Fortitude Crystal (L)',
    category: 'material',
    quantity: 12,
    flags: 0x01020304,
    flagsHex: '01020304',
    readOnly: true,
    gameVersion: '2.0.2',
  }, 'material', expectedAddress)

  assert.equal(record.hashHex, '89ABCDEF')
  assert.equal(record.quantity, 12)
  assert.equal(record.flagsHex, '01020304')

  assert.throws(
    () => view.normalizeRuntimePatchSelectedRecord({ ...record, selectedAddr: expectedAddress + 1 }, 'material', expectedAddress),
    /ExpectedSelectedAddr|selected address/i,
  )
  assert.throws(
    () => view.normalizeRuntimePatchSelectedRecord({ ...record, readOnly: false }, 'material', expectedAddress),
    /read.only/i,
  )
})

test('consuming one capture clears only its pointer and leaves the peer status intact', () => {
  assert.ok(view, 'runtimePatchMonitorView.js must exist')
  const status = view.normalizeRuntimePatchSelectedStatus(validSelectedStatus(), ownerToken, processInfo.pid)
  const consumed = view.consumeRuntimePatchSelectedCapture(status, 'material')

  assert.equal(consumed.material.captured, false)
  assert.equal(consumed.material.selectedAddr, 0)
  assert.deepEqual(consumed.keyItem, status.keyItem)
  assert.equal(status.material.captured, true, 'normalization result must not be mutated')
})

test('all runtime monitor contracts accept the audited 2.0.3 layouts without weakening unknown-version rejection', () => {
  assert.equal(view.normalizeRuntimePatchPartySnapshot(validPartySnapshot('2.0.3'), ownerToken, processInfo.pid).gameVersion, '2.0.3')

  const spatial = {
    ownerToken,
    pid: processInfo.pid,
    processCreated: 1337000,
    before: { x: 1, y: 2, z: 3 },
    requested: { x: 4, y: 5, z: 6 },
    observed: { x: 4, y: 5, z: 6 },
    gameVersion: '2.0.3',
    source: 'game_runtime_spatial_2.0.3',
    snapshotCount: 3,
    runtimeVerified: true,
  }
  assert.equal(view.normalizeRuntimeSpatialTeleport(spatial, ownerToken, processInfo.pid).gameVersion, '2.0.3')
  assert.doesNotThrow(() => view.normalizeRuntimeSpatialTeleport({ ...spatial, source: 'game_runtime_spatial_continuous_2.0.3' }, ownerToken, processInfo.pid))

  const gravity = view.normalizeRuntimeSpatialGravityStatus({
    ownerToken,
    enabled: true,
    available: true,
    owned: true,
    recoveryPending: false,
    address: 0x179D8E24,
    rva: 0x39D8E24,
    currentBytes: '90 90 90 90 90 90 90 90',
    pid: processInfo.pid,
    processCreated: 1337000,
    gameVersion: '2.0.3',
    source: 'game_runtime_gravity_patch_2.0.3',
    error: '',
  }, ownerToken, processInfo.pid)
  assert.equal(gravity.rva, 0x39D8E24)

  const jump = view.normalizeRuntimeSpatialJumpStatus({
    ownerToken,
    enabled: true,
    available: true,
    owned: true,
    recoveryPending: false,
    rvas: [0x1FA00AA, 0x1FA00DC],
    currentBytes: ['EB', '0C 01'],
    pid: processInfo.pid,
    processCreated: '134147703140663100',
    gameVersion: '2.0.3',
    source: 'game_runtime_continuous_jump_2.0.3',
    error: '',
  }, ownerToken, processInfo.pid)
  assert.equal(jump.processCreated, '134147703140663100')

  const hotkeys = view.normalizeRuntimeSpatialHotkeyStatus({
    enabled: true,
    foregroundOnly: true,
    speed: 8,
    ownerLeaseId: ownerToken,
    pid: processInfo.pid,
    processCreated: '134147703140663100',
    gameVersion: '2.0.3',
    source: 'game_runtime_spatial_hotkeys_2.0.3',
    inputMode: 'native_wasd',
    flightEnabled: false,
    flightMode: 'virtual_ground',
    verticalInputMode: 'same_frame_height_hook',
    lastError: '',
  }, ownerToken, processInfo.pid)
  assert.equal(hotkeys.gameVersion, '2.0.3')

  const selected = view.normalizeRuntimePatchSelectedStatus(validSelectedStatus('2.0.3'), ownerToken, processInfo.pid)
  assert.equal(selected.material.rva, 0x3F479F3)
  assert.equal(selected.keyItem.rva, 0x3F1C54C)

  assert.throws(
    () => view.normalizeRuntimePatchPartySnapshot({ ...validPartySnapshot('2.0.3'), gameVersion: '2.0.4' }, ownerToken, processInfo.pid),
    /not supported/i,
  )
})

test('monitor copy is complete in both Chinese and English', () => {
  assert.ok(view, 'runtimePatchMonitorView.js must exist')
  const keys = view.runtimeMonitorCopyKeys()
  assert.ok(keys.length >= 55, 'the page should centralize all visible copy')
  for (const key of keys) {
    const chinese = view.runtimeMonitorText(key, 'zh')
    const english = view.runtimeMonitorText(key, 'en')
    assert.ok(chinese.trim(), `${key}: Chinese copy`)
    assert.ok(english.trim(), `${key}: English copy`)
    assert.doesNotMatch(english, /[\u3400-\u9fff]/u, `${key}: English copy must not contain Chinese`)
  }
  assert.throws(() => view.runtimeMonitorText('not-a-real-key', 'en'), /unknown.*copy/i)
})

test('monitor data formatting is stable and never uses a fake numeric placeholder', () => {
  assert.ok(view, 'runtimePatchMonitorView.js must exist')
  assert.equal(view.formatRuntimeInteger(1234567, 'zh'), '1,234,567')
  assert.equal(view.formatRuntimeAddress(0x140030000), '0x0000000140030000')
  assert.equal(view.formatRuntimeCoordinate(-2.5), '-2.50')
})
