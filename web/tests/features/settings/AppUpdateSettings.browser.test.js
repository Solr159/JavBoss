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
    `Array.from(document.querySelectorAll('aside button')).find(button => button.textContent === 'Check for Updates').click()`
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
  'Docker update opens Compose instructions without requesting an installation',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { evaluate, waitFor } = await setup(t)
    await evaluate(`{
      window.updateStatus.can_install = false;
      window.updateStatus.disabled_reason = 'container';
      Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Check for updates').click();
    }`)
    await waitFor(`document.body.textContent.includes('Update Docker by pulling')`)
    const click = (text) =>
      evaluate(
        `Array.from(document.querySelectorAll('button')).find(button => button.textContent === '${text}').click()`
      )
    assert.equal(
      await evaluate(
        `Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Update and restart').disabled`
      ),
      false
    )
    await click('Update and restart')
    await waitFor(`document.getElementById('app-update-confirm-title')`)
    const content = await evaluate(
      `document.querySelector('[aria-labelledby="app-update-confirm-title"]').textContent`
    )
    assert.match(content, /Docker update instructions/)
    assert.match(content, /compose.yaml/)
    assert.equal(
      await evaluate(
        `document.getElementById('app-update-confirm-description').querySelector('code').textContent`
      ),
      'docker compose pull\ndocker compose up -d'
    )
    assert.doesNotMatch(content, /Continue update/)
    await click('Got it')
    await waitFor(`!document.getElementById('app-update-confirm-title')`)
    assert.equal(
      await evaluate(`window.updateRequests.some(r => r.path.endsWith('/install'))`),
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

test(
  'release notes preserve formatting, sanitize HTML and fall back to plain text',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { evaluate, waitFor } = await setup(t)
    const markup = `<h1>Release heading</h1><h2>Features</h2>
      <p><strong>Bold text</strong> and <em>emphasis</em></p>
      <ul><li>First item</li></ul><ol><li>Numbered item</li></ol>
      <pre><code>line one\nline two</code></pre><blockquote>A quotation</blockquote>
      <table><tbody><tr><th>Platform</th><td>Windows</td></tr></tbody></table>
      <p><a href="https://github.com/Solr159/JavBoss/releases/tag/v2.0.0">Release link</a>
      <a href="javascript:window.notesXSS=1">Unsafe link</a></p>
      <img src="javascript:alert(1)" onerror="window.notesXSS=1" alt="Unsafe image">
      <script>window.notesXSS=1</script><svg onload="window.notesXSS=1"></svg>
      <iframe srcdoc="<script>parent.notesXSS=1</script>"></iframe>
      <p style="position:fixed" class="fixed" id="spoofed">Safe text</p>`
    await evaluate(`{
      window.notesXSS = 0;
      window.updateStatus.notes = 'Plain notes';
      window.updateStatus.notes_html = ${JSON.stringify(markup)};
      Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Check for updates').click();
    }`)
    await waitFor(`document.querySelector('.release-notes h1')`)
    const result = await evaluate(`(() => {
      const root = document.querySelector('.release-notes');
      const link = root.querySelector('a');
      return {
        title: root.previousElementSibling.textContent,
        headingSize: parseFloat(getComputedStyle(root.querySelector('h1')).fontSize),
        bodySize: parseFloat(getComputedStyle(root).fontSize),
        weight: getComputedStyle(root.querySelector('strong')).fontWeight,
        listStyle: getComputedStyle(root.querySelector('ul')).listStyleType,
        orderedStyle: getComputedStyle(root.querySelector('ol')).listStyleType,
        code: root.querySelector('pre code').textContent,
        target: link.target, rel: link.rel,
        unsafeLink: root.querySelectorAll('a')[1].hasAttribute('href'),
        unsafeElements: root.querySelectorAll('script, svg, iframe, [onerror], [onload], [style], [class], [id]').length,
        unsafeImage: root.querySelector('img').hasAttribute('src'),
        xss: window.notesXSS
      };
    })()`)
    assert.equal(result.title, 'v2.0.0')
    assert.ok(result.headingSize > result.bodySize)
    assert.ok(Number(result.weight) >= 600)
    assert.equal(result.listStyle, 'disc')
    assert.equal(result.orderedStyle, 'decimal')
    assert.equal(result.code, 'line one\nline two')
    assert.equal(result.target, '_blank')
    assert.match(result.rel, /noopener/)
    assert.match(result.rel, /noreferrer/)
    assert.equal(result.unsafeLink, false)
    assert.equal(result.unsafeImage, false)
    assert.equal(result.unsafeElements, 0)
    assert.equal(result.xss, 0)

    await evaluate(`{
      window.updateStatus.notes_html = '';
      window.updateStatus.notes = 'Legacy <b>plain text</b>';
      Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Check for updates').click();
    }`)
    await waitFor(
      `!document.querySelector('.release-notes') && document.body.textContent.includes('Legacy <b>plain text</b>')`
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
