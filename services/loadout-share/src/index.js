const FRAME_MAGIC = 'GBLC'
const FRAME_VERSIONS = new Set([1, 2])
const FRAME_CODEC = 1
const FRAME_HEADER_SIZE = 18
const MAX_FRAME_BYTES = 8 * 1024
const MAX_RAW_BYTES = 1024 * 1024
const CODE_ALPHABET = '0123456789ABCDEFGHJKMNPQRSTVWXYZ'
const CODE_PATTERN = /^[0-9A-HJKMNP-TV-Z]{16,24}$/

const baseHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'Content-Type',
  'Access-Control-Allow-Methods': 'GET, HEAD, POST, OPTIONS',
  'X-Content-Type-Options': 'nosniff',
}

function jsonResponse(value, status = 200, extraHeaders = {}) {
  return new Response(JSON.stringify(value), {
    status,
    headers: {
      ...baseHeaders,
      ...extraHeaders,
      'Content-Type': 'application/json; charset=utf-8',
    },
  })
}

function errorResponse(message, status) {
  return jsonResponse({ error: message }, status, { 'Cache-Control': 'no-store' })
}

function readUint32LE(bytes, offset) {
  return new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength).getUint32(offset, true)
}

export function validateFrame(bytes) {
  if (!(bytes instanceof Uint8Array)) return '请求体必须是二进制配装帧'
  if (bytes.byteLength < FRAME_HEADER_SIZE) return '配装帧过短'
  if (bytes.byteLength > MAX_FRAME_BYTES) return '配装帧超过 8 KB'
  const magic = String.fromCharCode(...bytes.subarray(0, 4))
  if (magic !== FRAME_MAGIC) return '配装帧标识无效'
  if (!FRAME_VERSIONS.has(bytes[4])) return '不支持的配装帧版本'
  if (bytes[5] !== FRAME_CODEC) return '不支持的配装压缩格式'
  const rawSize = readUint32LE(bytes, 6)
  if (rawSize === 0 || rawSize > MAX_RAW_BYTES) return '配装原始数据大小无效'
  const compressedSize = readUint32LE(bytes, 14)
  if (compressedSize === 0 || compressedSize !== bytes.byteLength - FRAME_HEADER_SIZE) {
    return '配装帧长度不一致'
  }
  return ''
}

function encodeBase32(bytes) {
  let output = ''
  let buffer = 0
  let bits = 0
  for (const byte of bytes) {
    buffer = (buffer << 8) | byte
    bits += 8
    while (bits >= 5) {
      bits -= 5
      output += CODE_ALPHABET[(buffer >>> bits) & 31]
    }
  }
  if (bits > 0) output += CODE_ALPHABET[(buffer << (5 - bits)) & 31]
  return output
}

function toHex(bytes) {
  return Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('')
}

export async function frameIdentity(bytes) {
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))
  return {
    code: encodeBase32(digest.subarray(0, 10)),
    hash: toHex(digest),
  }
}

export function normalizeCode(value) {
  const code = String(value || '').toUpperCase().replace(/[-\s]/g, '')
  return CODE_PATTERN.test(code) ? code : ''
}

export function displayCode(code) {
  return normalizeCode(code).match(/.{1,4}/g)?.join('-') || ''
}

function objectKey(code) {
  return `v1/${code}`
}

async function readObjectBytes(object) {
  if (!object) return null
  return new Uint8Array(await object.arrayBuffer())
}

async function publish(request, env, origin) {
  const contentType = request.headers.get('Content-Type') || ''
  if (!contentType.toLowerCase().startsWith('application/octet-stream')) {
    return errorResponse('只接受 application/octet-stream', 415)
  }
  const contentLength = Number(request.headers.get('Content-Length') || 0)
  if (contentLength > MAX_FRAME_BYTES) return errorResponse('配装帧超过 8 KB', 413)
  const bytes = new Uint8Array(await request.arrayBuffer())
  const frameError = validateFrame(bytes)
  if (frameError) return errorResponse(frameError, 400)

  const identity = await frameIdentity(bytes)
  let code = identity.code
  let key = objectKey(code)
  let existing = await env.LOADOUTS.head(key)
  if (existing && existing.customMetadata?.sha256 !== identity.hash) {
    const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))
    code = encodeBase32(digest.subarray(0, 15))
    key = objectKey(code)
    existing = await env.LOADOUTS.head(key)
  }
  if (existing && existing.customMetadata?.sha256 !== identity.hash) {
    return errorResponse('短码冲突，请稍后重试', 409)
  }

  const reused = Boolean(existing)
  if (!existing) {
    await env.LOADOUTS.put(key, bytes, {
      httpMetadata: { contentType: 'application/vnd.gbfr.loadout' },
      customMetadata: {
        sha256: identity.hash,
        protocol: `GBLC${bytes[4]}`,
      },
    })
  }

  const shown = displayCode(code)
  return jsonResponse({
    code: shown,
    compactCode: code,
    url: `${origin}/s/${code}`,
    downloadUrl: `${origin}/download/${code}.gbfr-loadout`,
    bytes: bytes.byteLength,
    reused,
  }, reused ? 200 : 201, { 'Cache-Control': 'no-store' })
}

async function loadFrame(env, code) {
  const object = await env.LOADOUTS.get(objectKey(code))
  if (!object) return null
  const bytes = await readObjectBytes(object)
  if (!bytes || validateFrame(bytes)) return null
  return { object, bytes }
}

function binaryResponse(frame, attachmentName = '') {
  const headers = new Headers(baseHeaders)
  headers.set('Content-Type', 'application/vnd.gbfr.loadout')
  headers.set('Cache-Control', 'public, max-age=86400, immutable')
  if (frame.object.httpEtag) headers.set('ETag', frame.object.httpEtag)
  if (attachmentName) {
    headers.set('Content-Disposition', `attachment; filename="${attachmentName}"`)
  }
  return new Response(frame.bytes, { status: 200, headers })
}

function landingPage(origin, code) {
  const shown = displayCode(code)
  const download = `${origin}/download/${code}.gbfr-loadout`
  return `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <meta name="color-scheme" content="light">
  <title>GBFR 配装分享 ${shown}</title>
  <style>
    :root{font-family:system-ui,"Microsoft YaHei",sans-serif;color:#3d3023;background:#f4efe3}
    *{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;padding:24px}
    main{width:min(560px,100%);border:1px solid #c9b58d;background:#fffdf7;padding:28px;box-shadow:0 18px 50px #4a381926}
    small{color:#7b684b}h1{margin:8px 0 12px;font-size:26px;letter-spacing:0}
    p{line-height:1.7;color:#65553f}.code{margin:22px 0;padding:18px;border:1px dashed #9d7a3e;background:#f8f0dc;text-align:center;font:700 24px ui-monospace,monospace;letter-spacing:2px}
    nav{display:grid;grid-template-columns:1fr 1fr;gap:10px}a,button{min-height:44px;border:1px solid #94713b;background:#fffdf7;color:#62471f;font-weight:700;text-decoration:none;display:grid;place-items:center;cursor:pointer}
    a.primary{background:#8d6730;color:white}@media(max-width:480px){main{padding:20px}nav{grid-template-columns:1fr}.code{font-size:18px}}
  </style>
</head>
<body><main>
  <small>GBFR PE Patch Tool · 单套配装</small>
  <h1>收到一套配装</h1>
  <p>在工具的配装编辑页打开“分享”，输入下面的短码即可选择要导入的内容。</p>
  <div class="code">${shown}</div>
  <nav>
    <a class="primary" href="${download}">下载配装文件</a>
    <button type="button" onclick="navigator.clipboard.writeText('${shown}').then(()=>this.textContent='已复制')">复制短码</button>
  </nav>
</main></body></html>`
}

export default {
  async fetch(request, env) {
    if (request.method === 'OPTIONS') return new Response(null, { status: 204, headers: baseHeaders })
    const url = new URL(request.url)
    const origin = url.origin

    if (request.method === 'GET' && url.pathname === '/health') {
      return jsonResponse({ ok: true, protocol: 'GBLC', frameVersions: [...FRAME_VERSIONS] }, 200, { 'Cache-Control': 'no-store' })
    }
    if (request.method === 'POST' && url.pathname === '/api/v1/loadouts') {
      return publish(request, env, origin)
    }

    const apiMatch = url.pathname.match(/^\/api\/v1\/loadouts\/([^/]+)$/)
    if ((request.method === 'GET' || request.method === 'HEAD') && apiMatch) {
      const code = normalizeCode(apiMatch[1])
      if (!code) return errorResponse('短码格式无效', 400)
      const frame = await loadFrame(env, code)
      if (!frame) return errorResponse('没有找到这套配装', 404)
      if (request.method === 'HEAD') {
        return new Response(null, {
          status: 200,
          headers: { ...baseHeaders, 'Content-Type': 'application/vnd.gbfr.loadout' },
        })
      }
      return binaryResponse(frame)
    }

    const downloadMatch = url.pathname.match(/^\/download\/([^/]+)\.gbfr-loadout$/)
    if (request.method === 'GET' && downloadMatch) {
      const code = normalizeCode(downloadMatch[1])
      if (!code) return errorResponse('短码格式无效', 400)
      const frame = await loadFrame(env, code)
      if (!frame) return errorResponse('没有找到这套配装', 404)
      return binaryResponse(frame, `GBFR-${displayCode(code)}.gbfr-loadout`)
    }

    const shareMatch = url.pathname.match(/^\/s\/([^/]+)$/)
    if (request.method === 'GET' && shareMatch) {
      const code = normalizeCode(shareMatch[1])
      if (!code) return errorResponse('短码格式无效', 400)
      const object = await env.LOADOUTS.head(objectKey(code))
      if (!object) return errorResponse('没有找到这套配装', 404)
      return new Response(landingPage(origin, code), {
        status: 200,
        headers: {
          ...baseHeaders,
          'Content-Type': 'text/html; charset=utf-8',
          'Cache-Control': 'public, max-age=300',
          'Content-Security-Policy': "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'",
        },
      })
    }

    return errorResponse('Not found', 404)
  },
}
