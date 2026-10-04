import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'JAV deletion confirms scope, supports retry, disables duplicate actions and clears detail and selection',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/javDetailNavigation.html?view=jav`,
    })
    await waitFor(`document.querySelector('.jav-card button')`)
    await evaluate(`document.querySelector('.jav-card button').click()`)
    const detail = `document.querySelector('[aria-labelledby="jav-detail-title-1"]')`
    await waitFor(detail)
    await evaluate(`{
      window.deleteRequests = 0;
      window.allowDelete = false;
      window.deleteFailure = true;
      window.confirm = (message) => { window.deleteMessage = message; return window.allowDelete; };
      const originalFetch = window.fetch;
      window.fetch = (input, init) => {
        if (init?.method === 'DELETE') {
          window.deleteRequests++;
          window.deletePath = new URL(input, location.origin).pathname;
          if (window.deleteFailure) return Promise.resolve(Response.json({error_en: 'Deletion failed'}, {status: 500}));
          return new Promise(resolve => { window.finishDelete = () => resolve(Response.json({status:'ok', video_ids:[7]})); });
        }
        const path = new URL(input, location.origin).pathname;
        if (['/jav/studios','/jav/series','/jav/idols/options'].includes(path)) return Promise.resolve(Response.json({items:[],total:0}));
        if (path === '/jav/tags') return Promise.resolve(Response.json([]));
        return originalFetch(input, init);
      };
      const refresh = () => Promise.resolve();
      window.testStore.setState({
        videos:[{id:7},{id:8}],total:2,javTotal:1,
        selectedVideoIds:new Set(['7:10','8:11']),
        selectedVideoMeta:{'7:10':{video_id:7},'8:11':{video_id:8}},
        loadJavs:refresh,loadVideos:refresh,loadJavTags:refresh,loadTags:refresh,
        loadJavIdols:refresh,loadJavStudios:refresh,loadJavSeries:refresh,loadJavFavoriteGroups:refresh
      });
      [...${detail}.querySelectorAll('button')].find(b => b.textContent === 'Edit').click();
    }`)
    const modal = `document.querySelector('[aria-label="Edit JAV info"]')`
    const deleteButton = `[...${modal}.querySelectorAll('button')].find(b => b.textContent === 'Delete')`
    await waitFor(modal)
    await evaluate(`${deleteButton}.click()`)
    assert.equal(await evaluate('window.deleteRequests'), 0)
    assert.match(await evaluate('window.deleteMessage'), /matching subtitles, NFO files and images/)
    await evaluate(`window.allowDelete = true; ${deleteButton}.click()`)
    await waitFor(`${modal}.textContent.includes('Deletion failed')`)
    assert.ok(await evaluate(`Boolean(${detail})`))
    await evaluate(`window.deleteFailure = false; ${deleteButton}.click()`)
    await waitFor('window.finishDelete')
    assert.equal(
      await evaluate(
        `[...${modal}.querySelectorAll('button')].find(b => b.textContent === 'Deleting...').disabled`
      ),
      true
    )
    assert.equal(
      await evaluate(
        `[...${modal}.querySelectorAll('button')].find(b => b.textContent === 'Save').disabled`
      ),
      true
    )
    await evaluate('window.finishDelete()')
    await waitFor(`!${modal} && !${detail}`)
    assert.equal(await evaluate('window.deleteRequests'), 2)
    assert.deepEqual(await evaluate('window.testStore.getState().javItems'), [])
    assert.deepEqual(await evaluate('window.testStore.getState().videos.map(v => v.id)'), [8])
    assert.deepEqual(await evaluate('[...window.testStore.getState().selectedVideoIds]'), ['8:11'])
    assert.deepEqual(await evaluate('window.testStore.getState().javVideoDeletions[1]'), [7])
    assert.equal(await evaluate('window.deletePath'), '/jav/items/1/videos')
    assert.match(await evaluate('window.deleteMessage'), /JAV metadata and its cover are kept/)
    await evaluate('history.forward()')
    await waitFor(detail)
    assert.equal(await evaluate('window.cachedDetail.code'), 'ABC-001')
    assert.deepEqual(await evaluate('window.cachedDetail.videos'), [])
  }
)

for (const fromDetail of [false, true]) {
  test(
    `deleting from ${fromDetail ? 'details' : 'a card'} updates the list in place without refetching or losing scroll`,
    { skip: browserUnavailable, timeout: 60000 },
    async (t) => {
      const { origin, command, evaluate, waitFor } = await openBrowser(t)
      await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav` })
      await waitFor(`document.querySelector('.jav-card')`)
      await evaluate(`{
        window.javListRequests = [];
        window.javRows = Array.from({length: 80}, (_, i) => ({id: i + 1, code: 'ABC-' + (i + 1), title: 'Work ' + (i + 1), videos: [], tags: [], idols: []}));
        const originalFetch = window.fetch;
        window.fetch = (input, init = {}) => {
          const url = new URL(input, location.origin);
          if (init.method === 'DELETE' && url.pathname.endsWith('/videos')) {
            const id = Number(url.pathname.split('/')[3]);
            window.javRows = window.javRows.filter(item => item.id !== id);
            return Promise.resolve(Response.json({video_ids: []}));
          }
          if (url.pathname === '/jav') {
            const limit = Number(url.searchParams.get('limit'));
            const offset = Number(url.searchParams.get('offset'));
            window.javListRequests.push({limit, offset});
            return Promise.resolve(Response.json({items: window.javRows.slice(offset, offset + limit), total: window.javRows.length}));
          }
          return originalFetch(input, init);
        };
        window.confirm = () => true;
        window.testStore.setState({javGridColumns: 3});
      }`)
      await evaluate(
        `window.listLoad = window.testStore.getState().loadJavs({force: true}); void 0`
      )
      await waitFor(
        `window.testStore.getState().javItems.length === 24 && !window.testStore.getState().javLoading`
      )
      await evaluate(`window.moreLoad = window.testStore.getState().loadMoreJavs(); void 0`)
      await waitFor(`document.querySelectorAll('.jav-card').length === 48`)
      await evaluate(`window.scrollTo(0, 1800)`)
      await waitFor(`window.scrollY === 1800`)
      await evaluate(`{
        window.beforeDeleteScroll = window.scrollY;
        const card = [...document.querySelectorAll('.jav-card')].find(card => {
          const rect = card.getBoundingClientRect();
          return rect.top >= 0 && rect.bottom < window.innerHeight;
        });
        card.querySelector(${fromDetail ? "'button'" : '\'[aria-label="Edit JAV"]\''}).click();
      }`)
      if (fromDetail) {
        const detail = `document.querySelector('[role="dialog"][aria-labelledby^="jav-detail-title-"]')`
        await waitFor(detail)
        await evaluate(
          `[...${detail}.querySelectorAll('button')].find(button => button.textContent === 'Edit').click()`
        )
      }
      const modal = `document.querySelector('[aria-label="Edit JAV info"]')`
      await waitFor(modal)
      await evaluate(
        `[...${modal}.querySelectorAll('button')].find(button => button.textContent === 'Delete').click()`
      )
      await waitFor(
        `window.testStore.getState().javItems.length === 47 && !${modal} && !location.search.includes('jav_detail=')`
      )
      assert.equal(await evaluate('window.testStore.getState().javLoading'), false)
      assert.equal(await evaluate(`document.querySelectorAll('.jav-card').length`), 47)
      assert.equal(await evaluate('window.javListRequests.length'), 2)
      assert.ok(Math.abs(await evaluate('window.scrollY - window.beforeDeleteScroll')) <= 2)
      await waitFor(`window.testStore.getState().javTotal === 79`)
      assert.equal(await evaluate(`document.querySelectorAll('.jav-card').length`), 47)
      assert.ok(Math.abs(await evaluate('window.scrollY - window.beforeDeleteScroll')) <= 2)
      // Continue from the remaining 47 rows, without reloading the first two pages.
      await evaluate('window.moreLoad = window.testStore.getState().loadMoreJavs(); void 0')
      await waitFor('window.testStore.getState().javItems.length === 71')
      assert.deepEqual(await evaluate('window.javListRequests.at(-1)'), { limit: 24, offset: 47 })
      assert.equal(
        await evaluate('new Set(window.testStore.getState().javItems.map(item => item.id)).size'),
        71
      )
      assert.ok(Math.abs(await evaluate('window.scrollY - window.beforeDeleteScroll')) <= 2)
      // A later visit refreshes the invalidated catalog on demand.
      await evaluate('window.revisitLoad = window.testStore.getState().loadJavs(); void 0')
      await waitFor('window.testStore.getState().javItems.length === 24')
      assert.deepEqual(await evaluate('window.javListRequests.at(-1)'), { limit: 24, offset: 0 })
      assert.deepEqual(await evaluate('window.appErrors'), [])
    }
  )
}
