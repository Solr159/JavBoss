import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { createServer } from 'vite'
import { chromePath, startChrome } from './chrome.js'

const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

export const browserUnavailable = !existsSync(chromePath) || typeof WebSocket === 'undefined'

export async function openBrowser(t, { cacheDir = 'node_modules/.vite-app-test' } = {}) {
  const root = fileURLToPath(new URL('../../', import.meta.url))
  const server = await createServer({
    root,
    cacheDir,
    server: { port: 0, host: '127.0.0.1' },
  })
  await server.listen()
  t.after(() => server.close())
  const origin = `http://127.0.0.1:${server.httpServer.address().port}`
  const { port } = await startChrome(t)
  const target = await (
    await fetch(`http://127.0.0.1:${port}/json/new?about:blank`, { method: 'PUT' })
  ).json()
  const socket = new WebSocket(target.webSocketDebuggerUrl)
  await new Promise((resolve) => socket.addEventListener('open', resolve, { once: true }))
  t.after(() => socket.close())
  let sequence = 0
  const pending = new Map()
  socket.addEventListener('message', ({ data }) => {
    const message = JSON.parse(data)
    if (pending.has(message.id)) {
      pending.get(message.id)(message)
      pending.delete(message.id)
    }
  })
  const command = (method, params = {}) =>
    new Promise((resolve) => {
      const id = ++sequence
      pending.set(id, resolve)
      socket.send(JSON.stringify({ id, method, params }))
    })
  const evaluate = async (expression) => {
    const reply = await command('Runtime.evaluate', {
      expression,
      returnByValue: true,
      awaitPromise: true,
    })
    assert.ok(!reply.error && !reply.result?.exceptionDetails, JSON.stringify(reply))
    return reply.result.result.value
  }
  const waitFor = async (expression) => {
    for (let i = 0; i < 100; i++) {
      if (await evaluate(`Boolean(${expression})`)) {
        await pause(100)
        return
      }
      await pause(50)
    }
    assert.fail(`Timed out: ${expression}; URL: ${await evaluate('location.href')}`)
  }
  return { origin, command, evaluate, waitFor }
}
