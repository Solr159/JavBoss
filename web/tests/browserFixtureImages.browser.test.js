import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import test from 'node:test'
import { browserUnavailable, openBrowser } from './helpers/browser.js'

test(
  'native fixture images stay local while explicit image mocks and API proxying still work',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const backendRequests = []
    const backend = createServer((request, response) => {
      backendRequests.push(request.url)
      response.setHeader('Content-Type', 'application/json')
      response.end(JSON.stringify({ proxied: true }))
    })
    await new Promise((resolve) => backend.listen(0, '127.0.0.1', resolve))
    t.after(() => new Promise((resolve) => backend.close(resolve)))
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-fixture-images-test',
      plugins: [
        {
          name: 'explicit-thumbnail-fixture',
          config: () => ({
            server: {
              proxy: { '/videos': { target: `http://127.0.0.1:${backend.address().port}` } },
            },
          }),
          configureServer(server) {
            server.middlewares.use((request, response, next) => {
              if (request.url !== '/videos/1/thumbnail') return next()
              response.setHeader('Content-Type', 'image/svg+xml')
              response.end('<svg xmlns="http://www.w3.org/2000/svg" width="2" height="2"/>')
            })
          },
        },
      ],
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html` })
    await waitFor('window.testStore && window.requests')
    for (const [path, expected] of [
      ['/videos/1/thumbnail', 'loaded'],
      ['/videos/59/thumbnail?v=%7C', 'missing'],
      ['/jav/TEST-001/cover', 'missing'],
    ]) {
      assert.equal(
        await evaluate(`new Promise(resolve => {
        const image = new Image();
        image.onload = () => resolve('loaded');
        image.onerror = () => resolve('missing');
        image.src = ${JSON.stringify(path)};
        document.body.append(image);
      })`),
        expected
      )
    }
    assert.deepEqual(backendRequests, [])
    const response = await fetch(`${origin}/videos/59/thumbnail?v=%7C`, {
      headers: { 'Sec-Fetch-Dest': 'image' },
    })
    assert.equal(response.status, 404)
    // API requests are not hidden by the image fallback.
    assert.deepEqual(await (await fetch(`${origin}/videos`)).json(), { proxied: true })
    assert.deepEqual(backendRequests, ['/videos'])
    assert.equal(
      await evaluate(`window.requests.some(request => request.url.includes('/thumbnail'))`),
      false
    )
  }
)
