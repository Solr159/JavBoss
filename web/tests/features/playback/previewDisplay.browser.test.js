import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'timeline previews are bucketed, cached, cancelled on leave and reset for the next video',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/playerWindow.html` })
    await waitFor(`document.querySelector('#root > button')`)
    await evaluate(`(async () => {
      const wav = new Uint8Array(44 + 8000 * 60).fill(128);
      const header = new DataView(wav.buffer);
      const text = (at, value) => [...value].forEach((letter, i) => wav[at + i] = letter.charCodeAt(0));
      text(0, 'RIFF'); header.setUint32(4, wav.length - 8, true); text(8, 'WAVEfmt ');
      header.setUint32(16, 16, true); header.setUint16(20, 1, true); header.setUint16(22, 1, true);
      header.setUint32(24, 8000, true); header.setUint32(28, 8000, true);
      header.setUint16(32, 1, true); header.setUint16(34, 8, true);
      text(36, 'data'); header.setUint32(40, wav.length - 44, true);
      const mediaURL = URL.createObjectURL(new Blob([wav], {type:'audio/wav'}));
      const canvas = document.createElement('canvas'); canvas.width = 320; canvas.height = 180;
      const ctx = canvas.getContext('2d'); ctx.fillStyle = '#abcdef'; ctx.fillRect(0, 0, 320, 180);
      const blob = await new Promise(resolve => canvas.toBlob(resolve, 'image/jpeg'));
      window.previewRequests = [];
      window.fetch = async (input, options = {}) => {
        const url = new URL(input, location.origin);
        if (url.pathname.endsWith('/streams')) return Response.json({location_id:11,preferred_kind:'direct',sources:[{kind:'direct',src:mediaURL,mime_type:'audio/wav'}]});
        if (url.pathname.endsWith('/preview')) {
          const request = {path:url.pathname, second:Number(url.searchParams.get('second')), locationId:url.searchParams.get('location_id'), aborted:false};
          window.previewRequests.push(request);
          options.signal.addEventListener('abort', () => { request.aborted = true; });
          if (window.holdPreview) await new Promise(resolve => { window.finishPreview = resolve; });
          if (window.failPreview) return Response.json({error_en:'Unavailable'}, {status:503});
          return new Response(blob, {headers:{'Content-Type':'image/jpeg'}});
        }
        return Response.json({session_id:'test'});
      };
      document.querySelector('#root > button').click();
    })()`)
    await waitFor(`document.querySelector('.video-js')?.player?.readyState() > 0`)
    await evaluate(
      `window.player = document.querySelector('.video-js').player; player.pause(); player.currentTime(12); void 0`
    )
    await waitFor(`!player.seeking()`)
    const image = `document.querySelector('.player-timeline-preview')`
    const hover = async (fraction) => {
      const point = await evaluate(`(() => {
        const rect = document.querySelector('.vjs-progress-holder').getBoundingClientRect();
        return {x: rect.left + rect.width * ${fraction}, y: rect.top + rect.height / 2};
      })()`)
      await command('Input.dispatchMouseEvent', { type: 'mouseMoved', ...point })
    }
    const leave = () => command('Input.dispatchMouseEvent', { type: 'mouseMoved', x: 0, y: 0 })
    const settle = () => evaluate(`new Promise(resolve => setTimeout(resolve, 400))`)
    await hover(0.4)
    await waitFor(`${image} && !${image}.hidden && ${image}.naturalWidth === 320`)
    assert.deepEqual(
      await evaluate(`(() => {
        const image = document.querySelector('.player-timeline-preview');
        const rect = image.getBoundingClientRect();
        return {width:rect.width, height:rect.height, visible:image.checkVisibility({checkOpacity:true, checkVisibilityCSS:true})};
      })()`),
      { width: 240, height: 135, visible: true }
    )
    assert.deepEqual(await evaluate(`window.previewRequests`), [
      { path: '/videos/1/preview', second: 20, locationId: '11', aborted: false },
    ])
    assert.equal(await evaluate(`player.currentTime()`), 12)
    assert.equal(await evaluate(`player.paused()`), true)
    await hover(0.45)
    await settle()
    assert.equal(await evaluate(`previewRequests.length`), 1)
    await leave()
    await waitFor(`${image}.hidden`)
    await hover(0.4)
    await waitFor(`!${image}.hidden`)
    assert.equal(await evaluate(`previewRequests.length`), 1)

    // No request for a fleeting hover, including Video.js's trailing throttle.
    await hover(0.65)
    await leave()
    await settle()
    assert.equal(await evaluate(`previewRequests.length`), 1)

    await evaluate(`window.holdPreview = true`)
    await hover(0.65)
    await waitFor(`window.finishPreview`)
    await leave()
    await waitFor(`previewRequests[1].aborted`)
    await evaluate(
      `window.holdPreview = false; window.finishPreview(); window.finishPreview = null`
    )
    await settle()
    assert.equal(await evaluate(`${image}.hidden`), true)

    // Failure keeps the native time label and leaves the video untouched.
    await evaluate(`window.failPreview = true`)
    await hover(0.8)
    await waitFor(`previewRequests.length === 3`)
    await settle()
    assert.equal(await evaluate(`${image}.hidden`), true)
    assert.match(
      await evaluate(`document.querySelector('.vjs-mouse-display .vjs-time-tooltip').textContent`),
      /0:4[78]/
    )
    assert.equal(await evaluate(`player.currentTime()`), 12)

    await evaluate(`window.failPreview = false;
      document.querySelector('button[title="Next video"]').click()`)
    await waitFor(
      `player.readyState() > 0 && document.querySelector('.player-header h2')?.textContent.includes('Second')`
    )
    await evaluate(`player.pause(); void 0`)
    await leave()
    await hover(0.4)
    await waitFor(
      `previewRequests.some(request => request.path === '/videos/2/preview') && !${image}.hidden`
    )

    // Images stay inside the seek bar at both edges, also in fullscreen.
    for (const fullscreen of [false, true]) {
      if (fullscreen) {
        const point = await evaluate(`(() => {
          const rect = document.querySelector('.vjs-fullscreen-control').getBoundingClientRect();
          return {x:rect.left + rect.width / 2, y:rect.top + rect.height / 2};
        })()`)
        for (const type of ['mousePressed', 'mouseReleased']) {
          await command('Input.dispatchMouseEvent', {
            type,
            ...point,
            button: 'left',
            clickCount: 1,
          })
        }
        await waitFor(`player.isFullscreen()`)
      }
      for (const fraction of [0.001, 0.999]) {
        await hover(fraction)
        await waitFor(`!${image}.hidden`)
        assert.equal(
          await evaluate(`(() => {
          const image = document.querySelector('.player-timeline-preview').getBoundingClientRect();
          const bar = document.querySelector('.vjs-progress-holder').getBoundingClientRect();
            return image.width === 240 && image.height === 135 && image.bottom < bar.top &&
              image.left >= bar.left - 1 && image.right <= bar.right + 1;
        })()`),
          true
        )
      }
    }
    await evaluate(`player.dispose()`)
    await settle()
    assert.deepEqual(await evaluate(`window.appErrors`), [])
  }
)
