import assert from 'node:assert/strict'
import test from 'node:test'
import { loadModules } from '../../helpers/modules.js'

test('single-video playback sends the filename with normal and seek requests', async (t) => {
  const [{ playVideoFile }] = await loadModules(t, ['features/video/api.js'])
  const requests = []
  t.mock.method(globalThis, 'fetch', async (url, init) => {
    requests.push({ url, method: init.method, body: JSON.parse(init.body) })
    return Response.json({ status: 'ok' })
  })
  for (const startTime of [undefined, 12.5]) {
    await playVideoFile({
      id: 42,
      locationId: 7,
      title: '中文 {测试} 100%.mp4',
      path: '中文 {测试} 100%.mp4',
      dirPath: '/videos',
      startTime,
    })
    assert.deepEqual(requests.at(-1), {
      url: '/videos/play',
      method: 'POST',
      body: {
        video_id: 42,
        location_id: 7,
        title: '中文 {测试} 100%.mp4',
        path: '中文 {测试} 100%.mp4',
        dir_path: '/videos',
        ...(startTime === undefined ? {} : { start_time: startTime }),
      },
    })
  }
})

test('directory API resolves shared root paths without mixing identical subdirectory names', async (t) => {
  const [{ fetchVideoDirectories }] = await loadModules(t, ['features/video/api.js'])
  const items = [
    { directory_id: 1, path: '.', video_count: 0, videos: [] },
    { directory_id: 1, path: '旅行', video_count: 23, videos: ['one.mp4'] },
    { directory_id: 2, path: '旅行', video_count: 1, videos: ['two.mp4'] },
  ]
  t.mock.method(globalThis, 'fetch', async () =>
    Response.json({
      directory_paths: { 1: '/media/主目录', 2: '/archive' },
      items,
    })
  )
  assert.deepEqual(await fetchVideoDirectories(), [
    { ...items[0], directory_path: '/media/主目录' },
    { ...items[1], directory_path: '/media/主目录' },
    { ...items[2], directory_path: '/archive' },
  ])
})

test('directory API accepts older responses and empty directory lists', async (t) => {
  const [{ fetchVideoDirectories }] = await loadModules(t, ['features/video/api.js'])
  let payload = { items: [{ directory_id: 1, directory_path: '/videos', path: '.' }] }
  t.mock.method(globalThis, 'fetch', async () => Response.json(payload))
  assert.deepEqual(await fetchVideoDirectories(), payload.items)
  payload = { directory_paths: {}, items: [] }
  assert.deepEqual(await fetchVideoDirectories(), [])
})
