import assert from 'node:assert/strict'
import test from 'node:test'
import {
  changePlayerWindow,
  defaultPlayerWindow,
  fitPlayerWindow,
  isWindowGeometry,
} from '../../../src/features/playback/windowGeometry.js'

const viewport = { width: 1280, height: 900 }
const rect = { x: 100, y: 80, width: 640, height: 400 }

test('all eight resize handles keep their opposite edges fixed', () => {
  for (const direction of ['n', 'e', 's', 'w', 'ne', 'se', 'sw', 'nw']) {
    const next = changePlayerWindow(rect, direction, 50, 30, viewport)
    assert.deepEqual(next, {
      x: rect.x + (direction.includes('w') ? 50 : 0),
      y: rect.y + (direction.includes('n') ? 30 : 0),
      width: rect.width + (direction.includes('e') ? 50 : direction.includes('w') ? -50 : 0),
      height: rect.height + (direction.includes('s') ? 30 : direction.includes('n') ? -30 : 0),
    })
  }
})

test('movement and resizing respect the viewport and minimum size', () => {
  assert.deepEqual(changePlayerWindow(rect, 'move', -9999, 9999, viewport), {
    ...rect,
    x: 8,
    y: 492,
  })
  assert.deepEqual(changePlayerWindow(rect, 'nw', 9999, 9999, viewport), {
    x: 380,
    y: 240,
    width: 360,
    height: 240,
  })
  assert.deepEqual(changePlayerWindow(rect, 'se', 9999, 9999, viewport), {
    x: 100,
    y: 80,
    width: 1172,
    height: 812,
  })
  assert.deepEqual(fitPlayerWindow(rect, { width: 320, height: 200 }), {
    x: 8,
    y: 8,
    width: 304,
    height: 184,
  })
})

test('saved geometry validation rejects malformed data and defaults stay visible', () => {
  for (const value of [
    null,
    {},
    [],
    { ...rect, x: '20' },
    { ...rect, width: 0 },
    { ...rect, y: Infinity },
  ]) {
    assert.ok(!isWindowGeometry(value))
  }
  assert.ok(isWindowGeometry(rect))
  for (const size of [viewport, { width: 390, height: 844 }, { width: 300, height: 180 }]) {
    const initial = defaultPlayerWindow(size)
    assert.ok(initial.x >= 8 && initial.y >= 8)
    assert.ok(initial.x + initial.width <= size.width - 8)
    assert.ok(initial.y + initial.height <= size.height - 8)
  }
})
