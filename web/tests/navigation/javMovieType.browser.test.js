import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../helpers/browser.js'

test(
  'movie type is single-select, shares the rating row and survives reload',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor, reloadPage } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-movie-type-test',
    })
    await command('Emulation.setDeviceMetricsOverride', {
      width: 1200,
      height: 900,
      deviceScaleFactor: 1,
      mobile: false,
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav` })
    const openEditor = async () => {
      await waitFor(`document.querySelector('button[aria-label="Edit filters"]')`)
      await evaluate(`document.querySelector('button[aria-label="Edit filters"]').click()`)
      await waitFor(`document.querySelector('button[name="movie_type"]')`)
    }
    const apply = async () =>
      evaluate(
        `[...document.querySelectorAll('.jav-query-editor-modal button')].find(b => b.textContent.trim() === 'Apply Query').click()`
      )
    const selected = () =>
      evaluate(
        `[...document.querySelectorAll('button[name="movie_type"][aria-pressed="true"]')].map(i => i.value)`
      )
    await openEditor()
    assert.deepEqual(await selected(), [])
    assert.equal(
      await evaluate(
        `(() => { const type = document.querySelector('button[name="movie_type"]').closest('fieldset').getBoundingClientRect(); const rating = document.querySelector('.jav-query-editor-modal .favorite-rating-filter').getBoundingClientRect(); return type.right <= rating.left && Math.abs((type.top+type.bottom)/2-(rating.top+rating.bottom)/2) < 2 })()`
      ),
      true
    )
    await evaluate(`document.querySelector('button[name="movie_type"][value="censored"]').click()`)
    assert.deepEqual(await selected(), ['censored'])
    await evaluate(`document.querySelector('button[name="movie_type"][value="unknown"]').click()`)
    assert.deepEqual(await selected(), ['unknown'])
    await waitFor(
      `window.requests.some(r => r.url.startsWith('/jav/filter-options?') && new URL(r.url, location.origin).searchParams.get('movie_type') === 'unknown')`
    )
    await apply()
    await waitFor(
      `new URLSearchParams(location.search).get('movie_type') === 'unknown' && window.requests.some(r => r.url.startsWith('/jav?') && new URL(r.url, location.origin).searchParams.get('movie_type') === 'unknown')`
    )
    await reloadPage()
    await openEditor()
    assert.deepEqual(await selected(), ['unknown'])
    await evaluate(`document.querySelector('button[name="movie_type"][value="unknown"]').click()`)
    assert.deepEqual(await selected(), [])
    await evaluate(
      `document.querySelector('button[name="movie_type"][value="uncensored"]').focus()`
    )
    await waitFor(`document.activeElement?.name === 'movie_type'`)

    for (const key of ['Enter', ' ']) {
      const keyCode = key === 'Enter' ? 13 : 32
      await command('Input.dispatchKeyEvent', {
        type: 'keyDown',
        text: key === 'Enter' ? '\r' : ' ',
        key,
        code: key === 'Enter' ? 'Enter' : 'Space',
        windowsVirtualKeyCode: keyCode,
      })
      await command('Input.dispatchKeyEvent', {
        type: 'keyUp',
        key,
        code: key === 'Enter' ? 'Enter' : 'Space',
        windowsVirtualKeyCode: keyCode,
      })
      await waitFor(
        `document.querySelector('button[name="movie_type"][value="uncensored"]').getAttribute('aria-pressed') === '${key === 'Enter' ? 'true' : 'false'}'`
      )
      assert.deepEqual(await selected(), key === 'Enter' ? ['uncensored'] : [])
    }
    await apply()
    await waitFor(
      `!new URLSearchParams(location.search).has('movie_type') && !window.testStore.getState().javMovieType`
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
