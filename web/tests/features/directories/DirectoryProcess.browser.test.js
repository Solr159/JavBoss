import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'all directory tools configure before running and send explicit NFO overwrite choices',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/directoryCleanup.html` })
    await waitFor(`document.querySelector('button[aria-label="Tools"]')`)
    const modal = `document.querySelector('[aria-labelledby="directory-tools-title"]')`
    const button = (label) =>
      `[...${modal}.querySelectorAll('button')].find(b => b.textContent === ${JSON.stringify(label)})`
    const overwrite = `${modal}.querySelector('input[type="checkbox"]')`
    const folder = `document.querySelector('#directory-process-target-folder')`
    const setFolder = async (value) => {
      await evaluate(`
        Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(${folder}, ${JSON.stringify(value)});
        ${folder}.dispatchEvent(new Event('input', { bubbles: true }));
      `)
    }
    for (const [mode, layout, optIn, targetFolder] of [
      ['sidecar', 'prefix', false, 'JAV'],
      ['sidecar', 'prefix', true, 'JAV'],
      ['organize', 'code', false, '媒体库'],
      ['organize_with_sidecar', 'idol', true, 'My Videos'],
      ['organize', 'prefix', false, ''],
    ]) {
      const before = await evaluate('window.cleanupRequests.length')
      await evaluate(`document.querySelector('button[aria-label="Tools"]').click()`)
      await waitFor(modal)
      await evaluate(`document.querySelector('#directory-process-${mode}').click()`)
      assert.equal(
        await evaluate(`${modal}.querySelector('[name="directory-process-layout"]') === null`),
        true
      )
      assert.equal(await evaluate(`${overwrite} === null`), true)
      assert.equal(await evaluate(`${folder} === null`), true)
      assert.equal(await evaluate(`${button('Run')} === undefined`), true)
      await evaluate(`${button('Next')}.click()`)
      await waitFor(`${button('Run')}`)
      assert.equal(await evaluate('window.cleanupRequests.length'), before)
      if (mode === 'sidecar') {
        assert.equal(await evaluate(`${folder} === null`), true)
        assert.equal(
          await evaluate(`${modal}.querySelector('[name="directory-process-layout"]') === null`),
          true
        )
      } else {
        assert.equal(await evaluate(`${folder}.value`), 'JAV')
        for (const invalid of ['../outside', 'a\\b', 'a|b', 'a?b']) {
          await setFolder(invalid)
          assert.equal(await evaluate(`${folder}.checkValidity()`), false)
          await evaluate(`${button('Run')}.click()`)
          assert.equal(await evaluate('window.cleanupRequests.length'), before)
        }
        await setFolder(targetFolder)
        assert.equal(await evaluate(`${folder}.checkValidity()`), true)
        assert.equal(
          await evaluate(
            `${modal}.textContent.includes(${JSON.stringify(`selected directory/${targetFolder || 'JAV'}`)})`
          ),
          true
        )
        await evaluate(`document.querySelector('#directory-process-layout-${layout}').click()`)
      }
      if (mode === 'organize') {
        assert.equal(await evaluate(`${overwrite} === null`), true)
      } else {
        assert.equal(await evaluate(`${overwrite}.checked`), false)
        if (optIn) await evaluate(`${overwrite}.click()`)
      }
      await evaluate(`${button('Back')}.click()`)
      await waitFor(`${button('Next')}`)
      assert.equal(await evaluate('window.cleanupRequests.length'), before)
      await evaluate(`${button('Next')}.click()`)
      await evaluate(`${button('Run')}.click()`)
      await waitFor(`window.cleanupRequests.length === ${before + 1} && !${modal}`)
      assert.deepEqual(await evaluate('window.cleanupRequests.at(-1)'), {
        path: '/directories/1/process',
        payload: { mode, layout, overwrite_nfo: optIn, target_folder: targetFolder || 'JAV' },
      })
    }
  }
)
