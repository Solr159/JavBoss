import assert from 'node:assert/strict'
import test from 'node:test'
import {
  JAV_COVER_DEFAULT_CROP_LEFT,
  JAV_COVER_FRAME_ASPECT,
  JAV_COVER_VISIBLE_RATIO,
  javCoverVisibleRatio,
  normalizeJavCoverCropLeft,
  resolveJavCoverObjectPosition,
} from '../../src/utils/javCover.js'

test('default crop left matches the idol page right-portion crop', () => {
  assert.equal(JAV_COVER_DEFAULT_CROP_LEFT, 0.53)
  assert.equal(JAV_COVER_VISIBLE_RATIO, 0.47)
  assert.ok(Math.abs(JAV_COVER_FRAME_ASPECT - (800 * 0.47) / 538) < 1e-9)
})

test('normalizes crop left into the 0..1 range with a default fallback', () => {
  assert.equal(normalizeJavCoverCropLeft(undefined), JAV_COVER_DEFAULT_CROP_LEFT)
  assert.equal(normalizeJavCoverCropLeft('nope'), JAV_COVER_DEFAULT_CROP_LEFT)
  assert.equal(normalizeJavCoverCropLeft(-0.4), 0)
  assert.equal(normalizeJavCoverCropLeft(2), 1)
  assert.equal(normalizeJavCoverCropLeft(0.25), 0.25)
})

test('visible ratio depends on the cover aspect vs the compact frame aspect', () => {
  // A standard 800x538 cover shows the full frame ratio (0.47).
  assert.ok(Math.abs(javCoverVisibleRatio(800 / 538) - 0.47) < 1e-9)
  // Unknown or invalid aspect falls back to the default ratio.
  assert.equal(javCoverVisibleRatio(0), JAV_COVER_VISIBLE_RATIO)
  assert.equal(javCoverVisibleRatio(Number.NaN), JAV_COVER_VISIBLE_RATIO)
})

test('object position maps crop fractions onto the overflow range', () => {
  const imageAspect = 800 / 538
  // Default crop sits at the right edge.
  assert.ok(Math.abs(resolveJavCoverObjectPosition(0.53, imageAspect) - 100) < 1e-9)
  // Left edge.
  assert.equal(resolveJavCoverObjectPosition(0, imageAspect), 0)
  // Middle of the allowed range.
  assert.ok(Math.abs(resolveJavCoverObjectPosition(0.265, imageAspect) - 50) < 1e-9)
  // Values past the max clamp to the right edge.
  assert.ok(Math.abs(resolveJavCoverObjectPosition(0.9, imageAspect) - 100) < 1e-9)
  // A cover ratio that already fills the frame has no horizontal overflow.
  assert.equal(resolveJavCoverObjectPosition(0.53, JAV_COVER_FRAME_ASPECT), 0)
})
