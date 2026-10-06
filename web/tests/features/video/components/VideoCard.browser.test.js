import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'video cards hide unwatched icons and show half-hour progress at the right of the duration row',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-video-card-watch-time-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(`window.testStore && document.querySelector('aside') &&
      window.requests.some(request => request.url.startsWith('/videos?')) &&
      !window.testStore.getState().loading`)
    await evaluate(`window.testStore.setState({
      videos: [{id: 1, location_id: 1, filename: 'test.mp4', path: 'test.mp4',
        duration_sec: 2700, size: 1073741824, tags: []}], total: 1
    })`)
    const card = `document.querySelector('.video-card')`
    const indicator = `${card}.querySelector('.watch-time-icons')`
    await waitFor(card)
    assert.equal(await evaluate(`Boolean(${indicator})`), false)
    await evaluate(`window.testStore.setState(state => ({
      videos: state.videos.map(video => ({...video, watched_ms: 2700000}))
    }))`)
    await waitFor(indicator)
    assert.equal(await evaluate(`${indicator}.children.length`), 3)
    assert.equal(await evaluate(`Boolean(${card}.querySelector(':scope > [role="img"]'))`), false)
    assert.deepEqual(
      await evaluate(
        `[...${indicator}.querySelectorAll('svg:last-child')].map(icon => getComputedStyle(icon).clipPath)`
      ),
      ['inset(0px 0% 0px 0px)', 'inset(0px 50% 0px 0px)', 'inset(0px 100% 0px 0px)']
    )
    await evaluate(`${card}.style.width = '240px'`)
    const layout = await evaluate(`(() => {
      const indicator = ${indicator};
      const icons = indicator.getBoundingClientRect();
      const row = indicator.parentElement;
      const bounds = row.getBoundingClientRect();
      return {right: icons.right, rowRight: bounds.right, top: icons.top,
        rowTop: bounds.top, text: row.textContent};
    })()`)
    assert.equal(layout.right, layout.rowRight)
    assert.equal(layout.top, layout.rowTop)
    assert.match(layout.text, /45 (分钟|min)/)
    assert.match(await evaluate(`${indicator}.getAttribute('aria-label')`), /45 (分钟|min)/)
    await evaluate(`window.testStore.setState(state => ({
      videos: state.videos.map(video => ({...video, watched_ms: 0}))
    }))`)
    await waitFor(`!${indicator}`)
  }
)
