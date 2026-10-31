import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'video directory links filter in place without reloading the application',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-video-card-directory-test',
    })
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/app.html?view=video&search=test&page=3`,
    })
    await waitFor(`window.testStore && document.querySelector('aside') &&
      window.requests.some(request => request.url.startsWith('/videos?')) &&
      !window.testStore.getState().loading`)
    const timeOrigin = await evaluate('performance.timeOrigin')
    const initialRequests = await evaluate(`window.requests.filter(request =>
      ['/config', '/auth/status', '/directories'].includes(request.url)).length`)

    for (const directory of ['旅行/100%_fun', '.']) {
      await evaluate(`window.testStore.setState({
        videos: [{id: 1, location_id: 1, directory_id: 1, filename: 'test.mp4',
          path: ${JSON.stringify(directory === '.' ? 'test.mp4' : `${directory}/test.mp4`)},
          tags: []}], total: 1
      })`)
      const link = `document.querySelector('.video-card a')`
      await waitFor(link)
      const href = await evaluate(`${link}.href`)
      const query = new URL(href).searchParams
      assert.equal(query.get('directory_id'), '1')
      assert.equal(query.get('subdirectory'), directory)
      assert.equal(query.get('search'), 'test')
      assert.equal(query.get('page'), '1')
      assert.deepEqual(
        await evaluate(`['ctrlKey', 'metaKey', 'shiftKey', 'altKey'].map(key => {
          let prevented;
          document.getElementById('root').addEventListener('click', event => {
            prevented = event.defaultPrevented;
            // Observe React's handler, then prevent an actual new tab in the test.
            event.preventDefault();
          }, {once: true});
          ${link}.dispatchEvent(new MouseEvent('click', {
            bubbles: true, cancelable: true, [key]: true
          }));
          return prevented;
        })`),
        [false, false, false, false]
      )
      await evaluate(`${link}.click()`)
      await waitFor(`location.href === ${JSON.stringify(href)} &&
        window.testStore.getState().videoSubdirectory === ${JSON.stringify(directory)} &&
        !window.testStore.getState().loading`)
      assert.equal(await evaluate('performance.timeOrigin'), timeOrigin)
      assert.equal(await evaluate('window.testStore.getState().searchTerm'), 'test')
      assert.equal(await evaluate('window.testStore.getState().page'), 1)
      const request = new URL(
        await evaluate(
          `window.requests.filter(request => request.url.startsWith('/videos?')).at(-1).url`
        ),
        origin
      ).searchParams
      assert.equal(request.get('directory_id'), '1')
      assert.equal(request.get('subdirectory'), directory)
      assert.equal(request.get('search'), 'test')
    }
    assert.equal(
      await evaluate(`window.requests.filter(request =>
        ['/config', '/auth/status', '/directories'].includes(request.url)).length`),
      initialRequests
    )
    await evaluate('history.back()')
    await waitFor(`window.testStore.getState().videoSubdirectory === '旅行/100%_fun'`)
    assert.equal(await evaluate('performance.timeOrigin'), timeOrigin)
  }
)

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
      config: {...state.config, video_watch_time_icon_minutes: 60},
      videos: state.videos.map(video => ({...video, watched_ms: 2700000}))
    }))`)
    await waitFor(
      `getComputedStyle(${indicator}.querySelector('svg:last-child')).clipPath === 'inset(0px 25% 0px 0px)'`
    )
    await evaluate(`window.testStore.setState(state => ({
      videos: state.videos.map(video => ({...video, watched_ms: 0}))
    }))`)
    await waitFor(`!${indicator}`)
  }
)
