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
  const page = new EventTarget()
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
    page,
    document: null,
  })
  return {
    reports,
    close,
    events,
    pageEvent: (event, delta = 0) => {
      time += delta
      page.dispatchEvent(new Event(event))
    },
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
  assert.deepEqual(reports, [10000, 12000])
  release()
  await settle()
  assert.equal(reports.at(-1), 12000)
  assert.equal(f.cleared(), true)
})

test('pagehide sends the latest checkpoint before an outstanding report settles', async () => {
  let release
  const reports = []
  const f = fixture({
    report: (id, total) => {
      reports.push([id, total])
      if (reports.length === 1) return new Promise((resolve) => (release = resolve))
      return Promise.resolve(true)
    },
  })
  await settle()
  await f.emit('playing')
  await f.tick(10000)
  f.pageEvent('pagehide', 2000)
  // No response or promise continuation is needed to issue the final request.
  assert.deepEqual(reports, [
    ['session', 10000],
    ['session', 12000],
  ])
  await settle()
  release(true)
  await settle()

  // Restoring a cached page resumes counting without including its hidden time.
  f.pageEvent('pageshow', 30000)
  await f.tick(1000)
  assert.deepEqual(reports.at(-1), ['session', 13000])
  f.close()
  await settle()
})

test('a failed unload checkpoint is retried on the next timer tick', async () => {
  let release
  const reports = []
  const f = fixture({
    report: (id, total) => {
      reports.push(total)
      if (reports.length === 1) return new Promise((resolve) => (release = resolve))
      if (reports.length === 2) return Promise.reject(new Error('network failure'))
      return Promise.resolve(true)
    },
  })
  await settle()
  await f.emit('playing')
  await f.tick(10000)
  f.pageEvent('pagehide', 2000)
  assert.deepEqual(reports, [10000, 12000])
  await settle()
  release(true)
  await settle()
  await f.tick(10000)
  assert.deepEqual(reports, [10000, 12000, 12000])
  f.close()
  await settle()
})

test('concurrent flushes share one session creation', async () => {
  let resolveCreation
  let created = 0
  const f = fixture({
    create: () => {
      created++
      return new Promise((resolve) => (resolveCreation = resolve))
    },
  })
  await f.emit('playing')
  await f.tick(10000)
  await f.emit('pause', 2000)
  assert.equal(created, 1)
  assert.deepEqual(f.reports, [])
  resolveCreation('session')
  await settle()
  assert.ok(f.reports.length > 0)
  assert.ok(f.reports.every(([id, total]) => id === 'session' && total === 12000))
  f.close()
  await settle()
  assert.equal(created, 1)
  assert.equal(f.cleared(), true)
})

test('failed session creation can be retried without parallel creation requests', async () => {
  let rejectCreation
  let created = 0
  const f = fixture({
    create: () => {
      created++
      if (created === 1) return new Promise((resolve, reject) => (rejectCreation = reject))
      return Promise.resolve('session')
    },
  })
  await f.emit('playing')
  await f.tick(10000)
  assert.equal(created, 1)
  rejectCreation(new Error('network failure'))
  await settle()
  await f.tick(10000)
  assert.equal(created, 2)
  await f.tick(1000)
  assert.deepEqual(f.reports, [['session', 1000]])
  f.close()
  await settle()
})

test('ordinary flushes run concurrently and older successes cannot lower the acknowledged total', async () => {
  const requests = []
  const f = fixture({
    report: (id, total) => new Promise((resolve) => requests.push({ total, resolve })),
  })
  await settle()
  await f.emit('playing')
  await f.tick(10000)
  await f.emit('pause', 2000)
  assert.deepEqual(
    requests.map(({ total }) => total),
    [10000, 12000]
  )
  requests[1].resolve(true)
  await settle()
  requests[0].resolve(true)
  await settle()
  await f.tick(10000)
  f.close()
  await settle()
  assert.equal(requests.length, 2)
  assert.equal(f.cleared(), true)
})

for (const accepted of [true, false]) {
  test(`a stale ${accepted ? 'success' : 'expiry'} response cannot change the replacement session`, async () => {
    let created = 0
    let release
    const reports = []
    const f = fixture({
      create: async () => String(++created),
      report: (id, total) => {
        reports.push([id, total])
        if (reports.length === 1) return new Promise((resolve) => (release = resolve))
        return Promise.resolve(id !== '1')
      },
    })
    await settle()
    await f.emit('playing')
    await f.tick(10000)
    await f.tick(2000)
    assert.equal(created, 2)
    await f.tick(1000)
    assert.deepEqual(reports.at(-1), ['2', 1000])
    release(accepted)
    await settle()
    await f.tick(1000)
    assert.equal(created, 2)
    assert.deepEqual(reports.at(-1), ['2', 2000])
    f.close()
    await settle()
  })
}

test('an older success after close does not stop retries for the failed final total', async () => {
  let release
  const reports = []
  const f = fixture({
    report: (id, total) => {
      reports.push(total)
      if (reports.length === 1) return new Promise((resolve) => (release = resolve))
      if (reports.length === 2) return Promise.reject(new Error('network failure'))
      return Promise.resolve(true)
    },
  })
  await settle()
  await f.emit('playing')
  await f.tick(10000)
  await f.emit('waiting', 2000)
  f.close()
  assert.deepEqual(reports, [10000, 12000])
  await settle()
  release(true)
  await settle()
  assert.equal(f.cleared(), false)
  await f.tick(10000)
  assert.deepEqual(reports, [10000, 12000, 12000])
  assert.equal(f.cleared(), true)
})
