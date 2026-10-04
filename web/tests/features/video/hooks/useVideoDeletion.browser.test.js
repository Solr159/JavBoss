import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

for (const batch of [false, true]) {
  test(
    `${batch ? 'batch' : 'single'} video deletion preserves scroll, other copies and waterfall progress`,
    { skip: browserUnavailable, timeout: 60000 },
    async (t) => {
      const { origin, command, evaluate, waitFor } = await openBrowser(t)
      await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
      await waitFor(`document.querySelector('aside button[aria-label="JAV codes"]')`)
      await evaluate(`{
        window.videoRows = Array.from({length:80}, (_, i) => ({
          id: Math.ceil((i + 1) / 2), location_id: i + 1, filename: 'clip-' + (i + 1) + '.mp4',
          path: 'clip-' + (i + 1) + '.mp4', directory: {id:1, path:'/videos'}, tags: []
        }));
        window.videoRequests = [];
        window.deleteRequests = [];
        window.allowDelete = true;
        window.confirm = () => window.allowDelete;
        const originalFetch = window.fetch;
        window.fetch = (input, init = {}) => {
          const url = new URL(input, location.origin);
          if (url.pathname === '/videos') {
            const offset = Number(url.searchParams.get('offset'));
            const limit = Number(url.searchParams.get('limit'));
            window.videoRequests.push({offset,limit});
            return Promise.resolve(Response.json({items:window.videoRows.slice(offset,offset+limit),total:window.videoRows.length}));
          }
          if (init.method === 'DELETE' && url.pathname.includes('/locations/')) {
            const locationId = Number(url.pathname.split('/').at(-1));
            window.deleteRequests.push(locationId);
            if (locationId === window.failLocation) return Promise.resolve(Response.json({error_en:'File is busy'}, {status:500}));
            window.videoRows = window.videoRows.filter(video => video.location_id !== locationId);
            return Promise.resolve(Response.json({status:'ok'}));
          }
          return originalFetch(input, init);
        };
        window.initialLoad = window.testStore.getState().loadVideos({force:true});
      }`)
      await waitFor(`document.querySelectorAll('.video-card').length === 25`)
      await evaluate(`window.moreLoad = window.testStore.getState().loadMoreVideos(); void 0`)
      await waitFor(`document.querySelectorAll('.video-card').length === 50`)
      await evaluate(`{
        window.testStore.getState().setWaterfallModes(modes => ({...modes, video: true}));
        const card = document.querySelectorAll('.video-card')[24];
        window.scrollTo(0, card.getBoundingClientRect().top + window.scrollY - 150);
      }`)
      await evaluate('window.beforeDeleteScroll = window.scrollY')
      assert.ok(await evaluate('window.beforeDeleteScroll > 500'))
      if (batch) {
        await evaluate(`{
          const state = window.testStore.getState();
          state.toggleSelectVideo(state.videos[24]);
          state.toggleSelectVideo(state.videos[26]);
          window.failLocation = 27;
        }`)
        await waitFor(`document.querySelector('button.topbar-selection-action')`)
        await evaluate(`document.querySelector('button.topbar-selection-action').click()`)
        const modal = `document.querySelector('[aria-label="Selected Files"]')`
        await waitFor(modal)
        const deleteButton = `[...${modal}.querySelectorAll('button')].find(button => button.textContent === 'Delete Selected Videos')`
        await evaluate(`${deleteButton}.click()`)
        await waitFor(
          `window.testStore.getState().videos.length === 49 && ${deleteButton} && !${deleteButton}.disabled`
        )
        assert.deepEqual(await evaluate('[...window.testStore.getState().selectedVideoIds]'), [
          'loc:27',
        ])
        assert.equal(
          await evaluate(
            'window.testStore.getState().videos.some(video => video.location_id === 27)'
          ),
          true
        )
        await waitFor('!window.testStore.getState().videoLoadingMore')
        assert.equal(await evaluate('window.videoRequests.length'), 3)
        await evaluate(`window.failLocation = null; ${deleteButton}.click()`)
        await waitFor(`!${modal} && window.testStore.getState().videos.length === 48`)
        assert.deepEqual(await evaluate('window.deleteRequests'), [25, 27, 27])
      } else {
        const openMenu = `document.querySelectorAll('.video-card')[24].querySelector('[aria-label="Edit video"]').click()`
        const deleteButton = `[...document.querySelectorAll('.MuiPopover-root button')].find(button => button.textContent === 'Delete')`
        await evaluate(`window.allowDelete = false; ${openMenu}`)
        await waitFor(deleteButton)
        await evaluate(`${deleteButton}.click()`)
        assert.deepEqual(await evaluate('window.deleteRequests'), [])
        await evaluate(`window.allowDelete = true; ${openMenu}`)
        await waitFor(deleteButton)
        await evaluate(`${deleteButton}.click()`)
        await waitFor('window.testStore.getState().videos.length === 49')
        assert.deepEqual(await evaluate('window.deleteRequests'), [25])
      }
      const remaining = batch ? 48 : 49
      await waitFor('!window.testStore.getState().videoLoadingMore')
      assert.deepEqual(await evaluate('[...window.testStore.getState().selectedVideoIds]'), [])
      assert.equal(await evaluate('window.testStore.getState().loading'), false)
      assert.equal(await evaluate('window.testStore.getState().total'), batch ? 78 : 79)
      assert.equal(await evaluate('window.videoRequests.length'), batch ? 4 : 3)
      assert.deepEqual(await evaluate('window.videoRequests.at(-1)'), {
        offset: 0,
        limit: remaining,
      })
      assert.equal(
        await evaluate(
          'window.testStore.getState().videos.some(video => video.location_id === 26)'
        ),
        true
      )
      assert.ok(Math.abs(await evaluate('window.scrollY - window.beforeDeleteScroll')) <= 2)
      await evaluate('window.moreLoad = window.testStore.getState().loadMoreVideos(); void 0')
      await waitFor(`window.testStore.getState().videos.length === ${remaining + 25}`)
      assert.deepEqual(await evaluate('window.videoRequests.at(-1)'), {
        offset: remaining,
        limit: 25,
      })
      assert.equal(
        await evaluate(
          'new Set(window.testStore.getState().videos.map(video => video.location_id)).size'
        ),
        remaining + 25
      )
      assert.ok(Math.abs(await evaluate('window.scrollY - window.beforeDeleteScroll')) <= 2)
      assert.deepEqual(await evaluate('window.appErrors'), [])
    }
  )
}
