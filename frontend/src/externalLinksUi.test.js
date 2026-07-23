import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const readRoot = (path) => readFileSync(new URL(`../../${path}`, import.meta.url), 'utf8')

test('release metadata uses v1.91.12 consistently', () => {
  assert.match(readRoot('internal/backend/app.go'), /appVersion\s*=\s*"v1\.91\.12"/)
  assert.equal(JSON.parse(readRoot('frontend/package.json')).version, '1.91.12')
  assert.equal(JSON.parse(readRoot('frontend/package-lock.json')).version, '1.91.12')
  assert.equal(JSON.parse(readRoot('wails.json')).info.productVersion, '1.91.12')
})

test('application and evidence content links only to this repository', () => {
  const paths = [
    'docs/FORMULAS_2.0.2.md',
    'frontend/src/components/PatchTool.vue',
    'frontend/src/assets/gbfr/README.md',
    'internal/backend/data/summon_natural_rules_202.json',
    'internal/backend/data/wrightstone_traits.json',
  ]
  const allowedPrefix = 'https://github.com/Whitelinker574/GBFR-PE-Patch-Tool'

  for (const path of paths) {
    const urls = readRoot(path).match(/https?:\/\/[^\s)"'`]+/g) || []
    for (const url of urls) {
      assert.ok(url.startsWith(allowedPrefix), `${path} contains an external link: ${url}`)
    }
  }
})

test('README reference notes contain only the approved public links', () => {
  const allowed = new Set([
    'https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/releases/latest',
    'https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/actions/workflows/ci.yml',
    'https://github.com/BitterG/GBFR-PE-Patch-Tool',
    'https://b23.tv/xhiZ7fm',
    'https://lib.kannanote.top/%e7%a2%a7%e8%93%9d%e9%85%8d%e8%a3%85%e6%a8%a1%e6%8b%9f%e5%99%a8/',
    'https://b23.tv/mnwxgDf',
    'https://github.com/Nenkai',
    'https://b23.tv/lKSX4zy',
    'https://relinksummon.fate-go.top',
  ])
  for (const path of ['README.md', 'README_EN.md']) {
    const urls = readRoot(path).match(/https?:\/\/[^\s)"'`]+/g) || []
    for (const url of urls) {
      const isProjectLink = url.startsWith('https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/')
      assert.ok(isProjectLink || allowed.has(url), `${path} contains an unexpected link: ${url}`)
    }
  }
})

test('public sampling guide is self-contained and neutral', () => {
  const guide = readRoot('docs/角色公式采样操作说明.md')
  assert.match(guide, /## 提交样本用于复核/)
  assert.match(guide, /本仓库新建 Issue/)
  assert.doesNotMatch(guide, /交给我|发给我|把导出的.*给我|本轮最关键/)
})

test('repository states provenance and third-party boundaries', () => {
  const notices = readRoot('THIRD_PARTY_NOTICES.md')
  assert.match(notices, /originally forked from/)
  assert.match(notices, /does not grant a license/)
  assert.match(notices, /github\.com\/wailsapp\/wails\/v2/)
  assert.match(notices, /opencc-js/)
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
  const backend = readRoot('internal/backend/app.go')
  const shell = readRoot('frontend/src/components/PatchTool.vue')
  assert.match(backend, /func \(a \*App\) OpenReleasePage\(\) error/)
  assert.match(shell, /OpenReleasePage\(\)/)
  assert.doesNotMatch(shell, /OpenReleasePage\([^)]*releaseUrl/)
})
