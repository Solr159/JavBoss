import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../helpers/browser.js'

test(
  'compact rating filter requires the heart toggle and preserves its selected range',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor, reloadPage } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-rating-range-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav` })
    const openEditor = async () => {
      await waitFor(`document.querySelector('button[aria-label="Edit filters"]')`)
      await evaluate(`document.querySelector('button[aria-label="Edit filters"]').click()`)
      await waitFor(
        `document.querySelector('.jav-query-editor-modal input[aria-label="Minimum favorite rating"]')`
      )
    }
    const apply = async () => {
      await evaluate(
        `[...document.querySelectorAll('button')].find(b => b.textContent.trim() === 'Apply Query').click()`
      )
      await waitFor(
        `!document.querySelector('.jav-query-editor-modal input[aria-label="Minimum favorite rating"]')`
      )
    }
    const changeMinimum = async (key, code) => {
      await evaluate(
        `document.querySelector('.jav-query-editor-modal input[aria-label="Minimum favorite rating"]').focus()`
      )
      await command('Input.dispatchKeyEvent', {
        type: 'keyDown',
        key,
        code: key,
        windowsVirtualKeyCode: code,
      })
      await command('Input.dispatchKeyEvent', {
        type: 'keyUp',
        key,
        code: key,
        windowsVirtualKeyCode: code,
      })
    }
    await openEditor()
    assert.deepEqual(
      await evaluate(`(() => {
      const input = document.querySelector('.jav-query-editor-modal input[aria-label="Minimum favorite rating"]');
      const row = input.closest('section');
      const bounds = row.getBoundingClientRect();
      const slider = row.lastElementChild.getBoundingClientRect();
      return { checkboxes: row.querySelectorAll('input[type="checkbox"]').length, rightAligned: Math.abs(bounds.right - slider.right) < 1 };
    })()`),
      { checkboxes: 0, rightAligned: true }
    )
    await apply()
    assert.equal(await evaluate('window.testStore.getState().javFavoriteRatingEnabled'), false)
    await openEditor()
    assert.equal(
      await evaluate(
        `document.querySelector('.jav-query-editor-modal input[aria-label="Minimum favorite rating"]').disabled`
      ),
      true
    )
    await evaluate(
      `document.querySelector('.jav-query-editor-modal button[aria-label="Enable favorite rating filter"]').click()`
    )
    await waitFor(
      `!document.querySelector('.jav-query-editor-modal input[aria-label="Minimum favorite rating"]').disabled`
    )
    await changeMinimum('ArrowRight', 39)
    await waitFor(
      `document.querySelector('.jav-query-editor-modal input[aria-label="Minimum favorite rating"]').value === '1'`
    )
    await apply()
    await waitFor(`new URLSearchParams(location.search).get('favorite_rating_min') === '1'`)
    await reloadPage()
    await openEditor()
    assert.equal(
      await evaluate(
        `document.querySelector('.jav-query-editor-modal input[aria-label="Minimum favorite rating"]').value`
      ),
      '1'
    )
    await evaluate(
      `document.querySelector('.jav-query-editor-modal button[aria-label="Disable favorite rating filter"]').click()`
    )
    assert.equal(
      await evaluate(
        `document.querySelector('.jav-query-editor-modal input[aria-label="Minimum favorite rating"]').disabled`
      ),
      true
    )
    assert.equal(
      await evaluate(
        `document.querySelector('.jav-query-editor-modal input[aria-label="Minimum favorite rating"]').value`
      ),
      '1'
    )
    await evaluate(
      `document.querySelector('.jav-query-editor-modal button[aria-label="Enable favorite rating filter"]').click()`
    )
    await changeMinimum('ArrowLeft', 37)
    await waitFor(
      `document.querySelector('.jav-query-editor-modal input[aria-label="Minimum favorite rating"]').value === '0.5'`
    )
    assert.equal(
      await evaluate(
        `document.querySelector('.jav-query-editor-modal button[aria-label="Disable favorite rating filter"]').getAttribute('aria-pressed')`
      ),
      'true'
    )
    await evaluate(
      `document.querySelector('.jav-query-editor-modal button[aria-label="Disable favorite rating filter"]').click()`
    )
    await apply()
    await waitFor(
      `!new URLSearchParams(location.search).has('favorite_rating_min') && !window.testStore.getState().javFavoriteRatingEnabled`
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
