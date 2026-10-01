import assert from 'node:assert/strict'
import test from 'node:test'
import { restoreDetailScroll } from '../src/utils/restoreDetailScroll.js'

test('scroll restoration waits for deferred height and stops after reaching the saved position', (t) => {
  let resize
  let change
  class ResizeObserver {
    constructor(callback) {
      resize = callback
    }
    observe() {}
    disconnect() {}
  }
  class MutationObserver {
    constructor(callback) {
      change = callback
    }
    observe() {}
    disconnect() {}
  }
  const previous = [globalThis.ResizeObserver, globalThis.MutationObserver]
  globalThis.ResizeObserver = ResizeObserver
  globalThis.MutationObserver = MutationObserver
  t.after(() => {
    ;[globalThis.ResizeObserver, globalThis.MutationObserver] = previous
  })
  let height = 1255
  let top = 0
  let finished = 0
  const events = new Map()
  const node = {
    children: [{}],
    set scrollTop(value) {
      top = Math.min(value, height)
    },
    get scrollTop() {
      return top
    },
    addEventListener(name, callback) {
      events.set(name, callback)
    },
    removeEventListener(name) {
      events.delete(name)
    },
  }
  const cleanup = restoreDetailScroll(node, 1800, () => finished++)
  t.after(cleanup)
  assert.equal(top, 1255)
  assert.equal(finished, 0)
  height = 2200
  change()
  assert.equal(top, 1800)
  assert.equal(finished, 1)
  node.scrollTop = 400
  resize()
  assert.equal(top, 400, 'later content changes must not reset the user position')
  assert.equal(events.size, 0)

  height = 1000
  restoreDetailScroll(node, 1800, () => finished++)
  events.get('wheel')()
  node.scrollTop = 100
  height = 2200
  resize()
  assert.equal(top, 100, 'user interaction cancels pending restoration')
  assert.equal(finished, 2)
})
