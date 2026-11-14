import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'startup keeps one indicator until playing and exposes manual play only when autoplay is blocked',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-startup-test',
      chromeArgs: ['--autoplay-policy=no-user-gesture-required'],
    })
    const player = `document.querySelector('.video-js')?.player`
    const loading = `document.querySelector('[data-player-loading]')`
    for (const blocked of [false, true]) {
      await command('Page.navigate', { url: `${origin}/tests/fixtures/playerWindow.html` })
      await waitFor(`document.querySelector('#root > button')`)
      await evaluate(`{
        const wav = new Uint8Array(44 + 8000 * 60).fill(128);
        const header = new DataView(wav.buffer);
        const text = (at, value) => [...value].forEach((letter, i) => wav[at + i] = letter.charCodeAt(0));
        text(0, 'RIFF'); header.setUint32(4, wav.length - 8, true); text(8, 'WAVEfmt ');
        header.setUint32(16, 16, true); header.setUint16(20, 1, true); header.setUint16(22, 1, true);
        header.setUint32(24, 8000, true); header.setUint32(28, 8000, true);
        header.setUint16(32, 1, true); header.setUint16(34, 8, true);
        text(36, 'data'); header.setUint32(40, wav.length - 44, true);
        const mediaURL = URL.createObjectURL(new Blob([wav], {type:'audio/wav'}));
        const originalPlay = HTMLMediaElement.prototype.play;
        window.allowPlay = false;
        HTMLMediaElement.prototype.play = function() {
          if (window.allowPlay) return originalPlay.call(this);
          return new Promise((resolve, reject) => {
            window.finishPlay = () => {
              window.allowPlay = true;
              if (${blocked}) reject(new DOMException('User gesture required', 'NotAllowedError'));
              else resolve(originalPlay.call(this));
            };
          });
        };
        window.fetch = async (input) => {
          const url = new URL(input, location.origin);
          if (url.pathname.endsWith('/streams')) {
            await new Promise(resolve => window.finishInfo = resolve);
            return Response.json({preferred_kind:'direct', sources:[
              {kind:'direct', src:mediaURL, mime_type:'audio/wav'}
            ]});
          }
          return Response.json({session_id:'test'});
        };
        document.querySelector('#root > button').click();
      }`)
      await waitFor(`window.finishInfo && ${player} && ${loading}`)
      await evaluate(`window.startupIndicator = ${loading}; void 0`)
      const assertLoading = async () => {
        assert.equal(await evaluate(`${loading} === window.startupIndicator`), true)
        assert.equal(await evaluate(`${loading}?.textContent`), 'Loading video...')
        assert.equal(await evaluate(`document.querySelectorAll('[data-player-loading]').length`), 1)
        for (const selector of ['.vjs-big-play-button', '.vjs-loading-spinner']) {
          assert.equal(
            await evaluate(`getComputedStyle(document.querySelector('${selector}')).display`),
            'none'
          )
        }
      }
      await assertLoading()
      await evaluate('window.finishInfo()')
      await waitFor(`window.finishPlay && ${player}.readyState() > 0`)
      await assertLoading()
      await evaluate('window.finishPlay()')
      await waitFor(`!${loading}`)
      if (blocked) {
        assert.equal(await evaluate(`${player}.paused()`), true)
        assert.notEqual(
          await evaluate(
            `getComputedStyle(document.querySelector('.vjs-big-play-button')).display`
          ),
          'none'
        )
        await evaluate(`document.querySelector('.vjs-big-play-button').click()`)
      }
      await waitFor(`!${player}.paused() && ${player}.currentTime() > 0`)
      await evaluate(`${player}.pause()`)
      assert.equal(await evaluate(`Boolean(${loading})`), false)
      assert.deepEqual(await evaluate('window.appErrors'), [])
      await evaluate(`document.querySelector('button[aria-label="Close"]').click()`)
      await waitFor(`!document.querySelector('.player-window')`)
    }
  }
)
