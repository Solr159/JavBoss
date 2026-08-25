import assert from 'node:assert/strict'
import test from 'node:test'
import {
  JAV_PREFIX_INITIAL_OPTIONS,
  getAvailableJavPrefixInitials,
  matchesJavPrefixInitial,
} from '../src/utils/javPrefix.js'

test('provides digit and uppercase letter prefix filters in order', () => {
  assert.equal(JAV_PREFIX_INITIAL_OPTIONS.join(''), '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ')
})

test('only provides initials that exist in the loaded prefixes', () => {
  assert.deepEqual(
    getAvailableJavPrefixInitials([
      { prefix: 'XYZ' },
      { prefix: '  abc' },
      { prefix: '1pondo' },
      { prefix: 'ABC' },
      { prefix: '-unknown' },
      { prefix: '' },
    ]),
    ['1', 'A', 'X']
  )
})

test('matches the first non-whitespace prefix character case-insensitively', () => {
  assert.equal(matchesJavPrefixInitial('  1pondo', '1'), true)
  assert.equal(matchesJavPrefixInitial('abc', 'A'), true)
  assert.equal(matchesJavPrefixInitial('ABC', 'B'), false)
})

test('does not filter prefixes when no initial is selected', () => {
  assert.equal(matchesJavPrefixInitial('ABC', ''), true)
})
