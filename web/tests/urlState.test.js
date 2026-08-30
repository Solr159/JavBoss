import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const source = fs.readFileSync(new URL('../src/utils/urlState.js', import.meta.url), 'utf8')

test('page URL state no longer parses or serializes directory_ids', () => {
  assert.doesNotMatch(source, /directory_ids/)
})
