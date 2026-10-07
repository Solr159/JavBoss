import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'missing FFmpeg displays download guidance and stops the spinner for HLS and direct fallback',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-ffmpeg-test',
    })
    const player = `document.querySelector('.video-js')?.player`
    for (const direct of [false, true]) {
      await command('Page.navigate', { url: `${origin}/tests/fixtures/playerWindow.html` })
      await waitFor(`document.querySelector('#root > button')`)
      await evaluate(`{
        window.toolChecks = 0;
        window.fetch = async (input) => {
          const url = new URL(input, location.origin);
          if (url.pathname === '/tools') {
            window.toolChecks++;
            return Response.json({ffmpeg: {installed: false}});
          }
          if (url.pathname.endsWith('/streams')) return Response.json({
            preferred_kind: ${direct ? "'direct'" : "'hls'"},
            sources: [
              ...(${direct} ? [{kind:'direct', src:'/test.mp4', mime_type:'video/mp4'}] : []),
              {kind:'hls', src:'/test.m3u8', mime_type:'application/vnd.apple.mpegurl'}
            ]
          });
          return Response.json({session_id:'test'});
        };
        document.querySelector('#root > button').click();
      }`)
      await waitFor(`document.querySelector('[role="alert"]')?.textContent.includes('FFmpeg')`)
      const message = await evaluate(`document.querySelector('[role="alert"]').textContent`)
      assert.match(message, /Download FFmpeg in Settings → Tools/)
      assert.equal(await evaluate(`document.querySelectorAll('[role="alert"]').length`), 1)
      assert.equal(await evaluate(`document.querySelector('.vjs-error-display')`), null)
      assert.equal(
        await evaluate(`(() => {
          const error = document.querySelector('[role="alert"]').getBoundingClientRect();
          const video = document.querySelector('.video-js').getBoundingClientRect();
          return Math.abs(error.x + error.width / 2 - video.x - video.width / 2) < 1
            && Math.abs(error.y + error.height / 2 - video.y - video.height / 2) < 1;
        })()`),
        true
      )
      assert.equal(await evaluate('window.toolChecks'), 1)
      assert.equal(await evaluate(`${player}.currentSrc().includes('.m3u8')`), false)
      assert.equal(await evaluate(`${player}.paused()`), true)
      assert.equal(
        await evaluate(`getComputedStyle(document.querySelector('.vjs-loading-spinner')).display`),
        'none'
      )
      assert.deepEqual(await evaluate('window.appErrors'), [])
      await evaluate(`document.querySelector('button[aria-label="Close"]').click()`)
      await waitFor(`!document.querySelector('.player-window')`)
      assert.deepEqual(await evaluate('window.appErrors'), [])
    }
  }
)
