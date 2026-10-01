import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { existsSync } from 'node:fs'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createServer } from 'vite'

const chromePath = process.env.CHROME_BIN || '/usr/bin/google-chrome'
const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

export const browserUnavailable = !existsSync(chromePath) || typeof WebSocket === 'undefined'

export async function openBrowser(t) {
  const root = fileURLToPath(new URL('../../', import.meta.url))
  const server = await createServer({
    root,
    cacheDir: 'node_modules/.vite-app-test',
    server: { port: 0, host: '127.0.0.1' },
  })
  await server.listen()
  t.after(() => server.close())
  const origin = `http://127.0.0.1:${server.httpServer.address().port}`
  const profile = await mkdtemp(path.join(os.tmpdir(), 'javboss-history-'))
  const chrome = spawn(
    chromePath,
    [
      '--headless',
      '--no-sandbox',
      '--disable-gpu',
      '--remote-debugging-port=0',
      `--user-data-dir=${profile}`,
      '--no-first-run',
      '--no-default-browser-check',
      'about:blank',
    ],
    { stdio: 'ignore' }
  )
  t.after(async () => {
    if (chrome.exitCode === null && chrome.signalCode === null) {
      const stopped = new Promise((resolve) => chrome.once('exit', resolve))
      chrome.kill()
      await stopped
    }
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 })
  })
  let port
  for (let i = 0; i < 100; i++) {
    try {
      port = (await readFile(path.join(profile, 'DevToolsActivePort'), 'utf8')).split('\n')[0]
      break
    } catch {
      await pause(50)
    }
  }
  assert.ok(port, 'Chrome started')
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
