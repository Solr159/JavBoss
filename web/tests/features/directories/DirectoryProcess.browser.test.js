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
    for (const [mode, layout, optIn] of [
      ['sidecar', 'prefix', false],
      ['sidecar', 'prefix', true],
      ['organize', 'code', false],
      ['organize_with_sidecar', 'idol', true],
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
      assert.equal(await evaluate(`${button('Run')} === undefined`), true)
      await evaluate(`${button('Next')}.click()`)
      await waitFor(`${button('Run')}`)
      assert.equal(await evaluate('window.cleanupRequests.length'), before)
      if (mode === 'sidecar') {
        assert.equal(
          await evaluate(`${modal}.querySelector('[name="directory-process-layout"]') === null`),
          true
        )
      } else {
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
        payload: { mode, layout, overwrite_nfo: optIn },
      })
    }
  }
)
