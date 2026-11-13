import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'subtitle settings sit below default player settings, default off and save language preference',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(`document.querySelector('aside button[aria-label="Settings"]')`)
    await evaluate(`document.querySelector('aside button[aria-label="Settings"]').click()`)
    const modal = `document.querySelector('[aria-labelledby="global-settings-title"]')`
    await waitFor(modal)
    await evaluate(
      `[...${modal}.querySelectorAll('button')].find(b => b.textContent.includes('Player')).click()`
    )
    const checkbox = `${modal}.querySelector('input[name="enabled"]')`
    await waitFor(checkbox)
    assert.equal(
      await evaluate(
        `[...${modal}.querySelectorAll('button')].some(b => b.textContent === 'Subtitle Settings')`
      ),
      false
    )
    assert.equal(
      await evaluate(`(() => {
      const form = ${checkbox}.form;
      const settings = form.previousElementSibling;
      return settings?.tagName === 'SECTION' && settings.textContent.includes('Default Player') && settings.getBoundingClientRect().bottom <= form.getBoundingClientRect().top;
    })()`),
      true
    )
    assert.equal(await evaluate(`${checkbox}.checked`), false)
    assert.equal(await evaluate(`${checkbox}.getAttribute('role')`), 'switch')
    const language = `${checkbox}.form.querySelector('select[name="language"]')`
    assert.equal(await evaluate(`${language}.getClientRects().length`), 0)
    await evaluate(`${checkbox}.click()`)
    await waitFor(`${language}.getClientRects().length > 0`)
    await evaluate(`{
      const checkbox = ${checkbox};
      checkbox.form.querySelector('select').value = 'zh-TW';
      checkbox.form.querySelector('button[type="submit"]').click();
    }`)
    await waitFor(`window.testStore.getState().config.subtitle_auto_download === true`)
    assert.equal(await evaluate(`window.testStore.getState().config.subtitle_language`), 'zh-TW')
    await waitFor(`!${checkbox}.disabled`)
    await evaluate(`${checkbox}.click()`)
    assert.equal(await evaluate(`${language}.getClientRects().length`), 0)
    await evaluate(`${checkbox}.form.querySelector('button[type="submit"]').click()`)
    await waitFor(`window.testStore.getState().config.subtitle_auto_download === false`)
    assert.equal(await evaluate(`window.testStore.getState().config.subtitle_language`), 'zh-TW')
    await waitFor(`!${checkbox}.disabled`)
    await evaluate(`${checkbox}.click()`)
    await waitFor(`${language}.getClientRects().length > 0`)
    assert.equal(await evaluate(`${language}.value`), 'zh-TW')
    assert.deepEqual(await evaluate(`window.appErrors`), [])
  }
)
