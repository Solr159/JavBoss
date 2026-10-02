import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from './helpers/browser.js'

test(
  'application initializes and switches list pages without losing detail navigation',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(`window.requests?.some(r => r.url.startsWith('/videos?'))`)
    await waitFor(`document.querySelector('main')`)
    await waitFor(`document.title === 'Videos'`)
    assert.deepEqual(await evaluate('window.appErrors'), [])
    const display = `document.querySelector('aside button[aria-label="Display"]')`
    const settings = `document.querySelector('[role="dialog"][aria-label="Video Settings"]')`
    await evaluate(`${display}.click()`)
    await waitFor(settings)
    await evaluate(`{
      const input = ${settings}.querySelector('input[type="number"]');
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(input, '40');
      input.dispatchEvent(new Event('input', {bubbles: true}));
    }`)
    await evaluate(
      `Array.from(${settings}.querySelectorAll('button')).find(b => b.textContent === 'Cancel').click()`
    )
    assert.equal(await evaluate('window.testStore.getState().pageSize'), 25)
    await evaluate(`${display}.click()`)
    await waitFor(settings)
    assert.equal(await evaluate(`${settings}.querySelector('input[type="number"]').value`), '25')
    await evaluate(`{
      const input = ${settings}.querySelector('input[type="number"]');
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(input, '40');
      input.dispatchEvent(new Event('input', {bubbles: true}));
    }`)
    await evaluate(
      `Array.from(${settings}.querySelectorAll('button')).find(b => b.textContent === 'Save').click()`
    )
    await waitFor(`!${settings} && window.testStore.getState().pageSize === 40`)
    await evaluate(`document.querySelector('aside button[aria-label="JAV"]').click()`)
    await waitFor(`document.querySelector('.jav-card button')`)
    await waitFor(`document.title === 'JAV'`)
    await evaluate(`document.querySelector('.jav-card button').click()`)
    await waitFor(`document.querySelector('[aria-labelledby="jav-detail-title-1"]')`)
    await waitFor(`document.title === 'ABC-001 Test JAV'`)
    assert.ok((await evaluate('location.search')).includes('jav_detail=1'))
    await evaluate('history.back()')
    await waitFor(`!document.querySelector('[aria-labelledby="jav-detail-title-1"]')`)
    await waitFor(`document.title === 'JAV'`)
    for (const [tab, label] of [
      ['idol', 'Idols'],
      ['studio', 'Studios'],
      ['series', 'Series'],
      ['list', 'JAV'],
    ]) {
      await evaluate(`document.querySelector('aside button[aria-label="${label}"]').click()`)
      await waitFor(
        `window.testStore.getState().javTab === '${tab}' && document.querySelector('main')`
      )
      await waitFor(`document.title === '${label}'`)
    }
    await evaluate(
      `document.activeElement.blur(); window.dispatchEvent(new KeyboardEvent('keydown', {key: ' ', code: 'Space', bubbles: true}))`
    )
    await waitFor(`document.querySelector('[role="dialog"]')`)
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
