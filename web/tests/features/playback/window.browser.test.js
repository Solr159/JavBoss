import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'player window moves, resizes and restores its geometry across reopen and reload',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    const resizeViewport = async (width, height) => {
      await command('Emulation.setDeviceMetricsOverride', {
        width,
        height,
        deviceScaleFactor: 1,
        mobile: false,
      })
    }
    await resizeViewport(1280, 900)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/playerWindow.html` })
    const open = async () => {
      await waitFor(`document.querySelector('#root > button')`)
      await evaluate(`document.querySelector('#root > button').click()`)
      await waitFor(`document.querySelector('.video-js')?.player`)
      await evaluate(
        `new Promise(resolve => document.querySelector('.video-js').player.ready(resolve))`
      )
    }
    const box = () =>
      evaluate(`(() => {
      const {x,y,width,height} = document.querySelector('.player-window').getBoundingClientRect();
      return {x,y,width,height};
    })()`)
    const drag = async (selector, dx, dy) => {
      const start = await evaluate(`(() => {
        const {x,y,width,height} = document.querySelector(${JSON.stringify(selector)}).getBoundingClientRect();
        return {x:x+width/2,y:y+height/2};
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
        x: Math.max(0, start.x + dx),
        y: Math.max(0, start.y + dy),
        buttons: 1,
      })
      await command('Input.dispatchMouseEvent', {
        type: 'mouseReleased',
        x: Math.max(0, start.x + dx),
        y: Math.max(0, start.y + dy),
        button: 'left',
        clickCount: 1,
      })
    }
    await open()
    const initial = await box()
    const requests = await evaluate('window.streamRequests.length')
    await drag('[data-player-resize="se"]', 800 - initial.width, 480 - initial.height)
    assert.deepEqual(await box(), { ...initial, width: 800, height: 480 })
    await drag('.player-window h2', 180 - initial.x, 130 - initial.y)
    const saved = { x: 180, y: 130, width: 800, height: 480 }
    assert.deepEqual(await box(), saved)
    assert.equal(await evaluate('window.streamRequests.length'), requests)
    assert.deepEqual(
      await evaluate(`JSON.parse(localStorage.getItem('javboss.player.window'))`),
      saved
    )
    const heights = await evaluate(
      `['.player-shell','#browser-playlist'].map(selector=>document.querySelector(selector).getBoundingClientRect().height)`
    )
    assert.equal(heights[0], heights[1])
    assert.ok(heights[0] < saved.height)

    // The divider resizes only the playlist, keeping the player and source intact.
    const playlistWidth = () =>
      evaluate(`document.querySelector('#browser-playlist').getBoundingClientRect().width`)
    const beforeWidth = await playlistWidth()
    await evaluate(`window.playerBeforeResize = document.querySelector('.video-js').player; void 0`)
    await drag('[data-playlist-resize]', -100, 0)
    assert.equal(await playlistWidth(), beforeWidth + 100)
    await drag('[data-playlist-resize]', 40, 0)
    const savedPlaylistWidth = beforeWidth + 60
    assert.equal(await playlistWidth(), savedPlaylistWidth)
    assert.deepEqual(await box(), saved)
    assert.equal(await evaluate('window.streamRequests.length'), requests)
    assert.equal(
      await evaluate(`window.playerBeforeResize === document.querySelector('.video-js').player`),
      true
    )
    assert.equal(
      await evaluate(`Number(localStorage.getItem('javboss.player.playlistWidth'))`),
      savedPlaylistWidth
    )
    // Keyboard resizing must not be consumed by the player's seek shortcuts.
    await evaluate(`document.querySelector('[data-playlist-resize]').focus()`)
    await command('Input.dispatchKeyEvent', {
      type: 'keyDown',
      key: 'ArrowLeft',
      code: 'ArrowLeft',
      windowsVirtualKeyCode: 37,
    })
    await command('Input.dispatchKeyEvent', {
      type: 'keyUp',
      key: 'ArrowLeft',
      code: 'ArrowLeft',
      windowsVirtualKeyCode: 37,
    })
    await waitFor(
      `document.querySelector('#browser-playlist').getBoundingClientRect().width === ${savedPlaylistWidth + 16}`
    )
    await command('Input.dispatchKeyEvent', {
      type: 'keyDown',
      key: 'ArrowRight',
      code: 'ArrowRight',
      windowsVirtualKeyCode: 39,
    })
    await command('Input.dispatchKeyEvent', {
      type: 'keyUp',
      key: 'ArrowRight',
      code: 'ArrowRight',
      windowsVirtualKeyCode: 39,
    })
    await waitFor(
      `document.querySelector('#browser-playlist').getBoundingClientRect().width === ${savedPlaylistWidth}`
    )
    await evaluate(`document.querySelector('button[aria-label="Playlist"]').click()`)
    await waitFor(`!document.querySelector('[data-playlist-resize]')`)
    await evaluate(`document.querySelector('button[aria-label="Playlist"]').click()`)
    await waitFor(`document.querySelector('#browser-playlist')`)
    assert.equal(await playlistWidth(), savedPlaylistWidth)

    // Toolbar buttons must retain their click action without moving the window.
    await evaluate(`document.querySelector('button[aria-label="Next video"]').click()`)
    await waitFor(`document.querySelector('.player-window h2').textContent === 'Second video'`)
    assert.deepEqual(await box(), saved)
    await evaluate(`document.querySelector('button[aria-label="Close"]').click()`)
    await waitFor(`!document.querySelector('.player-window')`)
    await open()
    assert.deepEqual(await box(), saved)
    await command('Page.reload')
    await waitFor(`!document.querySelector('.player-window')`)
    await open()
    assert.deepEqual(await box(), saved)

    assert.equal(await playlistWidth(), savedPlaylistWidth)
    await resizeViewport(390, 320)
    await waitFor(`document.querySelector('.player-window').getBoundingClientRect().right <= 382`)
    const smaller = await box()
    assert.ok((await playlistWidth()) < savedPlaylistWidth)
    const mediaWidth = await evaluate(
      `document.querySelector('.player-shell').getBoundingClientRect().width`
    )
    assert.ok(mediaWidth > 100)
    assert.ok(
      await evaluate(
        `document.querySelector('#browser-playlist').getBoundingClientRect().right <= document.querySelector('.player-window').getBoundingClientRect().right`
      )
    )
    assert.ok(smaller.x >= 8 && smaller.y >= 8 && smaller.y + smaller.height <= 312)
    await resizeViewport(1280, 900)
    await waitFor(`document.querySelector('.player-window').getBoundingClientRect().width === 800`)
    assert.deepEqual(await box(), saved)

    assert.equal(await playlistWidth(), savedPlaylistWidth)
    // Dragging beyond either limit still leaves both panels usable.
    await drag('[data-playlist-resize]', -2000, 0)
    assert.equal(
      await playlistWidth(),
      await evaluate(
        `Number(document.querySelector('[data-playlist-resize]').getAttribute('aria-valuemax'))`
      )
    )
    await drag('[data-playlist-resize]', 700, 0)
    assert.equal(await playlistWidth(), 128)

    // Top-left resize moves the origin and preserves the opposite corner.
    await drag('[data-player-resize="nw"]', 40, 30)
    assert.deepEqual(await box(), { x: 220, y: 160, width: 760, height: 450 })
    await drag('.player-window h2', -900, -900)
    assert.deepEqual(await box(), { x: 8, y: 8, width: 760, height: 450 })
    assert.deepEqual(await evaluate('window.appErrors'), [])

    await evaluate(`localStorage.setItem('javboss.player.window', '{broken json')`)
    await command('Page.reload')
    await waitFor(`!document.querySelector('.player-window')`)
    await open()
    assert.deepEqual(await box(), initial)
  }
)
