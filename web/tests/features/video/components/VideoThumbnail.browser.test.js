import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'thumbnails retry transient errors, stop after three retries, and cancel on unmount',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const requests = new Map()
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-thumbnail-test',
      plugins: [
        {
          name: 'thumbnail-test-responses',
          configureServer(server) {
            server.middlewares.use((req, res, next) => {
              const url = new URL(req.url, 'http://localhost')
              if (!url.pathname.startsWith('/thumbnail-fixture/')) return next()
              const calls = requests.get(url.pathname) || []
              calls.push(url.searchParams.get('thumbnail_retry'))
              requests.set(url.pathname, calls)
              res.setHeader('Cache-Control', 'no-store')
              if (
                url.pathname.endsWith('/fail') ||
                url.pathname.endsWith('/cancel') ||
                (url.pathname.endsWith('/retry') && calls.length < 2)
              ) {
                res.statusCode = 503
                res.setHeader('Retry-After', '3')
                return res.end('pending')
              }
              res.setHeader('Content-Type', 'image/svg+xml')
              res.end('<svg xmlns="http://www.w3.org/2000/svg" width="2" height="2"/>')
            })
          },
        },
      ],
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/videoThumbnail.html` })
    await waitFor('window.ready')
    await evaluate("showThumbnail('/thumbnail-fixture/retry?v=original')")
    await waitFor('window.loads === 1')
    assert.deepEqual(requests.get('/thumbnail-fixture/retry'), [null, '1'])
    assert.equal(await evaluate("document.querySelector('img').style.visibility"), '')

    await evaluate("showThumbnail('/thumbnail-fixture/fail')")
    await waitFor('window.failures === 1')
    await new Promise((resolve) => setTimeout(resolve, 250))
    assert.deepEqual(requests.get('/thumbnail-fixture/fail'), [null, '1', '2', '3'])

    await evaluate("showThumbnail('/thumbnail-fixture/success')")
    await waitFor('window.loads === 2')
    assert.deepEqual(requests.get('/thumbnail-fixture/success'), [null])

    await evaluate("window.unmountOnError = true; showThumbnail('/thumbnail-fixture/cancel')")
    await waitFor("!document.querySelector('img')")
    await new Promise((resolve) => setTimeout(resolve, 250))
    assert.deepEqual(requests.get('/thumbnail-fixture/cancel'), [null])
  }
)
