import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'adding and manually scanning directories check for FFmpeg on macOS',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    const settings = `document.querySelector('aside button[aria-label="Settings"]')`
    const modal = `document.querySelector('[aria-labelledby="global-settings-title"]')`
    const button = (label) =>
      `[...${modal}.querySelectorAll('button')].find(b => b.textContent.includes('${label}'))`
    const prompt = `document.querySelector('[aria-labelledby="directory-ffmpeg-title"]')`
    const promptButton = (label) =>
      `[...${prompt}.querySelectorAll('button')].find(b => b.textContent.includes('${label}'))`
    const add = button('Add Directory')
    const notice = `${modal}.querySelector('p button.underline')`
    const download = button('Download FFmpeg')
    const scan = `${modal}.querySelector('button[aria-label^="Manual scan"]')`
    const pathInput = `document.querySelector('#dir-path-input')`
    await waitFor(`${settings} && window.testStore.getState().config.video_page_size`)
    await evaluate(`{
      window.setServerOS = runtime_os => window.testStore.setState(s => ({ config: { ...s.config, runtime_os } }));
      window.setServerOS('linux');
      const originalFetch = window.fetch;
      window.toolChecks = 0;
      window.scanRequests = 0;
      window.toolStatus = { installed: false };
      window.fetch = (input, init) => {
        const path = new URL(input, location.origin).pathname;
        if (path === '/directories/1/scan' && init.method === 'POST') {
          window.scanRequests++;
          return Promise.resolve(Response.json({}));
        }
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
    assert.equal(await evaluate(`Boolean(${prompt})`), false)
    await evaluate(`${add}.click()`)
    await waitFor(pathInput)
    await evaluate(`${button('Cancel')}.click()`)
    await evaluate(`${scan}.click()`)
    await waitFor('window.scanRequests === 1')
    await evaluate(`window.setServerOS('darwin')`)
    await waitFor(notice)
    assert.equal(await evaluate(`${notice}.textContent`), 'Tools')
    await evaluate(`${notice}.click()`)
    await waitFor(download)
    await evaluate(`${button('Directory Management')}.click()`)
    await waitFor(notice)
    await evaluate(`${add}.click()`)
    await waitFor(`${prompt}?.textContent.includes('FFmpeg is not installed')`)
    assert.equal(await evaluate(`Boolean(${pathInput})`), false)
    await evaluate(`${promptButton('Cancel')}.click()`)
    await waitFor(`!${prompt} && ${add}`)
    await evaluate(`${scan}.click()`)
    await waitFor(prompt)
    assert.equal(await evaluate('window.scanRequests'), 1)
    await evaluate(`${promptButton('Go to download')}.click()`)
    await waitFor(`${download} && !${download}.disabled`)
    await evaluate(`${download}.click()`)
    await waitFor(button('Downloading'))
    await evaluate(`${button('Directory Management')}.click()`)
    await waitFor(`${modal}.textContent.includes('/videos')`)
    assert.equal(await evaluate(`Boolean(${prompt})`), false)
    await evaluate(`${scan}.click()`)
    await waitFor(`${prompt}?.textContent.includes('Downloading FFmpeg')`)
    assert.equal(await evaluate('window.scanRequests'), 1)
    assert.equal(await evaluate(`Boolean(${pathInput})`), false)
    await evaluate(`${promptButton('View progress')}.click()`)
    await waitFor(button('Downloading'))
    await evaluate(`window.toolStatus = { installed: true }`)
    await waitFor(button('Installed'))
    await evaluate(`${button('Directory Management')}.click()`)
    assert.equal(await evaluate(`Boolean(${prompt})`), false)
    await evaluate(`${scan}.click()`)
    await waitFor('window.scanRequests === 2')
    assert.equal(await evaluate(`Boolean(${notice})`), false)
    await evaluate(`${add}.click()`)
    await waitFor(pathInput)
  }
)
