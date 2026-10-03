import assert from 'node:assert/strict'
import { createServer as createHTTPServer } from 'node:http'
import { fileURLToPath } from 'node:url'
import test from 'node:test'
import { createServer } from 'vite'
import config from '../vite.config.js'

test('resource requests pass through the Vite proxy with authentication and JSON responses', async (t) => {
  const backend = createHTTPServer((request, response) => {
    response.setHeader('Content-Type', 'application/json')
    if (request.url !== '/system/resources') {
      response.writeHead(404).end(JSON.stringify({ error: 'Not found' }))
    } else if (request.headers.cookie !== 'session=test-session') {
      response.writeHead(401).end(JSON.stringify({ error: 'Authentication required' }))
    } else {
      response.end(JSON.stringify({ process: { cpu_percent: 1.5 } }))
    }
  })
  await new Promise((resolve) => backend.listen(0, '127.0.0.1', resolve))
  t.after(() => new Promise((resolve) => backend.close(resolve)))
  const target = `http://127.0.0.1:${backend.address().port}`
  const server = await createServer({
    ...config,
    configFile: false,
    // This test only exercises forwarding; it does not serve frontend modules.
    optimizeDeps: { noDiscovery: true, include: [] },
    cacheDir: 'node_modules/.vite-proxy-test',
    root: fileURLToPath(new URL('../', import.meta.url)),
    logLevel: 'silent',
    server: {
      ...config.server,
      host: '127.0.0.1',
      port: 0,
      proxy: Object.fromEntries(
        Object.entries(config.server.proxy).map(([prefix, options]) => [
          prefix,
          { ...options, target },
        ])
      ),
    },
  })
  await server.listen()
  t.after(() => server.close())
  const origin = `http://127.0.0.1:${server.httpServer.address().port}`
  for (const [path, headers, status, body] of [
    ['/system/resources', {}, 401, { error: 'Authentication required' }],
    [
      '/system/resources',
      { Cookie: 'session=test-session' },
      200,
      { process: { cpu_percent: 1.5 } },
    ],
    ['/system/missing', {}, 404, { error: 'Not found' }],
  ]) {
    const response = await fetch(`${origin}${path}`, { headers })
    assert.equal(response.status, status)
    assert.match(response.headers.get('content-type'), /application\/json/)
    assert.deepEqual(await response.json(), body)
  }
})
