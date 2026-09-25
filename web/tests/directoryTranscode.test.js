import assert from 'node:assert/strict'
import test from 'node:test'
import {
  transcodeIsActive,
  transcodeOverallPercent,
  transcodeResultsChanged,
} from '../src/utils/directoryTranscode.js'

test('batch progress includes the current file and stays below completion during validation', () => {
  assert.equal(
    transcodeOverallPercent({ phase: 'transcoding', total: 4, processed: 1, current_percent: 50 }),
    37.5
  )
  assert.equal(
    transcodeOverallPercent({ phase: 'verifying', total: 1, processed: 0, current_percent: 100 }),
    99
  )
  assert.equal(transcodeOverallPercent({ phase: 'completed', total: 4, processed: 4 }), 100)
})

test('refreshes library results after conversion, including a new job with a smaller count', () => {
  const previous = { started_at_unix_ms: 10, converted: 4 }
  assert.equal(transcodeResultsChanged(previous, { ...previous }), false)
  assert.equal(transcodeResultsChanged(previous, { ...previous, converted: 5 }), true)
  assert.equal(transcodeResultsChanged(previous, { started_at_unix_ms: 20, converted: 1 }), true)
  assert.equal(transcodeResultsChanged(previous, { started_at_unix_ms: 20, converted: 0 }), false)
})

test('discovery and empty batches have finite progress; cancelled batches retain partial progress', () => {
  assert.equal(transcodeOverallPercent(null), 0)
  assert.equal(transcodeOverallPercent({ phase: 'discovering', total: 3 }), 0)
  assert.equal(transcodeOverallPercent({ phase: 'completed', total: 0 }), 0)
  assert.equal(
    transcodeOverallPercent({ phase: 'cancelled', total: 4, processed: 1, current_percent: 50 }),
    25
  )
  assert.equal(transcodeIsActive({ phase: 'verifying' }), true)
  for (const phase of ['completed', 'failed', 'cancelled'])
    assert.equal(transcodeIsActive({ phase }), false)
})
