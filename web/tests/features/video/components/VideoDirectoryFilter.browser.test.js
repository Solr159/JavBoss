import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'subdirectory selection scopes requests and bulk selection, survives history and reload, and clears',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor, reloadPage } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-video-directory-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(
      `window.testStore && document.querySelector('aside') && window.requests.some(request => request.url.startsWith('/videos?')) && !window.testStore.getState().loading`
    )
    await evaluate(`(() => {
      const originalFetch = window.fetch;
      window.directoryRequests = [];
      window.fetch = async (input, init) => {
        const url = new URL(input, location.origin);
        if (url.pathname === '/videos/directories') return Response.json({items: [
          {directory_id: 1, directory_path: '/videos', path: '.'},
          ...Array.from({length: 60}, (_, index) => ({directory_id: 1, directory_path: '/videos', path: 'folder-' + index})),
          {directory_id: 1, directory_path: '/videos', path: '旅行'},
          {directory_id: 1, directory_path: '/videos', path: '旅行/100%_fun'},
          {directory_id: 2, directory_path: '/other', path: '旅行'},
        ]});
        if (url.pathname === '/videos') {
          window.directoryRequests.push(url.search);
          return Response.json({items: [
            {id: 1, location_id: 1, filename: 'one.mp4', path: '旅行/100%_fun/one.mp4', tags: []},
            {id: 2, location_id: 2, filename: 'two.mp4', path: '旅行/100%_fun/deeper/two.mp4', tags: []},
          ], total: 2});
        }
        return originalFetch(input, init);
      };
      window.testStore.setState({page: 3});
    })()`)
    await waitFor(`new URLSearchParams(location.search).get('page') === '3'`)
    const trigger = `document.querySelector('button[aria-label="Select subdirectory"]')`
    const root = `document.querySelector('summary[aria-label="Expand or collapse directory /videos"]')`
    const branch = `document.querySelector('summary[aria-label="Expand or collapse directory /videos/旅行"]')`
    const leaf = `document.querySelector('button[aria-label="Select directory /videos/旅行/100%_fun"]')`
    await evaluate(`${trigger}.click()`)
    await waitFor(root)
    assert.equal(await evaluate(`${root}.parentElement.open`), false)
    assert.equal(await evaluate(`${leaf}.checkVisibility()`), false)
    assert.equal(
      await evaluate(
        `Boolean(document.querySelector('summary[aria-label="Expand or collapse directory /other"]'))`
      ),
      true
    )
    await evaluate(`${root}.click()`)
    await waitFor(`${branch}.checkVisibility()`)
    assert.equal(await evaluate(`${leaf}.checkVisibility()`), false)
    await evaluate(`${branch}.focus()`)
    for (const type of ['keyDown', 'keyUp']) {
      await command('Input.dispatchKeyEvent', {
        type,
        key: 'Enter',
        code: 'Enter',
        windowsVirtualKeyCode: 13,
        ...(type === 'keyDown' ? { text: '\r', unmodifiedText: '\r' } : {}),
      })
    }
    await waitFor(`${leaf}?.checkVisibility()`)
    assert.equal(await evaluate(`window.testStore.getState().videoDirectoryId`), null)
    await evaluate(`${branch}.click()`)
    await waitFor(`!${leaf}.checkVisibility()`)
    await evaluate(`${branch}.click()`)
    await evaluate(`${leaf}.click()`)
    await waitFor(
      `window.testStore.getState().videoSubdirectory === '旅行/100%_fun' && new URLSearchParams(location.search).get('subdirectory') === '旅行/100%_fun' && !window.testStore.getState().loading`
    )
    assert.equal(await evaluate(`window.testStore.getState().page`), 1)
    let query = new URLSearchParams(await evaluate(`window.directoryRequests.at(-1)`))
    assert.equal(query.get('directory_id'), '1')
    assert.equal(query.get('subdirectory'), '旅行/100%_fun')
    assert.equal(query.get('offset'), '0')
    await waitFor(`document.querySelector('.video-select-check')`)
    await evaluate(`document.querySelector('.video-select-check').click()`)
    await waitFor(`document.querySelector('button[aria-label="Selection menu"]')`)
    await evaluate(`document.querySelector('button[aria-label="Selection menu"]').click()`)
    await waitFor(`document.querySelector('[role="menuitem"]')`)
    await evaluate(
      `[...document.querySelectorAll('[role="menuitem"]')].find(item => item.textContent === 'Select all').click()`
    )
    await waitFor(`window.testStore.getState().selectedVideoIds.size === 2`)
    query = new URLSearchParams(await evaluate(`window.directoryRequests.at(-1)`))
    assert.equal(query.get('directory_id'), '1')
    assert.equal(query.get('subdirectory'), '旅行/100%_fun')
    assert.equal(query.get('limit'), '2')
    await evaluate(`${trigger}.click()`)
    await waitFor(`${leaf}?.checkVisibility()`)
    assert.equal(await evaluate(`${leaf}.getAttribute('aria-pressed')`), 'true')
    await waitFor(`(() => {
      const selected = ${leaf};
      const container = document.querySelector('[aria-label="Directory tree"]').parentElement;
      const itemRect = selected.getBoundingClientRect();
      const containerRect = container.getBoundingClientRect();
      return container.scrollTop > 0 && itemRect.top >= containerRect.top && itemRect.bottom <= containerRect.bottom;
    })()`)
    assert.equal(await evaluate('window.scrollY'), 0)

    await evaluate(`document.querySelector('[aria-label="Clear directory filter"]').click()`)
    await waitFor(
      `!window.testStore.getState().videoDirectoryId && !new URLSearchParams(location.search).has('directory_id')`
    )
    await evaluate('history.back()')
    await waitFor(
      `window.testStore.getState().videoDirectoryId === 1 && ${trigger}.title === '/videos/旅行/100%_fun'`
    )
    await reloadPage()
    await waitFor(
      `window.testStore?.getState().videoSubdirectory === '旅行/100%_fun' && document.querySelector('aside') && !window.testStore.getState().loading`
    )
    await waitFor(
      `window.requests.some(request => request.url.startsWith('/videos?') && new URL(request.url, location.origin).searchParams.get('directory_id') === '1')`
    )
    assert.equal(await evaluate(`${trigger}.title`), '/videos/旅行/100%_fun')
    assert.equal(
      await evaluate(`Boolean(document.querySelector('[aria-label^="Remove filter Directory:"]'))`),
      false
    )
    const layout = await evaluate(`(() => {
      const button = ${trigger};
      const wrapper = button.parentElement;
      return {first: wrapper === document.querySelector('.filter-topbar__controls').firstElementChild,
        border: getComputedStyle(wrapper).borderRightWidth};
    })()`)
    assert.deepEqual(layout, { first: true, border: '1px' })
    await evaluate(`window.testStore.setState({searchTerm: 'movie'})`)
    await waitFor(`document.querySelector('[aria-label="Remove filter Search: movie"]')`)
    await evaluate(
      `[...document.querySelectorAll('.filter-clear-button')].find(button => button.textContent === 'Clear').click()`
    )
    await waitFor(`!window.testStore.getState().searchTerm`)
    assert.equal(await evaluate('window.testStore.getState().videoDirectoryId'), 1)
    assert.equal(await evaluate('window.testStore.getState().videoSubdirectory'), '旅行/100%_fun')
    await evaluate(`${trigger}.click()`)
    await waitFor(`document.querySelector('[aria-label="Clear directory filter"]')`)
    await evaluate(`document.querySelector('[aria-label="Clear directory filter"]').click()`)
    await waitFor(`!window.testStore.getState().videoDirectoryId`)
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
