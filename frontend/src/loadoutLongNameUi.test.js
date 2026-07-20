import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const viewer = readFileSync(new URL('./components/LoadoutViewer.vue', import.meta.url), 'utf8')

test('character chips keep portrait name and count horizontal with a readable long-name fallback', () => {
  assert.match(viewer, /<span class="chara-chip-name" :title="g\.charaName">\{\{ g\.charaName \}\}<\/span>/)
  assert.match(viewer, /\.chara-row\s*\{[^}]*minmax\(156px,\s*1fr\)/is)
  assert.match(viewer, /\.chara-chip\s*\{[^}]*justify-content\s*:\s*flex-start/is)
  assert.match(viewer, /\.chara-chip-name\s*\{[^}]*min-width\s*:\s*0[^}]*overflow\s*:\s*hidden[^}]*text-overflow\s*:\s*ellipsis[^}]*white-space\s*:\s*nowrap/is)
  assert.match(viewer, /\.chara-chip img\s*\{[^}]*flex\s*:\s*0 0 auto/is)
  assert.match(viewer, /\.chara-chip i\s*\{[^}]*flex\s*:\s*0 0 auto/is)
})
