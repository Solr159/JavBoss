import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../helpers/browser.js'

test(
  'tabs independently restore filters, pages and scroll, including history and directory resets',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-tab-navigation-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(`window.testStore && document.querySelector('aside') &&
      window.requests.some(r => r.url.startsWith('/videos?')) && !window.testStore.getState().loading`)
    // Keep enough scrollable space even when a fixture's filtered list is empty.
    await evaluate(`document.querySelector('main').style.minHeight = '3200px'`)
    await evaluate(`window.testStore.subscribe((state, previous) => {
      const key = state.viewMode === 'jav' ? 'javTags' : 'selectedTags';
      if (state[key] !== previous[key]) window.filterRestores = (window.filterRestores || 0) + 1;
    })`)
    const timeOrigin = await evaluate('performance.timeOrigin')
    const selectTab = async (label) => {
      await evaluate(`window.filterRestores = 0;
        document.querySelector('aside button[aria-label="${label}"]').click()`)
      await waitFor(`document.querySelector('aside button[aria-label="${label}"][aria-current="page"]') &&
        !window.testStore.getState().loading && !window.testStore.getState().javLoading &&
        !window.testStore.getState().idolLoading && !window.testStore.getState().studioLoading &&
        !window.testStore.getState().seriesLoading`)
      assert.ok(
        (await evaluate('window.filterRestores')) <= 1,
        'committing the tab URL must not restore filters again and rerender the grid'
      )
    }
    const cases = [
      [
        'Video',
        {
          searchTerm: 'video clips',
          videoDirectoryId: 1,
          videoSubdirectory: 'clips',
          videoTempSort: 'filename',
          page: 3,
        },
      ],
      [
        'JAV',
        {
          javSearchTerm: 'jav clips',
          javPrefix: 'ABC',
          javTags: [7],
          javSoloOnly: true,
          javFavoriteRatingEnabled: true,
          javFavoriteRatingMin: 3,
          javFavoriteRatingMax: 5,
          javTempSort: 'release',
          javPage: 4,
        },
      ],
      [
        'Idols',
        {
          javSearchTerm: 'actress',
          idolTempSort: 'height',
          idolPage: 2,
          idolProfileFilters: { height: { enabled: true, min: 160, max: 175 } },
        },
      ],
      ['Studios', { javSearchTerm: 'studio', studioPage: 3, studioFavoriteGroupId: 9 }],
      ['Series', { javSearchTerm: 'series', seriesPage: 2, seriesFavoriteGroupId: 8 }],
    ]
    const saved = new Map()
    for (const [index, [label, state]] of cases.entries()) {
      await selectTab(label)
      assert.equal(await evaluate(`document.querySelector('.filter-search input').value`), '')
      await waitFor('window.scrollY === 0')
      await evaluate(`window.testStore.setState(${JSON.stringify(state)})`)
      const search = state.searchTerm || state.javSearchTerm
      await waitFor(
        `new URLSearchParams(location.search).get('search') === ${JSON.stringify(search)}`
      )
      await evaluate(`window.scrollTo(0, ${400 + index * 100})`)
      await waitFor(`window.scrollY === ${400 + index * 100}`)
      saved.set(label, {
        url: await evaluate('location.href'),
        y: await evaluate('window.scrollY'),
      })
    }
    for (const [label] of cases) {
      await selectTab(label)
      const entry = saved.get(label)
      await waitFor(
        `location.href === ${JSON.stringify(entry.url)} && window.scrollY === ${entry.y}`
      )
      assert.equal(await evaluate('performance.timeOrigin'), timeOrigin)
    }
    await evaluate('history.back()')
    await waitFor(
      `location.href === ${JSON.stringify(saved.get('Studios').url)} && window.scrollY === ${saved.get('Studios').y}`
    )
    await evaluate('history.forward()')
    await waitFor(
      `location.href === ${JSON.stringify(saved.get('Series').url)} && window.scrollY === ${saved.get('Series').y}`
    )

    // Clearing one tab must not replace another tab's saved filters.
    await selectTab('JAV')
    await evaluate(
      `[...document.querySelectorAll('.filter-clear-button')].find(button => button.textContent === 'Clear').click()`
    )
    await waitFor(
      `!window.testStore.getState().javSearchTerm && !new URLSearchParams(location.search).has('search')`
    )
    const clearedJav = await evaluate('location.href')
    await selectTab('Idols')
    await waitFor(`location.href === ${JSON.stringify(saved.get('Idols').url)}`)
    await selectTab('JAV')
    await waitFor(`location.href === ${JSON.stringify(clearedJav)}`)
    await selectTab('Video')
    await waitFor(`location.href === ${JSON.stringify(saved.get('Video').url)}`)
    await evaluate(`window.testStore.getState().loadRandom(42)`)
    await waitFor(`new URLSearchParams(location.search).get('seed') === '42'`)
    const randomVideo = await evaluate('location.href')
    await selectTab('JAV')
    await evaluate(`window.testStore.getState().loadJavRandom(73)`)
    await waitFor(`new URLSearchParams(location.search).get('seed') === '73'`)
    const randomJav = await evaluate('location.href')
    await selectTab('Video')
    await waitFor(`location.href === ${JSON.stringify(randomVideo)}`)
    assert.equal(await evaluate('window.testStore.getState().randomSeed'), 42)
    await selectTab('JAV')
    await waitFor(`location.href === ${JSON.stringify(randomJav)}`)
    assert.equal(await evaluate('window.testStore.getState().javRandomSeed'), 73)
    await selectTab('Video')
    await evaluate(
      `window.testStore.getState().setVideoDirectoryFilter({directory_id: 1, path: 'new'})`
    )
    await waitFor(
      `new URLSearchParams(location.search).get('subdirectory') === 'new' && window.scrollY === 0`
    )
    assert.equal(await evaluate(`document.querySelector('.filter-search input').value`), '')
    assert.equal(await evaluate('window.testStore.getState().page'), 1)
    assert.equal(await evaluate('window.testStore.getState().randomMode'), false)
    await selectTab('Series')
    await selectTab('Video')
    await waitFor(`window.testStore.getState().videoSubdirectory === 'new' && window.scrollY === 0`)
    assert.deepEqual(await evaluate('window.appErrors'), [])

    // A fresh page follows the URL; the original page keeps its own tab snapshots.
    const fresh = await openBrowser(t, { cacheDir: 'node_modules/.vite-tab-direct-link-test' })
    const url = randomVideo.replace(origin, fresh.origin)
    await fresh.command('Page.navigate', { url })
    await fresh.waitFor(`window.testStore?.getState().randomSeed === 42 &&
      document.querySelector('aside') && !window.testStore.getState().loading`)
    assert.equal(await fresh.evaluate('window.testStore.getState().videoSubdirectory'), 'clips')
    assert.equal(await fresh.evaluate('window.testStore.getState().searchTerm'), 'video clips')
    assert.equal(await evaluate('window.testStore.getState().videoSubdirectory'), 'new')
  }
)

test(
  'switching back to a cached JAV tab paints loading before rendering its cards',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-tab-async-loading-test',
      plugins: [
        {
          name: 'count-jav-card-renders',
          enforce: 'pre',
          transform(code, id) {
            if (!id.endsWith('/JavCard.jsx')) return
            return code.replace(
              'export default function JavCard(props) {',
              'export default function JavCard(props) { window.cardRenders = (window.cardRenders || 0) + 1;'
            )
          },
        },
      ],
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav` })
    await waitFor(
      `window.testStore && document.querySelector('.jav-card') && !window.testStore.getState().javLoading`
    )
    await evaluate(`window.testStore.setState({javSearchTerm: 'saved search', javPage: 3})`)
    await waitFor(
      `new URLSearchParams(location.search).get('page') === '3' && !window.testStore.getState().javLoading`
    )
    const savedUrl = await evaluate('location.href')
    await evaluate(`document.querySelector('aside button[aria-label="Video"]').click()`)
    await waitFor(
      `new URLSearchParams(location.search).get('view') === 'video' && !window.testStore.getState().loading`
    )
    await evaluate(`{
      const originalFetch = window.fetch;
      window.tabRequests = [];
      window.fetch = (input, init) => {
        const url = new URL(input, location.origin);
        if (url.pathname === '/jav') {
          window.tabRequests.push(url.search);
          return new Promise(resolve => {
            window.finishTabLoad = () => resolve(Response.json({
              items: [{id: 1, code: 'ABC-001', title: 'Loaded asynchronously', tags: [], idols: [], videos: []}], total: 100
            }));
          });
        }
        return originalFetch(input, init);
      };
      window.cardRenders = 0;
      document.querySelector('aside button[aria-label="JAV"]').click();
    }`)
    await waitFor(`location.href === ${JSON.stringify(savedUrl)} &&
      document.querySelector('aside button[aria-label="JAV"][aria-current="page"]')`)
    assert.equal(await evaluate('window.testStore.getState().javLoading'), true)
    assert.equal(await evaluate('window.cardRenders'), 0)
    assert.equal(
      await evaluate(`document.querySelector('main').textContent.includes('Loading...')`),
      true
    )
    const requests = await evaluate('window.tabRequests')
    assert.equal(requests.length, 1, 'the route must reuse the in-flight request')
    assert.equal(new URLSearchParams(requests[0]).get('search'), 'saved search')
    assert.equal(new URLSearchParams(requests[0]).get('offset'), '48')
    // Clicking the active tab must not abort/restart the pending request.
    await evaluate(`document.querySelector('aside button[aria-label="JAV"]').click()`)
    assert.equal(await evaluate('window.tabRequests.length'), 1)
    await evaluate('window.finishTabLoad()')
    await waitFor(`!window.testStore.getState().javLoading && document.querySelector('.jav-card')`)
    assert.ok(await evaluate('window.cardRenders > 0'))
    assert.equal(await evaluate('window.testStore.getState().javPage'), 3)
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
