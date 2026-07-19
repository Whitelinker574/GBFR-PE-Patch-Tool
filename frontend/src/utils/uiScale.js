// Keep the application on the WebView's real CSS-pixel viewport. Responsive
// breakpoints in the shell are responsible for compact layouts; scaling the
// whole app would make those breakpoints observe a compensated virtual canvas.

let raf = 0

function apply() {
  raf = 0
  const app = document.getElementById('app')
  if (!app) return

  app.style.zoom = '1'
  app.style.setProperty('--ui-zoom', '1')
  app.style.setProperty('--ui-scale-inverse', '1')
  app.style.removeProperty('width')
  app.style.removeProperty('height')
}

function schedule() {
  if (raf) return
  raf = requestAnimationFrame(apply)
}

export function installUiScale() {
  apply()
  window.addEventListener('resize', schedule, { passive: true })
}
