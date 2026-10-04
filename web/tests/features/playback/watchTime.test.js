import assert from 'node:assert/strict'
import test from 'node:test'
import { startWatchTracking } from '../../../src/features/playback/watchTime.js'

const settle = async () => {
  for (let i = 0; i < 12; i++) await Promise.resolve()
}

function fixture({ create, report } = {}) {
  let time = 0
  let timer
  let cleared = false
  let paused = false
  const events = new Map()
  const reports = []
  const player = {
    on: (event, handler) => events.set(event, handler),
    off: (event) => events.delete(event),
    paused: () => paused,
  }
  const close = startWatchTracking(player, {
    create: create || (async () => 'session'),
    report:
      report ||
      (async (id, total) => {
        reports.push([id, total])
        return true
      }),
    now: () => time,
    schedule: (handler) => {
      timer = handler
      return 1
    },
    unschedule: () => {
      cleared = true
    },
    page: null,
    document: null,
  })
  return {
    reports,
    close,
    events,
    tick: async (delta) => {
      time += delta
      timer()
      await settle()
    },
    emit: async (event, delta = 0) => {
      time += delta
      if (event === 'pause') paused = true
      if (event === 'playing' || event === 'play') paused = false
      events.get(event)?.()
      await settle()
    },
    cleared: () => cleared,
  }
}

test('counts unpaused wall time including seeks and buffering; flushes final tail', async () => {
  const f = fixture()
  await settle()
  await f.emit('playing')
  await f.tick(10000)
  await f.emit('pause', 2000)
  await f.tick(30000)
  await f.emit('playing')
  await f.emit('waiting', 3000)
  await f.tick(20000)
  await f.emit('playing')
  await f.emit('seeking', 4000)
  await f.emit('seeked', 5000)
  await f.tick(1000)
  await f.emit('ratechange', 2000)
  f.close()
  await settle()
  assert.deepEqual(
    f.reports.map(([, total]) => total),
    [10000, 12000, 35000, 45000, 47000]
  )
  assert.equal(f.cleared(), true)
  assert.equal(f.events.size, 0)
})

test('buffering keeps counting, pause stops it and unpausing resumes without a playing event', async () => {
  const f = fixture()
  await settle()
  await f.emit('playing')
  await f.emit('seeking', 1000)
  await f.emit('waiting', 1000)
  await f.emit('seeking', 1000)
  await f.emit('waiting', 1000)
  await f.tick(1000)
  assert.equal(f.reports.at(-1)[1], 5000)

  await f.emit('seeking')
  await f.emit('waiting')
  await f.tick(10000)
  await f.emit('seeked')
  await f.tick(10000)
  assert.equal(f.reports.at(-1)[1], 25000)
  await f.emit('pause')
  await f.emit('waiting', 10000)
  await f.tick(10000)
  assert.equal(f.reports.at(-1)[1], 25000)
  await f.emit('play')
  await f.emit('waiting')
  await f.tick(1000)
  assert.equal(f.reports.at(-1)[1], 26000)
  f.close()
  await settle()
})

test('seeking while paused, loading or ended does not start the clock', async () => {
  const f = fixture()
  await settle()
  await f.emit('play')
  await f.emit('waiting', 1000)
  await f.emit('seeking', 1000)
  await f.emit('seeked', 1000)
  await f.tick(1000)
  assert.deepEqual(f.reports, [])
  await f.emit('playing')
  await f.emit('pause', 1000)
  await f.emit('seeking', 1000)
  await f.emit('seeked', 1000)
  await f.tick(1000)
  assert.equal(f.reports.at(-1)[1], 1000)
  await f.emit('playing')
  await f.emit('ended', 1000)
  await f.emit('seeking', 1000)
  await f.emit('seeked', 1000)
  await f.tick(1000)
  assert.equal(f.reports.at(-1)[1], 2000)
  f.close()
  await settle()
})

test('changing sources requires actual playback to start again', async () => {
  const f = fixture()
  await settle()
  await f.emit('playing')
  await f.emit('emptied', 1000)
  await f.emit('play')
  await f.emit('waiting', 1000)
  await f.tick(10000)
  assert.equal(f.reports.at(-1)[1], 1000)
  await f.emit('playing')
  await f.tick(1000)
  f.close()
  await settle()
  assert.equal(f.reports.at(-1)[1], 2000)
})

test('retries the same cumulative value after a lost response and rebases on expiry', async () => {
  let created = 0
  const reports = []
  let fail = true
  let expired = false
  const f = fixture({
    create: async () => String(++created),
    report: async (id, total) => {
      reports.push([id, total])
      if (fail) {
        fail = false
        throw new Error('lost response')
      }
      if (expired) {
        expired = false
        return false
      }
      return true
    },
  })
  await settle()
  await f.emit('playing')
  await f.emit('pause', 10000)
  await f.tick(10000)
  assert.deepEqual(reports, [
    ['1', 10000],
    ['1', 10000],
  ])
  await f.emit('playing')
  expired = true
  await f.tick(5000)
  assert.equal(created, 2)
  await f.tick(2000)
  assert.deepEqual(reports.at(-1), ['2', 2000])
  f.close()
  await settle()
})

test('close during an in-flight request flushes the newer checkpoint', async () => {
  let release
  const reports = []
  const f = fixture({
    report: async (id, total) => {
      reports.push(total)
      if (!release)
        await new Promise((resolve) => {
          release = resolve
        })
      return true
    },
  })
  await settle()
  await f.emit('playing')
  await f.tick(10000)
  await f.emit('pause', 2000)
  f.close()
  release()
  await settle()
  assert.deepEqual(reports, [10000, 12000])
  assert.equal(f.cleared(), true)
})
