import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from './helpers/browser.js'

test(
  'JAV detail restores after filter links, nested studio details, reload and direct links',
  {
    skip: browserUnavailable,
    timeout: 60000,
  },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-jav-history-test',
    })
    const reloadPage = async () => {
      const previousTimeOrigin = await evaluate('performance.timeOrigin')
      await command('Page.reload')
      // The CDP reply can arrive while the previous document is still visible.
      await waitFor(`performance.timeOrigin !== ${previousTimeOrigin}`)
    }
    const modal = `document.querySelector('[role="dialog"][aria-labelledby="jav-detail-title-1"]')`
    const scroll = `${modal}.querySelector('.overflow-y-auto')`
    const fixture = `${origin}/tests/fixtures/javDetailNavigation.html`
    await command('Page.navigate', { url: `${fixture}?view=jav` })
    await waitFor(`document.querySelector('.jav-card button')`)
    const loadId = await evaluate('window.fixtureLoadId')
    await evaluate(`window.scrollTo(0,420); document.querySelector('.jav-card button').click()`)
    await waitFor(modal)
    await waitFor(`document.title === 'ABC-001 Test JAV'`)
    assert.ok((await evaluate('location.search')).includes('jav_detail=1'))
    assert.equal(
      await evaluate('window.javDetailRequests'),
      0,
      'opening a card uses its data immediately'
    )
    await evaluate(`${scroll}.scrollTop = 250`)
    await waitFor('history.state.usr.__javbossJavDetail?.scrollTop === 250')
    for (const [label, param] of [
      ['Test studio', 'studio_id=2'],
      ['Test series', 'series_id=3'],
      ['Test actress', 'idol_ids=4'],
      ['Tag 1', 'tag_ids=1'],
    ]) {
      await evaluate(
        `[...${modal}.querySelectorAll('a')].find(a => a.textContent.trim() === '${label}').click()`
      )
      await waitFor(`!${modal} && !location.search.includes('jav_detail=')`)
      assert.ok((await evaluate('location.search')).includes(param), `filter URL contains ${param}`)
      assert.equal(
        await evaluate('window.fixtureLoadId'),
        loadId,
        'internal links do not reload the page'
      )
      await evaluate('history.back()')
      await waitFor(modal)
      assert.equal(await evaluate(`${scroll}.scrollTop`), 250)
    }
    // Resolved samples must be available before restoring a position below the original content.
    await evaluate('window.releaseSampleImages()')
    await waitFor(`${scroll}.scrollHeight - ${scroll}.clientHeight > 1800`)
    await evaluate(`${scroll}.scrollTop = 1800`)
    await waitFor('history.state.usr.__javbossJavDetail?.scrollTop === 1800')
    await evaluate(
      `[...${modal}.querySelectorAll('a')].find(a => a.textContent.trim() === 'Test actress').click()`
    )
    await waitFor(`!${modal}`)
    await evaluate('history.back()')
    await waitFor(`${modal} && ${scroll}.scrollTop === 1800`)
    // A reload has no image cache; don't overwrite the saved position with the clamped value.
    await reloadPage()
    await waitFor(`${modal} && window.releaseSampleImages`)
    assert.equal(await evaluate('history.state.usr.__javbossJavDetail.scrollTop'), 1800)
    await evaluate('window.releaseSampleImages()')
    await waitFor(`${scroll}.scrollTop === 1800`)
    await evaluate(`${scroll}.scrollTop = 250`)
    await waitFor('history.state.usr.__javbossJavDetail?.scrollTop === 250')
    // A second detail layer must return to JAV details before closing to the list.
    await evaluate(`document.querySelector('#nested-studio').click()`)
    await waitFor('location.search.includes("studio_detail=2")')
    await waitFor(`document.title === 'Test studio'`)
    await evaluate('history.back()')
    await waitFor(`!location.search.includes('studio_detail=') && ${modal}`)
    await waitFor(`document.title === 'ABC-001 Test JAV'`)
    assert.equal(await evaluate(`${scroll}.scrollTop`), 250)
    await evaluate('history.forward()')
    await waitFor('location.search.includes("studio_detail=2")')
    await evaluate(`document.querySelector('[role="dialog"][aria-label] a').click()`)
    await waitFor(`!${modal} && location.search.includes('studio_id=2')`)
    await evaluate('history.back()')
    await waitFor('location.search.includes("studio_detail=2")')
    await evaluate(
      `document.querySelector('[role="dialog"][aria-label] button[aria-label]').click()`
    )
    await waitFor(`!location.search.includes('studio_detail=') && ${modal}`)
    await reloadPage()
    await waitFor(modal)
    assert.equal(await evaluate(`${scroll}.scrollTop`), 250)
    await evaluate(`${modal}.querySelector('button[aria-label]').click()`)
    await waitFor(`!${modal} && !location.search.includes('jav_detail=')`)
    await command('Page.navigate', { url: `${fixture}?view=jav&jav_detail=1` })
    await waitFor(modal)
    await waitFor(`document.title === 'ABC-001 Test JAV'`)
    assert.ok(
      await evaluate('window.javDetailRequests > 0'),
      'direct links load detail without requiring the item in the list'
    )
    await evaluate(`${modal}.querySelector('button[aria-label]').click()`)
    await waitFor(`!${modal} && !location.search.includes('jav_detail=')`)
    // Matching the existing filters still creates a result entry over the modal.
    await command('Page.navigate', { url: `${fixture}?view=jav&studio_id=2&jav_detail=1` })
    await waitFor(modal)
    await evaluate(
      `[...${modal}.querySelectorAll('a')].find(a => a.textContent.trim() === 'Test studio').click()`
    )
    await waitFor(`!${modal} && location.search.includes('studio_id=2')`)
    await evaluate('history.back()')
    await waitFor(modal)
    const star = `${modal}.querySelector(':is(section[aria-label="操作"], section[aria-label="Actions"]) button svg[data-testid="StarRoundedIcon"]')`
    await evaluate(
      `window.testStore.setState({javItems: []}); window.testStore.getState().patchJavFavoriteCount('jav', 1, [7], null)`
    )
    await waitFor(star)
    await waitFor('window.cachedDetail?.favorite_count === 1')
    await evaluate(
      `window.testStore.setState({javItems: [{...window.cachedDetail}], javTotal: 1}); window.testStore.getState().patchJavFavoriteCount('jav', 1, [], 7)`
    )
    await waitFor(`!${star} && window.cachedDetail?.favorite_count === 0`)
    assert.equal(
      await evaluate('window.testStore.getState().javItems.length'),
      0,
      'detail updates even after removal from its list'
    )
    await evaluate(`window.testStore.getState().patchJavFavoriteCount('jav', 1, [7], null)`)
    await waitFor(star)
    await evaluate(`window.testStore.getState().invalidateJavFavoriteCounts('jav')`)
    await waitFor(`!${star} && window.cachedDetail?.favorite_count === 0`)
    const seriesStar = `document.querySelector('#favorite-series svg[data-testid="StarRoundedIcon"]')`
    await evaluate(`window.testStore.getState().patchJavFavoriteCount('series', 3, [7], null)`)
    await waitFor(seriesStar)
    await evaluate(`window.testStore.getState().invalidateJavFavoriteCounts('series')`)
    await waitFor(`!${seriesStar}`)
    // Removing one group must retain the star when another membership remains.
    await evaluate(
      `window.favoriteSelections.series = [9]; window.testStore.getState().invalidateJavFavoriteCounts('series')`
    )
    await waitFor(seriesStar)
    await command('Page.navigate', { url: `${fixture}?view=jav&prefix=ABC&jav_detail=1` })
    await waitFor(modal)
    const idolLink = `[...${modal}.querySelectorAll('a')].find(a => a.textContent.trim() === 'Test actress')`
    assert.ok(!(await evaluate(`${idolLink}.href`)).includes('prefix='))
    await evaluate(`${idolLink}.click()`)
    await waitFor(`!${modal} && location.search.includes('idol_ids=4')`)
    assert.ok(
      !(await evaluate('location.search')).includes('prefix='),
      'ordinary actress clicks clear the same prefix as the anchor URL'
    )
    await command('Page.navigate', { url: `${fixture}?view=jav&jav_detail=99` })
    await waitFor(`document.querySelector('[role="alert"]')`)
    await evaluate(`document.querySelector('[role="dialog"] button').click()`)
    await waitFor(`!document.querySelector('[role="dialog"]')`)
  }
)
