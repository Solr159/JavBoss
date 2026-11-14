import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'the player frame and playlist follow the application theme, preserving dark playback controls',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-player-theme-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/playerWindow.html` })
    await waitFor(`document.querySelector('#root > button')`)
    await evaluate(`document.querySelector('#root > button').click()`)
    await waitFor(`document.querySelector('.video-js')?.player`)
    await evaluate(`{
      window.themePlayer = document.querySelector('.video-js').player;
      themePlayer.controls(true);
    }`)
    await evaluate(`import('/src/app/theme/preferences.js').then(({applyTheme}) => {
      window.applyPlayerTheme = applyTheme;
    })`)
    const requests = await evaluate('window.streamRequests.length')
    const colors = () =>
      evaluate(`['.player-window', '.player-header', '.player-playlist', '.vjs-control-bar',
        '.vjs-playback-rate .vjs-menu-content', '.video-js'].map(selector => {
          const style = getComputedStyle(document.querySelector(selector));
          return [style.color, style.backgroundColor];
        })`)
    for (const mode of ['light', 'dark', 'light']) {
      await evaluate(`applyPlayerTheme('${mode}')`)
      assert.deepEqual(
        await colors(),
        mode === 'light'
          ? [
              ['rgb(241, 245, 249)', 'rgb(255, 255, 255)'],
              ['rgb(39, 39, 42)', 'rgba(0, 0, 0, 0)'],
              ['rgb(39, 39, 42)', 'rgb(244, 244, 245)'],
              ['rgb(241, 245, 249)', 'rgb(8, 11, 17)'],
              ['rgb(241, 245, 249)', 'rgb(28, 36, 50)'],
              ['rgb(241, 245, 249)', 'rgb(8, 11, 17)'],
            ]
          : [
              ['rgb(241, 245, 249)', 'rgb(17, 21, 30)'],
              ['rgb(241, 245, 249)', 'rgba(0, 0, 0, 0)'],
              ['rgb(241, 245, 249)', 'rgb(23, 28, 39)'],
              ['rgb(241, 245, 249)', 'rgb(8, 11, 17)'],
              ['rgb(241, 245, 249)', 'rgb(28, 36, 50)'],
              ['rgb(241, 245, 249)', 'rgb(8, 11, 17)'],
            ]
      )
      assert.deepEqual(
        await evaluate(`['.player-header','.vjs-control-bar','.player-playlist-item'].map(
          selector=>document.querySelector(selector).getBoundingClientRect().height)`),
        [34, 50, 26]
      )
      assert.equal(
        await evaluate(`themePlayer === document.querySelector('.video-js').player`),
        true
      )
      assert.equal(await evaluate('window.streamRequests.length'), requests)
    }
    await command('Page.bringToFront')
    const fullscreen = await command('Runtime.evaluate', {
      expression: `document.querySelector('.video-js').requestFullscreen()`,
      userGesture: true,
      awaitPromise: true,
      returnByValue: true,
    })
    assert.ok(!fullscreen.error && !fullscreen.result?.exceptionDetails, JSON.stringify(fullscreen))
    await waitFor(`document.querySelector('.video-js.vjs-fullscreen')`)
    assert.equal((await colors())[3][0], 'rgb(241, 245, 249)')
    assert.deepEqual((await colors())[4], ['rgb(241, 245, 249)', 'rgb(28, 36, 50)'])
    await evaluate('document.exitFullscreen()')
    await waitFor(`!document.fullscreenElement`)
    assert.deepEqual((await colors())[3], ['rgb(241, 245, 249)', 'rgb(8, 11, 17)'])
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
