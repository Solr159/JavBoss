import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

async function setup(t, clientMode = false) {
  const browser = await openBrowser(t, { cacheDir: 'node_modules/.vite-update-settings-test' })
  const { origin, command, evaluate, waitFor } = browser
  await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
  await waitFor(`document.querySelector('aside button[aria-label="Settings"]')`)
  await evaluate(`{
    window.updateRequests = [];
    window.updateStatus = {
      current_version: 'v1.0.0', latest_version: 'v2.0.0',
      available: true, can_install: true, phase: 'idle', checked_at: Date.now()
    };
    const originalFetch = window.fetch;
    window.fetch = (input, init = {}) => {
      const path = new URL(input, location.origin).pathname;
      if (path.startsWith('/system/update') || path.startsWith('/__client/update')) {
        window.updateRequests.push({path, method: init.method || 'GET'});
        if (path.endsWith('/install')) window.updateStatus.phase = 'downloading';
        return Promise.resolve(Response.json(window.updateStatus));
      }
      return originalFetch(input, init);
    };
    window.testStore.setState({config: {...window.testStore.getState().config, runtime_client: '${clientMode}'}});
    document.querySelector('aside button[aria-label="Settings"]').click();
  }`)
  await waitFor(`document.querySelector('[aria-labelledby="global-settings-title"]')`)
  await evaluate(
    `Array.from(document.querySelectorAll('aside button')).find(button => button.textContent === 'App Updates').click()`
  )
  await waitFor(`document.body.textContent.includes('v2.0.0')`)
  return browser
}

test(
  'installation requires confirmation and download errors remain visible',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { evaluate, waitFor } = await setup(t)
    const click = (text) =>
      evaluate(
        `Array.from(document.querySelectorAll('button')).find(button => button.textContent === '${text}').click()`
      )
    await click('Update and restart')
    await waitFor(`document.getElementById('app-update-confirm-title')`)
    assert.equal(
      await evaluate(`window.updateRequests.filter(r => r.path.endsWith('/install')).length`),
      0
    )
    await click('Cancel')
    await waitFor(`!document.getElementById('app-update-confirm-title')`)
    assert.equal(
      await evaluate(`window.updateRequests.filter(r => r.path.endsWith('/install')).length`),
      0
    )
    await click('Update and restart')
    await waitFor(`document.getElementById('app-update-confirm-title')`)
    await click('Continue update')
    await waitFor(
      `window.updateRequests.filter(r => r.path === '/system/update/install').length === 1`
    )
    await waitFor(`document.body.textContent.includes('Downloading update')`)
    assert.equal(
      await evaluate(
        `Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Update and restart').disabled`
      ),
      true
    )
    await evaluate(
      `window.updateStatus.phase = 'error'; window.updateStatus.error = 'release SHA256 mismatch'`
    )
    await waitFor(`document.body.textContent.includes('release SHA256 mismatch')`)
    assert.equal(
      await evaluate(
        `Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Update and restart').disabled`
      ),
      false
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)

test(
  'client updates default to local and the remote target is explicit',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { evaluate, waitFor } = await setup(t, true)
    assert.ok(await evaluate(`window.updateRequests.some(r => r.path === '/__client/update')`))
    assert.equal(
      await evaluate(`window.updateRequests.some(r => r.path === '/system/update')`),
      false
    )
    await evaluate(
      `document.querySelector('[role="combobox"]').dispatchEvent(new MouseEvent('mousedown', {bubbles:true}))`
    )
    await waitFor(`document.querySelector('[role="option"][data-value="server"]')`)
    await evaluate(`document.querySelector('[role="option"][data-value="server"]').click()`)
    await waitFor(`window.updateRequests.some(r => r.path === '/system/update')`)
    await evaluate(
      `Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Update and restart').click()`
    )
    await waitFor(`document.body.textContent.includes('on the remote server')`)
    assert.equal(
      await evaluate(`window.updateRequests.some(r => r.path.endsWith('/install'))`),
      false
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)

for (const target of ['server', 'local client', 'remote server']) {
  test(
    `reconnect refreshes the old page when the ${target} returns without its update API`,
    { skip: browserUnavailable, timeout: 60000 },
    async (t) => {
      const { evaluate, waitFor } = await setup(t, target !== 'server')
      if (target === 'remote server') {
        await evaluate(
          `document.querySelector('[role="combobox"]').dispatchEvent(new MouseEvent('mousedown', {bubbles:true}))`
        )
        await waitFor(`document.querySelector('[role="option"][data-value="server"]')`)
        await evaluate(`document.querySelector('[role="option"][data-value="server"]').click()`)
        await waitFor(`window.updateRequests.some(r => r.path === '/system/update')`)
      }
      const started = await evaluate('performance.timeOrigin')
      const readinessPath = target === 'local client' ? '/healthz' : '/auth/status'
      await evaluate(`{
        window.targetReady = false;
        window.updateDisconnected = false;
        window.readinessRequests = [];
        const originalFetch = window.fetch;
        window.fetch = (input, init = {}) => {
          const path = new URL(input, location.origin).pathname;
          if (window.updateDisconnected && (path === '/system/update' || path === '/__client/update')) {
            return Promise.resolve(Response.json({error_en: 'Update endpoint unavailable'}, {status: 503}));
          }
          if (path === '${readinessPath}') {
            window.readinessRequests.push(path);
            return Promise.resolve(window.targetReady
              ? Response.json(${target === 'local client' ? "{status: 'ok'}" : '{authenticated: false}'})
              : Response.json({}, {status: 503}));
          }
          // A live client must not be mistaken for a restarted remote server.
          if (path === '/healthz') return Promise.resolve(Response.json({status: 'ok'}));
          return originalFetch(input, init);
        };
        Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Update and restart').click();
      }`)
      await waitFor(`document.getElementById('app-update-confirm-title')`)
      await evaluate(
        `Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Continue update').click()`
      )
      await waitFor(`window.updateRequests.some(r => r.path.endsWith('/install'))`)
      await evaluate(`window.updateDisconnected = true`)
      await waitFor(`document.body.textContent.includes('JavBoss is disconnected. Reconnecting')`)
      await waitFor(`window.readinessRequests.length > 0`)
      assert.equal(await evaluate('performance.timeOrigin'), started)
      assert.ok(
        await evaluate(
          `Array.from(document.querySelectorAll('button')).some(b => b.textContent === 'Refresh page' && !b.disabled)`
        )
      )
      await evaluate(`window.targetReady = true`)
      await waitFor(`performance.timeOrigin !== ${started}`)
      await waitFor(`document.querySelector('aside button[aria-label="Settings"]')`)
      assert.equal(
        await evaluate(
          `document.body.textContent.includes('JavBoss is disconnected. Reconnecting')`
        ),
        false
      )
      assert.deepEqual(await evaluate('window.appErrors'), [])
    }
  )
}

test(
  'a stalled update status request times out and probes the restarted service',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { evaluate, waitFor } = await setup(t)
    const started = await evaluate('performance.timeOrigin')
    await evaluate(`{
      window.stallUpdate = false;
      const originalTimeout = AbortSignal.timeout;
      AbortSignal.timeout = () => originalTimeout(100);
      const originalFetch = window.fetch;
      window.fetch = (input, init = {}) => {
        if (window.stallUpdate && new URL(input, location.origin).pathname === '/system/update') {
          return new Promise((resolve, reject) => {
            init.signal.addEventListener('abort', () => reject(init.signal.reason), {once: true});
          });
        }
        return originalFetch(input, init);
      };
      Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Update and restart').click();
    }`)
    await waitFor(`document.getElementById('app-update-confirm-title')`)
    await evaluate(
      `Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Continue update').click()`
    )
    await waitFor(`window.updateRequests.some(r => r.path.endsWith('/install'))`)
    await evaluate(`window.stallUpdate = true`)
    await waitFor(`performance.timeOrigin !== ${started}`)
    await waitFor(`document.querySelector('aside button[aria-label="Settings"]')`)
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
