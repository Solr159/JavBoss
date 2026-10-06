import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../helpers/browser.js'

test(
  'live and reconnect totals update video/JAV cards without reloading lists or replacing cards',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-watch-sync-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(
      'window.testStore && window.watchTimeSources.some(source=>!source.closed) && document.querySelector("aside")'
    )
    await evaluate(`{
  window.videoRows=[{id:1,location_id:11,filename:'first.mp4',watched_ms:0},{id:2,location_id:12,filename:'second.mp4',watched_ms:0}];
  const original=window.fetch;
  window.snapshot={videos:[],javs:[]};
  window.fetch=(input,init)=>{
   const url=new URL(input,location.origin);
   if(url.pathname==='/videos' && window.holdVideoList) return new Promise(resolve=>{
    window.finishVideoList=()=>resolve(Response.json({items:[{...window.videoRows[0],watched_ms:16000},window.videoRows[1]],total:2}));
   });
   if(url.pathname==='/videos') {window.requests.push({url:'/videos'});return Promise.resolve(Response.json({items:window.videoRows,total:2}));}
   if(url.pathname==='/videos/watched-time') return Promise.resolve(Response.json(window.snapshot));
   return original(input,init);
  };
  window.sendTotals=snapshot=>window.watchTimeSources.filter(source=>!source.closed).forEach(source=>source.dispatchEvent(new MessageEvent('watched-time',{data:JSON.stringify(snapshot)})));
  window.testStore.getState().loadVideos({force:true});
 }`)
    await waitFor('document.querySelectorAll(".video-card").length === 2')
    await evaluate(`{
  window.cards=[...document.querySelectorAll('.video-card')];
  window.listRequests=window.requests.filter(r=>r.url==='/videos').length;
  window.sendTotals({videos:[{id:1,watched_ms:19000}],javs:[{id:1,watched_ms:30000}]});
 }`)
    await waitFor(
      `document.querySelector('.video-card .watch-time-icons')?.getAttribute('aria-label') === 'Watched: 19 s'`
    )
    assert.equal(
      await evaluate(
        `window.cards.every((card,index)=>card===document.querySelectorAll('.video-card')[index])`
      ),
      true
    )
    assert.equal(
      await evaluate(`window.requests.filter(r=>r.url==='/videos').length === window.listRequests`),
      true
    )
    assert.equal(await evaluate('window.watchTimeSources.filter(source=>!source.closed).length'), 1)
    // A list request overlaps a live update and then returns an older snapshot.
    await evaluate(
      `window.holdVideoList=true; void window.testStore.getState().loadVideos({force:true})`
    )
    await waitFor('window.finishVideoList')
    await evaluate(`{
      window.sendTotals({videos:[{id:1,watched_ms:25000}],javs:[]});
      window.finishVideoList();
      window.holdVideoList=false;
    }`)
    await waitFor(
      `document.querySelector('.video-card .watch-time-icons')?.getAttribute('aria-label') === 'Watched: 25 s'`
    )
    assert.equal(await evaluate('window.testStore.getState().videos[0].watched_ms'), 16000)
    await evaluate(
      `window.snapshot={videos:[{id:1,watched_ms:31000}],javs:[{id:1,watched_ms:62000}]}; window.dispatchEvent(new Event('focus'))`
    )
    await waitFor(
      `document.querySelector('.video-card .watch-time-icons')?.getAttribute('aria-label') === 'Watched: 31 s'`
    )
    await evaluate(`document.querySelector('aside button[aria-label="JAV"]').click()`)
    await waitFor(
      `document.querySelector('.jav-card .watch-time-icons')?.getAttribute('aria-label') === 'Watched: 1 min 2 s'`
    )
    await evaluate(
      `window.snapshot={videos:[],javs:[{id:1,watched_ms:90000}]}; window.watchTimeSources.find(source=>!source.closed).dispatchEvent(new Event('open'))`
    )
    await waitFor(
      `document.querySelector('.jav-card .watch-time-icons')?.getAttribute('aria-label') === 'Watched: 1 min 30 s'`
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
