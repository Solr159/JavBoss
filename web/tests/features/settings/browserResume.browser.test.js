import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'web player resume setting defaults on, saves independently of MPV, and restores defaults',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(`document.querySelector('aside button[aria-label="Settings"]')`)
    await evaluate(`document.querySelector('aside button[aria-label="Settings"]').click()`)
    const modal = `document.querySelector('[aria-labelledby="global-settings-title"]')`
    const button = (text) =>
      `[...${modal}.querySelectorAll('button')].find(b => b.textContent.includes('${text}'))`
    await waitFor(modal)
    await evaluate(`${button('Player')}.click()`)
    await evaluate(`${button('Web Player')}.click()`)
    const checkbox = `[...${modal}.querySelectorAll('label')].find(label => label.textContent.includes('Resume From Last Position')).querySelector('input')`
    await waitFor(checkbox)
    assert.equal(await evaluate(`${checkbox}.checked`), true)
    await evaluate(`${checkbox}.click(); ${button('Save')}.click()`)
    await waitFor(`window.testStore.getState().config.browser_player_resume_playback === false`)
    assert.equal(
      await evaluate(`window.testStore.getState().config.player_resume_playback`),
      undefined
    )
    await evaluate(`${button('Restore Defaults')}.click()`)
    await waitFor(`${checkbox}.checked`)
    await evaluate(`${button('Save')}.click()`)
    await waitFor(`window.testStore.getState().config.browser_player_resume_playback === true`)
  }
)
