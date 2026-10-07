import assert from 'node:assert/strict'
import test from 'node:test'
import { createBrowserResume } from '../../../src/features/playback/browserResume.js'

function memoryStorage() {
  const data = new Map()
  return {
    getItem: (key) => data.get(key) ?? null,
    setItem: (key, value) => data.set(key, value),
    removeItem: (key) => data.delete(key),
  }
}

test('resume positions persist per video file, flush on close, and clear on completion', () => {
  const storage = memoryStorage()
  const options = { videoId: 1, locationId: 11, storage, now: () => 1 }
  const resume = createBrowserResume(options)
  resume.record(42.5, 300)
  assert.equal(createBrowserResume(options).position, 0)
  resume.flush()
  assert.equal(createBrowserResume(options).position, 42.5)
  assert.equal(createBrowserResume({ ...options, locationId: 12 }).position, 0)
  assert.equal(createBrowserResume({ ...options, videoId: 2 }).position, 0)
  const disabled = createBrowserResume({ ...options, enabled: false })
  assert.equal(disabled.position, 0)
  disabled.record(90, 300)
  disabled.complete()
  disabled.flush()
  assert.equal(createBrowserResume(options).position, 42.5)
  resume.complete()
  assert.equal(createBrowserResume(options).position, 0)
})

test('invalid media positions and unavailable storage do not break playback', () => {
  const storage = memoryStorage()
  const options = { videoId: 1, locationId: 11, storage }
  const resume = createBrowserResume(options)
  resume.record(15, 300)
  for (const [time, duration] of [
    [NaN, 300],
    [20, NaN],
    [Infinity, 300],
    [-1, 300],
  ]) {
    resume.record(time, duration)
    resume.flush()
    assert.equal(createBrowserResume(options).position, 15)
  }
  resume.record(300, 300)
  resume.flush()
  assert.equal(createBrowserResume(options).position, 0)
  for (const value of ['NaN', '-1', 'Infinity', 'invalid']) {
    storage.setItem('javboss.player.position.1.11', value)
    assert.equal(createBrowserResume(options).position, 0)
  }
  const fail = () => {
    throw new Error('Storage is blocked')
  }
  const blocked = createBrowserResume({
    ...options,
    storage: { getItem: fail, setItem: fail, removeItem: fail },
  })
  assert.equal(blocked.position, 0)
  assert.doesNotThrow(() => {
    blocked.record(15, 300)
    blocked.flush()
    blocked.complete()
  })
})
