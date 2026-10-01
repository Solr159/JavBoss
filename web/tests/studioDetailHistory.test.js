import assert from 'node:assert/strict'
import test from 'node:test'
import {
  getStudioDetailId,
  withStudioDetail,
  normalizeStudioDetailState,
  canReturnToStudioBackground,
} from '../src/utils/studioDetailHistory.js'

test('studio detail URL preserves the underlying filters and pagination', () => {
  const page = '/?view=jav&tab=studio&search=example&page=3&favorite_group_id=7'
  const detail = withStudioDetail(page, 12)
  assert.equal(getStudioDetailId(detail.split('?')[1]), 12)
  assert.equal(withStudioDetail(detail, null), page)
  assert.equal(withStudioDetail(detail, 13), `${page}&studio_detail=13`)
})

test('invalid studio identifiers do not open a detail modal', () => {
  for (const value of ['', '0', '-1', '1.2', '2abc', 'Infinity', '9007199254740992']) {
    assert.equal(getStudioDetailId(`studio_detail=${value}`), null)
  }
})

test('history restores independent sort modes and scroll with safe defaults', () => {
  assert.deepEqual(normalizeStudioDetailState(), {
    prefixSort: 'name_asc',
    seriesSort: 'work_count_desc',
    scrollTop: 0,
  })
  const saved = { prefixSort: 'work_count_desc', seriesSort: 'name_asc', scrollTop: 450 }
  assert.deepEqual(normalizeStudioDetailState(saved), saved)
  assert.equal(normalizeStudioDetailState({ scrollTop: -10 }).scrollTop, 0)
  assert.equal(normalizeStudioDetailState({ scrollTop: Infinity }).scrollTop, 0)
})

test('only modal entries created over the same page close by going back', () => {
  const page = '/?view=jav&tab=studio&page=2'
  assert.equal(canReturnToStudioBackground({ id: 1, backgroundRoute: page }, 1, page), true)
  assert.equal(canReturnToStudioBackground(undefined, 1, page), false)
  assert.equal(canReturnToStudioBackground({ id: 1 }, 1, page), false)
  assert.equal(canReturnToStudioBackground({ id: 2, backgroundRoute: page }, 1, page), false)
  assert.equal(canReturnToStudioBackground({ id: 1, backgroundRoute: '/' }, 1, page), false)
})
