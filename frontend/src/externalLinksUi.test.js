import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const readRoot = (path) => readFileSync(new URL(`../../${path}`, import.meta.url), 'utf8')

test('release metadata uses v1.91 consistently', () => {
  assert.match(readRoot('app.go'), /appVersion\s*=\s*"v1\.91"/)
  assert.equal(JSON.parse(readRoot('frontend/package.json')).version, '1.91.0')
  assert.equal(JSON.parse(readRoot('frontend/package-lock.json')).version, '1.91.0')
  assert.equal(JSON.parse(readRoot('wails.json')).info.productVersion, '1.91.0')
})

test('user-facing project content links only to this repository', () => {
  const paths = [
    'README.md',
    'README_EN.md',
    'docs/FORMULAS_2.0.2.md',
    'frontend/src/components/PatchTool.vue',
    'frontend/src/assets/gbfr/README.md',
    'data/summon_natural_rules_202.json',
    'data/wrightstone_traits.json',
  ]
  const allowedPrefix = 'https://github.com/Whitelinker574/GBFR-PE-Patch-Tool'

  for (const path of paths) {
    const urls = readRoot(path).match(/https?:\/\/[^\s)"'`]+/g) || []
    for (const url of urls) {
      assert.ok(url.startsWith(allowedPrefix), `${path} contains an external link: ${url}`)
    }
  }
})

test('packaged metadata no longer identifies another maintainer', () => {
  const metadata = readRoot('wails.json')
  const windowsInfo = readRoot('build/windows/info.json')
  assert.match(metadata, /"name": "Whitelinker574"/)
  assert.doesNotMatch(metadata, /BitterG|gourd_bitter|gitee\.com|苦瓜/)
  assert.match(windowsInfo, /"0409"/)
  assert.match(windowsInfo, /"product_version": "\{\{\.Info\.ProductVersion\}\}\.0"/)
  assert.match(windowsInfo, /"FileVersion": "\{\{\.Info\.ProductVersion\}\}\.0"/)
  assert.doesNotMatch(windowsInfo, /"0000"/)
})

test('release navigation cannot open a caller-supplied website', () => {
  const backend = readRoot('app.go')
  const shell = readRoot('frontend/src/components/PatchTool.vue')
  assert.match(backend, /func \(a \*App\) OpenReleasePage\(\) error/)
  assert.match(shell, /OpenReleasePage\(\)/)
  assert.doesNotMatch(shell, /OpenReleasePage\([^)]*releaseUrl/)
})
