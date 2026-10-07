import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'dragging the playlist divider does not take focus or handle keys and preserves playback hotkeys',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
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
      window.screenshotRequests = [];
      window.fetch = async (input, init = {}) => {
        const url = new URL(input, location.origin);
        if (url.pathname.endsWith('/streams')) return Response.json({location_id:11,
          preferred_kind:'direct', sources:[{kind:'direct',src:mediaURL,mime_type:'audio/wav'}]});
        if (url.pathname.endsWith('/screenshots')) window.screenshotRequests.push(JSON.parse(init.body));
        return Response.json({session_id:'test'});
      };
      document.querySelector('#root > button').click();
    }`)
    const player = `document.querySelector('.video-js').player`
    const divider = `document.querySelector('[data-playlist-resize]')`
    const width = () =>
      evaluate(`document.querySelector('#browser-playlist').getBoundingClientRect().width`)
    const press = async (key) => {
      await command('Input.dispatchKeyEvent', { type: 'keyDown', key })
      await command('Input.dispatchKeyEvent', { type: 'keyUp', key })
    }
    await waitFor(`${player}?.readyState() > 0 && !document.querySelector('[data-player-loading]')`)
    await evaluate(`${player}.pause(); ${player}.volume(0.5); ${player}.currentTime(10)`)
    await waitFor(`${player}.currentTime() === 10`)
    const initialWidth = await width()
    const start = await evaluate(`(() => {
      const rect = ${divider}.getBoundingClientRect();
      return {x: rect.x + rect.width / 2, y: rect.y + rect.height / 2};
    })()`)
    await command('Input.dispatchMouseEvent', { type: 'mouseMoved', ...start })
    await command('Input.dispatchMouseEvent', {
      type: 'mousePressed',
      ...start,
      button: 'left',
      clickCount: 1,
    })
    await command('Input.dispatchMouseEvent', {
      type: 'mouseMoved',
      x: start.x - 32,
      y: start.y,
      buttons: 1,
    })
    await command('Input.dispatchMouseEvent', {
      type: 'mouseReleased',
      x: start.x - 32,
      y: start.y,
      button: 'left',
      clickCount: 1,
    })
    assert.equal(await width(), initialWidth + 32)
    assert.equal(await evaluate(`document.activeElement === ${divider}`), false)

    // Player hotkeys still work without manually refocusing after dragging.
    await press('q')
    assert.equal(await evaluate(`${player}.volume()`), 0.45)
    await press('w')
    assert.equal(await evaluate(`${player}.volume()`), 0.5)
    await press('a')
    await waitFor(`${player}.currentTime() === 9`)
    await press('e')
    await waitFor('window.screenshotRequests.length === 1')
    assert.equal(await evaluate('window.screenshotRequests[0].second'), 9)
    await press(' ')
    await waitFor(`!${player}.paused()`)
    await press(' ')
    await waitFor(`${player}.paused()`)
    assert.equal(await evaluate(`document.activeElement === ${divider}`), false)

    const resizedWidth = await width()
    // The divider must neither resize nor consume any of its former keyboard controls.
    for (const key of ['ArrowLeft', 'ArrowRight', 'Home', 'End']) {
      assert.equal(
        await evaluate(`${divider}.dispatchEvent(new KeyboardEvent('keydown', {
        key: ${JSON.stringify(key)}, bubbles: true, cancelable: true
      }))`),
        true
      )
      assert.equal(await width(), resizedWidth)
    }
    await press('Escape')
    await waitFor(`!document.querySelector('.player-window')`)
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
