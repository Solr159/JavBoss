import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

const openStudio = `[...document.querySelectorAll('button')].find(button => button.textContent === 'Open studio').click()`

test(
  'opening studio details translates all missing series and updates the modal and cache',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/javStudioSeriesTranslation.html`,
    })
    await waitFor(`document.querySelector('button')`)
    assert.deepEqual(await evaluate('window.translationRequests'), [])
    await evaluate(openStudio)
    await waitFor(`window.cachedStudio?.series[22].zh_name === '自动译名 23'`)
    assert.deepEqual(await evaluate('window.translationRequests.map(ids => ids.length)'), [20, 3])
    assert.equal(
      await evaluate(
        `document.querySelector('[role="dialog"]').textContent.includes('自动译名 23')`
      ),
      true
    )
    assert.equal(await evaluate(`window.cachedStudio.series[23].zh_name`), '人工译名')
    assert.equal(await evaluate(`window.cachedStudio.series[0].name`), 'Original series 1')
    assert.equal(
      await evaluate(
        `decodeURIComponent(document.querySelector('[role="dialog"] a[href*="series=1&"]').href).includes('自动译名 1')`
      ),
      true
    )
    await evaluate(`document.querySelector('[aria-label="Close studio details"]').click()`)
    await waitFor(`!document.querySelector('[role="dialog"]')`)
    await evaluate(openStudio)
    await waitFor(`document.querySelector('[role="dialog"]')?.textContent.includes('自动译名 23')`)
    assert.equal(await evaluate('window.translationRequests.length'), 2)
  }
)

test(
  'studio series translation respects both switches and allows retry after failure',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/javStudioSeriesTranslation.html?manual`,
    })
    await waitFor(`document.querySelector('button')`)
    await evaluate(openStudio)
    await waitFor(
      `document.querySelector('[role="dialog"]')?.textContent.includes('Original series 23')`
    )
    assert.deepEqual(await evaluate('window.translationRequests'), [])
    await evaluate(
      `window.testStore.setState({config:{...window.testStore.getState().config,jav_series_translation_enabled:'false',jav_series_translation_auto_enabled:'true'}})`
    )
    await waitFor(`!document.querySelector('[role="dialog"]').textContent.includes('人工译名')`)
    assert.deepEqual(await evaluate('window.translationRequests'), [])
    await evaluate(
      `window.translationFailure = true; window.testStore.setState({config:{...window.testStore.getState().config,jav_series_translation_enabled:'true'}})`
    )
    await waitFor(
      `document.querySelector('[role="alert"]')?.textContent.includes('Translation failed')`
    )
    assert.equal(await evaluate('window.translationRequests.length'), 1)
    await evaluate(
      `window.translationFailure = false; [...document.querySelectorAll('button')].find(button => button.textContent === 'Retry').click()`
    )
    await waitFor(`window.cachedStudio?.series[22].zh_name === '自动译名 23'`)
    assert.equal(await evaluate(`document.querySelector('[role="alert"]')`), null)
    assert.equal(await evaluate('window.translationRequests.length'), 3)
  }
)

test(
  'series editor receives pointer input above the studio detail dialog',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/javStudioSeriesTranslation.html?manual`,
    })
    await waitFor(`document.querySelector('button')`)
    await evaluate(openStudio)
    await waitFor(`document.querySelector('[aria-label="Edit series translation"]')`)
    await evaluate(`document.querySelector('[aria-label="Edit series translation"]').click()`)
    const editor = `document.querySelector('[role="dialog"][aria-label="Edit series translation"]')`
    await waitFor(editor)
    const click = async (expression) => {
      const point = await evaluate(`(() => {
        const element = ${expression};
        const rect = element.getBoundingClientRect();
        const x = rect.x + rect.width / 2, y = rect.y + rect.height / 2;
        return {x, y, receivesPointer: element.contains(document.elementFromPoint(x, y))};
      })()`)
      assert.equal(point.receivesPointer, true)
      for (const type of ['mousePressed', 'mouseReleased']) {
        await command('Input.dispatchMouseEvent', {
          type,
          x: point.x,
          y: point.y,
          button: 'left',
          clickCount: 1,
        })
      }
    }
    await click(`${editor}.querySelector('textarea')`)
    await command('Input.insertText', { text: '人工校对系列' })
    await waitFor(`${editor}.querySelector('textarea').value === '人工校对系列'`)
    await click(
      `[...${editor}.querySelectorAll('button')].find(button => button.textContent === 'Save')`
    )
    await waitFor(`!${editor}`)
    await waitFor(
      `document.querySelector('[aria-label="Studio details"]')?.textContent.includes('人工校对系列')`
    )
    assert.deepEqual(await evaluate('window.translationRequests'), [])
  }
)
