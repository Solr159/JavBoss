import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../helpers/browser.js'

test(
  'actor count multi-select applies, restores and clears across navigation',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor, reloadPage } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-actor-counts-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav&solo=1` })
    const openEditor = async () => {
      await waitFor(`document.querySelector('button[aria-label="Edit filters"]')`)
      await evaluate(`document.querySelector('button[aria-label="Edit filters"]').click()`)
      await waitFor(`document.querySelector('input[name="actor_counts"]')`)
    }
    const clickButton = async (label) =>
      evaluate(
        `[...document.querySelectorAll('button')].find(b => b.textContent.trim() === ${JSON.stringify(label)}).click()`
      )
    await openEditor()
    assert.deepEqual(
      await evaluate(
        `[...document.querySelectorAll('input[name="actor_counts"]:checked')].map(i => i.value)`
      ),
      ['1']
    )
    await evaluate(`document.querySelector('input[name="actor_counts"][value="0"]').click()`)
    await waitFor(
      `window.requests.some(r => r.url.startsWith('/jav/filter-options?') && new URL(r.url, location.origin).searchParams.get('actor_counts') === '0,1')`
    )
    await clickButton('Apply Query')
    await waitFor(
      `new URLSearchParams(location.search).get('actor_counts') === '0,1' && window.requests.some(r => r.url.startsWith('/jav?') && new URL(r.url, location.origin).searchParams.get('actor_counts') === '0,1')`
    )
    await reloadPage()
    await openEditor()
    assert.deepEqual(
      await evaluate(
        `[...document.querySelectorAll('input[name="actor_counts"]:checked')].map(i => i.value)`
      ),
      ['0', '1']
    )
    await clickButton('Clear Filters')
    assert.equal(
      await evaluate(`document.querySelectorAll('input[name="actor_counts"]:checked').length`),
      0
    )
    await clickButton('Apply Query')
    await waitFor(
      `!new URLSearchParams(location.search).has('actor_counts') && !window.testStore.getState().javActorCounts.length`
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
