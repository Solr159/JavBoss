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
