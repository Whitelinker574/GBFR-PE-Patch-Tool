import test from 'node:test'
import assert from 'node:assert/strict'
import worker, {
  displayCode,
  frameIdentity,
  normalizeCode,
  validateFrame,
} from '../src/index.js'

function makeFrame(compressed = new Uint8Array([1, 2, 3, 4]), version = 1) {
  const frame = new Uint8Array(18 + compressed.length)
  frame.set(new TextEncoder().encode('GBLC'), 0)
  frame[4] = version
  frame[5] = 1
  const view = new DataView(frame.buffer)
  view.setUint32(6, 64, true)
  view.setUint32(10, 0x12345678, true)
  view.setUint32(14, compressed.length, true)
  frame.set(compressed, 18)
  return frame
}

function makeR2() {
  const objects = new Map()
  const wrap = entry => entry && {
    customMetadata: entry.customMetadata,
    httpEtag: `"${entry.customMetadata.sha256}"`,
    arrayBuffer: async () => entry.bytes.slice().buffer,
  }
  return {
    objects,
    async head(key) {
      return wrap(objects.get(key))
    },
    async get(key) {
      return wrap(objects.get(key))
    },
    async put(key, value, options) {
      objects.set(key, {
        bytes: new Uint8Array(value),
        customMetadata: options.customMetadata,
      })
    },
  }
}

test('frame validation accepts bounded GBLC v1 and v2 frames', () => {
  const frame = makeFrame()
  assert.equal(validateFrame(frame), '')
  assert.equal(validateFrame(makeFrame(new Uint8Array([1, 2, 3, 4]), 2)), '')
  assert.match(validateFrame(new TextEncoder().encode('plain text')), /过短|标识/)
  assert.match(validateFrame(makeFrame(new Uint8Array([1, 2, 3, 4]), 3)), /版本/)
})

test('short codes are deterministic and use readable Crockford characters', async () => {
  const first = await frameIdentity(makeFrame())
  const second = await frameIdentity(makeFrame())
  assert.equal(first.code, second.code)
  assert.match(first.code, /^[0-9A-HJKMNP-TV-Z]{16}$/)
  assert.equal(normalizeCode(displayCode(first.code)), first.code)
})

test('publish, load, download and landing routes round-trip one immutable frame', async () => {
  const r2 = makeR2()
  const env = { LOADOUTS: r2 }
  const frame = makeFrame()
  const publish = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/octet-stream' },
    body: frame,
  }), env)
  assert.equal(publish.status, 201)
  const result = await publish.json()
  assert.match(result.code, /^[0-9A-HJKMNP-TV-Z]{4}(?:-[0-9A-HJKMNP-TV-Z]{4}){3}$/)

  const repeated = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/octet-stream' },
    body: frame,
  }), env)
  assert.equal(repeated.status, 200)
  assert.equal((await repeated.json()).reused, true)

  const loaded = await worker.fetch(new Request(`https://share.example/api/v1/loadouts/${result.compactCode}`), env)
  assert.equal(loaded.status, 200)
  assert.deepEqual(new Uint8Array(await loaded.arrayBuffer()), frame)

  const landing = await worker.fetch(new Request(`https://share.example/s/${result.compactCode}`), env)
  assert.equal(landing.status, 200)
  assert.match(await landing.text(), new RegExp(result.code))

  const download = await worker.fetch(new Request(`https://share.example/download/${result.compactCode}.gbfr-loadout`), env)
  assert.equal(download.status, 200)
  assert.match(download.headers.get('Content-Disposition'), /\.gbfr-loadout/)
})

test('the service rejects arbitrary paste content and unknown codes', async () => {
  const env = { LOADOUTS: makeR2() }
  const invalid = await worker.fetch(new Request('https://share.example/api/v1/loadouts', {
    method: 'POST',
    headers: { 'Content-Type': 'text/plain' },
    body: 'hello',
  }), env)
  assert.equal(invalid.status, 415)

  const missing = await worker.fetch(new Request('https://share.example/api/v1/loadouts/0123-4567-89AB-CDEF'), env)
  assert.equal(missing.status, 404)
})
