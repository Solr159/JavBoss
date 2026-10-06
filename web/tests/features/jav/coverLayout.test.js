import assert from 'node:assert/strict'
import test from 'node:test'
import {
  getJavPortraitImageStyle,
  JAV_PORTRAIT_ASPECT_RATIO,
} from '../../../src/features/jav/coverLayout.js'

test('portrait covers use the idol frame and clamp the 53% crop start to the image edges', () => {
  assert.equal(JAV_PORTRAIT_ASPECT_RATIO, 376 / 538)
  for (const [width, height, expectedCropLeft] of [
    [800, 538, 0.53],
    [1500, 1000, 0.53],
    [1650, 1000, 0.53],
    [1200, 1000, 0.4175960346964064],
    [1000, 1000, 0.3011152416356877],
  ]) {
    const style = getJavPortraitImageStyle(width, height)
    assert.ok(style)
    const imageWidthRatio = width / height / JAV_PORTRAIT_ASPECT_RATIO
    const cropLeft = -parseFloat(style.left) / 100 / imageWidthRatio
    assert.ok(Math.abs(cropLeft - expectedCropLeft) < 1e-12)
    assert.ok(cropLeft >= 0 && cropLeft + 1 / imageWidthRatio <= 1 + 1e-12)
  }
})

test('portrait covers center images narrower than the frame without stretching', () => {
  for (const [width, height] of [
    [680, 1000],
    [500, 1000],
  ]) {
    const style = getJavPortraitImageStyle(width, height)
    const imageWidthRatio = width / height / JAV_PORTRAIT_ASPECT_RATIO
    assert.ok(Math.abs(parseFloat(style.left) / 100 - (1 - imageWidthRatio) / 2) < 1e-12)
  }
})

test('portrait covers wait for valid image dimensions', () => {
  for (const [width, height] of [
    [0, 1000],
    [1500, 0],
    [Infinity, 1000],
    [1000, NaN],
  ]) {
    assert.equal(getJavPortraitImageStyle(width, height), null)
  }
})
