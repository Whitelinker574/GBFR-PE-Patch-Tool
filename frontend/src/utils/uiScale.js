// Global proportional UI scaling.
//
// The whole layout is authored in px at a ~1120-wide reference. On high-DPI
// screens that renders large and "takes up too much area" for a modifier-style
// tool. We apply a single CSS `zoom` to #app so every element scales together
// (art included), keeping the compact look the user asked for. The factor grows
// gently as the window enlarges, so bigger windows gradually get a bigger UI.
//
// #app width/height are compensated to 1/zoom so the scaled content still fills
// the whole window (zoom participates in layout, unlike transform:scale).

// Reference size at which zoom reaches 1.0. Larger reference => more compact
// default. Tune REF_W/REF_H to shift the baseline density.
const REF_W = 1380
const REF_H = 880
const MIN_ZOOM = 0.72
const MAX_ZOOM = 1.12

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
  // Counter-size so the zoomed content fills the viewport exactly.
  app.style.width = `calc(100vw / ${z})`
  app.style.height = `calc(100vh / ${z})`
}

function schedule() {
  if (raf) return
  raf = requestAnimationFrame(apply)
}

export function installUiScale() {
  apply()
  window.addEventListener('resize', schedule, { passive: true })
}
