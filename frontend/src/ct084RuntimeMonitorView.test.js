import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import test from 'node:test'

const viewURL = new URL('./ct084RuntimeMonitorView.js', import.meta.url)
const view = existsSync(viewURL) ? await import(viewURL) : null

const ownerToken = 'runtime-monitor-owner'
const processInfo = { pid: 2468 }

function combatEntity(role, index) {
  return {
    role,
    displayName: role,
    address: 0x140001000 + index * 0x1000,
    hp: 20000 + index,
    maxHp: 30000 + index,
    dodgeCount: index,
    sba: 25 + index,
    maxSba: 100,
    position: { x: 1.25 + index, y: -2.5, z: 3.75 },
    capabilities: { dodge: true, sba: true, directPosition: false },
  }
}

function validPartySnapshot() {
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
        displayName: 'Vyrn',
        address: 0x140009000,
        hp: 999,
        maxHp: 1000,
        position: { x: 8, y: 9, z: 10 },
        directPosition: { x: 8.5, y: 9.5, z: 10.5 },
        capabilities: { dodge: false, sba: false, directPosition: true },
      },
    ],
    source: 'game_runtime_ct084_2.0.2',
    verification: 'three-snapshot topology verification',
    gameVersion: '2.0.2',
    snapshotCount: 3,
    runtimeVerified: true,
  }
}

function capture(kind, selectedAddr = 0) {
  return {
    kind,
    displayName: kind,
    found: true,
    hooked: true,
    address: kind === 'material' ? 0x140010000 : 0x140020000,
    rva: kind === 'material' ? 0x3F4BAC3 : 0x3F2061C,
    selectedAddr,
    captured: selectedAddr > 0,
  }
}

function validSelectedStatus() {
  return {
    ownerToken,
    pid: processInfo.pid,
    processCreated: 1337000,
    enabled: true,
    readOnly: true,
    gameVersion: '2.0.2',
    source: 'ct084_node_33552_read_only',
    material: capture('material', 0x140030000),
    keyItem: capture('keyItem'),
  }
}

test('party snapshots accept only the verified five-entity 2.0.2 contract', () => {
  assert.ok(view, 'ct084RuntimeMonitorView.js must exist')
  const normalized = view.normalizeCT084PartySnapshot(validPartySnapshot(), ownerToken, processInfo.pid)

  assert.equal(normalized.runtimeVerified, true)
  assert.equal(normalized.snapshotCount, 3)
  assert.deepEqual(normalized.entities.map(entity => entity.role), ['player', 'party1', 'party2', 'party3', 'companion'])
  assert.equal(normalized.entities[4].dodgeCount, undefined)
  assert.equal(normalized.entities[4].sba, undefined)
  assert.equal(normalized.entities[4].maxSba, undefined)
})

test('party snapshots reject stale ownership, changed process identity, and incomplete verification', () => {
  assert.ok(view, 'ct084RuntimeMonitorView.js must exist')
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
      () => view.normalizeCT084PartySnapshot(snapshot, ownerToken, processInfo.pid),
      /party|owner|process|verified|snapshot|entities|role/i,
    )
  }
})

test('unavailable party capabilities cannot masquerade as real zero values', () => {
  assert.ok(view, 'ct084RuntimeMonitorView.js must exist')
  const snapshot = validPartySnapshot()
  snapshot.entities[4].dodgeCount = 0
  assert.throws(
    () => view.normalizeCT084PartySnapshot(snapshot, ownerToken, processInfo.pid),
    /dodge.*unavailable|unavailable.*dodge/i,
  )

  const companion = view.normalizeCT084PartySnapshot(validPartySnapshot(), ownerToken, processInfo.pid).entities[4]
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
  assert.ok(view, 'ct084RuntimeMonitorView.js must exist')
  const snapshot = validPartySnapshot()
  snapshot.entities[0].dodgeCount = 0
  snapshot.entities[0].sba = 0
  const player = view.normalizeCT084PartySnapshot(snapshot, ownerToken, processInfo.pid).entities[0]

  assert.deepEqual(view.partyOptionalMetric(player, 'dodge', 'zh'), { available: true, text: '0' })
  assert.deepEqual(view.partyOptionalMetric(player, 'sba', 'en'), { available: true, text: '0.0 / 100.0 (0.0%)' })
})

test('selected-item status enforces a paired read-only capture contract', () => {
  assert.ok(view, 'ct084RuntimeMonitorView.js must exist')
  const status = view.normalizeCT084SelectedStatus(validSelectedStatus(), ownerToken, processInfo.pid)

  assert.equal(status.enabled, true)
  assert.equal(status.readOnly, true)
  assert.equal(status.material.selectedAddr, 0x140030000)
  assert.equal(status.keyItem.selectedAddr, 0)
})

test('selected-item status rejects write-capable, stale, or internally inconsistent responses', () => {
  assert.ok(view, 'ct084RuntimeMonitorView.js must exist')
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
      () => view.normalizeCT084SelectedStatus(status, ownerToken, processInfo.pid),
      /selected|owner|process|read.only|enabled|captured|kind/i,
    )
  }
})

test('one-shot item records are bound to the expected kind and selected address', () => {
  assert.ok(view, 'ct084RuntimeMonitorView.js must exist')
  const expectedAddress = 0x140030000
  const record = view.normalizeCT084SelectedRecord({
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
    () => view.normalizeCT084SelectedRecord({ ...record, selectedAddr: expectedAddress + 1 }, 'material', expectedAddress),
    /ExpectedSelectedAddr|selected address/i,
  )
  assert.throws(
    () => view.normalizeCT084SelectedRecord({ ...record, readOnly: false }, 'material', expectedAddress),
    /read.only/i,
  )
})

test('consuming one capture clears only its pointer and leaves the peer status intact', () => {
  assert.ok(view, 'ct084RuntimeMonitorView.js must exist')
  const status = view.normalizeCT084SelectedStatus(validSelectedStatus(), ownerToken, processInfo.pid)
  const consumed = view.consumeCT084SelectedCapture(status, 'material')

  assert.equal(consumed.material.captured, false)
  assert.equal(consumed.material.selectedAddr, 0)
  assert.deepEqual(consumed.keyItem, status.keyItem)
  assert.equal(status.material.captured, true, 'normalization result must not be mutated')
})

test('monitor copy is complete in both Chinese and English', () => {
  assert.ok(view, 'ct084RuntimeMonitorView.js must exist')
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
  assert.ok(view, 'ct084RuntimeMonitorView.js must exist')
  assert.equal(view.formatRuntimeInteger(1234567, 'zh'), '1,234,567')
  assert.equal(view.formatRuntimeAddress(0x140030000), '0x0000000140030000')
  assert.equal(view.formatRuntimeCoordinate(-2.5), '-2.50')
})
