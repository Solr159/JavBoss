import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { existsSync } from 'node:fs'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { createServer } from 'vite'

const chromePath = process.env.CHROME_BIN || '/usr/bin/google-chrome'
const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

test(
  'JAV detail restores after filter links, nested studio details, reload and direct links',
  {
    skip: !existsSync(chromePath) || typeof WebSocket === 'undefined',
    timeout: 60000,
  },
  async (t) => {
    const root = fileURLToPath(new URL('../', import.meta.url))
    const server = await createServer({
      root,
      cacheDir: 'node_modules/.vite-jav-history-test',
      server: { port: 18766, host: '127.0.0.1' },
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
    const modal = `document.querySelector('[role="dialog"][aria-labelledby="jav-detail-title-1"]')`
    const scroll = `${modal}.querySelector('.overflow-y-auto')`
    const fixture = `${origin}/tests/fixtures/javDetailNavigation.html`
    await command('Page.navigate', { url: `${fixture}?view=jav` })
    await waitFor(`document.querySelector('.jav-card button')`)
    const loadId = await evaluate('window.fixtureLoadId')
    await evaluate(`window.scrollTo(0,420); document.querySelector('.jav-card button').click()`)
    await waitFor(modal)
    assert.ok((await evaluate('location.search')).includes('jav_detail=1'))
    assert.equal(
      await evaluate('window.javDetailRequests'),
      0,
      'opening a card uses its data immediately'
    )
    await evaluate(`${scroll}.scrollTop = 250`)
    await waitFor('history.state.usr.__javbossJavDetail?.scrollTop === 250')
    for (const [label, param] of [
      ['Test studio', 'studio_id=2'],
      ['Test series', 'series_id=3'],
      ['Test actress', 'idol_ids=4'],
      ['Tag 1', 'tag_ids=1'],
    ]) {
      await evaluate(
        `[...${modal}.querySelectorAll('a')].find(a => a.textContent.trim() === '${label}').click()`
      )
      await waitFor(`!${modal} && !location.search.includes('jav_detail=')`)
      assert.ok((await evaluate('location.search')).includes(param), `filter URL contains ${param}`)
      assert.equal(
        await evaluate('window.fixtureLoadId'),
        loadId,
        'internal links do not reload the page'
      )
      await evaluate('history.back()')
      await waitFor(modal)
      assert.equal(await evaluate(`${scroll}.scrollTop`), 250)
    }
    // A second detail layer must return to JAV details before closing to the list.
    await evaluate(`document.querySelector('#nested-studio').click()`)
    await waitFor('location.search.includes("studio_detail=2")')
    await evaluate('history.back()')
    await waitFor(`!location.search.includes('studio_detail=') && ${modal}`)
    assert.equal(await evaluate(`${scroll}.scrollTop`), 250)
    await evaluate('history.forward()')
    await waitFor('location.search.includes("studio_detail=2")')
    await evaluate(`document.querySelector('[role="dialog"][aria-label] a').click()`)
    await waitFor(`!${modal} && location.search.includes('studio_id=2')`)
    await evaluate('history.back()')
    await waitFor('location.search.includes("studio_detail=2")')
    await evaluate(
      `document.querySelector('[role="dialog"][aria-label] button[aria-label]').click()`
    )
    await waitFor(`!location.search.includes('studio_detail=') && ${modal}`)
    await command('Page.reload')
    await waitFor(modal)
    assert.equal(await evaluate(`${scroll}.scrollTop`), 250)
    await evaluate(`${modal}.querySelector('button[aria-label]').click()`)
    await waitFor(`!${modal} && !location.search.includes('jav_detail=')`)
    await command('Page.navigate', { url: `${fixture}?view=jav&jav_detail=1` })
    await waitFor(modal)
    assert.ok(
      await evaluate('window.javDetailRequests > 0'),
      'direct links load detail without requiring the item in the list'
    )
    await evaluate(`${modal}.querySelector('button[aria-label]').click()`)
    await waitFor(`!${modal} && !location.search.includes('jav_detail=')`)
    // Matching the existing filters still creates a result entry over the modal.
    await command('Page.navigate', { url: `${fixture}?view=jav&studio_id=2&jav_detail=1` })
    await waitFor(modal)
    await evaluate(
      `[...${modal}.querySelectorAll('a')].find(a => a.textContent.trim() === 'Test studio').click()`
    )
    await waitFor(`!${modal} && location.search.includes('studio_id=2')`)
    await evaluate('history.back()')
    await waitFor(modal)
    await command('Page.navigate', { url: `${fixture}?view=jav&jav_detail=99` })
    await waitFor(`document.querySelector('[role="alert"]')`)
    await evaluate(`document.querySelector('[role="dialog"] button').click()`)
    await waitFor(`!document.querySelector('[role="dialog"]')`)
  }
)
