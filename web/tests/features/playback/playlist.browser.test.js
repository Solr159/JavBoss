import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'batch actions use the default player; browser playlists switch copies, advance and reset',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(`document.querySelector('aside button[aria-label="JAV codes"]')`)
    await evaluate(`{
      const wav = new Uint8Array(44 + 8000 * 60).fill(128);
      const header = new DataView(wav.buffer);
      const text = (at, value) => [...value].forEach((letter, i) => wav[at + i] = letter.charCodeAt(0));
      text(0, 'RIFF'); header.setUint32(4, wav.length - 8, true); text(8, 'WAVEfmt ');
      header.setUint32(16, 16, true); header.setUint16(20, 1, true); header.setUint16(22, 1, true);
      header.setUint32(24, 8000, true); header.setUint32(28, 8000, true);
      header.setUint16(32, 1, true); header.setUint16(34, 8, true);
      text(36, 'data'); header.setUint32(40, wav.length - 44, true);
      window.mediaURL = URL.createObjectURL(new Blob([wav], {type:'audio/wav'}));
      window.videoRows = [
        {id:1, location_id:11, filename:'first.mp4', path:'first.mp4', directory:{path:'/videos'}},
        {id:1, location_id:12, filename:'second-copy.mp4', path:'second-copy.mp4', directory:{path:'/videos'}},
        {id:2, location_id:13, filename:'third.mp4', path:'third.mp4', directory:{path:'/videos'}}
      ];
      window.playlistRequests = [];
      window.streamRequests = [];
      window.sessionRequests = [];
      window.screenshotRequests = [];
      window.holdInitialStreams = true;
      const originalFetch = window.fetch;
      window.fetch = async (input, init = {}) => {
        const url = new URL(input, location.origin);
        if (url.pathname === '/videos') return Response.json({items:window.videoRows,total:3});
        if (url.pathname === '/videos/playlist') {
          window.playlistRequests.push(JSON.parse(init.body));
          return Response.json({count:JSON.parse(init.body).items.length});
        }
        if (url.pathname.endsWith('/streams')) {
          window.streamRequests.push(url.pathname + url.search);
          if (window.holdInitialStreams) {
            window.holdInitialStreams = false;
            await new Promise(resolve => window.finishInitialStreams = resolve);
          }
          if (window.failStreams) return Response.json({error_en:'Missing media'}, {status:404});
          return Response.json({location_id:Number(url.searchParams.get('location_id')),
            preferred_kind:'direct', sources:[{kind:'direct',src:window.mediaURL,mime_type:'audio/wav'}]});
        }
        if (url.pathname.includes('/playback-sessions')) {
          window.sessionRequests.push({url:url.pathname, method:init.method, body:JSON.parse(init.body)});
          return Response.json({session_id:'session-' + window.sessionRequests.length});
        }
        if (url.pathname.endsWith('/screenshots') && init.method === 'POST') {
          window.screenshotRequests.push(url.pathname + url.search);
          if (window.delayScreenshot) return new Promise(resolve => window.finishScreenshot = () => resolve(Response.json({})));
          return Response.json({});
        }
        return originalFetch(input, init);
      };
      window.testStore.setState(state => ({config:{...state.config, default_player:'browser', mpv_enabled:'false', runtime_remote_request:'true', runtime_container:'true'}}));
      window.testStore.getState().loadVideos({force:true});
    }`)
    await waitFor(`document.querySelectorAll('.video-card').length === 3`)
    const playMenu = async (text) => {
      await evaluate(`document.querySelector('button[aria-label="Video bulk actions"]').click()`)
      const action = `[...document.querySelectorAll('.MuiMenuItem-root')].find(el => el.textContent === ${JSON.stringify(text)})`
      await waitFor(action)
      assert.notEqual(await evaluate(`${action}.getAttribute('aria-disabled')`), 'true')
      await evaluate(`${action}.click()`)
    }
    const playlist = `document.querySelector('#browser-playlist')`
    const player = `document.querySelector('.video-js')?.player`
    const ready = `!document.querySelector('[data-player-loading]') && ${player}?.readyState() > 0`
    const activeTitle = `${playlist}?.querySelector('[aria-current="true"]')?.title`
    await playMenu('Play page')
    await waitFor(
      `window.finishInitialStreams && ${player} && document.querySelector('[data-player-loading]')`
    )
    assert.equal(await evaluate(`${player}.controls()`), false)
    assert.equal(
      await evaluate(`getComputedStyle(document.querySelector('.vjs-big-play-button')).display`),
      'none'
    )
    assert.equal(
      await evaluate(`(() => {
      const loading = document.querySelector('[data-player-loading]');
      const rect = loading.getBoundingClientRect();
      return document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2) === loading;
    })()`),
      true
    )
    await evaluate('window.finishInitialStreams()')
    await waitFor(`${activeTitle} === 'first.mp4' && ${ready}`)
    assert.deepEqual(await evaluate('window.playlistRequests'), [])
    assert.equal(await evaluate(`${playlist}.querySelectorAll('li').length`), 3)
    assert.equal(
      await evaluate(`document.querySelector('[aria-label="Previous video"]').disabled`),
      true
    )
    assert.equal(
      await evaluate(
        `${playlist}.getBoundingClientRect().left > document.querySelector('.player-shell').getBoundingClientRect().left`
      ),
      true
    )
    await evaluate(`window.previousPlayer = ${player}; window.previousContainer = document.querySelector('[data-vjs-player]');
      ${player}.volume(0.6); ${player}.muted(true); ${player}.playbackRate(1.5)`)
    const fullscreen = await command('Runtime.evaluate', {
      expression: "document.querySelector('.video-js').requestFullscreen()",
      userGesture: true,
      awaitPromise: true,
      returnByValue: true,
    })
    assert.ok(!fullscreen.result?.exceptionDetails, JSON.stringify(fullscreen))
    await waitFor('document.fullscreenElement')
    const assertPlayerState = async () => {
      assert.deepEqual(
        await evaluate(`({
        samePlayer: window.previousPlayer === ${player},
        sameContainer: window.previousContainer === document.querySelector('[data-vjs-player]'),
        disposed: window.previousPlayer.isDisposed(),
        fullscreen: document.fullscreenElement === ${player}.el(),
        muted: ${player}.muted(), volume: ${player}.volume(), rate: ${player}.playbackRate(),
      })`),
        {
          samePlayer: true,
          sameContainer: true,
          disposed: false,
          fullscreen: true,
          muted: true,
          volume: 0.6,
          rate: 1.5,
        }
      )
    }
    // A screenshot response from the previous item must not show a success notice here.
    await evaluate(
      `window.delayScreenshot = true; window.dispatchEvent(new KeyboardEvent('keydown', {key:'e'}))`
    )
    await waitFor('window.finishScreenshot')
    await evaluate(`${playlist}.querySelectorAll('button')[1].click()`)
    await waitFor(`${activeTitle} === 'second-copy.mp4' && ${ready}`)
    await assertPlayerState()
    await evaluate(`window.finishScreenshot(); window.delayScreenshot = false`)
    assert.equal(
      await evaluate(
        `document.querySelector('.player-shell').textContent.includes('Screenshot saved')`
      ),
      false
    )
    assert.deepEqual(await evaluate('window.screenshotRequests'), [
      '/videos/1/screenshots?location_id=11',
    ])
    assert.equal(await evaluate('window.streamRequests.at(-1)'), '/videos/1/streams?location_id=12')
    await evaluate(`${player}.currentTime(${player}.duration() - 0.05); ${player}.play(); void 0`)
    await waitFor(`${activeTitle} === 'third.mp4' && ${ready}`)
    await assertPlayerState()
    await evaluate('document.exitFullscreen()')
    await evaluate(`window.dispatchEvent(new KeyboardEvent('keydown', {key:'e'}))`)
    await waitFor('window.screenshotRequests.length === 2')
    assert.equal(
      await evaluate('window.screenshotRequests.at(-1)'),
      '/videos/2/screenshots?location_id=13'
    )
    assert.deepEqual(
      await evaluate(
        `window.sessionRequests.filter(request=>request.method === 'POST').map(request=>[request.url,request.body.location_id])`
      ),
      [
        ['/videos/1/playback-sessions', 11],
        ['/videos/1/playback-sessions', 12],
        ['/videos/2/playback-sessions', 13],
      ]
    )
    assert.ok(
      await evaluate(
        `window.sessionRequests.some(request=>request.method === 'PUT' && request.url.startsWith('/videos/1/playback-sessions/') && request.body.watched_ms > 0)`
      )
    )
    assert.equal(
      await evaluate(`document.querySelector('[aria-label="Next video"]').disabled`),
      true
    )
    await evaluate(`${player}.trigger('ended')`)
    assert.equal(await evaluate(activeTitle), 'third.mp4')
    await evaluate(`document.querySelector('[aria-label="Previous video"]').click()`)
    await waitFor(`${activeTitle} === 'second-copy.mp4' && ${ready}`)
    await evaluate(`{
      const previousFetch = window.fetch;
      window.holdOldStreams = true;
      window.fetch = (input, init) => {
        if (window.holdOldStreams && String(input).includes('/videos/1/streams?location_id=11')) {
          window.holdOldStreams = false;
          return new Promise(resolve => window.finishOldStreams = () => previousFetch(input, init).then(resolve));
        }
        return previousFetch(input, init);
      };
      ${playlist}.querySelectorAll('button')[0].click();
    }`)
    await waitFor('window.finishOldStreams && document.querySelector("[data-player-loading]")')
    assert.equal(await evaluate(`${player}.paused()`), true)
    assert.equal(await evaluate(`${player} === window.previousPlayer`), true)
    const screenshotCount = await evaluate('window.screenshotRequests.length')
    await evaluate(
      `window.dispatchEvent(new KeyboardEvent('keydown', {key:'e'})); window.dispatchEvent(new KeyboardEvent('keydown', {key:' '}))`
    )
    assert.equal(await evaluate('window.screenshotRequests.length'), screenshotCount)
    assert.equal(await evaluate(`${player}.paused()`), true)
    await evaluate(`${playlist}.querySelectorAll('button')[1].click()`)
    await waitFor(`${activeTitle} === 'second-copy.mp4' && ${ready}`)
    const sessionCount = await evaluate(
      "window.sessionRequests.filter(request=>request.method === 'POST').length"
    )
    await evaluate('window.finishOldStreams()')
    await waitFor(`${activeTitle} === 'second-copy.mp4' && ${ready}`)
    assert.equal(
      await evaluate("window.sessionRequests.filter(request=>request.method === 'POST').length"),
      sessionCount
    )
    await evaluate(`document.querySelector('button[aria-label="Playlist"]').click()`)
    await waitFor(`!${playlist}`)
    await evaluate(`document.querySelector('button[aria-label="Playlist"]').click()`)
    await waitFor(`${activeTitle} === 'second-copy.mp4'`)
    await command('Emulation.setDeviceMetricsOverride', {
      width: 390,
      height: 844,
      deviceScaleFactor: 1,
      mobile: false,
    })
    await waitFor(`${playlist}.getBoundingClientRect().right <= innerWidth`)
    const close = async () => {
      await evaluate(`document.querySelector('[role="dialog"] button[aria-label="Close"]').click()`)
      await waitFor(`!${player} && !${playlist}`)
      assert.equal(await evaluate('window.previousPlayer.isDisposed()'), true)
    }
    await close()
    await command('Emulation.clearDeviceMetricsOverride')

    // A selection from another page only has saved metadata, including its copy ID.
    await evaluate(`{
      window.testStore.setState({selectedVideoIds:new Set(['loc:99','loc:12']), selectedVideoMeta:{
        'loc:99':{video_id:9, location_id:99, label:'off-page.mp4'},
        'loc:12':{video_id:1, location_id:12, label:'second-copy.mp4'}
      }});
    }`)
    await waitFor(`document.querySelector('button.topbar-selection-action')`)
    await evaluate(`document.querySelector('button.topbar-selection-action').click()`)
    await waitFor(`document.querySelector('[aria-label="Selected Files"]')`)
    await evaluate(
      `[...document.querySelectorAll('[aria-label="Selected Files"] button')].find(el => el.textContent === 'Play all').click()`
    )
    await waitFor(`${activeTitle} === 'off-page.mp4' && ${ready}`)
    assert.equal(await evaluate('window.streamRequests.at(-1)'), '/videos/9/streams?location_id=99')
    await close()

    // Loading errors retain the list so the user can move to another entry.
    await evaluate('window.failStreams = true')
    await playMenu('Play all')
    await waitFor(
      `${playlist} && document.querySelector('[role="alert"]')?.textContent === 'Missing media'`
    )
    await evaluate(
      `window.failStreams = false; document.querySelector('[aria-label="Next video"]').click()`
    )
    await waitFor(`${activeTitle} === 'second-copy.mp4' && ${ready}`)
    await close()

    for (const defaultPlayer of ['mpv', 'system']) {
      await evaluate(
        `window.testStore.setState(state => ({config:{...state.config, default_player:'${defaultPlayer}', mpv_enabled:'true', desktop_integration_enabled:'true', runtime_remote_request:'false', runtime_container:'false'}}))`
      )
      await playMenu('Play page')
      await waitFor(`window.playlistRequests.at(-1)?.player === '${defaultPlayer}'`)
      assert.deepEqual(
        await evaluate('window.playlistRequests.at(-1).items.map(item => item.location_id)'),
        [11, 12, 13]
      )
      assert.equal(await evaluate(`Boolean(${playlist})`), false)
    }
    await evaluate(`{
      window.testStore.setState(state => ({config:{...state.config, default_player:'browser', mpv_enabled:'false'}}));
      const previousFetch = window.fetch;
      window.fetch = async (input, init = {}) => {
        const url = new URL(input, location.origin);
        if (url.pathname === '/jav') return Response.json({items:[
          {id:1,code:'ABC-001',title:'Multipart',videos:window.videoRows,tags:[],idols:[]},
          {id:2,code:'ABC-002',title:'No videos',videos:[],tags:[],idols:[]}
        ],total:2});
        return previousFetch(input, init);
      };
      document.querySelector('aside button[aria-label="JAV"]').click();
    }`)
    await waitFor(`document.querySelectorAll('.jav-card').length === 2`)
    await evaluate(`document.querySelector('button[aria-label="JAV bulk actions"]').click()`)
    const javPlayAll = `[...document.querySelectorAll('.MuiMenuItem-root')].find(el => el.textContent === 'Play all')`
    await waitFor(javPlayAll)
    await evaluate(`${javPlayAll}.click()`)
    await waitFor(`${activeTitle} === 'first.mp4' && ${ready}`)
    assert.equal(await evaluate(`${playlist}.querySelectorAll('li').length`), 3)
    assert.equal(await evaluate('window.playlistRequests.length'), 2)
    await close()
    // Clicking a multipart JAV now starts the whole browser playlist too.
    await evaluate(`document.querySelector('.jav-card button[aria-label="Play"]').click()`)
    await waitFor(`${activeTitle} === 'first.mp4' && ${ready}`)
    assert.equal(await evaluate(`${playlist}.querySelectorAll('li').length`), 3)
    await close()
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
