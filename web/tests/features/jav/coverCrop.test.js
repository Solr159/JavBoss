import assert from 'node:assert/strict'
import test from 'node:test'
import {
  getCoverCropRect,
  normalizeCoverCropPosition,
  PORTRAIT_COVER_ASPECT,
} from '../../../src/features/jav/coverCrop.js'

test('portrait crop defaults to the right and clamps invalid positions', () => {
  for (const value of [undefined, null, '', NaN, Infinity])
    assert.equal(normalizeCoverCropPosition(value), 1)
  assert.equal(normalizeCoverCropPosition(-1), 0)
  assert.equal(normalizeCoverCropPosition(2), 1)
  assert.equal(normalizeCoverCropPosition(0), 0)
  const right = getCoverCropRect({ width: 800, height: 538 }, 1)
  assert.ok(Math.abs(right.left - 0.53) < 1e-12)
  assert.ok(Math.abs(right.width - 0.47) < 1e-12)
  assert.equal(right.height, 1)
  assert.equal(getCoverCropRect({ width: 800, height: 538 }, 0).left, 0)
})

test('crop geometry matches the portrait frame for different source proportions', () => {
  for (const [width, height] of [
    [800, 538],
    [1200, 400],
    [600, 600],
    [200, 800],
  ]) {
    const crop = getCoverCropRect({ width, height }, 0.5)
    assert.ok(
      Math.abs((crop.width * width) / (crop.height * height) - PORTRAIT_COVER_ASPECT) < 1e-12
    )
    assert.ok(Math.abs(crop.left - (1 - crop.width) / 2) < 1e-12)
    assert.equal(crop.top, (1 - crop.height) / 2)
  }
  assert.equal(getCoverCropRect(null, 1), null)
  assert.equal(getCoverCropRect({ width: 0, height: 538 }, 1), null)
})
