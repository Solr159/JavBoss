import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../helpers/browser.js'

for (const domain of ['video', 'jav']) {
  test(
    `${domain}: numbered-page reconciliation keeps the grid mounted and preserves scroll`,
    { skip: browserUnavailable, timeout: 60000 },
    async (t) => {
      const { origin, command, evaluate, waitFor } = await openBrowser(t)
      await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=${domain}` })
      await waitFor(`document.querySelector('aside button[aria-label="JAV codes"]')`)
      const items = domain === 'video' ? 'videos' : 'javItems'
      const loading = domain === 'video' ? 'loading' : 'javLoading'
      const loadingMore = domain === 'video' ? 'videoLoadingMore' : 'javLoadingMore'
      await evaluate(`{
        window.rows = Array.from({length:110}, (_, i) => ({
          id:i+1, location_id:i+1, filename:'clip-'+(i+1)+'.mp4',
          path:'clip-'+(i+1)+'.mp4', directory:{id:1,path:'/videos'},
          code:'ABC-'+(i+1), title:'Work '+(i+1), tags:[], idols:[], videos:[]
        }));
        const originalFetch = window.fetch;
        window.fetch = (input, init = {}) => {
          const url = new URL(input, location.origin);
          if (url.pathname !== '${domain === 'video' ? '/videos' : '/jav'}') return originalFetch(input,init);
          const offset = Number(url.searchParams.get('offset'));
          const limit = Number(url.searchParams.get('limit'));
          const respond = () => Response.json({items:window.rows.slice(offset,offset+limit), total:window.rows.length});
          if (window.holdReconcile) return new Promise(resolve => {window.finishReconcile = () => resolve(respond())});
          return Promise.resolve(respond());
        };
        window.testStore.getState().${domain === 'video' ? 'setPageSize' : 'setJavPageSize'}(50);
      }`)
      await waitFor(`document.querySelectorAll('.${domain}-card').length === 50`)
      await evaluate(`{
        const card = document.querySelectorAll('.${domain}-card')[24];
        window.scrollTo(0, card.getBoundingClientRect().top + window.scrollY - 150);
      }`)
      await evaluate('window.beforeDeleteScroll = window.scrollY')
      assert.ok(await evaluate('window.beforeDeleteScroll > 500'))
      await evaluate(`{
        window.holdReconcile = true;
        window.rows = window.rows.filter(row => row.id !== 46);
        window.deletion = window.testStore.getState().${domain === 'video' ? "removeVideoLocations(['loc:46'])" : 'removeJavVideos(46,[])'};
        void 0;
      }`)
      await waitFor(
        `window.finishReconcile && document.querySelectorAll('.${domain}-card').length === 49`
      )
      assert.equal(await evaluate(`window.testStore.getState().${loading}`), false)
      assert.ok(Math.abs(await evaluate('window.scrollY - window.beforeDeleteScroll')) <= 2)
      await evaluate('window.holdReconcile = false; window.finishReconcile()')
      await waitFor(
        `!window.testStore.getState().${loadingMore} && document.querySelectorAll('.${domain}-card').length === 50`
      )
      assert.equal(await evaluate(`window.testStore.getState().${items}.at(-1).id`), 51)
      assert.ok(Math.abs(await evaluate('window.scrollY - window.beforeDeleteScroll')) <= 2)
      await evaluate(
        `window.testStore.getState().${domain === 'video' ? 'setPage' : 'setJavPage'}(2)`
      )
      await waitFor(
        `!window.testStore.getState().${loading} && window.testStore.getState().${items}[0]?.id === 52`
      )
      assert.deepEqual(await evaluate('window.appErrors'), [])
    }
  )
}
