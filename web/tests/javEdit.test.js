import assert from 'node:assert/strict'
import test from 'node:test'

import { findJavEditOptionByName } from '../src/utils/javEdit.js'

test('finds an existing option by any supplied name', () => {
  const option = { id: 1, name: '明里つむぎ', aliases: ['明里紬'] }

  assert.equal(
    findJavEditOptionByName([option], '明里紬', (item) => [item.name, ...item.aliases]),
    option
  )
})

test('normalizes whitespace, width, and case when checking duplicate names', () => {
  const option = { id: 2, name: 'ＳＳＮＩ' }

  assert.equal(findJavEditOptionByName([option], '  ssni  '), option)
})

test('allows a new name when no option has the same name', () => {
  assert.equal(findJavEditOptionByName([{ id: 1, name: '已有标签' }], '新标签'), null)
})
