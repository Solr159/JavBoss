import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'seek release and stationary hover display the actual target, including rapid pointer moves',
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
      window.fetch = async (input) => String(input).includes('/streams')
        ? Response.json({preferred_kind:'direct',sources:[{kind:'direct',src:mediaURL,mime_type:'audio/wav'}]})
        : Response.json({session_id:'test'});
      document.querySelector('#root > button').click();
    }`)
    await waitFor(`document.querySelector('.video-js')?.player?.readyState() > 0`)
    await evaluate(`window.player = document.querySelector('.video-js').player;
      window.seekBar = player.getChild('controlBar').getChild('progressControl').getChild('seekBar');
      player.pause(); player.currentTime(12); void 0`)
    const displayed = `document.querySelector('.vjs-current-time-display').textContent`
    const paint = () =>
      evaluate(
        `new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))`
      )
    await waitFor(`${displayed} === '0:12' && !player.seeking()`)
    const target = await evaluate(`(() => {
      const rect = document.querySelector('.vjs-progress-holder').getBoundingClientRect();
      return {x:rect.left + rect.width / 2,y:rect.top + rect.height / 2};
    })()`)
    for (const type of ['mousePressed', 'mouseReleased']) {
      await command('Input.dispatchMouseEvent', { type, ...target, button: 'left', clickCount: 1 })
    }
    await waitFor(`Math.abs(player.currentTime() - 30) < 1 && !player.seeking()`)
    // A release emits a final timeupdate from SeekBar after clearing its pending
    // target. Re-emit it after native events settle to cover slow/buffering media
    // deterministically, without relying on a later timeupdate to repair the label.
    await evaluate(`window.emitSeekTime = () => player.trigger({type:'timeupdate', target:seekBar, manuallyTriggered:true});
      seekBar.pendingSeekTime(null); emitSeekTime();`)
    await paint()
    assert.match(await evaluate(displayed), /^0:(29|30)$/)
    assert.equal(await evaluate('player.paused()'), true)

    // Mobile/remote scrubbing still previews a real target, including zero.
    for (const [pending, expected] of [
      [24, '0:24'],
      [0, '0:00'],
    ]) {
      await evaluate(`seekBar.pendingSeekTime(${pending}); emitSeekTime()`)
      await paint()
      assert.equal(await evaluate(displayed), expected)
    }
    await evaluate(`seekBar.pendingSeekTime(null); emitSeekTime()`)
    await paint()
    assert.match(await evaluate(displayed), /^0:(29|30)$/)

    // Seeking to the actual start remains valid; the fix must not freeze a label.
    await evaluate(`player.currentTime(0); void 0`)
    await waitFor(`!player.seeking() && ${displayed} === '0:00'`)
    await evaluate(`player.play(); void 0`)
    await waitFor(`player.currentTime() >= 1 && ${displayed} !== '0:00'`)
    // A stationary hover followed by a click must keep the same target, even
    // when the pointer falls on a half pixel and the mouse indicator covers it.
    await evaluate(`player.pause(); document.querySelector('.player-shell').style.width = '701px'`)
    for (const fullscreen of [false, true]) {
      if (fullscreen) {
        const button = await evaluate(`(() => {
          const rect = document.querySelector('.vjs-fullscreen-control').getBoundingClientRect();
          return {x:rect.left + rect.width / 2, y:rect.top + rect.height / 2};
        })()`)
        for (const type of ['mousePressed', 'mouseReleased']) {
          await command('Input.dispatchMouseEvent', {
            type,
            ...button,
            button: 'left',
            clickCount: 1,
          })
        }
        await waitFor(`player.isFullscreen()`)
      }
      for (const fraction of [0.3, 0.5, 0.75]) {
        const point = await evaluate(`(() => {
          const rect = seekBar.el().getBoundingClientRect();
          return {x:rect.left + rect.width * ${fraction}, y:rect.top + rect.height / 2};
        })()`)
        await command('Input.dispatchMouseEvent', { type: 'mouseMoved', ...point })
        await evaluate(`new Promise(resolve => setTimeout(resolve, 150))`)
        // Move twice in one task and once on the next frame, then stop.
        // A leading-only throttle used to retain the first position forever.
        // Alternate direction to cover both an early and a late hover label.
        await evaluate(`(async () => {
          for (const x of [${point.x + (fraction === 0.5 ? -18 : 18)}, ${point.x + (fraction === 0.5 ? -9 : 9)}, ${point.x}]) {
            if (x === ${point.x}) await new Promise(requestAnimationFrame);
            document.elementFromPoint(x, ${point.y}).dispatchEvent(new MouseEvent('mousemove', {
              bubbles:true, clientX:x, clientY:${point.y}
            }));
          }
        })()`)
        await evaluate(`new Promise(resolve => setTimeout(resolve, 150))`)
        const hover = await evaluate(`(() => {
          const mouse = seekBar.getChild('mouseTimeDisplay').el();
          return {
            time:parseFloat(mouse.style.left) / seekBar.el().offsetWidth * player.duration(),
            label:mouse.querySelector('.vjs-time-tooltip').textContent,
            second:seekBar.getChild('mouseTimeDisplay').bucket
          };
        })()`)
        for (const type of ['mousePressed', 'mouseReleased']) {
          await command('Input.dispatchMouseEvent', {
            type,
            ...point,
            button: 'left',
            clickCount: 1,
          })
        }
        await waitFor(`!player.seeking()`)
        await paint()
        const actual = await evaluate(`player.currentTime()`)
        assert.ok(
          Math.abs(actual - hover.time) < 0.001,
          `fullscreen=${fullscreen}, position=${fraction}: hover=${hover.time}, seek=${actual}`
        )
        assert.equal(await evaluate(displayed), hover.label)
        assert.equal(hover.second, Math.floor(actual))
        assert.equal(await evaluate(`player.paused()`), true)
      }
    }
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
