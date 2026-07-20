import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/BadgeUnlock.vue', import.meta.url), 'utf8')

test('title catalog search learned pinyin matching from upstream 1.8.5', () => {
  assert.match(source, /import OpenCC from 'opencc-js\/t2cn'/)
  assert.match(source, /OpenCC\.Converter\(\{ from: 'tw', to: 'cn' \}\)/)
  assert.match(source, /import \{ matchText \} from '\.\.\/utils\/matchText\.js'/)
  assert.match(source, /\[badge\.id, badge\.nameZh, badge\.nameZhSimplified, badge\.nameEn\]\.some/)
  assert.match(source, /searchPlaceholder: 'Search by name or ID'/)
  assert.match(source, /searchPlaceholder: '搜索名称、拼音或编号'/)
})

test('Chinese title rows render the simplified name while English rows render only English', () => {
  assert.match(source, /language\.value === 'en' \? badge\.nameEn : badge\.nameZhSimplified/)
  assert.doesNotMatch(source, /<small>\{\{\s*[^}]*badge\.name(?:En|Zh)/)
})
