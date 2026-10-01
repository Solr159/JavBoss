import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from './helpers/browser.js'

const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

test(
  'studio detail history restores the modal, sorting, scroll and direct links',
  {
    skip: browserUnavailable,
    timeout: 60000,
  },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-studio-history-test',
    })
    const reloadPage = async () => {
      const previousTimeOrigin = await evaluate('performance.timeOrigin')
      await command('Page.reload')
      // The CDP reply can arrive while the previous document is still visible.
      await waitFor(`performance.timeOrigin !== ${previousTimeOrigin}`)
    }
    const modal = `document.querySelector('[role="dialog"][aria-label="片商详情"], [role="dialog"][aria-label="Studio details"]')`
    const loaded = `${modal}?.querySelectorAll('section').length === 2`
    const scroll = `${modal}.querySelector('.overflow-y-auto')`
    const fixture = `${origin}/tests/fixtures/studioDetailNavigation.html`
    await command('Page.navigate', { url: `${fixture}?view=jav&tab=studio&page=2` })
    await waitFor(`document.querySelector('[aria-haspopup="dialog"]')`)
    await evaluate(
      `window.holdStudioResponses = true; window.scrollTo(0, 420); document.querySelector('[aria-haspopup="dialog"]').click()`
    )
    await waitFor(loaded)
    assert.ok((await evaluate('location.search')).includes('studio_detail=1'))
    assert.ok(await evaluate('window.pendingStudioResponses.length > 0'))
    assert.equal(await evaluate(`${modal}.querySelector('a').textContent`), 'Test studio')

    await evaluate(
      `${modal}.querySelectorAll('section')[0].querySelector('button').click(); ${modal}.querySelectorAll('section')[1].querySelector('button').click(); ${scroll}.scrollTop = 350`
    )
    await waitFor(`history.state.usr.__javbossStudioDetail?.scrollTop === 350`)
    await evaluate('window.releaseStudioResponses()')
    await waitFor(`${modal}.querySelector('a').textContent === 'Updated studio'`)
    assert.equal(
      await evaluate(`${scroll}.scrollTop`),
      350,
      'background refresh preserves current scroll'
    )

    // The first series after switching to name sort is Series 1.
    await evaluate(`${modal}.querySelector('section:nth-child(2) a').click()`)
    await waitFor(`!${modal} && location.search.includes('series_id=1')`)
    await evaluate('window.holdStudioResponses = true; history.back()')
    await waitFor(loaded)
    assert.equal(await evaluate(`${scroll}.scrollTop`), 350)
    assert.equal(await evaluate(`${modal}.querySelector('a').textContent`), 'Updated studio')
    await waitFor('window.pendingStudioResponses.length > 0')
    await evaluate('window.releaseStudioResponses(true)')
    await pause(100)
    assert.ok(
      await evaluate(`Boolean(${loaded})`),
      'failed background refresh keeps cached content visible'
    )

    assert.equal(
      await evaluate('history.state.usr.__javbossStudioDetail.prefixSort'),
      'work_count_desc'
    )
    assert.equal(await evaluate('history.state.usr.__javbossStudioDetail.seriesSort'), 'name_asc')
    assert.equal(await evaluate('Math.round(window.scrollY)'), 420)
    await evaluate('history.forward()')
    await waitFor(`!${modal} && location.search.includes('series_id=1')`)
    await evaluate('history.back()')
    await waitFor(loaded)
    // A reload uses only the ID and history state, without needing the original card.
    await reloadPage()
    await waitFor(loaded)
    assert.equal(await evaluate(`${scroll}.scrollTop`), 350)
    await evaluate(`${modal}.querySelector('section:nth-child(2) a button').click()`)
    await waitFor(`document.querySelectorAll('[role="dialog"]').length === 2`)
    await evaluate(
      `document.querySelector('[aria-label="关闭收藏夹选择"], [aria-label="Close favorite group picker"]').click()`
    )
    await waitFor(`document.querySelectorAll('[role="dialog"]').length === 1`)
    await evaluate(`${modal}.querySelector('button').click()`)
    await waitFor(`!${modal} && location.search.includes('tab=studio')`)
    assert.equal(await evaluate('Math.round(window.scrollY)'), 420)
    // Direct links close in-place, without navigating to a previous website.
    await command('Page.navigate', { url: `${fixture}?view=jav&tab=studio&page=2&studio_detail=1` })
    await waitFor(loaded)
    await evaluate(`${modal}.querySelector('button').click()`)
    await waitFor(`!${modal} && !location.search.includes('studio_detail')`)
    assert.ok((await evaluate('location.href')).startsWith(fixture))
    // Navigating to the same underlying filters must still leave a history entry.
    await command('Page.navigate', {
      url: `${fixture}?view=jav&studio_id=1&studio_name=Test+studio&page=1`,
    })
    await waitFor(`document.querySelector('[aria-haspopup="dialog"]')`)
    await evaluate(`document.querySelector('[aria-haspopup="dialog"]').click()`)
    await waitFor(loaded)
    const detailLength = await evaluate('history.length')
    await evaluate(`${modal}.querySelector('a').click()`)
    await waitFor(`!${modal} && !location.search.includes('studio_detail')`)
    assert.equal(await evaluate('history.length'), detailLength + 1)
    await evaluate('history.back()')
    await waitFor(loaded)
    await command('Input.dispatchKeyEvent', {
      type: 'keyDown',
      key: 'Escape',
      code: 'Escape',
      windowsVirtualKeyCode: 27,
    })
    await waitFor(`!${modal}`)
    // Prefix navigation also restores the detail, and the backdrop closes only that entry.
    await evaluate(`document.querySelector('[aria-haspopup="dialog"]').click()`)
    await waitFor(loaded)
    await evaluate(`${modal}.querySelector('section .flex.flex-wrap.gap-2 button').click()`)
    await waitFor(`!${modal} && location.search.includes('prefix=ABC')`)
    await evaluate('history.back()')
    await waitFor(loaded)
    await evaluate(`document.querySelector('.MuiBackdrop-root').click()`)
    await waitFor(`!${modal}`)
    // Missing data and minimal direct URLs remain closable within the application.
    await command('Page.navigate', { url: `${fixture}?studio_detail=1` })
    await waitFor(loaded)
    await evaluate(`${modal}.querySelector('button').click()`)
    await waitFor(`!${modal} && !location.search.includes('studio_detail')`)
    await command('Page.navigate', {
      url: `${fixture}?view=jav&tab=studio&page=1&studio_detail=99`,
    })
    await waitFor(`${modal}?.querySelector('[role="alert"]')`)
    await evaluate(`${modal}.querySelector('button').click()`)
    await waitFor(`!${modal} && !location.search.includes('studio_detail')`)
  }
)
