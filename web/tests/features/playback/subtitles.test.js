import assert from 'node:assert/strict'
import { EventEmitter } from 'node:events'
import test from 'node:test'
import { startSubtitleLoading } from '../../../src/features/playback/subtitles.js'

function fakePlayer() {
  const player = new EventEmitter()
  player.tracks = []
  player.added = []
  player.removed = []
  player.textTracks = () => player.tracks
  player.isDisposed = () => false
  player.readyState = () => 1
  player.addRemoteTextTrack = (options) => {
    player.added.push(options)
    const track = { ...options }
    player.tracks.push(track)
    return { track }
  }
  player.removeRemoteTextTrack = (track) => player.removed.push(track)
  return player
}

const subtitle = () => ({ blob: new Blob(['WEBVTT\n']), language: 'zh-CN', source: 'test' })
const settle = () => new Promise((resolve) => setImmediate(resolve))

test('downloaded subtitles load once and cleanup removes the track and blob URL', async (t) => {
  const player = fakePlayer()
  const revoked = []
  t.mock.method(URL, 'revokeObjectURL', (url) => revoked.push(url))
  const statuses = []
  const stop = startSubtitleLoading(player, {
    autoDownload: true,
    resolve: async () => subtitle(),
    onStatus: (status) => statuses.push(status),
  })
  player.emit('loadedmetadata')
  await settle()
  assert.equal(player.added.length, 1)
  assert.equal(player.tracks[0].mode, 'showing')
  assert.deepEqual(statuses, ['searching', 'ready'])
  stop()
  stop()
  assert.equal(player.removed.length, 1)
  assert.deepEqual(revoked, [player.added[0].src])
})

test('a late response from the previous video never loads into the next video', async () => {
  const player = fakePlayer()
  let finish, signal
  const statuses = []
  const stop = startSubtitleLoading(player, {
    autoDownload: true,
    resolve: (value) => {
      signal = value
      return new Promise((resolve) => {
        finish = resolve
      })
    },
    onStatus: (status) => statuses.push(status),
  })
  stop()
  finish(subtitle())
  await settle()
  assert.equal(signal.aborted, true)
  assert.equal(player.added.length, 0)
  assert.deepEqual(statuses, ['searching'])
})

test('existing captions and user choices made during download take priority', async () => {
  const player = fakePlayer()
  player.tracks.push({ kind: 'captions', mode: 'disabled' })
  let calls = 0
  let stop = startSubtitleLoading(player, {
    resolve: async () => {
      calls++
      return subtitle()
    },
  })
  assert.equal(calls, 1)
  stop()
  player.tracks = []
  let finish
  stop = startSubtitleLoading(player, {
    resolve: () =>
      new Promise((resolve) => {
        finish = resolve
      }),
  })
  player.tracks.push({ kind: 'subtitles', mode: 'showing' })
  finish(subtitle())
  await settle()
  assert.equal(player.added.length, 1)
  assert.equal(player.tracks[0].mode, 'showing')
  assert.equal(player.tracks[1].mode, 'disabled')
  stop()
})

test('provider failures leave playback intact and expose an error status', async () => {
  const player = fakePlayer()
  const statuses = []
  const stop = startSubtitleLoading(player, {
    autoDownload: true,
    resolve: async () => {
      throw new Error('offline')
    },
    onStatus: (status) => statuses.push(status),
  })
  await settle()
  assert.deepEqual(statuses, ['searching', 'error'])
  assert.equal(player.added.length, 0)
  stop()
})

test('playback checks existing subtitles and manual download can retry without duplicate requests', async () => {
  const player = fakePlayer()
  let calls = 0
  let finish
  const requests = []
  const statuses = []
  const stop = startSubtitleLoading(player, {
    resolve: (signal, manual) => {
      calls++
      requests.push({ signal, manual })
      if (!manual) return Promise.resolve({ status: 'disabled' })
      return new Promise((resolve) => {
        finish = resolve
      })
    },
    onStatus: (status) => statuses.push(status),
  })
  player.emit('loadedmetadata')
  assert.deepEqual(statuses, [], 'cache lookup must not announce an online search')
  await settle()
  assert.equal(calls, 1)
  assert.equal(requests[0].manual, false)
  player.emit('javbossloadsubtitles')
  player.emit('javbossloadsubtitles')
  assert.equal(calls, 2)
  assert.equal(requests[1].manual, true)
  finish({ status: 'not_found' })
  await settle()
  player.emit('javbossloadsubtitles')
  assert.equal(calls, 3)
  finish(subtitle())
  await settle()
  assert.equal(player.added.length, 1)
  assert.deepEqual(statuses, ['disabled', 'searching', 'not_found', 'searching', 'ready'])
  stop()
  player.emit('javbossloadsubtitles')
  assert.equal(calls, 3)
  assert.equal(player.listenerCount('javbossloadsubtitles'), 0)
})

test('manual loading keeps local subtitles and adds a selectable download without duplicates', async () => {
  const player = fakePlayer()
  let calls = 0
  const stop = startSubtitleLoading(player, {
    resolve: async (_signal, manual, options) => {
      calls++
      return manual || options?.cache_only ? subtitle() : { ...subtitle(), source: 'local' }
    },
  })
  await settle()
  const local = player.tracks[0]
  assert.equal(local.mode, 'showing')
  player.emit('javbossloadsubtitles')
  await settle()
  const downloaded = player.tracks[1]
  assert.equal(player.tracks.length, 2)
  assert.equal(local.mode, 'disabled')
  assert.equal(downloaded.mode, 'showing')
  // Switch back through the native track modes, then load again.
  local.mode = 'showing'
  downloaded.mode = 'disabled'
  player.emit('javbossloadsubtitles')
  await settle()
  assert.equal(local.mode, 'disabled')
  assert.equal(downloaded.mode, 'showing')
  assert.equal(calls, 2)
  assert.equal(player.tracks.length, 2)
  stop()
  assert.equal(player.removed.length, 2)
})

for (const existing of [false, true]) {
  test(`restores off for ${existing ? 'existing' : 'cached'} subtitles and remembers user toggles only`, async () => {
    const player = fakePlayer()
    const events = new EventTarget()
    player.tracks.addEventListener = events.addEventListener.bind(events)
    player.tracks.removeEventListener = events.removeEventListener.bind(events)
    if (existing) player.tracks.push({ kind: 'captions', mode: 'showing' })
    const writes = []
    const stop = startSubtitleLoading(player, {
      getSettings: async () => ({ track: 'off' }),
      resolve: async () => subtitle(),
      saveSelection: async (track) => writes.push(track),
    })
    await settle()
    assert.equal(player.tracks[0].mode, 'disabled')
    events.dispatchEvent(new Event('change'))
    await settle()
    assert.deepEqual(writes, [])
    player.tracks[0].mode = 'showing'
    events.dispatchEvent(new Event('change'))
    await settle()
    player.tracks[0].mode = 'disabled'
    events.dispatchEvent(new Event('change'))
    await settle()
    assert.deepEqual(writes, [existing ? 'embedded:1' : 'download:zh-CN', 'off'])
    stop()
    player.tracks[0].mode = 'showing'
    events.dispatchEvent(new Event('change'))
    await settle()
    assert.deepEqual(writes, [existing ? 'embedded:1' : 'download:zh-CN', 'off'])
  })
}

test('a fresh automatic download opens subtitles even when this video remembered off', async () => {
  const player = fakePlayer()
  const writes = []
  const stop = startSubtitleLoading(player, {
    getSettings: async () => ({ track: 'off' }),
    resolve: async () => ({ ...subtitle(), downloaded: true }),
    saveSelection: async (track) => writes.push(track),
  })
  await settle()
  assert.equal(player.tracks[0].mode, 'showing')
  assert.deepEqual(writes, ['download:zh-CN'])
  stop()
})

test('a settings response after switching videos cannot restore or save the previous preference', async () => {
  const player = fakePlayer()
  player.tracks.push({ kind: 'captions', mode: 'showing' })
  let finish
  const stop = startSubtitleLoading(player, {
    getSettings: () =>
      new Promise((resolve) => {
        finish = resolve
      }),
    resolve: () => assert.fail('unexpected lookup'),
    saveSelection: () => assert.fail('unexpected save'),
  })
  stop()
  finish({ track: 'off' })
  await settle()
  assert.equal(player.tracks[0].mode, 'showing')
})

for (const preferred of ['local:movie.srt', 'download:zh-CN']) {
  test(`local and cached tracks load automatically and restore ${preferred}`, async () => {
    const player = fakePlayer()
    const events = new EventTarget()
    player.tracks.addEventListener = events.addEventListener.bind(events)
    player.tracks.removeEventListener = events.removeEventListener.bind(events)
    const requests = [],
      writes = []
    const stop = startSubtitleLoading(player, {
      getSettings: async () => ({ track: preferred }),
      resolve: async (_signal, manual, options) => {
        requests.push({ manual, options })
        return options?.cache_only
          ? { ...subtitle(), key: 'download:zh-CN' }
          : { ...subtitle(), source: 'local', key: 'local:movie.srt' }
      },
      saveSelection: async (track) => writes.push(track),
    })
    await settle()
    assert.equal(player.tracks.length, 2)
    const selected = preferred.startsWith('local:') ? 0 : 1
    assert.equal(player.tracks[selected].mode, 'showing')
    assert.equal(player.tracks[1 - selected].mode, 'disabled')
    assert.deepEqual(requests[1], {
      manual: false,
      options: { download_only: true, cache_only: true },
    })
    assert.deepEqual(writes, [])
    // A native menu change emits intermediate mode changes; only the final
    // enabled track is saved, even when subtitles stay on throughout.
    player.tracks[selected].mode = 'disabled'
    events.dispatchEvent(new Event('change'))
    player.tracks[1 - selected].mode = 'showing'
    events.dispatchEvent(new Event('change'))
    await settle()
    assert.deepEqual(writes, [selected === 0 ? 'download:zh-CN' : 'local:movie.srt'])
    stop()
  })
}

test('missing remembered track falls back without overwriting the preference', async () => {
  const player = fakePlayer()
  const stop = startSubtitleLoading(player, {
    getSettings: async () => ({ track: 'download:zh-CN' }),
    resolve: async (_signal, _manual, options) =>
      options?.cache_only
        ? { status: 'disabled' }
        : { ...subtitle(), source: 'local', key: 'local:movie.srt' },
    saveSelection: () => assert.fail('fallback must not overwrite preference'),
  })
  await settle()
  assert.equal(player.tracks.length, 1)
  assert.equal(player.tracks[0].mode, 'showing')
  stop()
})
