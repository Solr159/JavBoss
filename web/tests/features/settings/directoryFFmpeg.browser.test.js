import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'directory management links missing macOS FFmpeg to Tools and reflects installation',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    const settings = `document.querySelector('aside button[aria-label="Settings"]')`
    const modal = `document.querySelector('[aria-labelledby="global-settings-title"]')`
    const button = (label) =>
      `[...${modal}.querySelectorAll('button')].find(b => b.textContent.includes('${label}'))`
    const notice = button('Go to Tools to download FFmpeg')
    await waitFor(`${settings} && window.testStore.getState().config.video_page_size`)
    await evaluate(`{
      window.setServerOS = runtime_os => window.testStore.setState(s => ({ config: { ...s.config, runtime_os } }));
      window.setServerOS('linux');
      const originalFetch = window.fetch;
      window.toolChecks = 0;
      window.toolStatus = { installed: false };
      window.fetch = (input, init) => {
        const path = new URL(input, location.origin).pathname;
        if (path === '/tools') {
          window.toolChecks++;
          return Promise.resolve(Response.json({ ffmpeg: window.toolStatus }));
        }
        if (path === '/tools/ffmpeg/download' && init.method === 'POST') {
          window.toolStatus = { installed: false, downloading: true };
          return Promise.resolve(Response.json(window.toolStatus));
        }
        return originalFetch(input, init);
      };
      ${settings}.click();
    }`)
    await waitFor(modal)
    assert.equal(await evaluate('window.toolChecks'), 0)
    assert.equal(await evaluate(`Boolean(${notice})`), false)
    await evaluate(`window.setServerOS('darwin')`)
    await waitFor(notice)
    await evaluate(`${notice}.click()`)
    const download = button('Download FFmpeg')
    await waitFor(`${download} && !${download}.disabled`)
    await evaluate(`${download}.click()`)
    await waitFor(button('Downloading'))
    await evaluate(`${button('Directory Management')}.click()`)
    await waitFor(`${modal}.textContent.includes('/videos')`)
    assert.equal(await evaluate(`Boolean(${notice})`), false)
    await evaluate(`window.toolStatus = { installed: true }; window.beforePoll = window.toolChecks`)
    await waitFor('window.toolChecks > window.beforePoll')
    assert.equal(await evaluate(`Boolean(${notice})`), false)
  }
)
