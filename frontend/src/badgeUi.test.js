import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const saveEditor = readFileSync(new URL('./components/SaveEditor.vue', import.meta.url), 'utf8')
const badgeEditor = readFileSync(new URL('./components/BadgeUnlock.vue', import.meta.url), 'utf8')
const shell = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')

test('title records live inside the existing save-record page and reuse its portrait', () => {
  assert.match(saveEditor, /BadgeUnlock/)
  assert.match(saveEditor, /任务完成次数/)
  assert.match(saveEditor, /称号记录/)
  assert.match(saveEditor, /role="tablist"/)
  assert.match(shell, /save:\s*\{[\s\S]*?title:\s*'任务与称号记录'/)
  assert.doesNotMatch(shell, /items:\s*\[[^\]]*badge/)
})

test('title editor uses locked state APIs, shared confirmation, and preserves reward records', () => {
  assert.match(badgeEditor, /LoadBadgeState/)
  assert.match(badgeEditor, /SetBadgeState/)
  assert.match(badgeEditor, /SetAllBadgeStates/)
  assert.match(badgeEditor, /ConfirmDialog/)
  assert.match(badgeEditor, /const markViewed = ref\(true\)/)
  assert.match(badgeEditor, /奖励领取记录不会修改/)
  assert.doesNotMatch(badgeEditor, /SetBadgeReward|rewardClaimed\s*=/)
})

test('title list is windowed, resets the real scroller, and reflows on narrow panels', () => {
  assert.match(badgeEditor, /const visibleBadges = computed/)
  assert.match(badgeEditor, /listRef\.value\.scrollTop\s*=\s*0/)
  assert.match(badgeEditor, /@container\s*\(max-width:620px\)/)
  assert.match(badgeEditor, /role="list"/)
  assert.doesNotMatch(badgeEditor, /#[0-9a-fA-F]{6}/)
})

test('title list observes the scroller that appears after async state loading', () => {
  assert.match(badgeEditor, /watch\(listRef,\s*\(nextList,\s*previousList\)\s*=>\s*\{/)
  assert.match(badgeEditor, /resizeObserver\?\.unobserve\(previousList\)/)
  assert.match(badgeEditor, /resizeObserver\?\.observe\(nextList\)/)
  assert.match(badgeEditor, /onBeforeUnmount\(\(\)\s*=>\s*resizeObserver\?\.disconnect\(\)\)/)
})

test('title virtual scroller has a viewport-bounded height to avoid ResizeObserver growth feedback', () => {
  assert.match(badgeEditor, /\.badge-list\s*\{[^}]*height:\s*clamp\([^;]*dvh[^;]*\)/s)
  assert.match(badgeEditor, /\.badge-list\s*\{[^}]*flex:\s*none/s)
})
