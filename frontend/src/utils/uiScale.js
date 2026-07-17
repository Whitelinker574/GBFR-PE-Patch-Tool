// Global proportional UI scaling.
//
// The layout is authored in px at a ~1120-wide reference; on high-DPI screens
// that renders large for a modifier-style tool. We apply one CSS `zoom` to #app
// so everything scales together, keeping a compact look that grows gently as the
// window enlarges.
//
// IMPORTANT: #app is sized in PIXELS (innerWidth/z), never in vw/vh. Newer
// Chromium/WebView2 rescales viewport units inside a zoomed subtree, so a
// calc(100vw / z) compensation would mis-size #app there — which in turn mis-
// sizes the flexible art column and slides the character art over the cards.
// Pixels are unambiguous under any zoom behaviour, so the scale stays uniform
// and the hand-tuned art framing is preserved.

const REF_W = 1580
const REF_H = 950
const MIN_ZOOM = 0.66
const MAX_ZOOM = 1.06

const DIAG = false // set true to show a measurement overlay for debugging

function computeZoom(w, h) {
  const z = Math.min(w / REF_W, h / REF_H)
  return Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, z))
}

let raf = 0
function apply() {
  raf = 0
  const w = window.innerWidth
  const h = window.innerHeight
  if (!w || !h) return
  const z = Math.round(computeZoom(w, h) * 1000) / 1000
  const app = document.getElementById('app')
  if (!app) return
  app.style.zoom = String(z)
  // Pixel compensation so the zoomed content fills the viewport exactly.
  app.style.width = `${Math.round(w / z)}px`
  app.style.height = `${Math.round(h / z)}px`
  if (DIAG) diag(app, w, h, z)
}

function diag(app, w, h, z) {
  let el = document.getElementById('__uiscale_diag')
  if (!el) {
    el = document.createElement('div')
    el.id = '__uiscale_diag'
    el.style.cssText =
      'position:fixed;left:6px;top:6px;z-index:99999;background:#111;color:#0f0;' +
      'font:12px monospace;padding:6px 8px;border:1px solid #0f0;white-space:pre;pointer-events:none'
    document.body.appendChild(el)
  }
  const r = app.getBoundingClientRect()
  el.textContent =
    `iw=${w} ih=${h} dpr=${window.devicePixelRatio}\n` +
    `zoom=${z}\n` +
    `app rendered=${Math.round(r.width)}x${Math.round(r.height)}\n` +
    `fills=${Math.abs(r.width - w) < 2 && Math.abs(r.height - h) < 2}`
}

function schedule() {
  if (raf) return
  raf = requestAnimationFrame(apply)
}

export function installUiScale() {
  apply()
  window.addEventListener('resize', schedule, { passive: true })
}
