import assert from 'node:assert/strict'
import test from 'node:test'
import {
  JAV_SORT_OPTIONS,
  findSortOption,
  normalizeJavSort,
  normalizeJavSortRules,
  resolveJavSort,
  reverseSortValue,
} from '../../src/constants/jav.js'

test('JAV watch time sorting replaces play count and supports both directions', () => {
  assert.equal(
    JAV_SORT_OPTIONS.some((option) => option.base === 'play_count'),
    false
  )
  const option = findSortOption(JAV_SORT_OPTIONS, 'watched')
  assert.ok(option)
  assert.equal(option.label[0], '观看时长')
  assert.equal(findSortOption(JAV_SORT_OPTIONS, 'watched_asc'), option)
  assert.equal(reverseSortValue(JAV_SORT_OPTIONS, 'watched'), 'watched_asc')
  assert.equal(reverseSortValue(JAV_SORT_OPTIONS, 'watched_asc'), 'watched')
})

test('JAV saved defaults, temporary sorts and automatic rules retain legacy directions', () => {
  for (const [input, expected] of [
    ['watched', 'watched'],
    ['watched_asc', 'watched_asc'],
    ['watched_desc', 'watched'],
    ['play_count', 'watched'],
    ['play_count_desc', 'watched'],
    [' PLAY_COUNT_ASC ', 'watched_asc'],
  ]) {
    assert.equal(normalizeJavSort(input), expected)
    assert.equal(resolveJavSort({ javSort: input }).sort, expected)
    assert.equal(resolveJavSort({ javTempSort: input }).sort, expected)
    const rules = [{ id: 'watch', enabled: true, mode: 'all', active: ['idol'], sort: input }]
    assert.equal(normalizeJavSortRules({ version: 1, rules })[0].sort, expected)
    const resolved = resolveJavSort({ javIdolIds: [1], javSortRules: rules })
    assert.equal(resolved.sort, expected)
    assert.equal(resolved.source, 'rule')
  }
})
